package referee

import (
	"encoding/json"
	"math/rand"
	"sort"
)

// 테트리스 CPU 봇 — 서버 권위 상태를 보고 매 틱 한 액션씩 보낸다.
// 전략: 현재 피스의 4회전 × 전체 x에 대해 착지 후 보드를 평가해 최적 목표를 정하고,
// 회전/이동/하드드롭으로 스티어링한다. 가끔(8%) 최적이 아닌 수를 둬서 이길 수 있게 한다.

const botBoardW = 10
const botBoardH = 20

// botThinkSteps — 피스마다의 "생각 시간"(틱 수). 바로 하드드롭하면 1피스당 ~0.2초로
// 스택이 순식간에 차서 탑아웃된다. 사람처럼 ~1.5초씩 두도록 중력에 맡긴다.
const botThinkSteps = 30

type botState struct {
	Board [][]*string `json:"board"`
	Piece struct {
		T     string  `json:"t"`
		X     int     `json:"x"`
		Y     int     `json:"y"`
		Rot   int     `json:"rot"`
		Shape [][]int `json:"shape"`
	} `json:"piece"`
	Status string `json:"status"`
}

type TetrisBot struct {
	lastT     string // 목표를 계산한 피스
	targetX   int
	targetRot int
	steps     int
}

func NewTetrisBot() *TetrisBot {
	return &TetrisBot{targetX: -999, targetRot: -1}
}

func strPtr(s string) *string { return &s }

// Action — 게임 상태를 보고 다음 액션을 하나 돌려준다 (battle.Bot 구현).
func (b *TetrisBot) Action(state json.RawMessage) *string {
	var s botState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil
	}
	if s.Status != "playing" {
		return nil
	}
	p := &s.Piece

	// 새 피스가 스폰되면 최적 착지 목표를 다시 계산한다.
	if p.T != b.lastT {
		b.lastT = p.T
		b.steps = 0
		if mv := bestBotMove(s.Board, p.Shape); mv != nil {
		b.targetRot, b.targetX = mv.rot, mv.x
		} else {
			b.targetRot, b.targetX = p.Rot, p.X // 착지 불가 → 그냥 그 자리에서 드롭
		}
	}
	b.steps++

	// 생각 시간 — 중력에 맡겨 사람 페이스로 플레이한다.
	if b.steps <= botThinkSteps {
		return nil
	}
	// 너무 오래 헤매면(벽에 막힘 등) 강제 드롭으로 진행.
	if b.steps > botThinkSteps+25 {
		b.lastT = ""
		return strPtr("harddrop")
	}
	// 스티어링 — 한 번에 한 액션.
	if p.Rot != b.targetRot {
		return strPtr("rotate_cw")
	}
	if p.X < b.targetX {
		return strPtr("right")
	}
	if p.X > b.targetX {
		return strPtr("left")
	}
	return strPtr("harddrop")
}

type botMove struct {
	rot, x int
	score  float64
}

// rotateCW — Rust 엔진의 rotate_cw와 동일한 표준 시계방향 회전 (정사각 행렬).
func rotateCW(m [][]int) [][]int {
	n := len(m)
	out := make([][]int, n)
	for r := 0; r < n; r++ {
		out[r] = make([]int, n)
		for c := 0; c < n; c++ {
			out[r][c] = m[c][n-1-r]
		}
	}
	return out
}

func botCellsOf(shape [][]int) [][2]int {
	var cells [][2]int
	for r, row := range shape {
		for c, v := range row {
			if v != 0 {
				cells = append(cells, [2]int{c, r})
			}
		}
	}
	return cells
}

func botCollides(board [][]*string, cells [][2]int, x, y int) bool {
	for _, cell := range cells {
		bx, by := x+cell[0], y+cell[1]
		if bx < 0 || bx >= botBoardW || by >= botBoardH {
			return true
		}
		if by >= 0 && board[by][bx] != nil {
			return true
		}
	}
	return false
}

