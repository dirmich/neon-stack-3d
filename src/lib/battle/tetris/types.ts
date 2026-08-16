/** 테트리스 배틀 전용 타입 — 다른 게임은 이 파일 대신 자신만의 상태 타입을 정의한다. */

import type { PieceType } from '../../game/tetris';
import type { ServerMessage } from '../protocol';

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

/** 테트리스 배틀 메시지 */
export type TetrisMessage = ServerMessage<BattlePlayerState>;

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
