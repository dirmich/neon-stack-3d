import type { PieceType } from './tetris';

/**
 * SRS(Super Rotation System) 구현.
 * 회전 상태: 0=스폰, 1=R(CW), 2=2(180°), 3=L(CCW).
 * 좌표계는 보드와 동일 — x는 오른쪽, y는 아래쪽(y+1이 한 칸 아래).
 * 킥 테이블은 Tetris guideline의 y-up 테이블에서 y 부호를 뒤집어 이 좌표계에 맞췄다.
 */
export type RotationState = 0 | 1 | 2 | 3;

/** 시계 방향 90° 회전 (전치 후 행 뒤집기) */
export function rotateCW(matrix: number[][]): number[][] {
  return matrix[0].map((_, column) => matrix.map((row) => row[column])).reverse();
}

/** 반시계 방향 90° 회전 */
export function rotateCCW(matrix: number[][]): number[][] {
  return matrix[0].map((_, row) =>
    matrix
      .map((r) => r[row])
      .reverse()
  );
}

export function nextRotation(from: RotationState, dir: 1 | -1): RotationState {
  return (((from + (dir === 1 ? 1 : 3)) % 4) as RotationState);
}

type KickTable = Record<string, [number, number][]>;

const JLSTZ_KICKS: KickTable = {
  '0>1': [
    [0, 0],
    [-1, 0],
    [-1, -1],
    [0, 2],
    [-1, 2]
  ],
  '1>0': [
    [0, 0],
    [1, 0],
    [1, 1],
    [0, -2],
    [1, -2]
  ],
  '1>2': [
    [0, 0],
    [1, 0],
    [1, 1],
    [0, -2],
    [1, -2]
  ],
  '2>1': [
    [0, 0],
    [-1, 0],
    [-1, -1],
    [0, 2],
    [-1, 2]
  ],
  '2>3': [
    [0, 0],
    [1, 0],
    [1, -1],
    [0, 2],
    [1, 2]
  ],
  '3>2': [
    [0, 0],
    [-1, 0],
    [-1, 1],
    [0, -2],
    [-1, -2]
  ],
  '3>0': [
    [0, 0],
    [-1, 0],
    [-1, 1],
    [0, -2],
    [-1, -2]
  ],
  '0>3': [
    [0, 0],
    [1, 0],
    [1, -1],
    [0, 2],
    [1, 2]
  ]
};

const I_KICKS: KickTable = {
  '0>1': [
    [0, 0],
    [-2, 0],
    [1, 0],
    [-2, 1],
    [1, -2]
  ],
  '1>0': [
    [0, 0],
    [2, 0],
    [-1, 0],
    [2, -1],
    [-1, 2]
  ],
  '1>2': [
    [0, 0],
    [-1, 0],
    [2, 0],
    [-1, -2],
    [2, 1]
  ],
  '2>1': [
    [0, 0],
    [1, 0],
    [-2, 0],
    [1, 2],
    [-2, -1]
  ],
  '2>3': [
    [0, 0],
    [2, 0],
    [-1, 0],
    [2, -1],
    [-1, 2]
  ],
  '3>2': [
    [0, 0],
    [-2, 0],
    [1, 0],
    [-2, 1],
    [1, -2]
  ],
  '3>0': [
    [0, 0],
    [1, 0],
    [-2, 0],
    [1, 2],
    [-2, -1]
  ],
  '0>3': [
    [0, 0],
    [-1, 0],
    [2, 0],
    [-1, -2],
    [2, 1]
  ]
};

/** from → to 회전 시 시도할 킥 오프셋 목록 (O는 회전이 무의미하므로 [0,0]만) */
export function kicksFor(type: PieceType, from: RotationState, to: RotationState): [number, number][] {
  if (type === 'O') return [[0, 0]];
  const table = type === 'I' ? I_KICKS : JLSTZ_KICKS;
  return table[`${from}>${to}`] ?? [[0, 0]];
}

/**
 * T-spin 판정: T 피스가 회전을 마지막 동작으로 하여 락되었고,
 * 피벗 주변 4개 모서리 중 3개 이상이 채워져 있으면 T-spin.
 * 보드 밖(좌/우/아래)은 채워진 것으로 간주, 위(y<0)는 빈 것으로 간주.
 */
export function isTSpin(
  board: (PieceType | null)[][],
  piece: { x: number; y: number; type: PieceType },
  lastMove: 'move' | 'rotate' | 'drop' | 'none'
): boolean {
  if (piece.type !== 'T' || lastMove !== 'rotate') return false;
  // T 피스 피벗: 스폰 상태에서 (1,1). 3x3 바운딩 박스 안에서 피벗은 항상 중심.
  const pivotX = piece.x + 1;
  const pivotY = piece.y + 1;
  const corners: [number, number][] = [
    [pivotX - 1, pivotY - 1],
    [pivotX + 1, pivotY - 1],
    [pivotX - 1, pivotY + 1],
    [pivotX + 1, pivotY + 1]
  ];
  let filled = 0;
  for (const [x, y] of corners) {
    if (x < 0 || x >= 10 || y >= 20) {
      filled += 1;
    } else if (y >= 0 && board[y][x]) {
      filled += 1;
    }
  }
  return filled >= 3;
}
