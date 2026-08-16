package store

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	pool *pgxpool.Pool
}

type MatchRow struct {
	ID         string
	Code       string
	Game       string
	Status     string
	WinnerID   *string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

type User struct {
	ID         string
	Name       string
	Password   string
	Email      string
	GoogleSub  string
}

type RoomRow struct {
	MatchID     string    `json:"match_id"`
	Code        string    `json:"code"`
	HostName    string    `json:"host_name"`
	PlayerCount int       `json:"player_count"`
	CreatedAt   time.Time `json:"created_at"`
	IsMine      bool      `json:"is_mine"`
}

func Connect(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) CreateMatch(ctx context.Context, matchID, code, game, playerID, name string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO matches (id, code, game) VALUES ($1, $2, $3)`, matchID, code, game); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO match_players (match_id, player_id, player_name) VALUES ($1, $2, $3)`,
		matchID, playerID, name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListRooms returns joinable (waiting) rooms for a game, newest first.
func (s *Store) ListRooms(ctx context.Context, game, userID string, limit int) ([]RoomRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.code, host.player_name, pc.cnt, m.created_at,
		       EXISTS(SELECT 1 FROM match_players mp2 WHERE mp2.match_id = m.id AND mp2.player_id = $2)
		FROM matches m
		JOIN LATERAL (
			SELECT player_name FROM match_players
			WHERE match_id = m.id
			ORDER BY joined_at ASC
			LIMIT 1
		) host ON true
		JOIN LATERAL (
			SELECT count(*) AS cnt FROM match_players WHERE match_id = m.id
		) pc ON true
		WHERE m.status = 'created' AND m.game = $1
		ORDER BY m.created_at DESC
		LIMIT $3`, game, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoomRow{}
	for rows.Next() {
		var r RoomRow
		if err := rows.Scan(&r.MatchID, &r.Code, &r.HostName, &r.PlayerCount, &r.CreatedAt, &r.IsMine); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// JoinMatch returns the match for a join code. joinErr is non-nil if the match
// cannot accept another player.
func (s *Store) JoinMatch(ctx context.Context, code, playerID, name string) (*MatchRow, error) {
	var m MatchRow
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT m.id, m.code, m.game, m.status, m.winner_id, m.created_at, m.finished_at,
		       (SELECT count(*) FROM match_players mp WHERE mp.match_id = m.id)
		FROM matches m WHERE m.code = $1`, code).
		Scan(&m.ID, &m.Code, &m.Game, &m.Status, &m.WinnerID, &m.CreatedAt, &m.FinishedAt, &count)
	if err != nil {
		return nil, fmt.Errorf("방 코드 %q를 찾을 수 없습니다", code)
	}
	if m.Status != "created" {
		return nil, fmt.Errorf("이미 시작되었거나 종료된 방입니다 (%s)", m.Status)
	}
	var already int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM match_players WHERE match_id = $1 AND player_id = $2`,
		m.ID, playerID).Scan(&already); err != nil {
		return nil, err
	}
	if already > 0 {
		return nil, fmt.Errorf("이미 참가한 방입니다")
	}
	if count >= 2 {
		return nil, fmt.Errorf("방이 가득 찼습니다")
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO match_players (match_id, player_id, player_name) VALUES ($1, $2, $3)`,
		m.ID, playerID, name); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) MatchByID(ctx context.Context, id string) (*MatchRow, error) {
	var m MatchRow
	err := s.pool.QueryRow(ctx,
		`SELECT id, code, game, status, winner_id, created_at, finished_at FROM matches WHERE id = $1`, id).
		Scan(&m.ID, &m.Code, &m.Game, &m.Status, &m.WinnerID, &m.CreatedAt, &m.FinishedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ---------- 사용자/세션 ----------

func (s *Store) CreateUser(ctx context.Context, id, name, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, name, password_hash) VALUES ($1, $2, $3)`, id, name, passwordHash)
	return err
}

func (s *Store) UserByName(ctx context.Context, name string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(password_hash, ''), COALESCE(email, ''), COALESCE(google_sub, '') FROM users WHERE name = $1`, name).
		Scan(&u.ID, &u.Name, &u.Password, &u.Email, &u.GoogleSub)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// NameAvailable — 새 사용자 이름이 이미 사용 중인지 확인.
func (s *Store) NameAvailable(ctx context.Context, name string) (bool, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE name = $1`, name).Scan(&n); err != nil {
		return false, err
	}
	return n == 0, nil
}

// ---------- Google SSO ----------

func (s *Store) FindUserByGoogleSub(ctx context.Context, sub string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(password_hash, ''), COALESCE(email, ''), google_sub FROM users WHERE google_sub = $1`, sub).
		Scan(&u.ID, &u.Name, &u.Password, &u.Email, &u.GoogleSub)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(password_hash, ''), COALESCE(email, ''), COALESCE(google_sub, '') FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Name, &u.Password, &u.Email, &u.GoogleSub)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// LinkGoogle — 기존 사용자(이메일 기준)에 구글 계정을 연결한다.
