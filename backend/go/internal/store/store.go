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
	ID       string
	Name     string
	Password string
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
		`SELECT id, name, password_hash FROM users WHERE name = $1`, name).
		Scan(&u.ID, &u.Name, &u.Password)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateSession(ctx context.Context, token, userID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, user_id) VALUES ($1, $2)`, token, userID)
	return err
}

func (s *Store) UserByToken(ctx context.Context, token string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.password_hash
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = $1`, token).
		Scan(&u.ID, &u.Name, &u.Password)
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
	Name string
	Wins int
}

func (s *Store) Leaderboard(ctx context.Context, limit int) ([]LeaderboardRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT player_name, COUNT(*) FILTER (WHERE result = 'win') AS wins
		FROM match_players
		WHERE result IS NOT NULL
		GROUP BY player_name
		ORDER BY wins DESC, player_name ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LeaderboardRow{}
	for rows.Next() {
		var r LeaderboardRow
		if err := rows.Scan(&r.Name, &r.Wins); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
