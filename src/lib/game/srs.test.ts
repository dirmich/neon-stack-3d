import { describe, expect, it } from 'vitest';
import { SHAPES, type PieceType } from './tetris';
import { isTSpin, kicksFor, nextRotation, rotateCCW, rotateCW } from './srs';

const ALL_TYPES: PieceType[] = ['I', 'J', 'L', 'O', 'S', 'T', 'Z'];

describe('srs 회전', () => {
  it('CW 4번은 모든 피스에서 스폰 상태로 돌아온다', () => {
    for (const type of ALL_TYPES) {
      let shape = SHAPES[type].map((row) => [...row]);
      for (let i = 0; i < 4; i += 1) shape = rotateCW(shape);
      expect(shape, type).toEqual(SHAPES[type]);
    }
  });

  it('CCW 4번은 모든 피스에서 스폰 상태로 돌아온다', () => {
    for (const type of ALL_TYPES) {
      let shape = SHAPES[type].map((row) => [...row]);
      for (let i = 0; i < 4; i += 1) shape = rotateCCW(shape);
      expect(shape, type).toEqual(SHAPES[type]);
    }
  });

  it('CCW는 CW 3번과 같다', () => {
    for (const type of ALL_TYPES) {
      const cw3 = rotateCW(rotateCW(rotateCW(SHAPES[type])));
      expect(rotateCCW(SHAPES[type]), type).toEqual(cw3);
    }
  });

  it('O 피스는 회전해도 모양이 같다', () => {
    expect(rotateCW(SHAPES.O)).toEqual(SHAPES.O);
    expect(rotateCCW(SHAPES.O)).toEqual(SHAPES.O);
  });

  it('J 피스 CW는 오른쪽을 보는 상태가 된다', () => {
    expect(rotateCW(SHAPES.J)).toEqual([
      [0, 1, 0],
      [0, 1, 0],
      [1, 1, 0]
    ]);
  });

  it('T 피스 CW는 오른쪽을 보는 상태가 된다', () => {
    expect(rotateCW(SHAPES.T)).toEqual([
      [0, 1, 0],
      [1, 1, 0],
      [0, 1, 0]
    ]);
  });

  it('nextRotation이 상태를 순환시킨다', () => {
    expect(nextRotation(0, 1)).toBe(1);
    expect(nextRotation(3, 1)).toBe(0);
    expect(nextRotation(0, -1)).toBe(3);
    expect(nextRotation(2, -1)).toBe(1);
  });

  it('O 피스 킥은 [0,0]뿐이다', () => {
    expect(kicksFor('O', 0, 1)).toEqual([[0, 0]]);
  });
});

describe('srs 킥', () => {
  it('JLSTZ 0→1 첫 킥 후보가 y-down 좌표계와 일치한다', () => {
    expect(kicksFor('T', 0, 1)).toEqual([
      [0, 0],
      [-1, 0],
      [-1, -1],
      [0, 2],
      [-1, 2]
    ]);
  });

  it('I 0→1 킥에 I 특수 테이블이 적용된다', () => {
    expect(kicksFor('I', 0, 1)).toEqual([
      [0, 0],
      [-2, 0],
      [1, 0],
      [-2, 1],
      [1, -2]
    ]);
  });
});

describe('srs T-spin 판정', () => {
  const empty = () => Array.from({ length: 20 }, () => Array(10).fill(null));

  it('회전을 마지막 동작으로 하고 모서리 3개가 채워지면 T-spin', () => {
    const board = empty();
    board[15][3] = 'J';
    board[15][5] = 'J';
    board[17][3] = 'J';
    board[17][5] = 'J';
    // T 피스 (4,15),(3,16),(4,16),(4,17) — 피벗 (4,16)
    const piece = { x: 3, y: 15, type: 'T' as PieceType };
    expect(isTSpin(board, piece, 'rotate')).toBe(true);
  });

  it('움직임이 마지막 동작이면 T-spin이 아니다', () => {
    const board = empty();
    board[15][3] = 'J';
    board[15][5] = 'J';
    board[17][3] = 'J';
    board[17][5] = 'J';
    const piece = { x: 3, y: 15, type: 'T' as PieceType };
    expect(isTSpin(board, piece, 'move')).toBe(false);
  });

  it('T가 아니면 T-spin이 아니다', () => {
    const board = empty();
    const piece = { x: 3, y: 15, type: 'J' as PieceType };
    expect(isTSpin(board, piece, 'rotate')).toBe(false);
  });

  it('모서리가 2개만 채워지면 T-spin이 아니다', () => {
    const board = empty();
    board[15][5] = 'J';
    board[17][3] = 'J';
    const piece = { x: 3, y: 15, type: 'T' as PieceType };
    expect(isTSpin(board, piece, 'rotate')).toBe(false);
  });

  it('보드 밖 모서리(좌/우/아래)는 채워진 것으로 친다', () => {
    const board = empty();
    // 피벗 (x+1, y+1) = (-1, 16), 모서리: (-2,15),(0,15),(-2,17),(0,17)
    // 왼쪽 2개가 보드 밖 → 나머지 (0,15),(0,17)만 채우면 4개
    board[15][0] = 'J';
    board[17][0] = 'J';
    const piece = { x: -2, y: 15, type: 'T' as PieceType };
    expect(isTSpin(board, piece, 'rotate')).toBe(true);
  });
});
