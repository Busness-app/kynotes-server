CREATE TABLE push_registrations (device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,transport TEXT NOT NULL,token TEXT NOT NULL,updated_at TEXT NOT NULL);
