import { describe, expect, it } from 'vitest';
import { LOCK_DELAY, TetrisEngine, makeActive } from './engine';
import { BOARD_HEIGHT, BOARD_WIDTH, type Board, type PieceType } from './tetris';

const ALL_TYPES: PieceType[] = ['I', 'J', 'L', 'O', 'S', 'T', 'Z'];

function emptyBoard(): Board {
  return Array.from({ length: BOARD_HEIGHT }, () => Array<PieceType | null>(BOARD_WIDTH).fill(null));
}

function fillRow(board: Board, row: number, type: PieceType, except: number[] = []) {
  for (let col = 0; col < BOARD_WIDTH; col += 1) {
    if (!except.includes(col)) board[row][col] = type;
  }
}

describe('7-bag', () => {
  it('첫 7개 스폰은 정확히 한 뭉치의 순열이다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    const seen: PieceType[] = [];
    for (let i = 0; i < 7; i += 1) {
      seen.push(engine.active.type);
      engine.hardDrop();
    }
    expect([...seen].sort()).toEqual([...ALL_TYPES].sort());
  });

  it('뭉치가 비었을 때만 리필되어 7개 창마다 순열이 유지된다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    const seen: PieceType[] = [];
    for (let i = 0; i < 14; i += 1) {
      seen.push(engine.active.type);
      engine.hardDrop();
      engine.board = emptyBoard(); // 스택이 차 오르지 않도록 보드를 비운다 (뭉치에는 영향 없음)
    }
    expect([...seen.slice(0, 7)].sort()).toEqual([...ALL_TYPES].sort());
    expect([...seen.slice(7, 14)].sort()).toEqual([...ALL_TYPES].sort());
  });
});

describe('락 딜레이', () => {
  it('착지 후 LOCK_DELAY가 지나야 락된다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.active = { ...makeActive('T'), y: 18 }; // 바닥에 닿은 상태
    const before = engine.active;
    engine.update(LOCK_DELAY - 1);
    expect(engine.active).toBe(before);
    expect(engine.status).toBe('playing');
    engine.update(1);
    expect(engine.status).toBe('playing');
    expect(engine.board[19][4]).toBe('T');
    expect(engine.active.y).toBe(-1); // 새 피스 스폰
  });

  it('이동/회전이 락 타이머를 리셋한다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.active = { ...makeActive('T'), y: 18 };
    engine.update(300);
    engine.move(1); // 락 타이머 리셋
    engine.update(LOCK_DELAY - 1);
    expect(engine.active.y).toBe(18); // 아직 락되지 않음
    engine.update(1);
    expect(engine.board[19][4]).toBe('T'); // 락됨
  });
});

describe('DAS/ARR', () => {
  it('키를 누르면 즉시 한 칸, DAS 이후 자동 이동하다가 벽에서 멈춘다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.active = makeActive('T');
    expect(engine.active.x).toBe(3); // T 스폰 x
    engine.press('left');
    expect(engine.active.x).toBe(2);
    engine.update(140); // DAS 140ms 경과
    expect(engine.active.x).toBe(0); // 벽(x=-1)에서 차단
    engine.release('left');
    engine.update(1000);
    expect(engine.active.x).toBe(0);
  });

  it('DAS 전에는 자동 이동하지 않는다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.active = makeActive('T');
    engine.press('left');
    engine.update(139);
    expect(engine.active.x).toBe(2);
    engine.release('left');
  });
});

