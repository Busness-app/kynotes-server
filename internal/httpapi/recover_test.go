package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Busness-app/kynotes-server/internal/auth"
)

func TestRecoverRotatesCodeAndRevokesSessions(t *testing.T) {
	db, cfg := setupTestDB(t)
	mux := http.NewServeMux()
	AuthRoutes(mux, db, cfg)

	code, hash, err := auth.NewRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	oldHash, _ := auth.HashAuthSecret(strings.Repeat("0", 64))
	if _, err := db.Exec(`INSERT INTO users(id,username,auth_secret_hash,login_salt,login_iterations,recovery_hash,role,status,created_at,updated_at) VALUES('usr_r','alice',?,'salt',600000,?,'user','active',?,?)`, oldHash, hash, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(id,user_id,token_hash,csrf_hash,created_at,expires_at,hard_expires_at) VALUES('ses_r','usr_r','tok','csrf',?,?,?)`, now, now, now); err != nil {
		t.Fatal(err)
	}

	salt := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	body := `{"username":"alice","recoveryCode":"` + strings.ToUpper(code) + `","newAuthSecret":"` + strings.Repeat("1", 64) + `","newLoginSalt":"` + salt + `","iterations":100000}`
	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/auth/recover", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	rec := post()
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		RecoveryCode string `json:"recoveryCode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.RecoveryCode) != 14 {
		t.Fatalf("no replacement code in %s", rec.Body)
	}
	if out.RecoveryCode == code {
		t.Fatal("replacement code equals the used one")
	}

	var stored, revoked string
	if err := db.QueryRow(`SELECT recovery_hash FROM users WHERE id='usr_r'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if auth.VerifyRecoveryCode(out.RecoveryCode, stored) != nil {
		t.Fatal("stored hash does not verify the returned code")
	}
	if strings.Contains(stored, out.RecoveryCode) {
		t.Fatal("recovery code stored in the clear")
	}
	if err := db.QueryRow(`SELECT revoked_at FROM sessions WHERE id='ses_r'`).Scan(&revoked); err != nil || revoked == "" {
		t.Fatalf("session not revoked: %v %q", err, revoked)
	}

	// The used code is dead.
	if rec := post(); rec.Code != 401 {
		t.Fatalf("reused code: %d", rec.Code)
	}
}

func TestRecoverRefusesUserWithoutRecoveryCode(t *testing.T) {
	db, cfg := setupTestDB(t)
	mux := http.NewServeMux()
	AuthRoutes(mux, db, cfg)
	now := time.Now().UTC().Format(time.RFC3339)
	h, _ := auth.HashAuthSecret(strings.Repeat("0", 64))
	if _, err := db.Exec(`INSERT INTO users(id,username,auth_secret_hash,login_salt,login_iterations,role,status,created_at,updated_at) VALUES('usr_n','bob',?,'salt',600000,'user','active',?,?)`, h, now, now); err != nil {
		t.Fatal(err)
	}
	salt := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	body := `{"username":"bob","recoveryCode":"","newAuthSecret":"` + strings.Repeat("1", 64) + `","newLoginSalt":"` + salt + `","iterations":100000}`
	req := httptest.NewRequest("POST", "/api/v1/auth/recover", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("empty stored hash must never verify: %d", rec.Code)
	}
}
