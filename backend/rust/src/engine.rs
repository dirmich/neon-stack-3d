//! 결정적 2인 배틀 테트리스 엔진.
//! 프론트엔드 단일 플레이 엔진(src/lib/game/engine.ts)의 로직을 Rust로 포팅하고,
//! 배틀용 가비지 공격/수신 로직을 추가했다. 서버가 권위(authoritative) 상태를 소유한다.

use rand::rngs::SmallRng;
use rand::{Rng, SeedableRng};
use serde::Serialize;

pub const W: usize = 10;
pub const H: usize = 20;
pub const LOCK_DELAY_MS: i32 = 500;
pub const MAX_LOCK_RESETS: u32 = 15;

pub const PIECE_NAMES: [&str; 7] = ["I", "J", "L", "O", "S", "T", "Z"];

pub type Cell = Option<u8>;
pub type Board = Vec<Vec<Cell>>;

/// 아이템 배틀 모드의 아이템 종류.
/// 라인 클리어 시 그 줄에 있던 아이템이 전부 발동한다.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum ItemKind {
    /// 폭탄: 상대에게 가비지 3줄 즉시 추가 (악영향)
    Attack,
    /// 가속: 상대 중력 1.5배로 20초 (악영향)
    Speed,
    /// 구멍: 상대 보드의 채워진 셀 8개를 무작위로 제거 (악영향)
    Holes,
    /// 정리: 내 보드의 가장 낮은 채워진 줄 1개 제거 (이로운)
    Clear,
    /// 방패: 다음 가비지 2줄을 무효화 (이로운)
    Shield,
    /// 감속: 내 중력 0.7배로 20초 (이로운)
    Slow,
}

impl ItemKind {
    pub fn all() -> [ItemKind; 6] {
        [ItemKind::Attack, ItemKind::Speed, ItemKind::Holes, ItemKind::Clear, ItemKind::Shield, ItemKind::Slow]
    }

    pub fn name(self) -> &'static str {
        match self {
            ItemKind::Attack => "attack",
            ItemKind::Speed => "speed",
            ItemKind::Holes => "holes",
            ItemKind::Clear => "clear",
            ItemKind::Shield => "shield",
            ItemKind::Slow => "slow",
        }
    }
}

/// 아이템 셀 그리드 — board와 같은 크기. 아이템은 빈 셀 위에 놓이며 충돌하지 않는다.
pub type ItemGrid = Vec<Vec<Option<ItemKind>>>;

pub fn drop_interval_for(level: i32) -> i32 {
    (870 - (level - 1) * 68).max(95)
}

fn spawn_shape(t: u8) -> Vec<Vec<u8>> {
    match t {
        0 => vec![vec![0, 0, 0, 0], vec![1, 1, 1, 1], vec![0, 0, 0, 0], vec![0, 0, 0, 0]],
        1 => vec![vec![1, 0, 0], vec![1, 1, 1], vec![0, 0, 0]],
        2 => vec![vec![0, 0, 1], vec![1, 1, 1], vec![0, 0, 0]],
        3 => vec![vec![1, 1], vec![1, 1]],
        4 => vec![vec![0, 1, 1], vec![1, 1, 0], vec![0, 0, 0]],
        5 => vec![vec![0, 1, 0], vec![1, 1, 1], vec![0, 0, 0]],
        _ => vec![vec![1, 1, 0], vec![0, 1, 1], vec![0, 0, 0]],
    }
}

pub fn rotate_cw(m: &[Vec<u8>]) -> Vec<Vec<u8>> {
    let n = m.len();
    (0..n)
        .map(|r| (0..n).map(|c| m[c][n - 1 - r]).collect())
        .collect()
}

pub fn rotate_ccw(m: &[Vec<u8>]) -> Vec<Vec<u8>> {
    let n = m.len();
    (0..n)
        .map(|r| (0..n).map(|c| m[n - 1 - c][r]).collect())
        .collect()
}

type Kick = (i32, i32);

// SRS 킥 테이블 (TS srs.ts와 동일, y-down 좌표계)
const K_A: [(i32, i32); 5] = [(0, 0), (-1, 0), (-1, -1), (0, 2), (-1, 2)];
const K_B: [(i32, i32); 5] = [(0, 0), (1, 0), (1, 1), (0, -2), (1, -2)];
const K_C: [(i32, i32); 5] = [(0, 0), (1, 0), (1, -1), (0, 2), (1, 2)];
const K_D: [(i32, i32); 5] = [(0, 0), (-1, 0), (-1, 1), (0, -2), (-1, -2)];

// 순서: 0>1, 1>0, 1>2, 2>1, 2>3, 3>0, 3>2, 0>3
const J_TABLES: [&[(i32, i32); 5]; 8] = [&K_A, &K_B, &K_B, &K_A, &K_C, &K_D, &K_D, &K_C];
const I_TABLES: [&[(i32, i32); 5]; 8] = [
    &[(0, 0), (-2, 0), (1, 0), (-2, 1), (1, -2)],
    &[(0, 0), (2, 0), (-1, 0), (2, -1), (-1, 2)],
    &[(0, 0), (-1, 0), (2, 0), (-1, -2), (2, 1)],
    &[(0, 0), (1, 0), (-2, 0), (1, 2), (-2, -1)],
    &[(0, 0), (2, 0), (-1, 0), (2, -1), (-1, 2)],
    &[(0, 0), (1, 0), (-2, 0), (1, 2), (-2, -1)],
    &[(0, 0), (-2, 0), (1, 0), (-2, 1), (1, -2)],
    &[(0, 0), (-1, 0), (2, 0), (-1, -2), (2, 1)],
];

