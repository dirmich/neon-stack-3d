import {
  BOARD_HEIGHT,
  BOARD_WIDTH,
  clearFullRows,
  collides,
  createBoard,
  createPiece,
  cellsFor,
  ghostY,
  mergePiece,
  shuffledBag,
  type Board,
  type Piece,
  type PieceType
} from './tetris';
import { isTSpin, kicksFor, nextRotation, rotateCCW, rotateCW, type RotationState } from './srs';

export type GameStatus = 'ready' | 'playing' | 'paused' | 'over';

export interface ActivePiece extends Piece {
  rotation: RotationState;
}

export type EngineEvent =
  | { type: 'tone'; frequency: number; duration?: number; volume?: number }
  | { type: 'gameOver' }
  | { type: 'stateChange' };

export const DAS_DELAY = 140; // ms — 키를 누른 뒤 자동 이동 시작까지
export const ARR_DELAY = 35; // ms — 자동 이동 간격
export const LOCK_DELAY = 500; // ms — 착지 후 락까지
export const MAX_LOCK_RESETS = 15;
export const SOFT_DROP_REPEAT = 40; // ms — 아래 키를 누르고 있을 때 이동 간격

export function dropIntervalFor(level: number): number {
  return Math.max(95, 870 - (level - 1) * 68);
}

export function makeActive(type: PieceType): ActivePiece {
  return { ...createPiece(type), rotation: 0 };
}

const CLEAR_POINTS = [0, 100, 300, 500, 800];
const TSPIN_POINTS = [0, 800, 1200, 1600];

/**
 * 프레임워크에 독립적인 테트리스 게임 엔진.
 * Svelte 쪽에서는 $state로 인스턴스를 감싸 public 필드 변경을 반응형으로 쓴다.
 */
export class TetrisEngine {
  status: GameStatus = 'ready';
  board: Board = createBoard();
  active: ActivePiece = makeActive('T');
  held: PieceType | null = null;
  canHold = true;
  score = 0;
  lines = 0;
  level = 1;
  combo = 0;
  backToBack = false;
  clearFlash = 0;
  bag: PieceType[] = shuffledBag();

  onEvent?: (event: EngineEvent) => void;

  // 내부 상태 — $state 프록시와의 호환을 위해 접두사 _를 쓰는 공개 필드로 둔다.
  _lastMove: 'move' | 'rotate' | 'drop' | 'none' = 'none';
  _lockTimer = 0;
  _lockResets = 0;
  _gravityAccumulator = 0;
  _lastClearType: 'none' | 'tetris' | 'tspin' | 'other' = 'none';

  // DAS/ARR 키 상태
  _leftPressed = false;
  _rightPressed = false;
  _downPressed = false;
  _dir: 'left' | 'right' | null = null;
  _dasElapsed = 0;
  _repeatElapsed = 0;
  _downElapsed = 0;
  _dirty = false;

  nextQueue(count = 3): PieceType[] {
    return this.bag.slice(0, count);
  }

  /** 상태가 바뀌었을 때 stateChange 이벤트를 한 번만 내보낸다. */
  flush(): void {
    if (this._dirty) {
      this._dirty = false;
      this.onEvent?.({ type: 'stateChange' });
    }
  }

  private markDirty(): void {
    this._dirty = true;
  }

  reset(startImmediately = true) {
    this.bag = shuffledBag();
    this.board = createBoard();
    this.active = makeActive(this.takeFromBag());
    this.held = null;
    this.canHold = true;
    this.score = 0;
    this.lines = 0;
    this.level = 1;
    this.combo = 0;
    this.backToBack = false;
    this.clearFlash = 0;
    this._lastClearType = 'none';
    this._lockTimer = 0;
    this._lockResets = 0;
    this._gravityAccumulator = 0;
    this._dir = null;
    this._dasElapsed = 0;
    this._repeatElapsed = 0;
    this._downElapsed = 0;
    // 눌린 키 상태까지 완전히 초기화 (재시작 직후 DAS/소프트드롭 잔상 방지)
    this._leftPressed = false;
    this._rightPressed = false;
    this._downPressed = false;
    this._dirty = false;
    this.status = startImmediately ? 'playing' : 'ready';
    if (startImmediately) this.emitTone(520, 0.09);
    this.markDirty();
    this.flush();
  }

