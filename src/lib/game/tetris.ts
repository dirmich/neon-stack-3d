export const BOARD_WIDTH = 10;
export const BOARD_HEIGHT = 20;

export type PieceType = 'I' | 'J' | 'L' | 'O' | 'S' | 'T' | 'Z';
export type Cell = PieceType | null;
export type Board = Cell[][];
export type Point = [number, number];

export interface Piece {
  type: PieceType;
  shape: number[][];
  x: number;
  y: number;
}

export const COLORS: Record<PieceType, string> = {
  I: '#35d9ff',
  J: '#5271ff',
  L: '#ff9d36',
  O: '#ffe45e',
  S: '#65ef8a',
  T: '#c879ff',
  Z: '#ff5777'
};

export const SHAPES: Record<PieceType, number[][]> = {
  I: [
    [0, 0, 0, 0],
    [1, 1, 1, 1],
    [0, 0, 0, 0],
    [0, 0, 0, 0]
  ],
  J: [
    [1, 0, 0],
    [1, 1, 1],
    [0, 0, 0]
  ],
  L: [
    [0, 0, 1],
    [1, 1, 1],
    [0, 0, 0]
  ],
  O: [
    [1, 1],
    [1, 1]
  ],
  S: [
    [0, 1, 1],
    [1, 1, 0],
    [0, 0, 0]
  ],
  T: [
    [0, 1, 0],
    [1, 1, 1],
    [0, 0, 0]
  ],
  Z: [
    [1, 1, 0],
    [0, 1, 1],
    [0, 0, 0]
  ]
};

export const createBoard = (): Board =>
  Array.from({ length: BOARD_HEIGHT }, () => Array<Cell>(BOARD_WIDTH).fill(null));

export const cloneShape = (shape: number[][]) => shape.map((row) => [...row]);

export const createPiece = (type: PieceType): Piece => ({
  type,
  shape: cloneShape(SHAPES[type]),
  x: Math.floor((BOARD_WIDTH - SHAPES[type][0].length) / 2),
  y: -1
});

export const rotateShape = (shape: number[][]): number[][] =>
  shape[0].map((_, column) => shape.map((row) => row[column]).reverse());

export function collides(board: Board, piece: Piece, dx = 0, dy = 0, shape = piece.shape): boolean {
  for (let row = 0; row < shape.length; row += 1) {
    for (let column = 0; column < shape[row].length; column += 1) {
      if (!shape[row][column]) continue;
      const x = piece.x + column + dx;
      const y = piece.y + row + dy;
      if (x < 0 || x >= BOARD_WIDTH || y >= BOARD_HEIGHT || (y >= 0 && board[y][x])) return true;
    }
  }
  return false;
}

export function cellsFor(piece: Piece): Point[] {
  const cells: Point[] = [];
  piece.shape.forEach((row, y) =>
    row.forEach((filled, x) => {
      if (filled) cells.push([piece.x + x, piece.y + y]);
    })
  );
  return cells;
}

export function mergePiece(board: Board, piece: Piece): Board {
  const next = board.map((row) => [...row]);
  for (const [x, y] of cellsFor(piece)) {
    if (y >= 0) next[y][x] = piece.type;
  }
  return next;
}

export function clearFullRows(board: Board): { board: Board; count: number } {
  const remaining = board.filter((row) => row.some((cell) => cell === null));
  const count = BOARD_HEIGHT - remaining.length;
  return {
    board: [...Array.from({ length: count }, () => Array<Cell>(BOARD_WIDTH).fill(null)), ...remaining],
    count
  };
}

export function ghostY(board: Board, piece: Piece): number {
  let offset = 0;
  while (!collides(board, piece, 0, offset + 1)) offset += 1;
  return piece.y + offset;
}

export function shuffledBag(): PieceType[] {
  const bag: PieceType[] = ['I', 'J', 'L', 'O', 'S', 'T', 'Z'];
  for (let i = bag.length - 1; i > 0; i -= 1) {
    const j = Math.floor(Math.random() * (i + 1));
    [bag[i], bag[j]] = [bag[j], bag[i]];
  }
  return bag;
}
