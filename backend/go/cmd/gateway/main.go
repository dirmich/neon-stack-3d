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

	"neonstack/gateway/internal/auth"
	"neonstack/gateway/internal/battle"
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
	return "8000"
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://neon:neon@localhost:5432/neon"
	}
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
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

	authSvc := auth.New(st, googleClientID)
	ref := referee.New(refereeURL)
	// 게임별 CPU 봇 등록 — 새 게임은 여기에 봇 드라이버를 추가한다.
	bots := battle.BotProvider(func(game string) battle.Bot {
		if game == "tetris" {
			return referee.NewTetrisBot()
		}
		return nil
	})
	h := battle.New(ref, st, bots)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// ---------- 인증 ----------
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		u, token, err := authSvc.Register(r.Context(), strings.TrimSpace(body.Name), body.Password)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"token": token, "user": map[string]string{"id": u.ID, "name": u.Name}})
	})

	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		u, token, err := authSvc.Login(r.Context(), strings.TrimSpace(body.Name), body.Password)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": map[string]string{"id": u.ID, "name": u.Name}})
	})

	mux.HandleFunc("POST /api/auth/logout", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		_ = authSvc.Logout(r.Context(), auth.BearerToken(r))
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("GET /api/auth/me", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		writeJSON(w, http.StatusOK, map[string]string{"id": u.ID, "name": u.Name})
	}))

	// ---------- Google SSO ----------
	// 클라이언트에 Google OAuth client id 제공 (설정 안 됐으면 enabled=false)
	mux.HandleFunc("GET /api/auth/google/config", func(w http.ResponseWriter, r *http.Request) {
		cid := authSvc.GoogleClientID()
		writeJSON(w, http.StatusOK, map[string]any{"client_id": cid, "enabled": cid != ""})
	})

	// Google Identity Services가 발급한 ID 토큰(credential)을 검증하고 로그인/가입
	mux.HandleFunc("POST /api/auth/google", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Credential string `json:"credential"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		u, token, err := authSvc.GoogleLogin(r.Context(), strings.TrimSpace(body.Credential))
		if err != nil {
			status := http.StatusUnauthorized
			if err == auth.ErrGoogleNotConfigured {
				status = http.StatusServiceUnavailable
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": map[string]string{"id": u.ID, "name": u.Name}})
	})

	// ---------- 방 리스트 ----------
	mux.HandleFunc("GET /api/rooms", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		game := r.URL.Query().Get("game")
		if game == "" {
			game = "tetris"
		}
		u := auth.UserFrom(r.Context())
		rooms, err := st.ListRooms(r.Context(), game, u.ID, 50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
			return
		}
		writeJSON(w, http.StatusOK, rooms)
	}))

	// ---------- 매치 생성 (인증) ----------
	mux.HandleFunc("POST /api/matches", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var body struct {
			Game string `json:"game"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		game := strings.TrimSpace(body.Game)
		if game == "" {
			game = "tetris"
		}
		matchID, code := randomHex(8), joinCode()
		if err := st.CreateMatch(r.Context(), matchID, code, game, u.ID, u.Name); err != nil {
			log.Printf("create match: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{
			"match_id": matchID, "code": code, "player_id": u.ID, "player_name": u.Name, "game": game,
		})
	}))

	// ---------- 코드로 참가 (인증) ----------
	mux.HandleFunc("POST /api/matches/join", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
			return
		}
		code := strings.ToUpper(strings.TrimSpace(body.Code))
		if code == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code required"})
			return
		}
		m, err := st.JoinMatch(r.Context(), code, u.ID, u.Name)
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"match_id": m.ID, "code": m.Code, "player_id": u.ID, "player_name": u.Name, "game": m.Game,
		})
	}))

	// ---------- CPU 봇 상대 (솔로 연습) ----------
	mux.HandleFunc("POST /api/matches/solo", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var body struct {
			Game string `json:"game"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		game := strings.TrimSpace(body.Game)
		if game == "" {
			game = "tetris"
		}
		matchID, code := randomHex(8), joinCode()
		if err := st.CreateSoloMatch(r.Context(), matchID, code, game, u.ID, u.Name); err != nil {
			log.Printf("create solo match: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{
			"match_id": matchID, "code": code, "player_id": u.ID, "player_name": u.Name,
			"game": game, "solo": "true",
		})
	}))

	// ---------- 매치 상태 조회 ----------
	mux.HandleFunc("GET /api/matches/{id}", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
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
			"match_id": m.ID, "code": m.Code, "game": m.Game, "status": m.Status,
			"winner_id": m.WinnerID, "players": players,
		})
	}))

	// ---------- 리더보드 ----------
	mux.HandleFunc("GET /api/leaderboard", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		limit := 10
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = min(n, 50)
			}
		}
		u := auth.UserFrom(r.Context())
		rows, my, err := st.Leaderboard(r.Context(), limit, u.Name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "my": my})
	}))

	// ---------- 게임 WebSocket (token은 query 파라미터) ----------
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
