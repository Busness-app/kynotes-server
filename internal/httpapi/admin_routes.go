package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/yoshiofthewire/kynotes-server/internal/auth"
)

var supportedThemes = map[string]bool{"Dark Matter": true, "Light Matter": true, "Tropics": true, "Tropic Night": true, "Ocean": true, "Coffee": true, "White Cliffs": true, "Cyber Punk": true, "Neon Purple": true, "Space": true, "Sky": true, "Forest": true, "Sun": true, "Patina Ky": true, "Polished Ky": true}

// AdminRoutes exposes metadata-only administration. It never returns secrets,
// ciphertext, request bodies, or raw process logs.
func AdminRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.Handle("GET /api/v1/admin/settings", auth.RequireAdmin(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var theme string
		if db.QueryRow(`SELECT value FROM server_settings WHERE key='default_theme'`).Scan(&theme) != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		writeJSON(w, map[string]string{"defaultTheme": theme})
	})))
	mux.Handle("PATCH /api/v1/admin/settings", auth.RequireAdmin(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.CheckCSRF(r) != nil {
			WriteError(w, r, 403, "csrf_failed", "csrf validation failed")
			return
		}
		var in struct {
			DefaultTheme string `json:"defaultTheme"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || !supportedThemes[in.DefaultTheme] {
			WriteError(w, r, 400, "invalid_request", "invalid theme")
			return
		}
		if _, err := db.Exec(`UPDATE server_settings SET value=?,updated_at=? WHERE key='default_theme'`, in.DefaultTheme, time.Now().UTC().Format(time.RFC3339)); err != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	mux.Handle("GET /api/v1/admin/users", auth.RequireAdmin(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT id,username,role,status,quota_bytes,created_at FROM users ORDER BY username`)
		if err != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, username, role, status, created string
			var quota int64
			if rows.Scan(&id, &username, &role, &status, &quota, &created) != nil {
				WriteError(w, r, 500, "internal", "internal server error")
				return
			}
			out = append(out, map[string]any{"id": id, "username": username, "role": role, "status": status, "quotaBytes": quota, "createdAt": created})
		}
		writeJSON(w, out)
	})))
	mux.Handle("PATCH /api/v1/admin/users/{id}", auth.RequireAdmin(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.CheckCSRF(r) != nil {
			WriteError(w, r, 403, "csrf_failed", "csrf validation failed")
			return
		}
		id := r.PathValue("id")
		var in struct {
			Role, Status string
			QuotaBytes   *int64
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			WriteError(w, r, 400, "invalid_request", "invalid request")
			return
		}
		if in.Role != "admin" && in.Role != "user" {
			WriteError(w, r, 400, "invalid_request", "invalid role")
			return
		}
		if in.Status != "active" && in.Status != "disabled" {
			WriteError(w, r, 400, "invalid_request", "invalid status")
			return
		}
		if in.QuotaBytes == nil || *in.QuotaBytes < 0 {
			WriteError(w, r, 400, "invalid_request", "invalid quota")
			return
		}
		s, _ := auth.SessionFromContext(r)
		if id == s.UserID && (in.Role != "admin" || in.Status != "active") {
			WriteError(w, r, 400, "invalid_request", "cannot disable or demote the current administrator")
			return
		}
		if _, err := db.Exec(`UPDATE users SET role=?,status=?,quota_bytes=?,updated_at=? WHERE id=?`, in.Role, in.Status, *in.QuotaBytes, time.Now().UTC().Format(time.RFC3339), id); err != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	mux.Handle("GET /api/v1/admin/audit", auth.RequireAdmin(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := 100
		if value, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && value > 0 && value < 501 {
			limit = value
		}
		rows, err := db.Query(`SELECT event,outcome,actor_user_id,container_id,object_id,reason_code,at FROM audit_events ORDER BY at DESC LIMIT ?`, limit)
		if err != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		defer rows.Close()
		out := []map[string]string{}
		for rows.Next() {
			var event, outcome, actor, container, object, reason, at string
			if rows.Scan(&event, &outcome, &actor, &container, &object, &reason, &at) != nil {
				WriteError(w, r, 500, "internal", "internal server error")
				return
			}
			out = append(out, map[string]string{"event": event, "outcome": outcome, "actorUserId": actor, "containerId": container, "objectId": object, "reasonCode": reason, "at": at})
		}
		writeJSON(w, out)
	})))
}
