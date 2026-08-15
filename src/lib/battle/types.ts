import type { PieceType } from '../game/tetris';

/** 레퍼리가 보내는 보드 셀: 피스 타입 이름 또는 null */
export type BattleCell = PieceType | null;

export interface ActivePieceView {
  t: string;
  x: number;
  y: number;
  rot: number;
  shape: number[][];
}

export interface BattlePlayerState {
  player_id: string;
  board: BattleCell[][];
  piece: ActivePieceView;
  score: number;
  lines: number;
  level: number;
  hold: string | null;
  garbage: number;
  clear_flash: number;
  status: 'playing' | 'topout';
}

export interface BattleEvent {
  kind: string;
  by?: string;
  clear?: string;
  attack: number;
  winner?: number;
}

export type ServerMessage =
  | {
      type: 'start';
      match_id: string;
      you: string;
      opponent: string;
      opponent_name: string;
      your_index: number;
      opponent_index: number;
    }
  | { type: 'state'; you: BattlePlayerState; opponent: BattlePlayerState; events: BattleEvent[] }
  | {
      type: 'gameover';
      winner: string | null;
      your_result: 'win' | 'loss' | 'draw';
      your_score: number;
      opponent_score: number;
      forfeit?: boolean;
    }
  | { type: 'error'; message: string };

/** 방에 들어가기 위한 정보 (로비 → 배틀 뷰 전달) */
export interface MatchInfo {
  match_id: string;
  player_id: string;
  player_name: string;
  opponent_name: string;
  your_index: number;
}

export interface MatchCreateResponse {
  match_id: string;
  code: string;
  player_id: string;
  player_name: string;
}

/** 라인 클리어 종류 → 공격 피드 표시 이름 */
export const CLEAR_LABELS: Record<string, string> = {
  single: '싱글',
  double: '더블',
  triple: '트리플',
  tetris: '테트리스!',
  tspin_single: 'T-SPIN',
  tspin_double: 'T-SPIN 더블!',
  tspin_triple: 'T-SPIN 트리플!!'
};
