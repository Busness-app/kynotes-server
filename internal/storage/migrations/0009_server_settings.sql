CREATE TABLE server_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL);
INSERT INTO server_settings(key,value,updated_at) VALUES('default_theme','Patina Ky',CURRENT_TIMESTAMP);
