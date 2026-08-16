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

	"github.com/yoshiofthewire/kynotes-server/internal/auth"
	"github.com/yoshiofthewire/kynotes-server/internal/config"
	"github.com/yoshiofthewire/kynotes-server/internal/ids"
	"github.com/yoshiofthewire/kynotes-server/internal/sso"
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
	// Public endpoint for frontend to check if SSO is enabled
	mux.HandleFunc("GET /api/v1/auth/sso-config", func(w http.ResponseWriter, r *http.Request) {
		settings := ssoStore.Load()
		writeJSON(w, map[string]any{
			"enabled":   settings.Enabled,
			"issuerUrl": settings.IssuerURL,
			"clientId":  settings.ClientID,
		})
	})

	// Initiates OpenID Connect authorization code flow with PKCE
	mux.HandleFunc("GET /api/v1/auth/oidc/login", func(w http.ResponseWriter, r *http.Request) {
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
			redirectURI = fmt.Sprintf("%s://%s/api/v1/auth/oidc/callback", scheme, requestHost(r))
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
	})

	// Handles OAuth redirect callback, verifies PKCE, exchanges code for token, mints session
	mux.HandleFunc("GET /api/v1/auth/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
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
			redirectURI = fmt.Sprintf("%s://%s/api/v1/auth/oidc/callback", scheme, requestHost(r))
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
	})

	// Directory Sync Webhook from KySignOn
	mux.HandleFunc("POST /api/v1/sync/events", func(w http.ResponseWriter, r *http.Request) {
		settings := ssoStore.Load()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_request", "failed to read body")
			return
		}

		// Verify HMAC signature if secret is configured
		sigHeader := r.Header.Get("X-KySignOn-Signature")
		if settings.HMACSecret != "" && sigHeader != "" {
			mac := hmac.New(sha256.New, []byte(settings.HMACSecret))
			mac.Write(body)
			expectedSig := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(sigHeader), []byte(expectedSig)) {
				WriteError(w, r, http.StatusUnauthorized, "invalid_signature", "invalid webhook HMAC signature")
				return
			}
		}

		var event struct {
			EventID   string `json:"eventId"`
			EventType string `json:"eventType"`
			Timestamp string `json:"timestamp"`
			User      struct {
				ID          string `json:"id"`
				Username    string `json:"username"`
				Email       string `json:"email"`
				DisplayName string `json:"displayName"`
				Role        string `json:"role"`
				Status      string `json:"status"`
			} `json:"user"`
		}

		if err := json.Unmarshal(body, &event); err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_payload", "failed to parse sync event JSON")
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		switch event.EventType {
		case "user.created":
			var existingID string
			err := db.QueryRow(`SELECT id FROM users WHERE sso_subject=? OR username=?`, event.User.ID, strings.ToLower(event.User.Username)).Scan(&existingID)
			if err != nil {
				newID, _ := ids.Mint("usr")
				role := "user"
				if event.User.Role == "admin" {
					role = "admin"
				}
				status := "active"
				if event.User.Status == "disabled" {
					status = "disabled"
				}
				dummyBytes := make([]byte, 32)
				_, _ = rand.Read(dummyBytes)
				dummySecret := hex.EncodeToString(dummyBytes)
				dummyHash, _ := auth.HashAuthSecret(dummySecret)
				loginSalt := auth.SyntheticLoginSalt(cfg.Secrets.ServerSaltKey, event.User.Username)

				_, _ = db.Exec(`INSERT INTO users(id, username, auth_secret_hash, login_salt, login_iterations, role, status, sso_subject, created_at, updated_at) VALUES(?, ?, ?, ?, 600000, ?, ?, ?, ?, ?)`,
					newID, strings.ToLower(event.User.Username), dummyHash, loginSalt, role, status, event.User.ID, now, now)
				recordAudit(db, newID, "sync.user_created", "", "", r.Header.Get("X-Request-Id"))
			}
		case "user.updated":
			role := "user"
			if event.User.Role == "admin" {
				role = "admin"
			}
			status := "active"
			if event.User.Status == "disabled" {
				status = "disabled"
			}
			_, _ = db.Exec(`UPDATE users SET username=?, role=?, status=?, updated_at=? WHERE sso_subject=?`,
				strings.ToLower(event.User.Username), role, status, now, event.User.ID)
			if status == "disabled" {
				_, _ = db.Exec(`UPDATE sessions SET revoked_at=? WHERE user_id IN (SELECT id FROM users WHERE sso_subject=?)`, now, event.User.ID)
			}
			recordAudit(db, event.User.ID, "sync.user_updated", "", "", r.Header.Get("X-Request-Id"))
		case "user.status_changed":
			status := "active"
			if event.User.Status == "disabled" {
				status = "disabled"
			}
			_, _ = db.Exec(`UPDATE users SET status=?, updated_at=? WHERE sso_subject=?`, status, now, event.User.ID)
			if status == "disabled" {
				_, _ = db.Exec(`UPDATE sessions SET revoked_at=? WHERE user_id IN (SELECT id FROM users WHERE sso_subject=?)`, now, event.User.ID)
			}
			recordAudit(db, event.User.ID, "sync.status_changed", "", "", r.Header.Get("X-Request-Id"))
		}

		writeJSON(w, map[string]any{"status": "processed"})
	})
}