  togglePause() {
    if (this.status === 'ready') {
      this.status = 'playing';
      this.emitTone(520, 0.08);
      this.markDirty();
      this.flush();
      return;
    }
    if (this.status === 'over') return;
    this.status = this.status === 'paused' ? 'playing' : 'paused';
    this.emitTone(this.status === 'playing' ? 520 : 230, 0.06);
    this.markDirty();
    this.flush();
  }

  move(dir: 1 | -1, silent = false): boolean {
    if (this.status !== 'playing' || collides(this.board, this.active, dir, 0)) return false;
    this.active = { ...this.active, x: this.active.x + dir };
    this._lastMove = 'move';
    this.resetLockTimer();
    this.markDirty();
    this.flush();
    if (!silent) this.emitTone(260, 0.025, 0.012);
    return true;
  }

  rotate(dir: 1 | -1): boolean {
    if (this.status !== 'playing' || this.active.type === 'O') return false;
    const from = this.active.rotation;
    const to = nextRotation(from, dir);
    const shape = dir === 1 ? rotateCW(this.active.shape) : rotateCCW(this.active.shape);
    for (const [kickX, kickY] of kicksFor(this.active.type, from, to)) {
      const candidate: ActivePiece = {
        ...this.active,
        x: this.active.x + kickX,
        y: this.active.y + kickY,
        shape,
        rotation: to
      };
      if (!collides(this.board, candidate)) {
        this.active = candidate;
        this._lastMove = 'rotate';
        this.resetLockTimer();
        this.markDirty();
        this.flush();
        this.emitTone(410, 0.035, 0.018);
        return true;
      }
    }
    return false;
  }

  softDrop(silent = false): void {
    if (this.status !== 'playing') return;
    if (!collides(this.board, this.active, 0, 1)) {
      this.active = { ...this.active, y: this.active.y + 1 };
      this.score += 1;
      this._lastMove = 'move';
      this.resetLockTimer();
    } else {
      this.lockPiece();
    }
    this.markDirty();
    this.flush();
    if (!silent && this.status === 'playing') this.emitTone(200, 0.02, 0.01);
  }

  hardDrop(): void {
    if (this.status !== 'playing') return;
    const target = ghostY(this.board, this.active);
    const distance = target - this.active.y;
    const dropped: ActivePiece = { ...this.active, y: target };
    this.active = dropped;
    this.score += distance * 2;
    this._lastMove = 'drop';
    this.emitTone(110, 0.055, 0.04);
    this.lockPiece(dropped);
    this.markDirty();
    this.flush();
  }

  hold(): void {
    if (this.status !== 'playing' || !this.canHold) return;
    const outgoing = this.active.type;
    if (this.held) {
      this.active = makeActive(this.held);
      this.held = outgoing;
    } else {
      this.held = outgoing;
      this.active = makeActive(this.takeFromBag());
    }
    this.canHold = false;
    this._lastMove = 'none';
    this._lockTimer = 0;
    this._lockResets = 0;
    this._gravityAccumulator = 0;
    this.emitTone(330, 0.06, 0.025);
    if (collides(this.board, this.active)) this.gameOver();
    this.markDirty();
    this.flush();
  }

  press(dir: 'left' | 'right' | 'down'): void {
    if (dir === 'down') {
      this._downPressed = true;
      this._downElapsed = 0;
      return;
    }
    const key = dir === 'left' ? '_leftPressed' : '_rightPressed';
    if (this[key]) return;
    this[key] = true;
    this._dir = dir;
    this._dasElapsed = 0;
    this._repeatElapsed = 0;
    if (this.status === 'playing') this.move(dir === 'left' ? -1 : 1);
  }

  release(dir: 'left' | 'right' | 'down'): void {
    if (dir === 'down') {
      this._downPressed = false;
      this._downElapsed = 0;
      return;
    }
    const key = dir === 'left' ? '_leftPressed' : '_rightPressed';
    this[key] = false;
    if (this._dir === dir) {
      const other = dir === 'left' ? '_rightPressed' : '_leftPressed';
      if (this[other]) {
        this._dir = dir === 'left' ? 'right' : 'left';
        this._dasElapsed = 0;
        this._repeatElapsed = 0;
        if (this.status === 'playing') this.move(this._dir === 'left' ? -1 : 1);
      } else {
        this._dir = null;
      }
    }
  }

