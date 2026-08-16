package battle

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"neonstack/gateway/internal/store"
)

const tickInterval = 50 * time.Millisecond

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Hub — 게임 무관 배틀 허브. Referee를 주입받는다.
type Hub struct {
	mu      sync.Mutex
	rooms   map[string]*Room
	referee Referee
	store   *store.Store
	bots    BotProvider
}

func New(ref Referee, st *store.Store, bots BotProvider) *Hub {
	return &Hub{rooms: map[string]*Room{}, referee: ref, store: st, bots: bots}
}

type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	playerID string
	room     *Room
}

type Room struct {
	hub     *Hub
	matchID string
	game    string
	mu      sync.Mutex
	clients map[string]*Client
	order   []string
	botID   string
	bot     Bot
	started bool
	over    bool
	stopped bool
	ticker  *time.Ticker
	stopCh  chan struct{}
}

type c2sMessage struct {
	Type   string `json:"type"`
	Action string `json:"action"`
}

// HandleWS upgrades the connection and registers the client in its room.
// token은 query 파라미터로 받는다 (브라우저 WS는 헤더를 못 붙임).
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	matchID := r.URL.Query().Get("match_id")
	playerID := r.URL.Query().Get("player_id")
	token := r.URL.Query().Get("token")
	if matchID == "" || playerID == "" || token == "" {
		http.Error(w, "match_id, player_id, token required", http.StatusBadRequest)
		return
	}
	u, err := h.store.UserByToken(r.Context(), token)
	if err != nil || u.ID != playerID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	m, err := h.store.MatchByID(r.Context(), matchID)
	if err != nil {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	if m.Status != "created" && m.Status != "playing" {
		http.Error(w, "match is not joinable", http.StatusConflict)
		return
	}
	names, err := h.store.PlayerNames(r.Context(), matchID)
	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}
	if _, ok := names[playerID]; !ok {
		http.Error(w, "unknown player", http.StatusForbidden)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}

	h.mu.Lock()
	room := h.rooms[matchID]
	if room == nil {
		room = &Room{
			hub:     h,
			matchID: matchID,
			game:    m.Game,
			clients: map[string]*Client{},
			order:   []string{playerID},
			stopCh:  make(chan struct{}),
		}
		// CPU 봇 상대(솔로)면 봇을 두 번째 플레이어로 등록 — 봇은 WS 없이 서버에서 직접 구동된다.
		if botID, _, ok, err := h.store.BotPlayer(context.Background(), matchID); err == nil && ok {
			room.botID = botID
			room.order = append(room.order, botID)
			if h.bots != nil {
				room.bot = h.bots(m.Game)
			}
		}
		h.rooms[matchID] = room
	}
	h.mu.Unlock()

	room.addClient(playerID, conn)
}

func (r *Room) addClient(playerID string, conn *websocket.Conn) {
	c := &Client{conn: conn, send: make(chan []byte, 32), playerID: playerID, room: r}
	r.mu.Lock()
	if old, ok := r.clients[playerID]; ok {
		old.conn.Close()
	}
	r.clients[playerID] = c
	if !contains(r.order, playerID) {
		r.order = append(r.order, playerID)
	}
	started := r.started
	// 봇이 있으면(솔로) 사람 한 명만 들어와도 시작, 없으면 2명 모두 필요.
	need := len(r.order) >= 2
	if !started && r.botID != "" {
		need = len(r.clients) >= 1
	}
	r.mu.Unlock()

	go c.writePump()
	go c.readPump()

	if !started && need {
		r.start()
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (r *Room) start() {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	order := append([]string(nil), r.order...)
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := r.hub.referee.Create(ctx, r.matchID, [2]string{order[0], order[1]}); err != nil {
		log.Printf("room %s: referee create: %v", r.matchID, err)
		r.broadcastError("레퍼리 연결 실패")
		r.closeAll()
		return
	}
	_ = r.hub.store.MarkPlaying(ctx, r.matchID)

	names, _ := r.hub.store.PlayerNames(ctx, r.matchID)
	for i, pid := range order {
		opponent := order[1-i]
		startMsg := map[string]any{
			"type":           "start",
			"match_id":       r.matchID,
			"game":           r.game,
			"you":            pid,
			"opponent":       opponent,
			"opponent_name":  names[opponent],
			"your_index":     i,
			"opponent_index": 1 - i,
		}
		r.sendTo(pid, mustJSON(startMsg))
	}

	r.mu.Lock()
	r.ticker = time.NewTicker(tickInterval)
	r.mu.Unlock()
	go r.tickLoop()
}

func (r *Room) tickLoop() {
	ticker := r.ticker
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.mu.Lock()
			if r.over {
				r.mu.Unlock()
				return
			}
			r.mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			up, err := r.hub.referee.Tick(ctx, r.matchID, int(tickInterval/time.Millisecond))
			cancel()
			if err != nil {
				log.Printf("room %s: tick: %v", r.matchID, err)
				continue
			}
			r.broadcastUpdate(up)
			r.driveBot(up)
		}
	}
}

