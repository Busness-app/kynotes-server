CREATE TABLE sso_sync_events (event_id TEXT PRIMARY KEY, expires_at INTEGER NOT NULL);
CREATE INDEX sso_sync_events_expiry ON sso_sync_events(expires_at);
