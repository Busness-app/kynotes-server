package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenAndTransaction(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.WithTx(context.Background(), func(tx *sql.Tx) error { return nil }); e != nil {
		t.Fatal(e)
	}
	if e = s.IntegrityCheck(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestFrozenAuditAndIdempotencySchema(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for table, want := range map[string][]string{
		"audit_events":     {"at", "event", "outcome", "actor_user_id", "actor_device_id", "container_id", "object_id", "request_id", "reason_code"},
		"idempotency_keys": {"key", "response_id", "status_code", "created_at"},
	} {
		rows, err := s.DB().Query("SELECT name FROM pragma_table_info(?)", table)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatal(err)
			}
			got[name] = true
		}
		rows.Close()
		for _, name := range want {
			if !got[name] {
				t.Errorf("%s missing frozen column %s", table, name)
			}
		}
	}
}
