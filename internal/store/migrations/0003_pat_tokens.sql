-- AuraPanel şema v3: Personal Access Token'lar (W11)
-- Token HASH (argon2id) saklanır; ham token yalnızca oluşturma anında döner.

CREATE TABLE pat_tokens (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at  TEXT,
    last_used_at TEXT
);
CREATE INDEX idx_pat_user ON pat_tokens(user_id);