func (s *Store) LinkGoogle(ctx context.Context, userID, sub, email string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET google_sub = $1, email = $2 WHERE id = $3`, sub, email, userID)
	return err
}

func (s *Store) CreateGoogleUser(ctx context.Context, id, name, email, sub string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, name, email, google_sub) VALUES ($1, $2, $3, $4)`, id, name, email, sub)
	return err
}

func (s *Store) CreateSession(ctx context.Context, token, userID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, user_id) VALUES ($1, $2)`, token, userID)
	return err
}

func (s *Store) UserByToken(ctx context.Context, token string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, COALESCE(u.password_hash, ''), COALESCE(u.email, ''), COALESCE(u.google_sub, '')
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = $1`, token).
		Scan(&u.ID, &u.Name, &u.Password, &u.Email, &u.GoogleSub)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func (s *Store) PlayerNames(ctx context.Context, matchID string) (map[string]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT player_id, player_name FROM match_players WHERE match_id = $1`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}

func (s *Store) MarkPlaying(ctx context.Context, matchID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE matches SET status = 'playing' WHERE id = $1`, matchID)
	return err
}

func (s *Store) FinishMatch(ctx context.Context, matchID string, winnerID *string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE matches SET status = 'finished', winner_id = $1, finished_at = now() WHERE id = $2`,
		winnerID, matchID)
	return err
}

func (s *Store) AbortMatch(ctx context.Context, matchID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE matches SET status = 'aborted', finished_at = now() WHERE id = $1`, matchID)
	return err
}

func (s *Store) FinishPlayer(ctx context.Context, matchID, playerID string, score, lines int64, result string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE match_players SET score = $1, lines = $2, result = $3 WHERE match_id = $4 AND player_id = $5`,
		score, lines, result, matchID, playerID)
	return err
}

type LeaderboardRow struct {
	Rank    int    `json:"rank"`
	Name    string `json:"name"`
	Wins    int    `json:"wins"`
	Losses  int    `json:"losses"`
	Games   int    `json:"games"`
	WinRate int    `json:"win_rate"`
}

// Leaderboard — 승패 통계 전체를 계산해 상위 limit개와, 요청 사용자(이름 기준)의 순위를 함께 반환한다.
// 사용자가 상위권에 없어도 자신의 전적이 항상 보이도록 my 항목을 별도로 돌려준다.
func (s *Store) Leaderboard(ctx context.Context, limit int, myName string) ([]LeaderboardRow, *LeaderboardRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT player_name,
		       COUNT(*) FILTER (WHERE result = 'win')  AS wins,
		       COUNT(*) FILTER (WHERE result = 'loss') AS losses
		FROM match_players
		WHERE result IS NOT NULL
		GROUP BY player_name
		ORDER BY wins DESC, losses ASC, player_name ASC`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	all := []LeaderboardRow{}
	for rows.Next() {
		var r LeaderboardRow
		if err := rows.Scan(&r.Name, &r.Wins, &r.Losses); err != nil {
			return nil, nil, err
		}
		r.Games = r.Wins + r.Losses
		if r.Games > 0 {
			r.WinRate = int(float64(r.Wins) / float64(r.Games) * 100)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	for i := range all {
		all[i].Rank = i + 1
	}
	out := all
	if len(out) > limit {
		out = out[:limit]
	}
	var my *LeaderboardRow
	for i := range all {
		if all[i].Name == myName {
			v := all[i]
			my = &v
			break
		}
	}
	return out, my, nil
}
