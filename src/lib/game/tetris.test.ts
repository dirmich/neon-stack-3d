import { describe, expect, it } from 'vitest';
import {
  BOARD_HEIGHT,
  BOARD_WIDTH,
  clearFullRows,
  collides,
  createBoard,
  createPiece,
  ghostY,
  mergePiece,
  type Board,
  type PieceType
} from './tetris';

describe('clearFullRows', () => {
  it('가득 찬 줄만 제거하고 나머지를 위로 쌓는다', () => {
    const board = createBoard();
    board[19].fill('T');
    board[18].fill('J');
    board[17][3] = 'L';
    const { board: next, count } = clearFullRows(board);
    expect(count).toBe(2);
    // 제거된 2줄만큼 위쪽에 빈 줄이 생기고, 남은 L은 아래로 내려온다
    expect(next[19][3]).toBe('L');
    expect(next[18].every((cell) => cell === null)).toBe(true);
    expect(next[17].every((cell) => cell === null)).toBe(true);
    expect(next.length).toBe(BOARD_HEIGHT);
  });

  it('빈 보드는 count 0', () => {
    const { count } = clearFullRows(createBoard());
    expect(count).toBe(0);
  });
});

describe('collides', () => {
  it('보드 밖(좌/우/아래)은 충돌로 친다', () => {
    const board = createBoard();
    const piece = createPiece('T');
    expect(collides(board, { ...piece, x: -1 })).toBe(true);
    expect(collides(board, { ...piece, y: 19 })).toBe(true);
  });

  it('위(y<0)는 충돌로 치지 않는다', () => {
    const board = createBoard();
    const piece = { ...createPiece('T'), y: -1 };
    expect(collides(board, piece)).toBe(false);
  });
});

describe('mergePiece', () => {
  it('보드 위 셀(y<0)은 무시하고 병합한다', () => {
    const board = createBoard();
    const piece = { ...createPiece('T'), y: -1 };
    const next = mergePiece(board, piece);
    const count = next.flat().filter((cell): cell is PieceType => cell !== null).length;
    // T 셀: (3..5,0) + (4,-1) → y>=0만 병합되므로 3개
    expect(count).toBe(3);
  });
});

describe('ghostY', () => {
  it('빈 보드에서 T는 바닥 y=18에 닿는다', () => {
    const board = createBoard();
    const piece = createPiece('T');
    expect(ghostY(board, piece)).toBe(18);
  });

  it('쌓인 블록 위에 멈춘다', () => {
    const board = createBoard();
    board[15][3] = 'J';
    board[15][4] = 'J';
    board[15][5] = 'J';
    const piece = createPiece('T');
    // T 바(3..5)가 15행 블록 바로 위인 14행에 서야 한다 → y=13
    expect(ghostY(board, piece)).toBe(13);
  });
});