// driveBot — 봇이 있으면 최신 상태를 보고 액션을 하나씩 보낸다 (틱당 최대 1개).
func (r *Room) driveBot(up *Update) {
	if r.bot == nil || r.botID == "" {
		return
	}
	states, err := SplitStates(up.States)
	if err != nil {
		return
	}
	bs, ok := states[r.botID]
	if !ok {
		return
	}
	if act := r.bot.Action(bs); act != nil {
		r.handleAction(r.botID, *act)
	}
}

func (r *Room) handleAction(playerID, action string) {
	r.mu.Lock()
	started, over := r.started, r.over
	r.mu.Unlock()
	if !started || over {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	up, err := r.hub.referee.Action(ctx, r.matchID, playerID, action)
	cancel()
	if err != nil {
		log.Printf("room %s: action %s: %v", r.matchID, action, err)
		return
	}
	r.broadcastUpdate(up)
}

func (r *Room) broadcastUpdate(up *Update) {
	r.mu.Lock()
	if r.over {
		r.mu.Unlock()
		return
	}
	if up.Over {
		r.over = true
	}
	states, err := SplitStates(up.States)
	if err != nil {
		log.Printf("room %s: split states: %v", r.matchID, err)
		r.mu.Unlock()
		return
	}
	order := append([]string(nil), r.order...)
	// 락을 잡은 채 sendTo를 호출하면 데드락 — 메시지를 먼저 만든 뒤 락 밖에서 전송한다
	out := make([]struct {
		pid string
		b   []byte
	}, len(order))
	for i, pid := range order {
		opponent := order[1-i]
		msg := map[string]any{
			"type":     "state",
			"you":      json.RawMessage(states[pid]),
			"opponent": json.RawMessage(states[opponent]),
			"events":   json.RawMessage(up.Events),
		}
		out[i] = struct {
			pid string
			b   []byte
		}{pid, mustJSON(msg)}
	}
	over := up.Over
	r.mu.Unlock()

	for _, m := range out {
		r.sendTo(m.pid, m.b)
	}
	if over {
		r.finish(up, states, order)
	}
}

func (r *Room) finish(up *Update, states map[string]json.RawMessage, order []string) {
	r.hub.mu.Lock()
	delete(r.hub.rooms, r.matchID)
	r.hub.mu.Unlock()
	r.stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var winnerID *string
	if up.Winner != nil {
		w := order[*up.Winner]
		winnerID = &w
	}
	_ = r.hub.store.FinishMatch(ctx, r.matchID, winnerID)
	for i, pid := range order {
		result := "draw"
		if up.Winner != nil {
			result = "loss"
			if int(*up.Winner) == i {
				result = "win"
			}
		}
		scores := scoreFromState(states[pid])
		_ = r.hub.store.FinishPlayer(ctx, r.matchID, pid, scores.Score, scores.Lines, result)
	}
	_ = r.hub.referee.Delete(ctx, r.matchID)

	for i, pid := range order {
		opponent := order[1-i]
		var yourResult, winner string
		if up.Winner == nil {
			yourResult, winner = "draw", ""
		} else if int(*up.Winner) == i {
			yourResult, winner = "win", pid
		} else {
			yourResult, winner = "loss", order[int(*up.Winner)]
		}
		msg := map[string]any{
			"type":           "gameover",
			"winner":         winner,
			"your_result":    yourResult,
			"your_score":     scoreFromState(states[pid]).Score,
			"opponent_score": scoreFromState(states[opponent]).Score,
		}
		r.sendTo(pid, mustJSON(msg))
	}
	time.AfterFunc(500*time.Millisecond, r.closeAll)
}

// scoreFromState — 게임별 상태에서 범용 점수 필드를 꺼낸다 (없으면 0).
// 특정 게임이 다른 결과 필드를 저장하려면 저장 로직을 게임별로 확장한다.
func scoreFromState(raw json.RawMessage) struct {
	Score int64
	Lines int64
} {
	var v struct {
		Score int64 `json:"score"`
		Lines int64 `json:"lines"`
	}
	_ = json.Unmarshal(raw, &v)
	return struct {
		Score int64
		Lines int64
	}{v.Score, v.Lines}
}

// clientGone is called when a client's read pump exits.
func (r *Room) clientGone(playerID string) {
	r.mu.Lock()
	delete(r.clients, playerID)
	remaining := len(r.clients)
	started, over, stopped := r.started, r.over, r.stopped
	r.mu.Unlock()

	if over || stopped {
		return
	}
	if started {
		// 기권 처리: 남은 플레이어 승
		var winner string
		r.mu.Lock()
		r.over = true
		for _, pid := range r.order {
			if pid != playerID {
				winner = pid
			}
		}
		order := append([]string(nil), r.order...)
		r.mu.Unlock()

		r.hub.mu.Lock()
		delete(r.hub.rooms, r.matchID)
		r.hub.mu.Unlock()
		r.stop()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = r.hub.store.FinishMatch(ctx, r.matchID, &winner)
		for _, pid := range order {
			result := "loss"
			if pid == winner {
				result = "win"
			}
			_ = r.hub.store.FinishPlayer(ctx, r.matchID, pid, 0, 0, result)
		}
		_ = r.hub.referee.Delete(ctx, r.matchID)
		r.sendTo(winner, mustJSON(map[string]any{
			"type": "gameover", "winner": winner, "your_result": "win",
			"your_score": 0, "opponent_score": 0, "forfeit": true,
		}))
		time.AfterFunc(500*time.Millisecond, r.closeAll)
		return
	}
	if remaining == 0 {
		// 시작 전 전원 이탈 → 매치 폐기
		r.hub.mu.Lock()
		delete(r.hub.rooms, r.matchID)
		r.hub.mu.Unlock()
		r.stop()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = r.hub.store.AbortMatch(ctx, r.matchID)
	}
}

func (r *Room) broadcastError(msg string) {
	b := mustJSON(map[string]any{"type": "error", "message": msg})
	r.mu.Lock()
	for _, c := range r.clients {
		select {
		case c.send <- b:
		default:
		}
	}
	r.mu.Unlock()
}

func (r *Room) sendTo(playerID string, b []byte) {
	r.mu.Lock()
	c, ok := r.clients[playerID]
	r.mu.Unlock()
	if !ok {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}

func (r *Room) closeAll() {
	r.mu.Lock()
	clients := make([]*Client, 0, len(r.clients))
	for _, c := range r.clients {
		clients = append(clients, c)
	}
	r.mu.Unlock()
	for _, c := range clients {
		c.conn.Close()
	}
}

func (r *Room) stop() {
	r.mu.Lock()
	if !r.stopped {
		r.stopped = true
		close(r.stopCh)
	}
	r.mu.Unlock()
}

func (c *Client) readPump() {
	defer c.room.clientGone(c.playerID)
	c.conn.SetReadLimit(4096)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg c2sMessage
		if json.Unmarshal(data, &msg) != nil || msg.Type != "action" {
			continue
		}
		c.room.handleAction(c.playerID, msg.Action)
	}
}

func (c *Client) writePump() {
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
