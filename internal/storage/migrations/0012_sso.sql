ALTER TABLE users ADD COLUMN sso_subject TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_users_sso_subject ON users(sso_subject);