// landingY — (x, cells)로 떨어뜨렸을 때 착지 y. 실패하면 ok=false.
func landingY(board [][]*string, cells [][2]int, x int) (int, bool) {
	y := -1
	for i := 0; i < botBoardH+4; i++ {
		if botCollides(board, cells, x, y+1) {
			return y, true
		}
		y++
	}
	return y, false
}

// evaluate — 착지 후 보드 형상을 평가한다. 보드 위로 삐져나오면 valid=false.
func evaluate(board [][]*string, cells [][2]int, x, y int) (float64, bool) {
	sim := make([][]*string, botBoardH)
	for r := range board {
		row := make([]*string, botBoardW)
		copy(row, board[r])
		sim[r] = row
	}
	landingCols := map[int]bool{}
	for _, cell := range cells {
		bx, by := x+cell[0], y+cell[1]
		if by < 0 || bx < 0 || bx >= botBoardW {
			return 0, false
		}
		sim[by][bx] = strPtr("X")
		landingCols[bx] = true
	}

	cleared := 0
	colHeight := make([]int, botBoardW)
	for c := 0; c < botBoardW; c++ {
		height := 0
		for r := 0; r < botBoardH; r++ {
			if sim[r][c] != nil {
				height = botBoardH - r // 바닥에서부터의 높이
			}
		}
		colHeight[c] = height
	}
	for r := 0; r < botBoardH; r++ {
		full := true
		for c := 0; c < botBoardW; c++ {
			if sim[r][c] == nil {
				full = false
				break
			}
		}
		if full {
			cleared++
		}
	}
	holes := 0
	for c := 0; c < botBoardW; c++ {
		seen := false
		for r := 0; r < botBoardH; r++ {
			if sim[r][c] != nil {
				seen = true
			} else if seen {
				holes++
			}
		}
	}
	agg, maxH, bump, landingHeight := 0, 0, 0, 0
	for c := 0; c < botBoardW; c++ {
		agg += colHeight[c]
		if colHeight[c] > maxH {
			maxH = colHeight[c]
		}
		if landingCols[c] {
			landingHeight += colHeight[c] // 착지한 열의 높이 — 높은 벽에 계속 얹는 걸 막는다
		}
		if c > 0 {
			d := colHeight[c] - colHeight[c-1]
			if d < 0 {
				d = -d
			}
			bump += d
		}
	}
	// 클리어를 크게 우대. 착지 높이/총높이/구멍에 벌점을 줘서 빈 열로 퍼뜨리며
	// 낮고 평평한 스택을 유지한다 (bump는 가볍게 — 강하면 벽에 계속 붙는다).
	return float64(1500*cleared) - float64(3*agg+15*maxH+10*holes+8*landingHeight+bump), true
}

// bestBotMove — 모든 회전×x 조합 중 최고 점수의 착지 목표를 찾는다.
func bestBotMove(board [][]*string, shape [][]int) *botMove {
	best := &botMove{score: -1e18}
	var moves []botMove
	sh := shape
	for rot := 0; rot < 4; rot++ {
		if rot > 0 {
			sh = rotateCW(sh)
		}
		cells := botCellsOf(sh)
		for x := -4; x < botBoardW; x++ {
			y, ok := landingY(board, cells, x)
			if !ok {
				continue
			}
			score, valid := evaluate(board, cells, x, y)
			if !valid {
				continue
			}
			mv := botMove{rot: rot, x: x, score: score}
			moves = append(moves, mv)
			if score > best.score {
				*best = mv
			}
		}
	}
	if len(moves) == 0 {
		return nil
	}
	if rand.Float64() < 0.06 { // 가끔 실수 — 이길 수 있게 (최상위 30% 안에서만)
		sort.Slice(moves, func(a, b int) bool { return moves[a].score > moves[b].score })
		n := len(moves) * 30 / 100
		if n < 1 {
			n = 1
		}
		m := moves[rand.Intn(n)]
		return &m
	}
	return best
}