  /** 프레임마다 호출. 중력, 락 딜레이, DAS/ARR, 소프트 드롭 반복을 처리한다. */
  update(deltaMs: number): void {
    if (this.status !== 'playing') return;

    // DAS/ARR
    if (this._dir) {
      this._dasElapsed += deltaMs;
      if (this._dasElapsed >= DAS_DELAY) {
        this._repeatElapsed += deltaMs;
        let guard = 0;
        while (this._repeatElapsed >= ARR_DELAY && guard < 4) {
          this._repeatElapsed -= ARR_DELAY;
          if (!this.move(this._dir === 'left' ? -1 : 1, true)) break;
          guard += 1;
        }
      }
    }

    // 아래 키 반복 (소프트 드롭)
    if (this._downPressed) {
      this._downElapsed += deltaMs;
      let guard = 0;
      while (this._downElapsed >= SOFT_DROP_REPEAT && guard < 4) {
        this._downElapsed -= SOFT_DROP_REPEAT;
        this.softDrop(true);
        if (this.status !== 'playing') break;
        guard += 1;
      }
    }

    // 중력 + 락 딜레이
    if (!collides(this.board, this.active, 0, 1)) {
      this._gravityAccumulator += deltaMs;
      const interval = dropIntervalFor(this.level);
      let guard = 0;
      while (this._gravityAccumulator >= interval && guard < 8) {
        this._gravityAccumulator -= interval;
        if (collides(this.board, this.active, 0, 1)) break;
        this.active = { ...this.active, y: this.active.y + 1 };
        this.markDirty();
        guard += 1;
      }
    } else {
      this._lockTimer += deltaMs;
      if (this._lockTimer >= LOCK_DELAY) this.lockPiece();
    }
    this.flush();
  }

  takeFromBag(): PieceType {
    // 7-bag: 뭉치가 완전히 비었을 때만 새 뭉치를 채운다.
    if (this.bag.length === 0) this.bag = shuffledBag();
    return this.bag.shift()!;
  }

  spawnNext(): void {
    const next = makeActive(this.takeFromBag());
    if (collides(this.board, next)) {
      this.gameOver();
      return;
    }
    this.active = next;
    this.canHold = true;
    this._lastMove = 'none';
    this._lockTimer = 0;
    this._lockResets = 0;
    this._gravityAccumulator = 0;
    this.markDirty();
  }

  lockPiece(piece: ActivePiece = this.active): void {
    if (this.status !== 'playing') return;
    // 락 아웃: 피스 전체가 보드 위(y<0)에 있을 때 락되면 게임 오버.
    const cells = cellsFor(piece);
    if (cells.length > 0 && cells.every(([, y]) => y < 0)) {
      this.gameOver();
      return;
    }
    const tspin = isTSpin(this.board, piece, this._lastMove);
    const merged = mergePiece(this.board, piece);
    const cleared = clearFullRows(merged);
    const count = cleared.count;

    if (count > 0) {
      this.board = cleared.board;
      this.lines += count;
      this.level = Math.floor(this.lines / 10) + 1;
      this.clearFlash += 1;

      const base = tspin ? TSPIN_POINTS[count] ?? 0 : CLEAR_POINTS[count] ?? 0;
      const isB2BClear = tspin || count === 4;
      if (isB2BClear && (this._lastClearType === 'tetris' || this._lastClearType === 'tspin')) {
        this.backToBack = true;
      } else {
        this.backToBack = false;
      }
      const b2bMultiplier = this.backToBack && isB2BClear ? 1.5 : 1;

      this.combo += 1;
      const comboBonus = this.combo > 1 ? 50 * (this.combo - 1) * this.level : 0;

      this.score += Math.round(base * b2bMultiplier * this.level) + comboBonus;
      this._lastClearType = tspin ? 'tspin' : count === 4 ? 'tetris' : 'other';
      this.emitTone(tspin ? 1080 : count === 4 ? 960 : 740, 0.14, 0.055);
    } else {
      this.board = cleared.board;
      this.combo = 0;
      this._lastClearType = 'none';
      this.emitTone(180, 0.035, 0.018);
    }
    this.markDirty();
    this.spawnNext();
  }

  resetLockTimer(): void {
    if (this._lockResets >= MAX_LOCK_RESETS) return;
    this._lockResets += 1;
    this._lockTimer = 0;
  }

  gameOver(): void {
    this.status = 'over';
    this.combo = 0;
    this.backToBack = false;
    this._dir = null;
    this.markDirty();
    this.flush();
    this.onEvent?.({ type: 'gameOver' });
  }

  emitTone(frequency: number, duration = 0.06, volume = 0.035): void {
    this.onEvent?.({ type: 'tone', frequency, duration, volume });
  }
}
