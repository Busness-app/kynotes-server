package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/Busness-app/kynotes-server/internal/ids"
)

const sessionCookie = "kynotes_session"
const csrfCookie = "csrf_token"

type Session struct {
	ID, UserID, CSRF                    string
	CreatedAt, ExpiresAt, HardExpiresAt time.Time
	StepUpAt                            time.Time // zero until the login secret was re-proven
}

func MintSession(db *sql.DB, w http.ResponseWriter, userID string, insecure bool, now time.Time) (Session, error) {
	id, err := ids.Mint("ses")
	if err != nil {
		return Session{}, err
	}
	token := make([]byte, 32)
	csrf := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return Session{}, err
	}
	if _, err = rand.Read(csrf); err != nil {
		return Session{}, err
	}
	hash := sha256.Sum256(token)
	csrfHash := sha256.Sum256(csrf)
	s := Session{ID: id, UserID: userID, CSRF: base64.RawURLEncoding.EncodeToString(csrf), CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour), HardExpiresAt: now.Add(7 * 24 * time.Hour)}
	_, err = db.Exec(`INSERT INTO sessions(id,user_id,token_hash,csrf_hash,created_at,expires_at,hard_expires_at) VALUES(?,?,?,?,?,?,?)`, id, userID, hex.EncodeToString(hash[:]), hex.EncodeToString(csrfHash[:]), s.CreatedAt.UTC().Format(time.RFC3339), s.ExpiresAt.UTC().Format(time.RFC3339), s.HardExpiresAt.UTC().Format(time.RFC3339))
	if err != nil {
		return Session{}, err
	}
	secure := !insecure
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: base64.RawURLEncoding.EncodeToString(token), Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 60 * 60})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: s.CSRF, Path: "/", HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 60 * 60})
	return s, nil
}

func ResolveSession(db *sql.DB, r *http.Request, now time.Time) (Session, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return Session{}, errors.New("unauthenticated")
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) != 32 {
		return Session{}, errors.New("unauthenticated")
	}
	h := sha256.Sum256(raw)
	var s Session
	var created, expires, hard, revoked, status, stepup string
	err = db.QueryRow(`SELECT s.id,s.user_id,s.created_at,s.expires_at,s.hard_expires_at,s.revoked_at,s.stepup_at,u.status FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=?`, hex.EncodeToString(h[:])).Scan(&s.ID, &s.UserID, &created, &expires, &hard, &revoked, &stepup, &status)
	if err != nil || revoked != "" || status != "active" {
		return Session{}, errors.New("unauthenticated")
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339, created)
	s.ExpiresAt, _ = time.Parse(time.RFC3339, expires)
	s.HardExpiresAt, _ = time.Parse(time.RFC3339, hard)
	if stepup != "" {
		s.StepUpAt, _ = time.Parse(time.RFC3339, stepup)
	}
	if now.After(s.ExpiresAt) || now.After(s.HardExpiresAt) {
		return Session{}, errors.New("unauthenticated")
	}
	newExpiry := now.Add(24 * time.Hour)
	if newExpiry.After(s.HardExpiresAt) {
		newExpiry = s.HardExpiresAt
	}
	if newExpiry.Sub(s.ExpiresAt) >= 5*time.Minute {
		if _, e := db.Exec(`UPDATE sessions SET expires_at=? WHERE id=?`, newExpiry.UTC().Format(time.RFC3339), s.ID); e == nil {
			s.ExpiresAt = newExpiry
		}
	}
	return s, nil
}

func RevokeSession(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE sessions SET revoked_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339), id)
	return err
}
