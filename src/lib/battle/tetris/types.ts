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

/** 아이템 종류 (아이템 배틀 모드) */
export type BattleItemKind = 'attack' | 'speed' | 'holes' | 'clear' | 'shield' | 'slow';

export interface BattlePlayerState {
  player_id: string;
  board: BattleCell[][];
  /** 아이템 배틀 모드 — 보드와 같은 크기의 아이템 셀 (없으면 null) */
  items: (BattleItemKind | null)[][];
  piece: ActivePieceView;
  score: number;
  lines: number;
  level: number;
  hold: string | null;
  garbage: number;
  clear_flash: number;
  status: 'playing' | 'topout';
  /** 아이템 모드: 방패 잔여 횟수 / 가속·감속 상태 */
  shield?: number;
  speed?: boolean;
  slow?: boolean;
}

/** 아이템 이름 → 표시 라벨/설명 */
export const ITEM_LABELS: Record<BattleItemKind, { name: string; desc: string; good: boolean }> = {
  attack: { name: '폭탄', desc: '상대에게 가비지 3줄', good: false },
  speed: { name: '가속', desc: '상대 중력 1.5배 (20초)', good: false },
  holes: { name: '구멍', desc: '상대 블록 8개 제거', good: false },
  clear: { name: '정리', desc: '내 블록 한 줄 제거', good: true },
  shield: { name: '방패', desc: '가비지 2줄 차단', good: true },
  slow: { name: '감속', desc: '내 중력 0.7배 (20초)', good: true }
};

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
