package referee

import (
	"encoding/json"
	"testing"
)

// 회귀 방지 (v1.3.2): Shape를 [][]uint8로 선언하면 Go JSON 인코더가 []byte로 취급해
// 각 행을 base64 문자열로 인코딩한다 — 브라우저 cellsFor()가 깨지는 원인이었다.
// shape가 숫자 배열로 직렬화되는지 검증한다.
func TestPlayerStateMarshalShapeIsNumberArray(t *testing.T) {
	st := PlayerState{
		PlayerID: "p1",
		Board: [][]*string{
			{nil, nil, strPtr("I"), nil, nil, nil, nil, nil, nil, nil},
			{nil, nil, nil, nil, nil, nil, nil, nil, nil, nil},
		},
		Piece: PieceState{
			T: "L", X: 3, Y: -1, Rot: 0,
			Shape: [][]int{{0, 0, 1}, {1, 1, 1}, {0, 0, 0}},
		},
		Score: 0, Lines: 0, Level: 1, Hold: nil, Garbage: 0, ClearFlash: 0, Status: "playing",
	}

	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// 1) shape는 [][]int로 다시 파싱돼야 한다 — base64 문자열("AAAB" 등)이면 실패.
	var v struct {
		Piece struct {
			Shape [][]int `json:"shape"`
		} `json:"piece"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("shape가 숫자 배열로 직렬화되지 않음 (base64 회귀): %v\nraw=%s", err, raw)
	}
	want := [][]int{{0, 0, 1}, {1, 1, 1}, {0, 0, 0}}
	if len(v.Piece.Shape) != len(want) {
		t.Fatalf("shape 행 수 = %d, want %d\nraw=%s", len(v.Piece.Shape), len(want), raw)
	}
	for i := range want {
		if len(v.Piece.Shape[i]) != len(want[i]) {
			t.Fatalf("shape[%d] 길이 = %d, want %d\nraw=%s", i, len(v.Piece.Shape[i]), len(want[i]), raw)
		}
		for j := range want[i] {
			if v.Piece.Shape[i][j] != want[i][j] {
				t.Fatalf("shape[%d][%d] = %d, want %d\nraw=%s", i, j, v.Piece.Shape[i][j], want[i][j], raw)
			}
		}
	}

	// 2) board는 null/문자열 2차원 배열이어야 한다.
	var b struct {
		Board [][]*string `json:"board"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("board 파싱 실패: %v\nraw=%s", err, raw)
	}
	if len(b.Board) != 2 || len(b.Board[0]) != 10 {
		t.Fatalf("board 크기 = %d x %d, want 2 x 10\nraw=%s", len(b.Board), len(b.Board[0]), raw)
	}
	if b.Board[0][2] == nil || *b.Board[0][2] != "I" {
		t.Fatalf("board[0][2] = %v, want \"I\"\nraw=%s", b.Board[0][2], raw)
	}
	if b.Board[0][0] != nil {
		t.Fatalf("board[0][0] = %v, want null\nraw=%s", *b.Board[0][0], raw)
	}
}

