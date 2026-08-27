package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"presentation-raffle/internal/domain/entity"
	"presentation-raffle/internal/infrastructure/config"
)

type Pending struct {
	State, Verifier string
	ExpiresAt       time.Time
}
type CommonID struct {
	cfg     config.Config
	http    *http.Client
	mu      sync.Mutex
	pending map[string]Pending
}

type SessionStatus struct {
	CommonUserID string `json:"common_user_id"`
	Email        string `json:"email"`
	Status       string `json:"status"`
}

func NewCommonID(cfg config.Config) *CommonID {
	return &CommonID{cfg: cfg, http: http.DefaultClient, pending: make(map[string]Pending)}
}

func randomString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (c *CommonID) AuthorizeURL(intent string) (string, error) {
	state, err := randomString()
	if err != nil {
		return "", err
	}
	verifier, err := randomString()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.pending[state] = Pending{State: state, Verifier: verifier, ExpiresAt: time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()
	q := url.Values{"client_id": {c.cfg.CommonIDClientID}, "redirect_uri": {c.cfg.CommonIDRedirectURI}, "response_type": {"code"}, "scope": {"openid profile email"}, "state": {state}, "code_challenge": {challenge(verifier)}, "code_challenge_method": {"S256"}, "intent": {intent}}
	return strings.TrimRight(c.cfg.CommonIDOrigin, "/") + "/auth?" + q.Encode(), nil
}

func (c *CommonID) Exchange(ctx context.Context, values url.Values) (entity.AuthenticatedUser, error) {
	state, code := values.Get("state"), values.Get("code")
	if state == "" || code == "" {
		return entity.AuthenticatedUser{}, fmt.Errorf("authorization callback is incomplete")
	}
	c.mu.Lock()
	pending, ok := c.pending[state]
	if ok {
		delete(c.pending, state)
	}
	c.mu.Unlock()
	if !ok || time.Now().After(pending.ExpiresAt) {
		return entity.AuthenticatedUser{}, fmt.Errorf("invalid or expired authorization state")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {c.cfg.CommonIDClientID}, "redirect_uri": {c.cfg.CommonIDRedirectURI}, "code_verifier": {pending.Verifier}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.CommonIDAPIOrigin, "/")+"/v1/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return entity.AuthenticatedUser{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-API-Key", c.cfg.CommonIDAPIKey)
	res, err := c.http.Do(req)
	if err != nil {
		return entity.AuthenticatedUser{}, fmt.Errorf("common id token exchange: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return entity.AuthenticatedUser{}, fmt.Errorf("common id token exchange failed: status %d", res.StatusCode)
	}
	var payload struct {
		UserID string `json:"common_user_id"`
		Email  string `json:"email"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return entity.AuthenticatedUser{}, err
	}
	if payload.UserID == "" {
		return entity.AuthenticatedUser{}, fmt.Errorf("common id response has no user id")
	}
	return entity.AuthenticatedUser{UID: payload.UserID, Email: payload.Email, Provider: "common-id", EmailVerified: false}, nil
}

// CheckSession validates the browser's Common ID session from the application
// backend. The session token is never returned to browser code or logged.
func (c *CommonID) CheckSession(ctx context.Context, sessionToken string) (SessionStatus, error) {
	if sessionToken == "" {
		return SessionStatus{}, fmt.Errorf("common id session is missing")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.cfg.CommonIDAPIOrigin, "/")+"/v1/oauth/session", nil)
	if err != nil {
		return SessionStatus{}, err
	}
	req.Header.Set("X-API-Key", c.cfg.CommonIDAPIKey)
	req.Header.Set("X-Client-ID", c.cfg.CommonIDClientID)
	req.AddCookie(&http.Cookie{Name: "common_id_session", Value: sessionToken})
	res, err := c.http.Do(req)
	if err != nil {
		return SessionStatus{}, fmt.Errorf("common id session check: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return SessionStatus{}, fmt.Errorf("common id session is invalid: status %d", res.StatusCode)
	}
	var status SessionStatus
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		return SessionStatus{}, err
	}
	if status.Status != "active" || status.CommonUserID == "" {
		return SessionStatus{}, fmt.Errorf("common id session is invalid")
	}
	return status, nil
}

func (c *CommonID) LogoutURL() (string, error) {
	state, err := randomString()
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.pending[state] = Pending{State: state, ExpiresAt: time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()
	q := url.Values{"client_id": {c.cfg.CommonIDClientID}, "post_logout_redirect_uri": {c.cfg.CommonIDLogoutRedirectURI}, "state": {state}}
	return strings.TrimRight(c.cfg.CommonIDOrigin, "/") + "/logout?" + q.Encode(), nil
}

func (c *CommonID) ValidateLogout(values url.Values) error {
	state := values.Get("state")
	c.mu.Lock()
	pending, ok := c.pending[state]
	if ok {
		delete(c.pending, state)
	}
	c.mu.Unlock()
	if !ok || time.Now().After(pending.ExpiresAt) || (values.Get("logout") != "success" && values.Get("result") != "success") {
		return fmt.Errorf("common id logout failed")
	}
	return nil
}
