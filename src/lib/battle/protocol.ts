/**
 * 게임 무관 배틀 프로토콜 — 모든 게임(테트리스 등)이 공유하는 메시지 봉투.
 * 상태(you/opponent)만 게임별 타입 S로 치환된다.
 */

/** 공격/클리어/아이템 이벤트 (게임별 kind, clear 값 사용) */
export interface BattleEvent {
  kind: string;
  by?: string;
  clear?: string;
  attack: number;
  winner?: number;
  /** 아이템 발동 이벤트 전용 (kind === 'item') */
  item?: string;
  target?: string;
}

/** 서버 → 클라이언트 메시지 봉투 (S = 게임별 플레이어 상태) */
export type ServerMessage<S> =
  | {
      type: 'start';
      match_id: string;
      you: string;
      opponent: string;
      opponent_name: string;
      your_index: number;
      opponent_index: number;
      /** 'normal' | 'item' — 아이템 배틀 모드 */
      mode?: string;
    }
  | { type: 'state'; you: S; opponent: S; events: BattleEvent[] }
  | {
      type: 'gameover';
      winner: string | null;
      your_result: 'win' | 'loss' | 'draw';
      your_score: number;
      opponent_score: number;
      forfeit?: boolean;
    }
  | { type: 'error'; message: string };

/** 배틀 뷰에 필요한 매치 정보 (로비 → 룸 전달) */
export interface MatchInfo {
  match_id: string;
  player_id: string;
  player_name: string;
  opponent_name: string;
  your_index: number;
}

/** 방 생성/참가 응답 */
export interface MatchCreateResponse {
  match_id: string;
  code: string;
  player_id: string;
  player_name: string;
  game: string;
  mode?: string;
  solo?: boolean;
}

/** 방 리스트 항목 */
export interface RoomRow {
  match_id: string;
  code: string;
  host_name: string;
  player_count: number;
  created_at: string;
  is_mine: boolean;
  /** 'normal' | 'item' */
  mode: string;
}

/** 리더보드 항목 (승패 통계) */
export interface LeaderboardEntry {
  rank: number;
  name: string;
  wins: number;
  losses: number;
  games: number;
  win_rate: number;
}

/** 리더보드 응답 — 상위 목록 + 내 전적 (상위권 밖이어도 항상 포함) */
export interface LeaderboardResponse {
  rows: LeaderboardEntry[];
  my: LeaderboardEntry | null;
}
