package referee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"neonstack/gateway/internal/battle"
)

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{
		base: base,
		http: &http.Client{Timeout: 3 * time.Second},
	}
}

type PieceState struct {
	T     string  `json:"t"`
	X     int     `json:"x"`
	Y     int     `json:"y"`
	Rot   uint8   `json:"rot"`
	// uint8는 []byte로 인식되어 JSON 인코더가 base64로 인코딩한다(→ 프론트 shape가 깨짐).
	// 숫자 배열로 직렬화되도록 int를 쓴다.
	Shape [][]int `json:"shape"`
}

type PlayerState struct {
	PlayerID   string      `json:"player_id"`
	Board      [][]*string `json:"board"`
	Items      [][]*string `json:"items"`
	Piece      PieceState  `json:"piece"`
	Score      int64       `json:"score"`
	Lines      int         `json:"lines"`
	Level      int         `json:"level"`
	Hold       *string     `json:"hold"`
	Garbage    uint32      `json:"garbage"`
	ClearFlash uint32      `json:"clear_flash"`
	Status     string      `json:"status"`
	Shield     uint32      `json:"shield"`
	Speed      bool        `json:"speed"`
	Slow       bool        `json:"slow"`
	// 다가올 다음 블록 (7-bag 남은 순서, 최대 3개) — NEXT 프리뷰용
	Next []string `json:"next"`
}

type Event struct {
	Kind   string  `json:"kind"`
	By     *string `json:"by,omitempty"`
	Clear  *string `json:"clear,omitempty"`
	Attack uint32  `json:"attack"`
	Winner *uint8  `json:"winner,omitempty"`
	Item   *string `json:"item,omitempty"`
	Target *string `json:"target,omitempty"`
	// 발동된 아이템 셀 좌표 [x, y] — 프론트 폭발 연출용
	Cell []int `json:"cell,omitempty"`
}

type Update struct {
	States []PlayerState `json:"states"`
	Events []Event       `json:"events"`
	Over   bool          `json:"over"`
	Winner *uint8        `json:"winner"`
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("referee %s: status %d", path, resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Create(ctx context.Context, matchID string, players [2]string, itemMode bool) error {
	return c.post(ctx, "/match", map[string]any{"match_id": matchID, "players": players, "item_mode": itemMode}, nil)
}

func (c *Client) Delete(ctx context.Context, matchID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/match/"+matchID, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// toBattle — 게임별 Update를 게임 무관 battle.Update(불투명 JSON)로 감싼다.
func toBattle(up *Update) *battle.Update {
	if up == nil {
		return nil
	}
	states, _ := json.Marshal(up.States)
	events, _ := json.Marshal(up.Events)
	return &battle.Update{States: states, Events: events, Over: up.Over, Winner: up.Winner}
}

// Action — battle.Referee 구현.
func (c *Client) Action(ctx context.Context, matchID, playerID, action string) (*battle.Update, error) {
	var up Update
	err := c.post(ctx, "/action", map[string]any{
		"match_id": matchID, "player_id": playerID, "action": action,
	}, &up)
	if err != nil {
		return nil, err
	}
	return toBattle(&up), nil
}

// Tick — battle.Referee 구현.
func (c *Client) Tick(ctx context.Context, matchID string, dtMs int) (*battle.Update, error) {
	var up Update
	err := c.post(ctx, "/tick", map[string]any{"match_id": matchID, "dt_ms": dtMs}, &up)
	if err != nil {
		return nil, err
	}
	return toBattle(&up), nil
}
