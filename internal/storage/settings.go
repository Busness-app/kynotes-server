package storage

import (
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound is returned for a setting that was never written. A key set to
// the empty string is present, not missing.
var ErrNotFound = errors.New("storage: not found")

func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM server_settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO server_settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, key, value, now)
	return err
}

// DeleteSetting is a no-op for an absent key.
func (s *Store) DeleteSetting(key string) error {
	_, err := s.db.Exec(`DELETE FROM server_settings WHERE key=?`, key)
	return err
}