describe('점수', () => {
  it('소프트 드롭은 한 칸당 1점', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.softDrop();
    expect(engine.score).toBe(1);
    expect(engine.active.y).toBe(0);
  });

  it('하드 드롭은 거리당 2점', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.hardDrop();
    // 빈 보드 T: 고스트 y=18, 스폰 y=-1 → 거리 19 → 38점
    expect(engine.score).toBe(38);
  });

  it('테트리스 800점, 연속 테트리스는 백투백 ×1.5 + 콤보 보너스', () => {
    const engine = new TetrisEngine();
    engine.reset(true);

    // 첫 테트리스: I 피스가 (3..6,16)에 놓여 16~19줄을 채운다
    engine.board = emptyBoard();
    for (let row = 16; row < 20; row += 1) fillRow(engine.board, row, 'J', row === 16 ? [3, 4, 5, 6] : []);
    engine.active = { ...makeActive('I'), x: 3, y: 15 };
    engine.update(600);
    expect(engine.lines).toBe(4);
    expect(engine.score).toBe(800);
    expect(engine.backToBack).toBe(false);

    // 두 번째 테트리스: 12~15줄을 채운다
    engine.board = emptyBoard();
    for (let row = 12; row < 16; row += 1) fillRow(engine.board, row, 'J', row === 13 ? [3, 4, 5, 6] : []);
    engine.active = { ...makeActive('I'), x: 3, y: 12 };
    engine.update(600);
    expect(engine.lines).toBe(8);
    expect(engine.backToBack).toBe(true);
    expect(engine.combo).toBe(2);
    // 800 + round(800*1.5) + 50(콤보 보너스)
    expect(engine.score).toBe(800 + 1200 + 50);
  });

  it('T-spin 더블은 1200점', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.board = emptyBoard();
    fillRow(engine.board, 15, 'J', [4]); // (4,15)는 T 셀
    fillRow(engine.board, 16, 'J', [3, 4]); // (3,16),(4,16)은 T 셀
    engine.board[17][3] = 'J';
    engine.board[17][5] = 'J';
    engine.board[18][4] = 'J'; // 바닥
    engine.active = { ...makeActive('T'), x: 3, y: 15 };
    expect(engine.rotate(1)).toBe(true);
    engine.update(600);
    expect(engine.lines).toBe(2);
    expect(engine.score).toBe(1200);
    expect(engine.combo).toBe(1);
    expect(engine.backToBack).toBe(false);
  });

  it('줄이 안 지워지는 락은 콤보를 초기화한다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.board = emptyBoard();
    fillRow(engine.board, 15, 'J', [4]);
    fillRow(engine.board, 16, 'J', [3, 4]);
    engine.board[17][3] = 'J';
    engine.board[17][5] = 'J';
    engine.board[18][4] = 'J';
    engine.active = { ...makeActive('T'), x: 3, y: 15 };
    engine.rotate(1);
    engine.update(600);
    expect(engine.combo).toBe(1);

    // 줄을 만들지 않는 락 → 콤보 리셋
    engine.active = { ...makeActive('T'), x: 3, y: 16 };
    engine.update(600);
    expect(engine.combo).toBe(0);
  });
});

describe('탑아웃', () => {
  it('스택이 꼭대기까지 차면 다음 스폰에서 게임 오버(블록 아웃)', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.board = emptyBoard();
    // 상단 스택 — 줄은 완성되지 않도록 구멍을 남긴다
    fillRow(engine.board, 0, 'J', [4, 7]); // (4,0)은 T 셀, (7,0)은 영구 구멍
    fillRow(engine.board, 1, 'J', [3, 4, 5, 8]); // T 셀 3개 + 영구 구멍
    engine.board[2][3] = 'J';
    engine.board[2][4] = 'J';
    engine.board[2][5] = 'J'; // T가 (3,0)에서 닿는 바닥
    engine.active = { ...makeActive('T'), x: 3, y: 0 };
    engine.update(LOCK_DELAY + 1);
    // 락되었지만 줄이 안 지워져 스택이 그대로 → 다음 스폰이 충돌
    expect(engine.board[0][4]).toBe('T');
    expect(engine.status).toBe('over');
  });
});

describe('상태 전이', () => {
  it('ready에서 togglePause하면 stateChange를 내보내고 playing이 된다', () => {
    const engine = new TetrisEngine();
    const events: string[] = [];
    engine.onEvent = (event) => events.push(event.type);
    engine.togglePause();
    expect(engine.status).toBe('playing');
    expect(events).toContain('stateChange');
  });

  it('reset은 눌린 키 상태를 초기화한다 (재시작 후 DAS 잔상 방지)', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.press('left');
    engine.press('down');
    engine.reset(true);
    expect(engine._leftPressed).toBe(false);
    expect(engine._downPressed).toBe(false);
    expect(engine._dir).toBeNull();
  });

  it('hold는 중력 누적을 초기화한다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine._gravityAccumulator = 500;
    engine.hold();
    expect(engine._gravityAccumulator).toBe(0);
  });
});

describe('HOLD', () => {
  it('보관 후 다시 보관하면 원래 피스가 돌아온다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    const first = engine.active.type;
    engine.hold();
    expect(engine.held).toBe(first);
    expect(engine.canHold).toBe(false);
    const second = engine.active.type;
    expect(second).not.toBe(first);
    // 한 번에 연속 홀드는 불가 (canHold=false)
    engine.hold();
    expect(engine.active.type).toBe(second);
    expect(engine.held).toBe(first);
    // 락 후에 다시 홀드하면 스왑백된다
    engine.hardDrop();
    expect(engine.canHold).toBe(true);
    const third = engine.active.type;
    engine.hold();
    expect(engine.active.type).toBe(first);
    expect(engine.held).toBe(third);
    expect(engine.canHold).toBe(false);
  });
});

describe('SRS 통합', () => {
  it('바닥의 I 피스를 회전하면 위로 킥되어 들어간다', () => {
    const engine = new TetrisEngine();
    engine.reset(true);
    engine.active = { ...makeActive('I'), x: 0, y: 18 };
    expect(engine.rotate(1)).toBe(true);
    expect(engine.active.x).toBe(1);
    expect(engine.active.y).toBe(16);
    expect(engine.active.rotation).toBe(1);
  });
});
