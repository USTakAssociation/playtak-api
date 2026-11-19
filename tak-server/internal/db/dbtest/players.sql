-- The players database after scripts/migrations have run, used by tests.
-- Keep in sync with scripts/development/create_databases.sh and the
-- 2026-09 migrations. No players column except id is NOT NULL, so the store
-- treats NULLs as zero values.
CREATE TABLE players (id INTEGER PRIMARY KEY, name VARCHAR(20), password VARCHAR(50), email VARCHAR(50), rating real default 1000, boost real default 750, ratedgames int default 0, maxrating real default 1000, ratingage real default 0, ratingbase int default 0, unrated int default 0, isbot int default 0, fatigue text default '{}', is_admin int default 0, is_mod int default 0, is_gagged int default 0, is_banned int default 0, participation_rating int default 1000);

CREATE TABLE auth_tokens (
	token_hash BLOB PRIMARY KEY,
	player_id  INTEGER NOT NULL REFERENCES players (id) ON DELETE CASCADE,
	kind       TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX idx_auth_tokens_expires ON auth_tokens (expires_at);
CREATE INDEX idx_auth_tokens_player ON auth_tokens (player_id);
