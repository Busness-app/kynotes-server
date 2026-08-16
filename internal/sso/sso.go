package sso

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SSOSettings holds the OpenID Connect and KySignOn sync configuration.
type SSOSettings struct {
	Enabled       bool   `json:"enabled"`
	IssuerURL     string `json:"issuerUrl"`
	ClientID      string `json:"clientId"`
	ClientSecret  string `json:"clientSecret,omitempty"`
	RedirectURI   string `json:"redirectUri,omitempty"`
	AutoProvision bool   `json:"autoProvision"`
	HMACSecret    string `json:"hmacSecret,omitempty"`
}

// Store manages SSOSettings loaded and persisted to the SQLite server_settings table.
type Store struct {
	mu       sync.RWMutex
	db       *sql.DB
	cached   SSOSettings
	inMemory bool
}

// NewStore initializes a new Store backed by the SQLite database.
func NewStore(db *sql.DB) *Store {
	s := &Store{db: db}
	_ = s.Reload()
	return s
}

// Reload re-reads settings from the database.
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil
	}

	var enabledStr, issuerURL, clientID, clientSecret, redirectURI, autoProvStr, hmacSecret string
	_ = s.db.QueryRow(`SELECT value FROM server_settings WHERE key='sso_enabled'`).Scan(&enabledStr)
	_ = s.db.QueryRow(`SELECT value FROM server_settings WHERE key='sso_issuer_url'`).Scan(&issuerURL)
	_ = s.db.QueryRow(`SELECT value FROM server_settings WHERE key='sso_client_id'`).Scan(&clientID)
	_ = s.db.QueryRow(`SELECT value FROM server_settings WHERE key='sso_client_secret'`).Scan(&clientSecret)
	_ = s.db.QueryRow(`SELECT value FROM server_settings WHERE key='sso_redirect_uri'`).Scan(&redirectURI)
	_ = s.db.QueryRow(`SELECT value FROM server_settings WHERE key='sso_auto_provision'`).Scan(&autoProvStr)
	_ = s.db.QueryRow(`SELECT value FROM server_settings WHERE key='sso_hmac_secret'`).Scan(&hmacSecret)

	s.cached = SSOSettings{
		Enabled:       enabledStr == "true" || enabledStr == "1",
		IssuerURL:     issuerURL,
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		RedirectURI:   redirectURI,
		AutoProvision: autoProvStr == "" || autoProvStr == "true" || autoProvStr == "1",
		HMACSecret:    hmacSecret,
	}
	return nil
}

// Load returns the current SSOSettings.
func (s *Store) Load() SSOSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cached
}

// Save persists new SSOSettings to the database.
func (s *Store) Save(settings SSOSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		s.cached = settings
		return nil
	}

	enabledVal := "false"
	if settings.Enabled {
		enabledVal = "true"
	}
	autoProvVal := "false"
	if settings.AutoProvision {
		autoProvVal = "true"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	upsert := func(key, value string) error {
		_, err := tx.Exec(`INSERT INTO server_settings(key, value, updated_at) VALUES(?, ?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, key, value, now)
		return err
	}

	if err := upsert("sso_enabled", enabledVal); err != nil {
		return err
	}
	if err := upsert("sso_issuer_url", settings.IssuerURL); err != nil {
		return err
	}
	if err := upsert("sso_client_id", settings.ClientID); err != nil {
		return err
	}
	if err := upsert("sso_client_secret", settings.ClientSecret); err != nil {
		return err
	}
	if err := upsert("sso_redirect_uri", settings.RedirectURI); err != nil {
		return err
	}
	if err := upsert("sso_auto_provision", autoProvVal); err != nil {
		return err
	}
	if err := upsert("sso_hmac_secret", settings.HMACSecret); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.cached = settings
	return nil
}

// DiscoveryDoc represents the OpenID Provider Configuration document.
type DiscoveryDoc struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// DiscoverEndpoints fetches the OpenID configuration from the issuer URL.
func DiscoverEndpoints(ctx context.Context, issuerURL string) (*DiscoveryDoc, error) {
	issuerURL = strings.TrimRight(issuerURL, "/")
	wellKnown := issuerURL + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discovery request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned HTTP %d", resp.StatusCode)
	}

	var doc DiscoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}

	return &doc, nil
}

// GeneratePKCE creates a code_verifier and code_challenge (S256).
func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

// GenerateState generates a random 16-byte hex state parameter.
func GenerateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// TokenResponse represents the token response from the authorization server.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ExchangeCode exchanges an authorization code for tokens.
func ExchangeCode(ctx context.Context, tokenEndpoint, clientID, clientSecret, code, redirectURI, codeVerifier string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var tok TokenResponse
	if err := json.Unmarshal(bodyBytes, &tok); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tok, nil
}

// Claims represents standard OpenID Connect claims.
type Claims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Username      string `json:"preferred_username"`
	Role          string `json:"role"`
}

// ParseClaims parses claims from an ID token (unverified JWS payload) or userinfo endpoint.
func ParseClaims(ctx context.Context, idToken, accessToken, userinfoEndpoint string) (*Claims, error) {
	var claims Claims

	if idToken != "" {
		parts := strings.Split(idToken, ".")
		if len(parts) >= 2 {
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err == nil {
				_ = json.Unmarshal(payload, &claims)
			}
		}
	}

	if (claims.Subject == "" || claims.Email == "") && userinfoEndpoint != "" && accessToken != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, userinfoEndpoint, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+accessToken)
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var uClaims Claims
				if err := json.NewDecoder(resp.Body).Decode(&uClaims); err == nil {
					if claims.Subject == "" {
						claims.Subject = uClaims.Subject
					}
					if claims.Email == "" {
						claims.Email = uClaims.Email
					}
					if claims.Name == "" {
						claims.Name = uClaims.Name
					}
					if claims.Username == "" {
						claims.Username = uClaims.Username
					}
					if claims.Role == "" {
						claims.Role = uClaims.Role
					}
				}
			}
		}
	}

	if claims.Subject == "" {
		return nil, errors.New("no subject claim found in ID token or userinfo")
	}

	if claims.Username == "" {
		if claims.Email != "" {
			claims.Username = strings.Split(claims.Email, "@")[0]
		} else {
			claims.Username = claims.Subject
		}
	}

	return &claims, nil
}

// SystemPairingRequest represents the registration payload sent to KySignOn.
type SystemPairingRequest struct {
	PairingToken string `json:"pairingToken"`
	SystemName   string `json:"systemName"`
	SystemType   string `json:"systemType"`
	CallbackURL  string `json:"callbackUrl"`
}

// SystemPairingResponse represents KySignOn's response to pairing registration.
type SystemPairingResponse struct {
	SystemID   string `json:"systemId"`
	HMACSecret string `json:"hmacSecret"`
	Status     string `json:"status"`
}

// PairWithKySignOn redeems a 90s pairing token against KySignOn to automatically obtain credentials.
func PairWithKySignOn(ctx context.Context, issuerURL, pairingToken, callbackURL string) (*SystemPairingResponse, error) {
	issuerURL = strings.TrimRight(issuerURL, "/")
	registerURL := issuerURL + "/api/systems/register"

	payload := SystemPairingRequest{
		PairingToken: pairingToken,
		SystemName:   "KyNotes",
		SystemType:   "kynotes",
		CallbackURL:  callbackURL,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pairing request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pairing failed (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var pairResp SystemPairingResponse
	if err := json.Unmarshal(bodyBytes, &pairResp); err != nil {
		return nil, fmt.Errorf("failed to parse pairing response: %w", err)
	}

	return &pairResp, nil
}
