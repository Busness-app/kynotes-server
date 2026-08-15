package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct{ db *sql.DB }

var migrateMu sync.Mutex

func (s *Store) DB() *sql.DB { return s.db }

func Open(path string) (*Store, error) {
	d, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_txlock=immediate&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", path))
	if err != nil {
		return nil, err
	}
	s := &Store{db: d}
	if err = s.migrate(); err != nil {
		d.Close()
		return nil, err
	}
	if err = s.IntegrityCheck(context.Background()); err != nil {
		d.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) migrate() error {
	migrateMu.Lock()
	defer migrateMu.Unlock()
	rows, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name() < rows[j].Name() })
	for i, f := range rows {
		var v int
		if _, err = fmt.Sscanf(f.Name(), "%04d_", &v); err != nil {
			return err
		}
		if v != i+1 {
			return fmt.Errorf("migration versions are not contiguous at %04d", v)
		}
		var n int
		_ = s.db.QueryRow("SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&n)
		if v <= n {
			continue
		}
		b, e := migrationFS.ReadFile("migrations/" + f.Name())
		if e != nil {
			return e
		}
		tx, e := s.db.Begin()
		if e != nil {
			return e
		}
		if _, e = tx.Exec(string(b)); e == nil {
			_, e = tx.Exec("INSERT INTO schema_migrations(version, applied_at) VALUES(?,?)", v, time.Now().UTC().Format(time.RFC3339))
		}
		if e != nil {
			_ = tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
func (s *Store) IntegrityCheck(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity check: %s", result)
	}
	return nil
}
func (s *Store) NextChangeSeq(tx *sql.Tx, containerID string) (int64, error) {
	var n int64
	err := tx.QueryRow("UPDATE containers SET change_seq=change_seq+1 WHERE id=? RETURNING change_seq", containerID).Scan(&n)
	return n, err
}
