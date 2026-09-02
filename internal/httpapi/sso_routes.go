package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Busness-app/kynotes-server/internal/auth"
	"github.com/Busness-app/kynotes-server/internal/config"
	"github.com/Busness-app/kynotes-server/internal/ids"
	"github.com/Busness-app/kynotes-server/internal/sso"
)

const ssoCookieName = "kynotes_sso_state"

func isRequestSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func requestHost(r *http.Request) string {
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		return host
	}
	return r.Host
}

// SSORoutes registers OIDC SSO and directory sync endpoints.
func SSORoutes(mux *http.ServeMux, db *sql.DB, cfg config.Config, ssoStore *sso.Store) {
	handleSSOConfig := func(w http.ResponseWriter, r *http.Request) {
		settings := ssoStore.Load()
		writeJSON(w, map[string]any{
			"enabled":   settings.Enabled,
			"issuerUrl": settings.IssuerURL,
			"clientId":  settings.ClientID,
		})
	}
	mux.HandleFunc("GET /api/v1/auth/sso-config", handleSSOConfig)
	mux.HandleFunc("GET /api/auth/sso-config", handleSSOConfig)

	// Initiates OpenID Connect authorization code flow with PKCE
	handleOIDCFlow := func(w http.ResponseWriter, r *http.Request) {
		settings := ssoStore.Load()
		if !settings.Enabled || settings.IssuerURL == "" {
			WriteError(w, r, http.StatusServiceUnavailable, "sso_disabled", "Single Sign-On is not configured or disabled")
			return
		}

		verifier, challenge, err := sso.GeneratePKCE()
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "pkce_failed", "failed to generate PKCE challenge")
			return
		}

		state := sso.GenerateState()

		cookieVal := fmt.Sprintf("%s|%s", state, verifier)
		secure := !cfg.Server.DevInsecureCookies && isRequestSecure(r)
		http.SetCookie(w, &http.Cookie{
			Name:     ssoCookieName,
			Value:    cookieVal,
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   300,
		})

		disc, err := sso.DiscoverEndpoints(r.Context(), settings.IssuerURL)
		if err != nil {
			WriteError(w, r, http.StatusBadGateway, "discovery_failed", "failed to discover OIDC endpoints: "+err.Error())
			return
		}

		scheme := "http"
		if isRequestSecure(r) {
			scheme = "https"
		}
		redirectURI := settings.RedirectURI
		if redirectURI == "" {
			redirectURI = fmt.Sprintf("%s://%s%s", scheme, requestHost(r), strings.Replace(r.URL.Path, "/login", "/callback", 1))
		}

		authURL, err := url.Parse(disc.AuthorizationEndpoint)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "invalid_endpoint", "invalid authorization endpoint")
			return
		}
		q := authURL.Query()
		q.Set("response_type", "code")
		q.Set("client_id", settings.ClientID)
		q.Set("redirect_uri", redirectURI)
		q.Set("scope", "openid profile email")
		q.Set("state", state)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
		authURL.RawQuery = q.Encode()

		http.Redirect(w, r, authURL.String(), http.StatusFound)
	}
	mux.HandleFunc("GET /api/v1/auth/oidc/login", handleOIDCFlow)
	mux.HandleFunc("GET /api/auth/oidc/login", handleOIDCFlow)
	mux.HandleFunc("GET /auth/oidc/login", handleOIDCFlow)

	// Handles OAuth redirect callback, verifies PKCE, exchanges code for token, mints session
	handleOIDCCallback := func(w http.ResponseWriter, r *http.Request) {
		settings := ssoStore.Load()
		if !settings.Enabled || settings.IssuerURL == "" {
			WriteError(w, r, http.StatusServiceUnavailable, "sso_disabled", "Single Sign-On is not configured or disabled")
			return
		}

		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" || state == "" {
			WriteError(w, r, http.StatusBadRequest, "invalid_request", "missing code or state")
			return
		}

		cookie, err := r.Cookie(ssoCookieName)
		if err != nil || cookie.Value == "" {
			WriteError(w, r, http.StatusBadRequest, "invalid_state", "missing or expired SSO cookie")
			return
		}

		parts := strings.Split(cookie.Value, "|")
		if len(parts) < 2 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(state)) != 1 {
			WriteError(w, r, http.StatusBadRequest, "invalid_state", "invalid SSO state parameter")
			return
		}
		codeVerifier := parts[1]

		disc, err := sso.DiscoverEndpoints(r.Context(), settings.IssuerURL)
		if err != nil {
			WriteError(w, r, http.StatusBadGateway, "discovery_failed", "failed to discover OIDC endpoints: "+err.Error())
			return
		}

		scheme := "http"
		if isRequestSecure(r) {
			scheme = "https"
		}
		redirectURI := settings.RedirectURI
		if redirectURI == "" {
			redirectURI = fmt.Sprintf("%s://%s%s", scheme, requestHost(r), r.URL.Path)
		}

		tok, err := sso.ExchangeCode(r.Context(), disc.TokenEndpoint, settings.ClientID, settings.ClientSecret, code, redirectURI, codeVerifier)
		if err != nil {
			WriteError(w, r, http.StatusBadGateway, "exchange_failed", "failed to exchange token: "+err.Error())
			return
		}

		claims, err := sso.ParseClaims(r.Context(), tok.IDToken, tok.AccessToken, disc.UserinfoEndpoint)
		if err != nil {
			WriteError(w, r, http.StatusBadGateway, "invalid_claims", "failed to parse claims: "+err.Error())
			return
		}

		var userID, userStatus, userRole string
		// 1. Try finding user by sso_subject
		err = db.QueryRow(`SELECT id, status, role FROM users WHERE sso_subject=?`, claims.Subject).Scan(&userID, &userStatus, &userRole)
		if err != nil {
			// 2. Try finding user by username or email
			err = db.QueryRow(`SELECT id, status, role FROM users WHERE username=?`, strings.ToLower(claims.Username)).Scan(&userID, &userStatus, &userRole)
			if err == nil {
				// Link sso_subject
				_, _ = db.Exec(`UPDATE users SET sso_subject=?, updated_at=? WHERE id=?`, claims.Subject, time.Now().UTC().Format(time.RFC3339), userID)
			}
		}

		// 3. Auto-provision if user does not exist
		if userID == "" {
			if !settings.AutoProvision {
				WriteError(w, r, http.StatusForbidden, "user_not_found", "Account does not exist and auto-provisioning is disabled")
				return
			}

			userID, err = ids.Mint("usr")
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, "internal", "failed to mint user id")
				return
			}

			role := "user"
			if claims.Role == "admin" {
				role = "admin"
			}
			userStatus = "active"
			userRole = role

			dummyBytes := make([]byte, 32)
			_, _ = rand.Read(dummyBytes)
			dummySecret := hex.EncodeToString(dummyBytes)
			dummyHash, _ := auth.HashAuthSecret(dummySecret)
			loginSalt := auth.SyntheticLoginSalt(cfg.Secrets.ServerSaltKey, claims.Username)
			now := time.Now().UTC().Format(time.RFC3339)

			_, err = db.Exec(`INSERT INTO users(id, username, auth_secret_hash, login_salt, login_iterations, role, status, sso_subject, created_at, updated_at) VALUES(?, ?, ?, ?, 600000, ?, 'active', ?, ?, ?)`,
				userID, strings.ToLower(claims.Username), dummyHash, loginSalt, role, claims.Subject, now, now)
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, "internal", "failed to auto-provision user: "+err.Error())
				return
			}
			recordAudit(db, userID, "user.auto_provision", "", "", r.Header.Get("X-Request-Id"))
		}

		if userStatus != "active" {
			WriteError(w, r, http.StatusForbidden, "account_disabled", "Account is disabled")
			return
		}

		// Clear SSO state cookie
		clearCookie(w, ssoCookieName, true, !cfg.Server.DevInsecureCookies && isRequestSecure(r))

		// Mint KyNotes session
		_, err = auth.MintSession(db, w, userID, cfg.Server.DevInsecureCookies, time.Now().UTC())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "internal", "failed to create session")
			return
		}

		recordAudit(db, userID, "auth.sso_login", "", "", r.Header.Get("X-Request-Id"))
		http.Redirect(w, r, "/", http.StatusFound)
	}
	mux.HandleFunc("GET /api/v1/auth/oidc/callback", handleOIDCCallback)
	mux.HandleFunc("GET /api/auth/oidc/callback", handleOIDCCallback)
	mux.HandleFunc("GET /auth/oidc/callback", handleOIDCCallback)

	// Directory Sync Webhook from KySignOn
	handleSyncEvents := func(w http.ResponseWriter, r *http.Request) {
		settings := ssoStore.Load()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_request", "failed to read body")
			return
		}

		// This webhook has no session authentication: the signature is the only
		// control on it, and it can create admins, delete users, and rebind an
		// account's SSO subject. An absent signature or unconfigured secret must
		// fail closed.
		if settings.HMACSecret == "" {
			WriteError(w, r, http.StatusUnauthorized, "sync_not_configured", "directory sync is not configured")
			return
		}
		mac := hmac.New(sha256.New, []byte(settings.HMACSecret))
		mac.Write(body)
		expectedSig := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(r.Header.Get("X-KySignOn-Signature")), []byte(expectedSig)) {
			WriteError(w, r, http.StatusUnauthorized, "invalid_signature", "invalid webhook HMAC signature")
			return
		}

		var event struct {
			EventID   string         `json:"eventId"`
			EventType string         `json:"eventType"`
			Timestamp string         `json:"timestamp"`
			User      *sso.SyncUser  `json:"user,omitempty"`
			Users     []sso.SyncUser `json:"users,omitempty"`
		}

		if err := json.Unmarshal(body, &event); err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_json", "failed to decode sync payload")
			return
		}

		switch event.EventType {
		case "user.created", "user.updated", "user.status_changed", "user.disabled", "user.enabled":
			if event.User != nil {
				if err := syncSingleUser(db, cfg, event.User); err != nil {
					WriteError(w, r, http.StatusInternalServerError, "sync_failed", err.Error())
					return
				}
			}
		case "user.deleted":
			if event.User != nil {
				_, _ = db.Exec(`DELETE FROM users WHERE sso_subject=? OR (username != '' AND username=?)`, event.User.ID, strings.ToLower(event.User.Username))
			}
		case "directory.resync":
			for _, u := range event.Users {
				_ = syncSingleUser(db, cfg, &u)
			}
		}

		writeJSON(w, map[string]any{"status": "applied", "eventId": event.EventID})
	}
	mux.HandleFunc("POST /api/v1/sync/events", handleSyncEvents)
	mux.HandleFunc("POST /api/sync/events", handleSyncEvents)
	mux.HandleFunc("POST /sync/events", handleSyncEvents)
}