// toBattle 경로(실제 운영 경로) 회귀 방지: 레퍼리 응답을 디코드 → battle.Update로
// 재직렬화해도 각 상태의 shape가 숫자 배열로 유지돼야 한다.
// 회귀 방지 (v1.4.0): 게이트웨이가 레퍼리 응답을 자체 PlayerState로 디코드→재직렬화하는데,
// items/shield/speed/slow 필드가 없으면 아이템 배틀 상태가 통째로 드롭된다.
// 아이템 필드가 왕복 후에도 유지되는지 검증한다.
func TestUpdateMarshalStatesKeepsItemFields(t *testing.T) {
	// Rust 레퍼리가 아이템 모드로 보내는 형태의 원시 JSON.
	rustRaw := []byte(`{
		"states": [{
			"player_id": "p1",
			"board": [[null, null], [null, null]],
			"items": [["attack", null], [null, "shield"]],
			"piece": {"t": "L", "x": 3, "y": -1, "rot": 0,
				"shape": [[0, 0, 1], [1, 1, 1], [0, 0, 0]]},
			"score": 0, "lines": 0, "level": 1, "hold": null,
			"garbage": 0, "clear_flash": 0, "status": "playing",
			"shield": 2, "speed": true, "slow": false
		}],
		"events": [],
		"over": false,
		"winner": null
	}`)

	var up Update
	if err := json.Unmarshal(rustRaw, &up); err != nil {
		t.Fatalf("레퍼리 응답 디코드 실패: %v", err)
	}

	// toBattle와 동일하게 States를 재직렬화한다.
	states, err := json.Marshal(up.States)
	if err != nil {
		t.Fatalf("states 재직렬화 실패: %v", err)
	}

	var parsed []struct {
		Items [][]*string `json:"items"`
		Shield uint32     `json:"shield"`
		Speed  bool       `json:"speed"`
		Slow   bool       `json:"slow"`
	}
	if err := json.Unmarshal(states, &parsed); err != nil {
		t.Fatalf("재직렬화된 states 파싱 실패 (items 드롭 회귀): %v\nraw=%s", err, states)
	}
	if len(parsed) != 1 {
		t.Fatalf("states 수 = %d, want 1", len(parsed))
	}
	p := parsed[0]
	if len(p.Items) != 2 || p.Items[0][0] == nil || *p.Items[0][0] != "attack" || p.Items[0][1] != nil || p.Items[1][1] == nil || *p.Items[1][1] != "shield" {
		t.Fatalf("items 드롭 회귀: items = %v\nraw=%s", p.Items, states)
	}
	if p.Shield != 2 || !p.Speed || p.Slow {
		t.Fatalf("shield/speed/slow 드롭 회귀: shield=%d speed=%v slow=%v\nraw=%s", p.Shield, p.Speed, p.Slow, states)
	}
}

func TestUpdateMarshalStatesKeepsNumericShape(t *testing.T) {
	// Rust 레퍼리가 보내는 형태의 원시 JSON.
	rustRaw := []byte(`{
		"states": [{
			"player_id": "p1",
			"board": [[null, "I", null], [null, null, null]],
			"piece": {"t": "L", "x": 3, "y": -1, "rot": 0,
				"shape": [[0, 0, 1], [1, 1, 1], [0, 0, 0]]},
			"score": 0, "lines": 0, "level": 1, "hold": null,
			"garbage": 0, "clear_flash": 0, "status": "playing"
		}],
		"events": [],
		"over": false,
		"winner": null
	}`)

	var up Update
	if err := json.Unmarshal(rustRaw, &up); err != nil {
		t.Fatalf("레퍼리 응답 디코드 실패: %v", err)
	}

	// toBattle와 동일하게 States를 재직렬화한다.
	states, err := json.Marshal(up.States)
	if err != nil {
		t.Fatalf("states 재직렬화 실패: %v", err)
	}

	// 각 상태의 shape가 숫자 배열인지 검증 (base64 회귀 방지).
	var parsed []struct {
		Piece struct {
			Shape [][]int `json:"shape"`
		} `json:"piece"`
	}
	if err := json.Unmarshal(states, &parsed); err != nil {
		t.Fatalf("재직렬화된 states 파싱 실패 (base64 회귀): %v\nraw=%s", err, states)
	}
	if len(parsed) != 1 {
		t.Fatalf("states 수 = %d, want 1", len(parsed))
	}
	want := [][]int{{0, 0, 1}, {1, 1, 1}, {0, 0, 0}}
	got := parsed[0].Piece.Shape
	if len(got) != len(want) {
		t.Fatalf("shape 행 수 = %d, want %d\nraw=%s", len(got), len(want), states)
	}
	for i := range want {
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("shape[%d][%d] = %d, want %d\nraw=%s", i, j, got[i][j], want[i][j], states)
			}
		}
	}
}

