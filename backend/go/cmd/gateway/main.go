package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"neonstack/gateway/internal/hub"
	"neonstack/gateway/internal/referee"
	"neonstack/gateway/internal/store"
)

const joinCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func joinCode() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	out := make([]byte, 4)
	for i := range out {
		out[i] = joinCodeAlphabet[int(b[i])%len(joinCodeAlphabet)]
	}
	return string(out)
}

func port() string {
	if p := os.Getenv("PORT"); p != "" && p != "0" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n <= 65535 {
			return p
		}
	}
	return "8080"
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://neon:neon@localhost:5432/neon"
	}
	refereeURL := os.Getenv("REFEREE_URL")
	if refereeURL == "" {
		refereeURL = "http://localhost:8081"
	}

	// PostgreSQL 연결 재시도 (컨테이너 기동 대기)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var st *store.Store
	var err error
	for {
		st, err = store.Connect(ctx, dbURL)
		if err == nil {
			break
		}
		log.Printf("store connect 실패 (%v) — 2초 후 재시도", err)
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			log.Fatalf("store connect timeout: %v", ctx.Err())
		}
	}
	defer st.Close()
	log.Printf("postgres 연결 완료")

	ref := referee.New(refereeURL)
	h := hub.New(ref, st)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// 매치 생성
	mux.HandleFunc("POST /api/matches", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}
		matchID, code, playerID := randomHex(8), joinCode(), randomHex(8)
		if err := st.CreateMatch(r.Context(), matchID, code, playerID, strings.TrimSpace(body.Name)); err != nil {
			log.Printf("create match: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{
			"match_id": matchID, "code": code, "player_id": playerID, "player_name": strings.TrimSpace(body.Name),
		})
	})

	// 코드로 참가
	mux.HandleFunc("POST /api/matches/join", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Code string `json:"code"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		code := strings.ToUpper(strings.TrimSpace(body.Code))
		name := strings.TrimSpace(body.Name)
		if code == "" || name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code and name required"})
			return
		}
		playerID := randomHex(8)
		m, err := st.JoinMatch(r.Context(), code, playerID, name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"match_id": m.ID, "code": m.Code, "player_id": playerID, "player_name": name,
		})
	})

	// 매치 상태 조회
	mux.HandleFunc("GET /api/matches/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := st.MatchByID(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "match not found"})
			return
		}
		names, _ := st.PlayerNames(r.Context(), id)
		players := make([]map[string]any, 0, len(names))
		for pid, name := range names {
			players = append(players, map[string]any{"player_id": pid, "player_name": name})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"match_id": m.ID, "code": m.Code, "status": m.Status,
			"winner_id": m.WinnerID, "players": players,
		})
	})

	// 리더보드
	mux.HandleFunc("GET /api/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		rows, err := st.Leaderboard(r.Context(), 10)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
			return
		}
		writeJSON(w, http.StatusOK, rows)
	})

	// 게임 WebSocket
	mux.HandleFunc("GET /ws", h.HandleWS)

	addr := ":" + port()
	log.Printf("gateway listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
