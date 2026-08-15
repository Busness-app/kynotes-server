package httpapi

import (
	"database/sql"
	"encoding/json"
	"github.com/yoshiofthewire/kynotes-server/internal/auth"
	"net/http"
)

func PushPayload(containerID string, changeSeq int64) []byte {
	b, _ := json.Marshal(map[string]any{"type": "kynotes.changes", "containerId": containerID, "changeSeq": changeSeq})
	return b
}

func PushRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.Handle("GET /api/v1/sync/pending", auth.RequireDevice(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, _ := auth.DeviceFromContext(r)
		rows, e := db.Query(`SELECT dc.container_id,c.change_seq FROM device_containers dc JOIN containers c ON c.id=dc.container_id WHERE dc.device_id=? AND c.deleted_at=''`, d.ID)
		if e != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var cid string
			var seq int64
			_ = rows.Scan(&cid, &seq)
			out = append(out, map[string]any{"containerId": cid, "changeSeq": seq})
		}
		writeJSON(w, out)
	})))
	mux.Handle("POST /api/v1/push/registrations", auth.RequireDevice(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, 503, "unavailable", "push transport is unavailable")
	})))
}
