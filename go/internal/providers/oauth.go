package providers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type oauthFlow struct {
	state, verifier, redirect, receiver, clientID string
	expires                                       time.Time
	generation                                    uint64
}

func randomURL() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (m *Manager) clientID() string {
	var local struct {
		ClientID string `json:"clientId"`
	}
	if raw, err := privatefs.Read(m.dir, "frontier-client.json"); err == nil {
		if json.Unmarshal(raw, &local) != nil || !validClientID(local.ClientID) {
			return ""
		}
		return local.ClientID
	} else if !os.IsNotExist(err) {
		return ""
	}
	if m.opts.FrontierClientID != "" {
		return m.opts.FrontierClientID
	}
	return FrontierClientID
}
func (m *Manager) BeginAuthorization(base string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states["frontier"]
	if m.closed || (m.opts.RequireHistory && m.history == nil) {
		return "", errors.New("Provider service closed")
	}
	if m.clientID() == "" {
		return "", errors.New("Frontier developer application approval required")
	}
	if m.opts.RequireHistory && (m.accountFID == "" || m.accountFID != m.fid) {
		return "", errors.New("Start the selected journal commander before connecting Frontier")
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
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1024 || port > 65535 {
		return "", errors.New("Invalid loopback receiver port")
	}
	state = "erid1." + strconv.Itoa(port) + "." + state
	redirect := FrontierRedirectURI
	m.oauth = &oauthFlow{state: state, verifier: verifier, redirect: redirect, clientID: m.clientID(), receiver: base + "/owner/frontier-callback", expires: m.opts.Now().Add(5 * time.Minute), generation: s.generation}
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{"audience": {"frontier"}, "scope": {"auth capi"}, "response_type": {"code"}, "client_id": {m.clientID()}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}, "state": {state}, "redirect_uri": {redirect}}
	return "https://auth.frontierstore.net/auth?" + q.Encode(), nil
}
func (m *Manager) CompleteAuthorization(ctx context.Context, redirect, state, code string) error {
	m.mu.Lock()
	s := m.states["frontier"]
	f := m.oauth
	if f == nil || subtle.ConstantTimeCompare([]byte(state), []byte(f.state)) != 1 || redirect != f.receiver || !m.opts.Now().Before(f.expires) || s.generation != f.generation || !s.config.Read || s.config.Paused || code == "" || len(code) > 2048 {
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
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {f.verifier}, "redirect_uri": {f.redirect}, "client_id": {f.clientID}}
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
	next["frontier"] = Credential{JournalFID: m.accountFID, AccessToken: t.Access, RefreshToken: t.Refresh, ExpiresAt: m.opts.Now().Add(time.Duration(t.Expires) * time.Second)}
	m.mu.Unlock()
	vaultErr := m.opts.Vault.Save(next)
	m.mu.Lock()
	if vaultErr != nil {
		return errors.New("Cannot save Frontier tokens in OS vault")
	}
	m.secrets = next
	return nil
}

// A public client ID is configurable locally, never supplied by paired displays.
func validClientID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, c := range v {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func (m *Manager) CancelAuthorization(receiver, state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.oauth
	if f == nil || f.receiver != receiver || subtle.ConstantTimeCompare([]byte(state), []byte(f.state)) != 1 {
		return errors.New("Invalid authorization state")
	}
	m.oauth = nil
	return nil
}

func (m *Manager) ConfigureFrontierClient(id string) error {
	if !validClientID(id) {
		return errors.New("Enter the registered public client UUID, never a shared secret")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, _ := json.Marshal(map[string]string{"clientId": id})
	if e := privatefs.Write(m.dir, "frontier-client.json", raw); e != nil {
		return errors.New("Cannot save private Frontier client settings")
	}
	m.oauth = nil
	return nil
}

func (m *Manager) verifyFrontierAccount(generation uint64, name string) error {
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.states["frontier"].generation != generation || name == "" || m.name != name || m.accountFID != m.fid {
		return errors.New("Frontier account changed during verification")
	}
	next := map[string]Credential{}
	for k, c := range m.secrets {
		next[k] = c
	}
	c := next["frontier"]
	c.Commander = name
	c.JournalFID = m.accountFID
	next["frontier"] = c
	m.mu.Unlock()
	e := m.opts.Vault.Save(next)
	m.mu.Lock()
	if e != nil {
		return errors.New("Cannot persist verified Frontier association")
	}
	m.secrets = next
	return nil
}