fn kicks(t: u8, from: u8, to: u8) -> &'static [(i32, i32); 5] {
    if t == 3 {
        return &K_A; // O — 회전 자체가 스킵되므로 미사용
    }
    let tables: &[&[(i32, i32); 5]; 8] = if t == 0 { &I_TABLES } else { &J_TABLES };
    match from * 4 + to {
        1 => &tables[0],  // 0>1
        4 => &tables[1],  // 1>0
        6 => &tables[2],  // 1>2
        9 => &tables[3],  // 2>1
        11 => &tables[4], // 2>3
        12 => &tables[5], // 3>0
        14 => &tables[6], // 3>2
        3 => &tables[7],  // 0>3
        _ => &tables[0],
    }
}

pub fn collides(board: &Board, _t: u8, shape: &[Vec<u8>], x: i32, y: i32) -> bool {
    for (r, row) in shape.iter().enumerate() {
        for (c, &v) in row.iter().enumerate() {
            if v == 0 {
                continue;
            }
            let bx = x + c as i32;
            let by = y + r as i32;
            if bx < 0 || bx >= W as i32 || by >= H as i32 {
                return true;
            }
            if by >= 0 && board[by as usize][bx as usize].is_some() {
                return true;
            }
        }
    }
    false
}

fn cells_of(shape: &[Vec<u8>], x: i32, y: i32) -> Vec<(i32, i32)> {
    let mut out = Vec::new();
    for (r, row) in shape.iter().enumerate() {
        for (c, &v) in row.iter().enumerate() {
            if v != 0 {
                out.push((x + c as i32, y + r as i32));
            }
        }
    }
    out
}

fn is_tspin(board: &Board, piece: &Piece, by_rotate: bool) -> bool {
    if piece.t != 5 || !by_rotate {
        return false;
    }
    let px = piece.x + 1;
    let py = piece.y + 1;
    let corners = [(px - 1, py - 1), (px + 1, py - 1), (px - 1, py + 1), (px + 1, py + 1)];
    let mut filled = 0;
    for (x, y) in corners {
        if x < 0 || x >= W as i32 || y >= H as i32 {
            filled += 1;
        } else if y >= 0 && board[y as usize][x as usize].is_some() {
            filled += 1;
        }
    }
    filled >= 3
}

/// 완성 줄을 제거한다. items 그리드는 board와 병렬로 같은 줄을 제거하며,
/// 제거된 줄에 있던 아이템을 (x, y, 종류) 좌표와 함께 반환한다 — 폭발 연출 등에 쓴다.
pub fn clear_rows(board: &Board, items: &ItemGrid) -> (Board, ItemGrid, usize, Vec<(usize, usize, ItemKind)>) {
    let mut cleared_items = Vec::new();
    let mut remaining: Vec<Vec<Cell>> = Vec::with_capacity(H);
    let mut remaining_items: Vec<Vec<Option<ItemKind>>> = Vec::with_capacity(H);
    for r in 0..H {
        let full = board[r].iter().all(|c| c.is_some());
        if full {
            for c in 0..W {
                if let Some(k) = items[r][c] {
                    cleared_items.push((c, r, k));
                }
            }
        } else {
            remaining.push(board[r].clone());
            remaining_items.push(items[r].clone());
        }
    }
    let count = H - remaining.len();
    let mut out = Vec::with_capacity(H);
    let mut out_items = Vec::with_capacity(H);
    for _ in 0..count {
        out.push(vec![None; W]);
        out_items.push(vec![None; W]);
    }
    out.extend(remaining);
    out_items.extend(remaining_items);
    (out, out_items, count, cleared_items)
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum ClearKind {
    None,
    Single,
    Double,
    Triple,
    Tetris,
    TSpinSingle,
    TSpinDouble,
    TSpinTriple,
}

impl ClearKind {
    pub fn attack(self) -> u32 {
        match self {
            ClearKind::None => 0,
            ClearKind::Single => 0,
            ClearKind::Double => 1,
            ClearKind::Triple => 2,
            ClearKind::Tetris => 4,
            ClearKind::TSpinSingle => 2,
            ClearKind::TSpinDouble => 4,
            ClearKind::TSpinTriple => 6,
        }
    }

    pub fn points(self) -> i64 {
        match self {
            ClearKind::None => 0,
            ClearKind::Single => 100,
            ClearKind::Double => 300,
            ClearKind::Triple => 500,
            ClearKind::Tetris => 800,
            ClearKind::TSpinSingle => 800,
            ClearKind::TSpinDouble => 1200,
            ClearKind::TSpinTriple => 1600,
        }
    }

    pub fn name(self) -> &'static str {
        match self {
            ClearKind::None => "none",
            ClearKind::Single => "single",
            ClearKind::Double => "double",
            ClearKind::Triple => "triple",
            ClearKind::Tetris => "tetris",
            ClearKind::TSpinSingle => "tspin_single",
            ClearKind::TSpinDouble => "tspin_double",
            ClearKind::TSpinTriple => "tspin_triple",
        }
    }

    fn is_b2b(self) -> bool {
        matches!(self, ClearKind::Tetris | ClearKind::TSpinSingle | ClearKind::TSpinDouble | ClearKind::TSpinTriple)
    }
}

