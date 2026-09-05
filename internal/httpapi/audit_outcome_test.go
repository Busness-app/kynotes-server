package httpapi

import "testing"

func TestRecordAuditOutcomeStoresFailure(t *testing.T) {
	db, _ := setupTestDB(t)
	recordAuditOutcome(db, "usr_x", "admin.backup_run", "", "cap_1", "failure", "KyRecovery refused", "req-1")
	var outcome, reason, object, actor string
	if err := db.QueryRow(`SELECT outcome,reason_code,object_id,actor_user_id FROM audit_events WHERE event='admin.backup_run'`).Scan(&outcome, &reason, &object, &actor); err != nil {
		t.Fatal(err)
	}
	if outcome != "failure" || reason != "KyRecovery refused" || object != "cap_1" || actor != "usr_x" {
		t.Fatalf("got %q %q %q %q", outcome, reason, object, actor)
	}
}

func TestRecordAuditStillWritesSuccess(t *testing.T) {
	db, _ := setupTestDB(t)
	recordAudit(db, "usr_x", "admin.user.create", "", "usr_y", "req-2")
	var outcome, reason string
	if err := db.QueryRow(`SELECT outcome,reason_code FROM audit_events WHERE event='admin.user.create'`).Scan(&outcome, &reason); err != nil {
		t.Fatal(err)
	}
	if outcome != "success" || reason != "" {
		t.Fatalf("got %q %q", outcome, reason)
	}
}
