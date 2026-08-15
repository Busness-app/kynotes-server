ALTER TABLE audit_events ADD COLUMN at TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN outcome TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN actor_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN actor_device_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_events ADD COLUMN reason_code TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_audit_at ON audit_events(at);
CREATE INDEX idx_audit_actor ON audit_events(actor_user_id,at);
CREATE TABLE idempotency_keys (
  key TEXT PRIMARY KEY,
  response_id TEXT NOT NULL,
  status_code INTEGER NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX idx_idempotency_created ON idempotency_keys(created_at);
CREATE INDEX idx_containers_owner ON containers(owner_user_id);
CREATE INDEX idx_containers_team ON containers(team_id);
CREATE INDEX idx_attachments_preview ON attachments(preview_digest);
CREATE INDEX idx_upload_sessions_expiry ON upload_sessions(status,expires_at);
CREATE INDEX idx_upload_sessions_user ON upload_sessions(user_id,status);
