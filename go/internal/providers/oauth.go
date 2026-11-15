package providers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

type oauthFlow struct {
	state, verifier, redirect string
	expires                   time.Time
	generation                uint64
}

func randomURL() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (m *Manager) clientID() string {
	if m.opts.FrontierClientID != "" {
		return m.opts.FrontierClientID
	}
	return FrontierClientID
}
func (m *Manager) BeginAuthorization(base string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states["frontier"]
	if m.closed {
		return "", errors.New("Provider service closed")
	}
	if m.clientID() == "" {
		return "", errors.New("Frontier developer application approval required")
	}
	if !s.config.Read || s.config.Paused {
		return "", errors.New("Enable Frontier read consent first")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return "", errors.New("callback must use the existing loopback service")
	}
	state, err := randomURL()
	if err != nil {
		return "", err
	}
	verifier, err := randomURL()
	if err != nil {
		return "", err
	}
	redirect := base + "/owner/frontier-callback"
	m.oauth = &oauthFlow{state: state, verifier: verifier, redirect: redirect, expires: m.opts.Now().Add(5 * time.Minute), generation: s.generation}
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{"audience": {"frontier"}, "scope": {"auth capi"}, "response_type": {"code"}, "client_id": {m.clientID()}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}, "state": {state}, "redirect_uri": {redirect}}
	return "https://auth.frontierstore.net/auth?" + q.Encode(), nil
}
func (m *Manager) CompleteAuthorization(ctx context.Context, redirect, state, code string) error {
	m.mu.Lock()
	s := m.states["frontier"]
	f := m.oauth
	if f == nil || subtle.ConstantTimeCompare([]byte(state), []byte(f.state)) != 1 || redirect != f.redirect || !m.opts.Now().Before(f.expires) || s.generation != f.generation || !s.config.Read || s.config.Paused || code == "" || len(code) > 2048 {
		m.mu.Unlock()
		return errors.New("Invalid or expired Frontier authorization; start again")
	}
	m.oauth = nil
	generation := s.generation
	sessionCtx := s.ctx
	m.mu.Unlock()
	tokenCtx, cancel := context.WithTimeout(sessionCtx, 8*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {f.verifier}, "redirect_uri": {redirect}, "client_id": {m.clientID()}}
	raw, status, err := m.request(tokenCtx, "POST", "https://auth.frontierstore.net/token", "application/x-www-form-urlencoded", []byte(form.Encode()), "")
	if err != nil || status != 200 {
		return errors.New("Frontier token exchange failed; approval and registered callback must match")
	}
	return m.storeTokens(raw, generation, "")
}
func (m *Manager) storeTokens(raw []byte, generation uint64, previousRefresh string) error {
	var t struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Type    string `json:"token_type"`
		Expires int    `json:"expires_in"`
	}
	if json.Unmarshal(raw, &t) != nil || t.Access == "" || len(t.Access) > 8192 || len(t.Refresh) > 8192 || !strings.EqualFold(t.Type, "Bearer") || t.Expires < 1 || t.Expires > 172800 {
		return errors.New("Invalid Frontier token response")
	}
	if t.Refresh == "" {
		t.Refresh = previousRefresh
	}
	if t.Refresh == "" {
		return errors.New("Frontier did not provide a refresh token")
	}
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states["frontier"]
	if m.closed || generation != s.generation || !s.config.Read || s.config.Paused {
		return errors.New("Frontier consent was revoked")
	}
	next := map[string]Credential{}
	for k, v := range m.secrets {
		next[k] = v
	}
	next["frontier"] = Credential{AccessToken: t.Access, RefreshToken: t.Refresh, ExpiresAt: m.opts.Now().Add(time.Duration(t.Expires) * time.Second)}
	m.mu.Unlock()
	vaultErr := m.opts.Vault.Save(next)
	m.mu.Lock()
	if vaultErr != nil {
		return errors.New("Cannot save Frontier tokens in OS vault")
	}
	m.secrets = next
	return nil
}
