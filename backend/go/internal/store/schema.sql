CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  password_hash TEXT,
  email TEXT,
  google_sub TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS matches (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  game TEXT NOT NULL DEFAULT 'tetris',
  status TEXT NOT NULL DEFAULT 'created',
  winner_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS match_players (
  match_id TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  player_id TEXT NOT NULL,
  player_name TEXT NOT NULL,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  score BIGINT NOT NULL DEFAULT 0,
  lines INTEGER NOT NULL DEFAULT 0,
  result TEXT,
  PRIMARY KEY (match_id, player_id)
);

CREATE INDEX IF NOT EXISTS idx_match_players_name ON match_players(player_name);
CREATE INDEX IF NOT EXISTS idx_matches_status ON matches(status);

-- 기존 배포 마이그레이션: 신규 컬럼 추가
ALTER TABLE matches ADD COLUMN IF NOT EXISTS game TEXT NOT NULL DEFAULT 'tetris';
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS joined_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Google SSO (v1.3.0): 사용자는 비밀번호 없이 구글 계정으로 가입할 수 있다
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_sub TEXT;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_sub ON users(google_sub) WHERE google_sub IS NOT NULL;

-- CPU 봇 상대 (v1.3.4): 솔로 매치 = 두 번째 플레이어가 서버 봇
ALTER TABLE matches ADD COLUMN IF NOT EXISTS solo BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS is_bot BOOLEAN NOT NULL DEFAULT false;

-- 아이템 배틀 (v1.4.0): 'normal' | 'item'
ALTER TABLE matches ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'normal';
