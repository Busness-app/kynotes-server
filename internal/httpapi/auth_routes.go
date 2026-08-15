package httpapi

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yoshiofthewire/kynotes-server/internal/auth"
	"github.com/yoshiofthewire/kynotes-server/internal/config"
	"github.com/yoshiofthewire/kynotes-server/internal/ids"
)

var dummyOnce sync.Once
var dummyHash string
var loginLockout = auth.NewLockout(3, 15*time.Minute, 50000)
var recoveryLockout = auth.NewLockout(3, 60*time.Minute, 50000)

func dummyVerifier() string {
	dummyOnce.Do(func() { dummyHash, _ = auth.HashAuthSecret(strings.Repeat("0", 64)) })
	return dummyHash
}

func AuthRoutes(mux *http.ServeMux, db *sql.DB, cfg config.Config) {
	mux.HandleFunc("POST /api/v1/auth/login-params", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Username string `json:"username"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Username) == "" {
			WriteError(w, r, 400, "invalid_request", "invalid request")
			return
		}
		var salt string
		var iterations int
		if err := db.QueryRow(`SELECT login_salt,login_iterations FROM users WHERE username=?`, strings.ToLower(in.Username)).Scan(&salt, &iterations); err != nil {
			salt = auth.SyntheticLoginSalt(cfg.Secrets.ServerSaltKey, in.Username)
			iterations = 600000
		}
		writeJSON(w, map[string]any{"loginSalt": salt, "iterations": iterations})
	})
	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Username   string `json:"username"`
			AuthSecret string `json:"authSecret"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.Username == "" || len(in.AuthSecret) != 64 {
			WriteError(w, r, 400, "invalid_request", "invalid request")
			return
		}
		key := strings.ToLower(in.Username) + "\x00" + clientIP(r)
		if !recoveryLockout.Try(key, time.Now().UTC()) {
			WriteError(w, r, 429, "rate_limited", "login temporarily unavailable")
			return
		}
		var id, stored, status, role string
		err := db.QueryRow(`SELECT id,auth_secret_hash,status,role FROM users WHERE username=?`, strings.ToLower(in.Username)).Scan(&id, &stored, &status, &role)
		if err != nil {
			stored = dummyVerifier()
		}
		valid := auth.VerifyAuthSecret(in.AuthSecret, stored) == nil
		if err != nil || !valid || status != "active" {
			recoveryLockout.Fail(key, time.Now().UTC())
			WriteError(w, r, 401, "unauthenticated", "invalid credentials")
			return
		}
		recoveryLockout.Success(key)
		s, err := auth.MintSession(db, w, id, cfg.Server.DevInsecureCookies, time.Now().UTC())
		if err != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		writeJSON(w, map[string]any{"user": map[string]string{"id": id, "role": role}, "expiresAt": s.ExpiresAt.UTC().Format(time.RFC3339), "hardExpiresAt": s.HardExpiresAt.UTC().Format(time.RFC3339)})
	})
	mux.Handle("GET /api/v1/auth/session", auth.RequireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := auth.SessionFromContext(r)
		role := "user"
		_ = db.QueryRow(`SELECT role FROM users WHERE id=?`, s.UserID).Scan(&role)
		writeJSON(w, map[string]any{"user": map[string]string{"id": s.UserID, "role": role}, "expiresAt": s.ExpiresAt.UTC().Format(time.RFC3339), "hardExpiresAt": s.HardExpiresAt.UTC().Format(time.RFC3339)})
	})))
	mux.Handle("POST /api/v1/auth/logout", auth.RequireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.CheckCSRF(r) != nil {
			WriteError(w, r, 403, "csrf_failed", "csrf validation failed")
			return
		}
		s, _ := auth.SessionFromContext(r)
		if err := auth.RevokeSession(db, s.ID); err != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		secure := !cfg.Server.DevInsecureCookies
		clearCookie(w, "kynotes_session", true, secure)
		clearCookie(w, "csrf_token", false, secure)
		w.WriteHeader(http.StatusNoContent)
	})))
	mux.Handle("POST /api/v1/auth/logout-all", auth.RequireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.CheckCSRF(r) != nil {
			WriteError(w, r, 403, "csrf_failed", "csrf validation failed")
			return
		}
		s, _ := auth.SessionFromContext(r)
		if time.Since(s.CreatedAt) >= 5*time.Minute {
			WriteError(w, r, 403, "forbidden", "re-authentication required")
			return
		}
		if _, e := db.Exec(`UPDATE sessions SET revoked_at=? WHERE user_id=?`, time.Now().UTC().Format(time.RFC3339), s.UserID); e != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	mux.HandleFunc("POST /api/v1/auth/recover", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Username, RecoveryCode, NewAuthSecret, NewLoginSalt string
			Iterations                                          int    `json:"iterations"`
			NewRecoveryCode                                     string `json:"newRecoveryCode"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.Username == "" || in.NewRecoveryCode == "" || len(in.NewAuthSecret) != 64 {
			WriteError(w, r, 400, "invalid_request", "invalid request")
			return
		}
		if _, e := auth.DeriveAuthSecret("", in.NewLoginSalt, in.Iterations); e != nil {
			WriteError(w, r, 400, "invalid_request", "invalid request")
			return
		}
		key := strings.ToLower(in.Username) + "\x00" + clientIP(r)
		if !loginLockout.Try(key, time.Now().UTC()) {
			WriteError(w, r, 429, "rate_limited", "recovery temporarily unavailable")
			return
		}
		var uid, stored, usedAt string
		if e := db.QueryRow(`SELECT id,recovery_hash,recovery_used_at FROM users WHERE username=?`, strings.ToLower(in.Username)).Scan(&uid, &stored, &usedAt); e != nil || usedAt != "" || auth.VerifyAuthSecret(in.RecoveryCode, stored) != nil {
			loginLockout.Fail(key, time.Now().UTC())
			WriteError(w, r, 401, "unauthenticated", "invalid recovery credentials")
			return
		}
		if auth.VerifyAuthSecret(in.NewRecoveryCode, stored) == nil {
			WriteError(w, r, 400, "invalid_request", "replacement recovery code must be new")
			return
		}
		newHash, e := auth.HashAuthSecret(in.NewAuthSecret)
		if e != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		recoveryHash, e := auth.HashAuthSecret(in.NewRecoveryCode)
		if e != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		auditID, _ := ids.Mint("aud")
		e = dbTx(db, func(tx *sql.Tx) error {
			if _, e := tx.Exec(`UPDATE users SET auth_secret_hash=?,login_salt=?,login_iterations=?,recovery_hash=?,recovery_used_at='',updated_at=? WHERE id=?`, newHash, in.NewLoginSalt, in.Iterations, recoveryHash, now, uid); e != nil {
				return e
			}
			if _, e := tx.Exec(`UPDATE sessions SET revoked_at=? WHERE user_id=?`, now, uid); e != nil {
				return e
			}
			if _, e := tx.Exec(`UPDATE devices SET revoked_at=? WHERE user_id=?`, now, uid); e != nil {
				return e
			}
			_, e = tx.Exec(`DELETE FROM key_envelopes WHERE device_id IN (SELECT id FROM devices WHERE user_id=?)`, uid)
			if e != nil {
				return e
			}
			_, e = tx.Exec(`INSERT INTO audit_events(id,user_id,event,container_id,object_id,created_at,at,outcome,actor_user_id,request_id,reason_code) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, auditID, uid, "account.recovery", "", "", now, now, "success", uid, r.Header.Get("X-Request-Id"), "")
			return e
		})
		if e != nil {
			WriteError(w, r, 500, "internal", "internal server error")
			return
		}
		loginLockout.Success(key)
		w.WriteHeader(http.StatusNoContent)
	})
}

func clientIP(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
		ip = ip.To16()
		for i := 8; i < len(ip); i++ {
			ip[i] = 0
		}
		return ip.String()
	}
	return host
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func clearCookie(w http.ResponseWriter, name string, httpOnly, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: httpOnly, Secure: secure, SameSite: http.SameSiteLaxMode})
}
