package battle

import (
	"encoding/json"
	"testing"
)

// SplitStates가 player_id별로 상태를 나누는지 확인한다 (허브 broadcastUpdate 경로).
func TestSplitStatesByPlayerID(t *testing.T) {
	raw := []byte(`[
		{"player_id": "a", "score": 1},
		{"player_id": "b", "score": 2}
	]`)
	m, err := SplitStates(raw)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(m) != 2 || m["a"] == nil || m["b"] == nil {
		t.Fatalf("split 결과 = %v", m)
	}
	var sa, sb struct {
		Score int `json:"score"`
	}
	if err := json.Unmarshal(m["a"], &sa); err != nil || sa.Score != 1 {
		t.Fatalf("a.score = %d (err %v), want 1", sa.Score, err)
	}
	if err := json.Unmarshal(m["b"], &sb); err != nil || sb.Score != 2 {
		t.Fatalf("b.score = %d (err %v), want 2", sb.Score, err)
	}
}