func syncSingleUser(db *sql.DB, cfg config.Config, u *sso.SyncUser) error {
	var existingID, existingUsername, existingRole, existingStatus string
	err := db.QueryRow(`SELECT id, username, role, status FROM users WHERE sso_subject=? OR (username != '' AND username=?)`, u.ID, strings.ToLower(u.Username)).Scan(&existingID, &existingUsername, &existingRole, &existingStatus)
	now := time.Now().UTC().Format(time.RFC3339)

	username := strings.ToLower(u.Username)
	if username == "" {
		username = existingUsername
	}
	role := existingRole
	if role == "" {
		role = "user"
	}
	if u.Role != "" {
		if u.Role == "admin" {
			role = "admin"
		} else {
			role = "user"
		}
	}
	status := existingStatus
	if status == "" {
		status = "active"
	}
	if u.Status != "" {
		if u.Status == "disabled" {
			status = "disabled"
		} else {
			status = "active"
		}
	}

	if err != nil {
		// Insert new user
		if username == "" {
			username = u.ID
		}
		newID, _ := ids.Mint("usr")
		dummyBytes := make([]byte, 32)
		_, _ = rand.Read(dummyBytes)
		dummySecret := hex.EncodeToString(dummyBytes)
		dummyHash, _ := auth.HashAuthSecret(dummySecret)
		loginSalt := auth.SyntheticLoginSalt(cfg.Secrets.ServerSaltKey, username)

		_, err = db.Exec(`INSERT INTO users(id, username, auth_secret_hash, login_salt, login_iterations, role, status, sso_subject, created_at, updated_at) VALUES(?, ?, ?, ?, 600000, ?, ?, ?, ?, ?)`,
			newID, username, dummyHash, loginSalt, role, status, u.ID, now, now)
		return err
	}

	// Update existing user
	_, err = db.Exec(`UPDATE users SET username=?, role=?, status=?, sso_subject=?, updated_at=? WHERE id=?`,
		username, role, status, u.ID, now, existingID)
	return err
}
