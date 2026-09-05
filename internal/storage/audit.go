package storage

import (
	"database/sql"
	"github.com/Busness-app/kynotes-server/internal/ids"
	"time"
)

// RecordAuditOutcome returns persistence errors so export and backup callers cannot
// claim an audited operation when its audit row was lost. reason must contain no secrets.
func RecordAuditOutcome(db *sql.DB, actor, event, container, object, outcome, reason, requestID string) error {
	id, err := ids.Mint("aud")
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO audit_events(id,user_id,event,container_id,object_id,created_at,at,outcome,actor_user_id,request_id,reason_code) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, actor, event, container, object, now, now, outcome, actor, requestID, reason)
	return err
}
