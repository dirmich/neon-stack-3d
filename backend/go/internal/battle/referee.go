// Package battle — 게임에 무관한 2인 실시간 배틀 프레임워크.
// 방/룸 수명주기, 매치메이킹, WebSocket 라우팅, 승패 저장을 담당하며,
// 게임별 규칙은 Referee 인터페이스로 플러그인한다 (예: 테트리스 → internal/referee).
package battle

import (
	"context"
	"encoding/json"
)

// Referee — 게임별 권위 상태를 소유하는 백엔드(예: Rust 레퍼리).
// 다른 게임을 추가하려면 이 인터페이스를 구현하고 matches.game에 게임명을 쓰면 된다.
type Referee interface {
	// Create — 매치 시작 (두 플레이어 확정 후 호출).
	Create(ctx context.Context, matchID string, players [2]string) error
	// Action — 한 플레이어의 입력을 적용하고 최신 상태를 반환.
	Action(ctx context.Context, matchID, playerID, action string) (*Update, error)
	// Tick — 경과 시간만큼 시뮬레이션 진행 (중력 등).
	Tick(ctx context.Context, matchID string, dtMs int) (*Update, error)
	// Delete — 매치 메모리 정리.
	Delete(ctx context.Context, matchID string) error
}

// Update — 게임 무관 래퍼. States/Events는 게임별 페이로드(불투명 JSON)다.
// 각 State는 판정용으로 최소한 player_id 필드를 가져야 한다.
type Update struct {
	States json.RawMessage `json:"states"`
	Events json.RawMessage `json:"events"`
	Over   bool            `json:"over"`
	Winner *uint8          `json:"winner"`
}

// SplitStates — states 배열을 player_id별로 분리한다.
func SplitStates(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var states []json.RawMessage
	if err := json.Unmarshal(raw, &states); err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(states))
	for _, s := range states {
		var env struct {
			PlayerID string `json:"player_id"`
		}
		if err := json.Unmarshal(s, &env); err != nil {
			return nil, err
		}
		out[env.PlayerID] = s
	}
	return out, nil
}
