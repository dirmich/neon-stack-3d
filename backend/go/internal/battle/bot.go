package battle

import "encoding/json"

// Bot — 게임 무관 CPU 봇 드라이버.
// 게임별 상태(불투명 JSON)를 받아 다음 액션 문자열을 돌려준다. 액션이 없으면 nil.
// 다른 게임에 봇을 추가하려면 이 인터페이스를 구현해 BotProvider에 등록하면 된다.
type Bot interface {
	Action(state json.RawMessage) *string
}

// BotProvider — 게임 이름 → 새 Bot 인스턴스 (매치마다 새로 생성, 상태 저장 가능).
// 지원하지 않는 게임이면 nil을 돌려준다.
type BotProvider func(game string) Bot
