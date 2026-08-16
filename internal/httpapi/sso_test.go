package httpapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yoshiofthewire/kynotes-server/internal/auth"
	"github.com/yoshiofthewire/kynotes-server/internal/config"
	"github.com/yoshiofthewire/kynotes-server/internal/ids"
	"github.com/yoshiofthewire/kynotes-server/internal/logging"
	"github.com/yoshiofthewire/kynotes-server/internal/sso"
	"github.com/yoshiofthewire/kynotes-server/internal/storage"
)

func setupTestDB(t *testing.T) (*sql.DB, config.Config) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kynotes_test.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := config.Defaults()
	cfg.DataDir = dir
	cfg.Secrets.ServerSaltKey = "test-salt-key-32-bytes-long-1234"
	cfg.Server.DevInsecureCookies = true

	return store.DB(), cfg
}

func createAdminUser(t *testing.T, db *sql.DB) (string, string) {
	userID, _ := ids.Mint("usr")
	secret, _ := auth.HashAuthSecret(strings.Repeat("a", 64))
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO users(id, username, auth_secret_hash, login_salt, login_iterations, role, status, created_at, updated_at) VALUES(?, 'admin', ?, 'salt', 600000, 'admin', 'active', ?, ?)`, userID, secret, now, now)
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}
	return userID, "admin"
}

func TestSSOConfigEndpoint(t *testing.T) {
	db, cfg := setupTestDB(t)
	ssoStore := sso.NewStore(db)

	mux := http.NewServeMux()
	SSORoutes(mux, db, cfg, ssoStore)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/auth/sso-config", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var res map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&res)
	if res["enabled"] != false {
		t.Fatalf("expected enabled=false initially")
	}

	// Update settings
	_ = ssoStore.Save(sso.SSOSettings{
		Enabled:   true,
		IssuerURL: "https://auth.example.com",
		ClientID:  "kynotes",
	})

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req)
	var res2 map[string]any
	_ = json.NewDecoder(rec2.Body).Decode(&res2)
	if res2["enabled"] != true || res2["issuerUrl"] != "https://auth.example.com" || res2["clientId"] != "kynotes" {
		t.Fatalf("expected updated settings in response, got %+v", res2)
	}
}

func TestSSOFullFlow(t *testing.T) {
	db, cfg := setupTestDB(t)
	ssoStore := sso.NewStore(db)

	// Mock OIDC Identity Provider (KySignOn)
	var mockServer *httptest.Server
	mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sso.DiscoveryDoc{
				Issuer:                mockServer.URL,
				AuthorizationEndpoint: mockServer.URL + "/oauth/authorize",
				TokenEndpoint:         mockServer.URL + "/oauth/token",
				UserinfoEndpoint:      mockServer.URL + "/oauth/userinfo",
			})
		case "/oauth/token":
			claimsObj := map[string]any{
				"sub":                "kysignon-sub-789",
				"preferred_username": "bob",
				"email":              "bob@example.com",
				"role":               "user",
			}
			claimsJSON, _ := json.Marshal(claimsObj)
			headerJSON, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
			mockIDToken := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON) + ".fakesig"

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sso.TokenResponse{
				AccessToken: "mock-access-token",
				IDToken:     mockIDToken,
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// Configure SSO in KyNotes
	_ = ssoStore.Save(sso.SSOSettings{
		Enabled:       true,
		IssuerURL:     mockServer.URL,
		ClientID:      "kynotes",
		AutoProvision: true,
	})

	router := NewRouter(logging.New(io.Discard, "error", "json"), 1048576, func() bool { return true }, db, cfg)

	// 1. Initiate Login
	loginReq := httptest.NewRequest("GET", "/api/v1/auth/oidc/login", nil)
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect from oidc/login, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	redirectLocation := loginRec.Header().Get("Location")
	if !strings.Contains(redirectLocation, "/oauth/authorize") {
		t.Fatalf("expected redirect to /oauth/authorize, got %s", redirectLocation)
	}

	// Extract state cookie
	var ssoCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == ssoCookieName {
			ssoCookie = c
			break
		}
	}
	if ssoCookie == nil {
		t.Fatalf("expected %s cookie to be set", ssoCookieName)
	}

	parts := strings.Split(ssoCookie.Value, "|")
	state := parts[0]

	// 2. Callback
	cbReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/auth/oidc/callback?code=mock-code-123&state=%s", state), nil)
	cbReq.AddCookie(ssoCookie)
	cbRec := httptest.NewRecorder()
	router.ServeHTTP(cbRec, cbReq)

	if cbRec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect from oidc/callback, got %d: %s", cbRec.Code, cbRec.Body.String())
	}

	// Verify session cookie was issued
	var sessionCookie *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == "kynotes_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected kynotes_session cookie to be issued")
	}

	// Verify user was auto-provisioned in DB
	var dbUser struct {
		ID         string
		Username   string
		SSOSubject string
		Role       string
		Status     string
	}
	err := db.QueryRow(`SELECT id, username, sso_subject, role, status FROM users WHERE sso_subject='kysignon-sub-789'`).Scan(&dbUser.ID, &dbUser.Username, &dbUser.SSOSubject, &dbUser.Role, &dbUser.Status)
	if err != nil {
		t.Fatalf("user was not provisioned in database: %v", err)
	}
	if dbUser.Username != "bob" || dbUser.Role != "user" || dbUser.Status != "active" {
		t.Fatalf("unexpected provisioned user data: %+v", dbUser)
	}
}

func TestDirectorySyncWebhook(t *testing.T) {
	db, cfg := setupTestDB(t)
	ssoStore := sso.NewStore(db)
	secret := "shared-hmac-secret-123"

	_ = ssoStore.Save(sso.SSOSettings{
		Enabled:    true,
		HMACSecret: secret,
	})

	router := NewRouter(logging.New(io.Discard, "error", "json"), 1048576, func() bool { return true }, db, cfg)

	// 1. Create user event
	eventPayload := map[string]any{
		"eventId":   "ev_1",
		"eventType": "user.created",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"user": map[string]any{
			"id":          "kysignon-user-999",
			"username":    "charlie",
			"displayName": "Charlie Brown",
			"email":       "charlie@example.com",
			"role":        "admin",
			"status":      "active",
		},
	}
	bodyBytes, _ := json.Marshal(eventPayload)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/api/v1/sync/events", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-KySignOn-Signature", sig)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from sync event, got %d: %s", rec.Code, rec.Body.String())
	}

	var uRole, uStatus string
	err := db.QueryRow(`SELECT role, status FROM users WHERE sso_subject='kysignon-user-999'`).Scan(&uRole, &uStatus)
	if err != nil || uRole != "admin" || uStatus != "active" {
		t.Fatalf("failed to replicate user from sync event: err=%v, role=%s, status=%s", err, uRole, uStatus)
	}

	// 2. Disable user event
	eventPayload2 := map[string]any{
		"eventId":   "ev_2",
		"eventType": "user.status_changed",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"user": map[string]any{
			"id":     "kysignon-user-999",
			"status": "disabled",
		},
	}
	bodyBytes2, _ := json.Marshal(eventPayload2)
	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write(bodyBytes2)
	sig2 := hex.EncodeToString(mac2.Sum(nil))

	req2 := httptest.NewRequest("POST", "/api/v1/sync/events", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-KySignOn-Signature", sig2)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 from sync event, got %d", rec2.Code)
	}

	_ = db.QueryRow(`SELECT status FROM users WHERE sso_subject='kysignon-user-999'`).Scan(&uStatus)
	if uStatus != "disabled" {
		t.Fatalf("expected status=disabled, got %s", uStatus)
	}
}

func TestAdminSSOAndPairing(t *testing.T) {
	db, cfg := setupTestDB(t)
	adminID, _ := createAdminUser(t, db)

	// Mint admin session and get cookies
	mintRec := httptest.NewRecorder()
	sess, err := auth.MintSession(db, mintRec, adminID, true, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to mint session: %v", err)
	}

	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range mintRec.Result().Cookies() {
		if c.Name == "kynotes_session" {
			sessionCookie = c
		}
		if c.Name == "csrf_token" {
			csrfCookie = c
		}
	}

	// Mock KySignOn Server for System Pairing
	var mockKySignOn *httptest.Server
	mockKySignOn = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/systems/register" {
			var in sso.SystemPairingRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in.PairingToken != "valid-pairing-token" {
				http.Error(w, `{"error":"invalid_token"}`, http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sso.SystemPairingResponse{
				SystemID:   "sys_paired_123",
				HMACSecret: "shared-hmac-secret-xyz",
				Status:     "active",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockKySignOn.Close()

	router := NewRouter(logging.New(io.Discard, "error", "json"), 1048576, func() bool { return true }, db, cfg)

	// Call POST /api/v1/admin/sso/pair
	pairBody := map[string]string{
		"issuerUrl":    mockKySignOn.URL,
		"pairingToken": "valid-pairing-token",
	}
	pairJSON, _ := json.Marshal(pairBody)
	req := httptest.NewRequest("POST", "/api/v1/admin/sso/pair", bytes.NewReader(pairJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", sess.CSRF)
	req.AddCookie(sessionCookie)
	req.AddCookie(csrfCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from admin sso pair, got %d: %s", rec.Code, rec.Body.String())
	}

	var pairRes map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&pairRes)
	if pairRes["success"] != true || pairRes["systemId"] != "sys_paired_123" {
		t.Fatalf("unexpected pair result: %+v", pairRes)
	}

	// Verify GET /api/v1/admin/sso returns paired configuration
	getReq := httptest.NewRequest("GET", "/api/v1/admin/sso", nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET /api/v1/admin/sso, got %d", getRec.Code)
	}

	var ssoSettings sso.SSOSettings
	_ = json.NewDecoder(getRec.Body).Decode(&ssoSettings)
	if !ssoSettings.Enabled || ssoSettings.IssuerURL != mockKySignOn.URL || ssoSettings.HMACSecret != "shared-hmac-secret-xyz" {
		t.Fatalf("unexpected sso settings in admin response: %+v", ssoSettings)
	}
}
