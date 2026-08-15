CREATE TABLE share_links (id TEXT PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE, object_id TEXT NOT NULL REFERENCES objects(id) ON DELETE CASCADE, object_version INTEGER NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, revoked_at TEXT NOT NULL DEFAULT '');
CREATE INDEX idx_share_links_expiry ON share_links(expires_at);