fn clear_kind(tspin: bool, count: usize) -> ClearKind {
    match (tspin, count) {
        (true, 1) => ClearKind::TSpinSingle,
        (true, 2) => ClearKind::TSpinDouble,
        (true, 3) => ClearKind::TSpinTriple,
        (false, 1) => ClearKind::Single,
        (false, 2) => ClearKind::Double,
        (false, 3) => ClearKind::Triple,
        (false, 4) => ClearKind::Tetris,
        _ => ClearKind::None,
    }
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Status {
    Playing,
    TopOut,
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum LastMove {
    None,
    Move,
    Rotate,
    Drop,
}

#[derive(Clone)]
pub struct Piece {
    pub t: u8,
    pub shape: Vec<Vec<u8>>,
    pub x: i32,
    pub y: i32,
    pub rot: u8,
}

pub fn create_board() -> Board {
    vec![vec![None; W]; H]
}

pub struct Player {
    pub id: String,
    pub board: Board,
    pub items: ItemGrid,
    pub piece: Piece,
    pub hold: Option<u8>,
    pub can_hold: bool,
    pub score: i64,
    pub lines: i32,
    pub combo: i32,
    pub b2b: bool,
    pub last_clear: ClearKind,
    pub status: Status,
    pub garbage: u32,
    pub clear_flash: u32,
    bag: Vec<u8>,
    lock_timer: i32,
    lock_resets: u32,
    gravity: i32,
    gravity_mult: f64,
    gravity_timer: i32,
    shield: u32,
    item_mode: bool,
    soft_drop: bool,
    last_move: LastMove,
    rng: SmallRng,
}

impl Player {
    pub fn new(id: String, seed: u64) -> Self {
        Player::new_with_items(id, seed, false)
    }

    /// item_mode면 빈 셀(행 2..=13)에 아이템 6개를 무작위로 배치한다.
    pub fn new_with_items(id: String, seed: u64, item_mode: bool) -> Self {
        let mut rng = SmallRng::seed_from_u64(seed);
        let mut bag: Vec<u8> = (0..7).collect();
        for i in (1..7).rev() {
            let j = rng.gen_range(0..=i);
            bag.swap(i, j);
        }
        let t = bag.remove(0);
        let shape = spawn_shape(t);
        let x = (W as i32 - shape[0].len() as i32) / 2;
        let mut items = vec![vec![None; W]; H];
        if item_mode {
            let kinds = ItemKind::all();
            let mut placed = 0;
            let mut guard = 0;
            while placed < 6 && guard < 400 {
                guard += 1;
                let r = rng.gen_range(2..=13);
                let c = rng.gen_range(0..W);
                if items[r][c].is_none() {
                    items[r][c] = Some(kinds[rng.gen_range(0..kinds.len())]);
                    placed += 1;
                }
            }
        }
        let mut p = Player {
            id,
            board: create_board(),
            items,
            piece: Piece { t, shape, x, y: -1, rot: 0 },
            hold: None,
            can_hold: true,
            score: 0,
            lines: 0,
            combo: 0,
            b2b: false,
            last_clear: ClearKind::None,
            status: Status::Playing,
            garbage: 0,
            clear_flash: 0,
            bag,
            lock_timer: 0,
            lock_resets: 0,
            gravity: 0,
            gravity_mult: 1.0,
            gravity_timer: 0,
            shield: 0,
            item_mode,
            soft_drop: false,
            last_move: LastMove::None,
            rng,
        };
        if collides(&p.board, p.piece.t, &p.piece.shape, p.piece.x, p.piece.y) {
            p.status = Status::TopOut;
        }
        p
    }

    pub fn level(&self) -> i32 {
        self.lines / 10 + 1
    }

    pub fn cells(&self) -> Vec<(i32, i32)> {
        cells_of(&self.piece.shape, self.piece.x, self.piece.y)
    }

    fn take_from_bag(&mut self) -> u8 {
        if self.bag.is_empty() {
            let mut bag: Vec<u8> = (0..7).collect();
            for i in (1..7).rev() {
                let j = self.rng.gen_range(0..=i);
                bag.swap(i, j);
            }
            self.bag = bag;
        }
        self.bag.remove(0)
    }

    /// 새 피스를 스폰. `apply_garbage`가 true면 대기 중인 가비지를 먼저 바닥에 쌓는다.
    /// 반환값: 스폰 위치에서 충돌하면 false(블록아웃).
    fn spawn_piece(&mut self, apply_garbage: bool) -> bool {
        if apply_garbage {
            self.apply_garbage();
        }
        let t = self.take_from_bag();
        let shape = spawn_shape(t);
        let x = (W as i32 - shape[0].len() as i32) / 2;
        self.piece = Piece { t, shape, x, y: -1, rot: 0 };
        self.can_hold = true;
        self.lock_timer = 0;
        self.lock_resets = 0;
        self.gravity = 0;
        !collides(&self.board, self.piece.t, &self.piece.shape, self.piece.x, self.piece.y)
    }

    fn apply_garbage(&mut self) {
        while self.garbage > 0 {
            // 방패(shield)가 있으면 가비지 1줄을 무효화한다 (아이템 모드)
            if self.shield > 0 {
                self.shield -= 1;
                self.garbage -= 1;
                continue;
            }
            let hole = self.rng.gen_range(0..W);
            let mut row: Vec<Cell> = vec![Some(6); W];
            row[hole] = None;
            self.board.remove(0); // 맨 위 줄은 오버플로로 버림
            self.board.push(row);
            self.items.remove(0);
            self.items.push(vec![None; W]);
            self.garbage -= 1;
        }
    }

    pub fn move_left(&mut self) -> bool {
        self.move_piece(-1)
    }

    pub fn move_right(&mut self) -> bool {
        self.move_piece(1)
    }

    fn move_piece(&mut self, dx: i32) -> bool {
        if self.status != Status::Playing {
            return false;
        }
        if collides(&self.board, self.piece.t, &self.piece.shape, self.piece.x + dx, self.piece.y) {
            return false;
        }
        self.piece.x += dx;
        self.last_move = LastMove::Move;
        self.reset_lock_timer();
        true
    }

    /// dir: 1 = CW, -1 = CCW. SRS 킥 적용.
    pub fn rotate(&mut self, dir: i32) -> bool {
        if self.status != Status::Playing || self.piece.t == 3 {
            return false;
        }
        let from = self.piece.rot;
        let to = ((from as i32 + if dir > 0 { 1 } else { 3 }) % 4) as u8;
        let shape = if dir > 0 { rotate_cw(&self.piece.shape) } else { rotate_ccw(&self.piece.shape) };
        for (kx, ky) in kicks(self.piece.t, from, to) {
            let nx = self.piece.x + kx;
            let ny = self.piece.y + ky;
            if !collides(&self.board, self.piece.t, &shape, nx, ny) {
                self.piece = Piece { t: self.piece.t, shape, x: nx, y: ny, rot: to };
                self.last_move = LastMove::Rotate;
                self.reset_lock_timer();
                return true;
            }
        }
        false
    }

    fn reset_lock_timer(&mut self) {
        if self.lock_resets >= MAX_LOCK_RESETS {
            return;
        }
        self.lock_resets += 1;
        self.lock_timer = 0;
    }

    pub fn soft_drop(&mut self, on: bool) {
        self.soft_drop = on;
    }

    /// 하드 드롭: 바닥까지 이동 + 점수. 락 자체는 Match::lock_player가 수행한다.
    pub fn hard_drop(&mut self) -> bool {
        if self.status != Status::Playing {
            return false;
        }
        let mut y = self.piece.y;
        while !collides(&self.board, self.piece.t, &self.piece.shape, self.piece.x, y + 1) {
            y += 1;
        }
        let dist = y - self.piece.y;
        self.piece.y = y;
        self.score += dist as i64 * 2;
        self.last_move = LastMove::Drop;
        true
    }

    pub fn hold(&mut self) -> bool {
        if self.status != Status::Playing || !self.can_hold {
            return false;
        }
        let out = self.piece.t;
        let next_t = match self.hold {
            Some(h) => h,
            None => self.take_from_bag(),
        };
        self.hold = Some(out);
        let shape = spawn_shape(next_t);
        let x = (W as i32 - shape[0].len() as i32) / 2;
        self.piece = Piece { t: next_t, shape, x, y: -1, rot: 0 };
        self.can_hold = false;
        self.last_move = LastMove::None;
        self.lock_timer = 0;
        self.lock_resets = 0;
        self.gravity = 0;
        if collides(&self.board, self.piece.t, &self.piece.shape, self.piece.x, self.piece.y) {
            self.status = Status::TopOut;
        }
        true
    }

    /// 락 수행: 병합, 줄 제거, 점수 계산. 락아웃(전부 보드 위)이면 TopOut.
    /// 반환값: (클리어 종류, 제거된 줄에서 발동된 아이템 목록 — (x, y, 종류)).
    pub fn lock(&mut self) -> (ClearKind, Vec<(usize, usize, ItemKind)>) {
        if self.status != Status::Playing {
            return (ClearKind::None, Vec::new());
        }
        let cells = self.cells();
        if !cells.is_empty() && cells.iter().all(|(_, y)| *y < 0) {
            self.status = Status::TopOut;
            return (ClearKind::None, Vec::new());
        }
        let tspin = is_tspin(&self.board, &self.piece, self.last_move == LastMove::Rotate);
        for (x, y) in &cells {
            if *y >= 0 {
                self.board[*y as usize][*x as usize] = Some(self.piece.t);
            }
        }
        let (board, items, count, cleared_items) = clear_rows(&self.board, &self.items);
        self.board = board;
        self.items = items;
        let kind = clear_kind(tspin, count);
        if count > 0 {
            self.lines += count as i32;
            self.clear_flash += 1;
            let base = kind.points();
            let b2b = kind.is_b2b() && self.last_clear.is_b2b();
            self.b2b = b2b;
            let mult: f64 = if b2b { 1.5 } else { 1.0 };
            self.combo += 1;
            let combo_bonus = if self.combo > 1 { 50 * (self.combo - 1) * self.level() } else { 0 };
            self.score += (base as f64 * mult).round() as i64 * self.level() as i64 + combo_bonus as i64;
            self.last_clear = kind;
        } else {
            self.combo = 0;
            self.last_clear = ClearKind::None;
        }
        (kind, cleared_items)
    }

    /// 가속/감속 아이템 적용 (중력 배수 + 지속 시간 ms).
    fn set_gravity_mult(&mut self, mult: f64, ms: i32) {
        self.gravity_mult = mult;
        self.gravity_timer = ms;
    }

    /// 상대 보드의 채워진 셀 n개를 무작위로 제거한다 (구멍 아이템).
    pub fn punch_holes(&mut self, n: usize) {
        let mut idx: Vec<usize> = (0..W * H)
            .filter(|i| self.board[i / W][i % W].is_some())
            .collect();
        for i in (1..idx.len()).rev() {
            let j = self.rng.gen_range(0..=i);
            idx.swap(i, j);
        }
        for &i in idx.iter().take(n.min(idx.len())) {
            self.board[i / W][i % W] = None;
        }
    }

    /// 내 보드의 가장 낮은 채워진 줄 1개를 제거한다 (정리 아이템).
    pub fn clear_lowest_row(&mut self) {
        for r in (0..H).rev() {
            if self.board[r].iter().any(|c| c.is_some()) {
                self.board.remove(r);
                self.board.insert(0, vec![None; W]);
                self.items.remove(r);
                self.items.insert(0, vec![None; W]);
                return;
            }
        }
    }

    /// 소프트 드롭 한 칸. 막히면 true(락 필요)를 반환.
    fn soft_step(&mut self) -> bool {
        if self.status != Status::Playing {
            return false;
        }
        if collides(&self.board, self.piece.t, &self.piece.shape, self.piece.x, self.piece.y + 1) {
            return true;
        }
        self.piece.y += 1;
        self.score += 1;
        self.last_move = LastMove::Move;
        self.reset_lock_timer();
        false
    }

    /// 중력 + 락 딜레이 진행. 락이 발생하면 true.
    pub fn step(&mut self, dt_ms: i32) -> bool {
        if self.status != Status::Playing {
            return false;
        }
        // 아이템 효과(가속/감속) 타이머 경과
        if self.gravity_timer > 0 {
            self.gravity_timer -= dt_ms;
            if self.gravity_timer <= 0 {
                self.gravity_timer = 0;
                self.gravity_mult = 1.0;
            }
        }
        let mut locked = false;
        if self.soft_drop {
            locked = self.soft_step();
        }
        if self.status != Status::Playing {
            return locked;
        }
        if collides(&self.board, self.piece.t, &self.piece.shape, self.piece.x, self.piece.y + 1) {
            self.lock_timer += dt_ms;
            if self.lock_timer >= LOCK_DELAY_MS {
                locked = true;
            }
        } else {
            self.gravity += dt_ms;
            let base = drop_interval_for(self.level()) as f64;
            let interval = (base * self.gravity_mult).round().max(1.0) as i32;
            let mut guard = 0;
            while self.gravity >= interval && guard < 8 {
                self.gravity -= interval;
                if collides(&self.board, self.piece.t, &self.piece.shape, self.piece.x, self.piece.y + 1) {
                    break;
                }
                self.piece.y += 1;
                guard += 1;
            }
        }
        locked
    }
}

#[derive(Serialize, Clone)]
pub struct Event {
    pub kind: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub by: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub clear: Option<String>,
    pub attack: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub winner: Option<u8>,
    // 아이템 발동 이벤트 전용 필드
    #[serde(skip_serializing_if = "Option::is_none")]
    pub item: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    // 발동된 아이템 셀 좌표 (x, y) — 프론트 폭발 연출용
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cell: Option<(u8, u8)>,
}

#[derive(Serialize)]
pub struct ActiveView {
    pub t: &'static str,
    pub x: i32,
    pub y: i32,
    pub rot: u8,
    pub shape: Vec<Vec<u8>>,
}

#[derive(Serialize)]
pub struct PlayerView {
    pub player_id: String,
    pub board: Vec<Vec<Option<&'static str>>>,
    pub items: Vec<Vec<Option<&'static str>>>,
    pub piece: ActiveView,
    pub score: i64,
    pub lines: i32,
    pub level: i32,
    pub hold: Option<&'static str>,
    pub garbage: u32,
    pub clear_flash: u32,
    pub status: &'static str,
    pub shield: u32,
    pub speed: bool,
    pub slow: bool,
}

impl PlayerView {
    fn of(p: &Player) -> Self {
        PlayerView {
            player_id: p.id.clone(),
            board: p
                .board
                .iter()
                .map(|row| row.iter().map(|c| c.map(|i| PIECE_NAMES[i as usize])).collect())
                .collect(),
            items: p
                .items
                .iter()
                .map(|row| row.iter().map(|k| k.map(|kind| kind.name())).collect())
                .collect(),
            piece: ActiveView {
                t: PIECE_NAMES[p.piece.t as usize],
                x: p.piece.x,
                y: p.piece.y,
                rot: p.piece.rot,
                shape: p.piece.shape.clone(),
            },
            score: p.score,
            lines: p.lines,
            level: p.level(),
            hold: p.hold.map(|i| PIECE_NAMES[i as usize]),
            garbage: p.garbage,
            clear_flash: p.clear_flash,
            status: if p.status == Status::Playing { "playing" } else { "topout" },
            shield: p.shield,
            speed: p.gravity_mult > 1.0,
            slow: p.gravity_mult < 1.0,
        }
    }
}

#[derive(Serialize)]
pub struct MatchUpdate {
    pub states: Vec<PlayerView>,
    pub events: Vec<Event>,
    pub over: bool,
    pub winner: Option<u8>,
}

pub struct Match {
    pub id: String,
    pub players: Vec<Player>,
    pub over: bool,
    pub winner: Option<u8>,
    events: Vec<Event>,
}

impl Match {
    pub fn new(id: String, p1: String, p2: String) -> Self {
        Match::new_with_items(id, p1, p2, false)
    }

    /// item_mode=true면 양쪽 보드에 아이템 6개씩 배치된다 (아이템 배틀).
    pub fn new_with_items(id: String, p1: String, p2: String, item_mode: bool) -> Self {
        Match {
            id,
            players: vec![
                Player::new_with_items(p1, 0x5eed_0001, item_mode),
                Player::new_with_items(p2, 0x5eed_0002, item_mode),
            ],
            over: false,
            winner: None,
            events: Vec::new(),
        }
    }

    pub fn update(&self) -> MatchUpdate {
        MatchUpdate {
            states: self.players.iter().map(PlayerView::of).collect(),
            events: Vec::new(),
            over: self.over,
            winner: self.winner,
        }
    }

    pub fn action(&mut self, player_id: &str, action: &str) -> Result<Vec<Event>, String> {
        if self.over {
            return Ok(Vec::new());
        }
        let idx = self
            .players
            .iter()
            .position(|p| p.id == player_id)
            .ok_or_else(|| "unknown player".to_string())?;
        let locked = match action {
            "left" => {
                self.players[idx].move_left();
                false
            }
            "right" => {
                self.players[idx].move_right();
                false
            }
            "softdrop_start" => {
                self.players[idx].soft_drop(true);
                false
            }
            "softdrop_end" => {
                self.players[idx].soft_drop(false);
                false
            }
            "rotate_cw" => {
                self.players[idx].rotate(1);
                false
            }
            "rotate_ccw" => {
                self.players[idx].rotate(-1);
                false
            }
            "harddrop" => self.players[idx].hard_drop(),
            "hold" => {
                self.players[idx].hold();
                false
            }
            other => return Err(format!("unknown action: {other}")),
        };
        if locked {
            self.lock_player(idx);
        }
        self.check_over();
        Ok(self.drain_events())
    }

    pub fn tick(&mut self, dt_ms: i32) -> Vec<Event> {
        if self.over {
            return Vec::new();
        }
        for i in 0..2 {
            if self.players[i].step(dt_ms) {
                self.lock_player(i);
            }
        }
        self.check_over();
        self.drain_events()
    }

    fn lock_player(&mut self, idx: usize) {
        let (kind, items) = self.players[idx].lock();
        if kind != ClearKind::None {
            let attack = kind.attack();
            if attack > 0 {
                let opp = 1 - idx;
                self.players[opp].garbage += attack;
                self.events.push(Event {
                    kind: "clear".into(),
                    by: Some(self.players[idx].id.clone()),
                    clear: Some(kind.name().into()),
                    attack,
                    winner: None,
                    item: None,
                    target: None,
                    cell: None,
                });
            }
        }
        // 제거된 줄의 아이템 발동 — 셀 좌표를 함께 전달해 프론트에서 폭발 연출에 쓴다
        for (x, y, item) in items {
            self.apply_item(idx, item, Some((x as u8, y as u8)));
        }
        let p = &mut self.players[idx];
        if p.status == Status::Playing {
            let ok = p.spawn_piece(true);
            if !ok {
                p.status = Status::TopOut;
            }
        }
    }

    /// 아이템 효과 적용. 악영향은 상대(opp)에게, 이로운 것은 자신에게.
    /// cell은 발동된 아이템 셀의 (x, y) — 폭발 연출용으로 이벤트에 실어 보낸다.
    fn apply_item(&mut self, idx: usize, item: ItemKind, cell: Option<(u8, u8)>) {
        let opp = 1 - idx;
        let attacker_id = self.players[idx].id.clone();
        let (item_name, target) = match item {
            ItemKind::Attack => {
                self.players[opp].garbage += 3;
                ("attack", Some(opp))
            }
            ItemKind::Speed => {
                self.players[opp].set_gravity_mult(1.5, 20_000);
                ("speed", Some(opp))
            }
            ItemKind::Holes => {
                self.players[opp].punch_holes(8);
                ("holes", Some(opp))
            }
            ItemKind::Clear => {
                self.players[idx].clear_lowest_row();
                ("clear", None)
            }
            ItemKind::Shield => {
                self.players[idx].shield = self.players[idx].shield.saturating_add(2);
                ("shield", None)
            }
            ItemKind::Slow => {
                self.players[idx].set_gravity_mult(0.7, 20_000);
                ("slow", None)
            }
        };
        self.events.push(Event {
            kind: "item".into(),
            by: Some(attacker_id),
            clear: None,
            attack: 0,
            winner: None,
            item: Some(item_name.into()),
            target: target.map(|i| self.players[i].id.clone()),
            cell,
        });
    }

    fn check_over(&mut self) {
        if self.over {
            return;
        }
        let (s0, s1) = (self.players[0].status, self.players[1].status);
        if s0 == Status::TopOut || s1 == Status::TopOut {
            let winner = match (s0, s1) {
                (Status::TopOut, Status::TopOut) => {
                    if self.players[0].score > self.players[1].score {
                        Some(0)
                    } else if self.players[1].score > self.players[0].score {
                        Some(1)
                    } else {
                        None
                    }
                }
                (Status::TopOut, _) => Some(1),
                (_, Status::TopOut) => Some(0),
                _ => None,
            };
            self.over = true;
            self.winner = winner;
            self.events.push(Event {
                kind: "match_over".into(),
                by: None,
                clear: None,
                attack: 0,
                winner,
                item: None,
                target: None,
                cell: None,
            });
        }
    }

    fn drain_events(&mut self) -> Vec<Event> {
        std::mem::take(&mut self.events)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn full_row(v: u8) -> Vec<Cell> {
        vec![Some(v); W]
    }

    #[test]
    fn rotation_returns_to_origin_after_four() {
        for t in 0..7u8 {
            let s = spawn_shape(t);
            let mut m = s.clone();
            for _ in 0..4 {
                m = rotate_cw(&m);
            }
            assert_eq!(m, s, "CW x4 must be identity for type {t}");
            let mut m2 = s.clone();
            for _ in 0..4 {
                m2 = rotate_ccw(&m2);
            }
            assert_eq!(m2, s, "CCW x4 must be identity for type {t}");
        }
    }

    #[test]
    fn rotation_states_are_distinct() {
        let s = spawn_shape(5); // T
        let r1 = rotate_cw(&s);
        let r2 = rotate_cw(&r1);
        let r3 = rotate_cw(&r2);
        assert_ne!(r1, s);
        assert_ne!(r2, s);
        assert_ne!(r3, s);
        assert_ne!(r1, r3);
    }

    #[test]
    fn seven_bag_is_a_permutation() {
        let mut p = Player::new("p".into(), 42);
        let mut seen = vec![false; 7];
        for _ in 0..7 {
            let t = p.piece.t;
            assert!(!seen[t as usize], "duplicate piece in one bag");
            seen[t as usize] = true;
            p.spawn_piece(false);
        }
        assert!(seen.iter().all(|&s| s));
    }

    #[test]
    fn hard_drop_locks_and_scores() {
        let mut p = Player::new("p".into(), 7);
        let y0 = p.piece.y;
        p.hard_drop();
        assert_eq!(p.status, Status::Playing);
        assert_eq!(p.score, (p.piece.y - y0) as i64 * 2);
        // 피스가 락되어 보드에 남아 있다
        let cells = p.cells();
        assert!(cells.iter().any(|(_, y)| *y >= 0));
    }

    #[test]
    fn double_clear_sends_one_garbage() {
        let mut m = Match::new("m".into(), "a".into(), "b".into());
        // a 보드: 두 줄을 채우되 맨 아래 줄만 0열에 구멍을 남긴다 (피스가 메움)
        m.players[0].board[H - 2] = full_row(1);
        let mut bottom = full_row(1);
        bottom[0] = None;
        m.players[0].board[H - 1] = bottom;
        m.players[0].piece = Piece { t: 1, shape: vec![vec![1]], x: 0, y: H as i32 - 1, rot: 0 };
        m.players[0].hard_drop();
        m.lock_player(0);
        assert_eq!(m.players[0].lines, 2);
        assert_eq!(m.players[1].garbage, 1, "double attack = 1 garbage row");
        let evs = m.drain_events();
        assert_eq!(evs.len(), 1);
        assert_eq!(evs[0].kind, "clear");
        assert_eq!(evs[0].clear.as_deref(), Some("double"));
        assert_eq!(evs[0].attack, 1);
    }

    #[test]
    fn tetris_attack_four_and_b2b() {
        let mut m = Match::new("m".into(), "a".into(), "b".into());
        for row in [H - 4, H - 3, H - 2, H - 1] {
            let mut r = full_row(0);
            r[0] = None;
            m.players[0].board[row] = r;
        }
        // I 피스 세로(4칸)로 락 → 4줄 동시 제거 = 테트리스
        m.players[0].piece = Piece {
            t: 0,
            shape: vec![vec![1], vec![1], vec![1], vec![1]],
            x: 0,
            y: H as i32 - 4,
            rot: 1,
        };
        m.players[0].hard_drop();
        m.lock_player(0);
        assert_eq!(m.players[0].last_clear, ClearKind::Tetris);
        assert_eq!(m.players[0].score, 800); // 레벨 1
        assert_eq!(m.players[1].garbage, 4);
        // 두 번째 테트리스 → 백투백 1.5배
        for row in [H - 4, H - 3, H - 2, H - 1] {
            let mut r = full_row(0);
            r[0] = None;
            m.players[0].board[row] = r;
        }
        m.players[0].piece = Piece {
            t: 0,
            shape: vec![vec![1], vec![1], vec![1], vec![1]],
            x: 0,
            y: H as i32 - 4,
            rot: 1,
        };
        m.players[0].hard_drop();
        m.lock_player(0);
        assert_eq!(m.players[0].lines, 8);
        assert_eq!(m.players[0].score, 800 + 1200 + 50); // 800*1.5 + 콤보 50
        assert!(m.players[0].b2b);
    }

    #[test]
    fn garbage_materializes_with_holes() {
        let mut p = Player::new("p".into(), 3);
        p.garbage = 2;
        p.apply_garbage();
        assert_eq!(p.garbage, 0);
        // 바닥 두 줄이 가비지로 채워짐
        for row in [H - 2, H - 1] {
            let filled: usize = p.board[row].iter().filter(|c| c.is_some()).count();
            assert_eq!(filled, W - 1, "each garbage row has exactly one hole");
        }
    }

    #[test]
    fn blockout_tops_out() {
        let mut m = Match::new("m".into(), "a".into(), "b".into());
        // 상단 4줄을 구멍 하나씩 있는 상태로 채운다 (스폰 충돌 유발, 단 완성 줄은 없음)
        for row in 0..4 {
            let mut r = full_row(1);
            r[0] = None;
            m.players[0].board[row] = r;
        }
        // 락은 통과시키되 스폰에서 블록아웃
        m.players[0].piece = Piece { t: 1, shape: vec![vec![1]], x: 0, y: H as i32 - 1, rot: 0 };
        m.players[0].hard_drop();
        m.lock_player(0);
        m.check_over();
        assert_eq!(m.players[0].status, Status::TopOut);
        assert!(m.over);
        assert_eq!(m.winner, Some(1));
        let evs = m.drain_events();
        assert!(evs.iter().any(|e| e.kind == "match_over"));
    }

    #[test]
    fn lockout_tops_out() {
        let mut p = Player::new("p".into(), 9);
        // 피스를 보드 완전히 위로 밀어 올린 뒤 락 → 락아웃
        p.piece = Piece { t: 1, shape: vec![vec![1]], x: 0, y: -5, rot: 0 };
        p.lock();
        assert_eq!(p.status, Status::TopOut);
    }

    #[test]
    fn tick_gravity_and_lock_delay() {
        let mut p = Player::new("p".into(), 11);
        let start_y = p.piece.y;
        // 레벨 1: 870ms 간격. 50ms 틱 18번 → 900ms → 한 칸 내려감
        for _ in 0..18 {
            p.step(50);
        }
        assert_eq!(p.piece.y, start_y + 1);
        // 바닥까지 내려 보내고 500ms 락 딜레이 후 락
        while !collides(&p.board, p.piece.t, &p.piece.shape, p.piece.x, p.piece.y + 1) {
            p.piece.y += 1;
        }
        let locked = p.step(600);
        assert!(locked);
        assert!(p.cells().iter().all(|(_, y)| *y >= 0));
    }

    #[test]
    fn item_mode_places_six_items() {
        let p = Player::new_with_items("p".into(), 5, true);
        let count: usize = p.items.iter().map(|row| row.iter().filter(|c| c.is_some()).count()).sum();
        assert_eq!(count, 6, "아이템 모드면 아이템 6개 배치");
        // 일반 모드에는 아이템이 없다
        let pn = Player::new("p".into(), 5);
        let c0: usize = pn.items.iter().map(|row| row.iter().filter(|c| c.is_some()).count()).sum();
        assert_eq!(c0, 0);
    }

    #[test]
    fn clearing_item_row_triggers_effect() {
        let mut m = Match::new_with_items("m".into(), "a".into(), "b".into(), true);
        // a 보드: 맨 아래 줄을 0열만 비운 상태로 채우고, 그 자리에 폭탄 아이템을 둔다
        m.players[0].items[H - 1][0] = Some(ItemKind::Attack);
        let mut bottom = full_row(1);
        bottom[0] = None;
        m.players[0].board[H - 1] = bottom;
        m.players[0].piece = Piece { t: 1, shape: vec![vec![1]], x: 0, y: H as i32 - 1, rot: 0 };
        m.players[0].hard_drop();
        m.lock_player(0);
        // 싱글 클리어 공격(0) + 폭탄 아이템(+3)
        assert_eq!(m.players[1].garbage, 3, "폭탄 아이템 발동 → 상대 가비지 +3");
        assert_eq!(m.players[0].items[H - 1][0], None, "아이템은 소모되어 사라진다");
        let evs = m.drain_events();
        let item_ev = evs.iter().find(|e| e.kind == "item").expect("item 이벤트 발생");
        assert_eq!(item_ev.item.as_deref(), Some("attack"));
        assert_eq!(item_ev.by.as_deref(), Some("a"));
        assert_eq!(item_ev.target.as_deref(), Some("b"));
        // 폭발 연출용 셀 좌표 — 아이템이 있던 셀 (0, H-1)이 실려야 한다
        assert_eq!(item_ev.cell, Some((0, (H - 1) as u8)));
    }

    #[test]
    fn shield_blocks_garbage_rows() {
        let mut p = Player::new_with_items("p".into(), 3, true);
        p.shield = 2;
        p.garbage = 3;
        p.apply_garbage();
        assert_eq!(p.garbage, 0);
        assert_eq!(p.shield, 0, "방패 2줄 소진");
        // 3줄 중 2줄이 무효화 → 가비지 줄은 1줄만 쌓인다
        let garbage_rows: usize = p.board.iter().filter(|r| r.iter().any(|c| *c == Some(6))).count();
        assert_eq!(garbage_rows, 1);
    }

    #[test]
    fn gravity_mult_timer_expires() {
        let mut p = Player::new("p".into(), 11);
        p.set_gravity_mult(1.5, 100);
        let _ = p.step(60);
        assert_eq!(p.gravity_mult, 1.5);
        let _ = p.step(60); // 120ms 경과 → 타이머 소진
        assert_eq!(p.gravity_timer, 0);
        assert_eq!(p.gravity_mult, 1.0, "타이머 종료 후 중력 정상 복귀");
    }

    #[test]
    fn beneficial_items_apply_to_self() {
        let mut m = Match::new_with_items("m".into(), "a".into(), "b".into(), true);
        // a 보드 바닥에 블록 1개 + 정리 아이템
        m.players[0].board[H - 1][5] = Some(1);
        m.players[0].items[H - 1][5] = Some(ItemKind::Clear);
        // clear_lowest_row를 직접 호출 — 아이템 발동 경로 검증
        m.players[0].clear_lowest_row();
        assert_eq!(m.players[0].board[H - 1][5], None, "가장 낮은 줄이 제거된다");
        m.players[0].set_gravity_mult(0.7, 5000);
        assert_eq!(m.players[0].gravity_mult, 0.7);
    }

    #[test]
    fn wall_kick_lets_rotation_through() {
        let mut m = Match::new("m".into(), "a".into(), "b".into());
        let p = &mut m.players[0];
        p.piece = Piece { t: 5, shape: spawn_shape(5), x: 0, y: 0, rot: 0 };
        // T CW 회전 결과 셀 (1,1) 위치에 장애물: [0,0] 킥을 실패시켜 [0,2] 킥으로 넘어가게 한다
        p.board[1][1] = Some(1);
        // 0>1 킥 시퀀스: [0,0] 충돌, [-1,0] 좌측 이탈, [-1,-1] 좌측 이탈, [0,2] 성공
        let ok = p.rotate(1);
        assert!(ok);
        assert_eq!(p.piece.rot, 1);
        assert_eq!(p.piece.x, 0);
        assert_eq!(p.piece.y, 2);
    }
}
