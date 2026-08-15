CREATE INDEX idx_share_links_object ON share_links(object_id, revoked_at);
CREATE TABLE sealed_share_links (id TEXT PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE, ciphertext BLOB NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL, revoked_at TEXT NOT NULL DEFAULT '');
CREATE INDEX idx_sealed_share_links_expiry ON sealed_share_links(expires_at);
