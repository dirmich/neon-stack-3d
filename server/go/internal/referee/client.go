package referee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
	T     string    `json:"t"`
	X     int       `json:"x"`
	Y     int       `json:"y"`
	Rot   uint8     `json:"rot"`
	Shape [][]uint8 `json:"shape"`
}

type PlayerState struct {
	PlayerID   string      `json:"player_id"`
	Board      [][]*string `json:"board"`
	Piece      PieceState  `json:"piece"`
	Score      int64       `json:"score"`
	Lines      int         `json:"lines"`
	Level      int         `json:"level"`
	Hold       *string     `json:"hold"`
	Garbage    uint32      `json:"garbage"`
	ClearFlash uint32      `json:"clear_flash"`
	Status     string      `json:"status"`
}

type Event struct {
	Kind   string  `json:"kind"`
	By     *string `json:"by,omitempty"`
	Clear  *string `json:"clear,omitempty"`
	Attack uint32  `json:"attack"`
	Winner *uint8  `json:"winner,omitempty"`
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

func (c *Client) Create(ctx context.Context, matchID string, players [2]string) error {
	return c.post(ctx, "/match", map[string]any{"match_id": matchID, "players": players}, nil)
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

func (c *Client) Action(ctx context.Context, matchID, playerID, action string) (*Update, error) {
	var up Update
	err := c.post(ctx, "/action", map[string]any{
		"match_id": matchID, "player_id": playerID, "action": action,
	}, &up)
	if err != nil {
		return nil, err
	}
	return &up, nil
}

func (c *Client) Tick(ctx context.Context, matchID string, dtMs int) (*Update, error) {
	var up Update
	err := c.post(ctx, "/tick", map[string]any{"match_id": matchID, "dt_ms": dtMs}, &up)
	if err != nil {
		return nil, err
	}
	return &up, nil
}
