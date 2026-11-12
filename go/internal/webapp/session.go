package webapp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type sessionStore struct {
	mu         sync.Mutex
	tokens     map[string]time.Time
	cookieName string
	now        func() time.Time
	closed     bool
}

func newSessionStore(scope string, clock func() time.Time) *sessionStore {
	return &sessionStore{tokens: make(map[string]time.Time), cookieName: "daedalus_g1a_" + scope, now: clock}
}
func (s *sessionStore) issue() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("service stopped")
	}
	for token, expires := range s.tokens {
		if !s.now().Before(expires) {
			delete(s.tokens, token)
		}
	}
	if len(s.tokens) >= 128 {
		return "", errors.New("session limit")
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.New("session unavailable")
	}
	token := hex.EncodeToString(bytes)
	s.tokens[token] = s.now().Add(12 * time.Hour)
	return token, nil
}
func (s *sessionStore) valid(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, ok := s.tokens[token]
	return !s.closed && ok && s.now().Before(expires)
}
func (s *sessionStore) clear() { s.mu.Lock(); defer s.mu.Unlock(); s.closed = true; clear(s.tokens) }
func (s *sessionStore) token(r *http.Request) string {
	cookie, err := r.Cookie(s.cookieName)
	if err != nil || len(cookie.Value) != 64 {
		return ""
	}
	return cookie.Value
}

func (s *Server) trustedPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || r.Host != strings.TrimPrefix(s.url, "http://") {
		return false
	}
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
		if len(r.Header.Values(name)) > 0 {
			return false
		}
	}
	return true
}
func (s *Server) sameOrigin(r *http.Request) bool {
	return len(r.Header.Values("Origin")) == 1 && r.Header.Get("Origin") == s.url
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "POST" || !s.sameOrigin(r) {
		http.Error(w, "Forbidden", 403)
		return
	}
	if _, err := io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 4096)); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}
	token := s.sessions.token(r)
	if !s.sessions.valid(token) {
		var err error
		token, err = s.sessions.issue()
		if err != nil {
			http.Error(w, "Session unavailable", 503)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: s.sessions.cookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	w.Header().Set("Content-Type", "application/json")
	stage := "G1A"
	if s.remote != nil {
		stage = "G1B"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"role": "owner", "prototype": map[string]any{"stage": stage, "offline": true}})
}
