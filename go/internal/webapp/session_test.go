package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ownerClient(t *testing.T, s *Server) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 3 * time.Second}
	req, _ := http.NewRequest("POST", s.URL()+"/auth/session", strings.NewReader(`{"role":"viewer"}`))
	req.Header.Set("Origin", s.URL())
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("bootstrap: %d", response.StatusCode)
	}
	var body struct {
		Role      string `json:"role"`
		Prototype struct {
			Stage   string `json:"stage"`
			Offline bool   `json:"offline"`
		} `json:"prototype"`
	}
	expectedStage := "G1A"
	if s.remote != nil {
		expectedStage = "G1B"
	}
	if s.game != nil {
		expectedStage = "G2A"
		if s.g2b {
			expectedStage = "G2B"
			if s.g2c {
				expectedStage = "G2C"
				if s.g2d {
					expectedStage = "G2D"
				}
			}
		}
	}
	if json.NewDecoder(response.Body).Decode(&body) != nil || body.Role != "owner" || body.Prototype.Stage != expectedStage || body.Prototype.Offline != (s.game == nil) {
		t.Fatalf("incorrect bootstrap: %+v", body)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].MaxAge != 43200 || !strings.HasPrefix(cookies[0].Name, "daedalus_g1a_") {
		t.Fatalf("insecure cookie attributes: %+v", cookies)
	}
	return client
}

func TestLocalOwnerBootstrap(t *testing.T) {
	s, err := StartLocal(context.Background(), fixtureFS())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	ownerClient(t, s)
	for _, tc := range []struct{ name, method, host, origin, peer, forwarded string }{
		{"missing origin", "POST", "", "", "127.0.0.1:42", ""},
		{"cross origin", "POST", "", "https://evil.example", "127.0.0.1:42", ""},
		{"null origin", "POST", "", "null", "127.0.0.1:42", ""},
		{"wrong method", "GET", "", "SAME", "127.0.0.1:42", ""},
		{"rebound host", "POST", "evil.example", "http://evil.example", "127.0.0.1:42", ""},
		{"LAN peer", "POST", "", "SAME", "192.0.2.5:42", "127.0.0.1"},
		{"proxy headers", "POST", "", "SAME", "127.0.0.1:42", "192.0.2.5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, s.URL()+"/auth/session", nil)
			req.RemoteAddr = tc.peer
			if tc.host != "" {
				req.Host = tc.host
			}
			origin := tc.origin
			if origin == "SAME" {
				origin = s.URL()
			}
			if origin != "" {
				req.Header.Set("Origin", origin)
			}
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			response := httptest.NewRecorder()
			s.http.Handler.ServeHTTP(response, req)
			if response.Code != 403 || len(response.Result().Cookies()) != 0 {
				t.Fatalf("untrusted bootstrap accepted: %d", response.Code)
			}
		})
	}
}

func TestSessionExpiryAndBound(t *testing.T) {
	now := time.Unix(0, 0)
	store := newSessionStore("test", func() time.Time { return now })
	token, err := store.issue()
	if err != nil || !store.valid(token) {
		t.Fatal("session not issued")
	}
	now = now.Add(12 * time.Hour)
	if store.valid(token) {
		t.Fatal("expired session accepted")
	}
	for range 128 {
		if _, err := store.issue(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.issue(); err == nil {
		t.Fatal("unbounded sessions")
	}
	store.clear()
	if store.valid(token) {
		t.Fatal("revoked session accepted")
	}
	if _, err := store.issue(); err == nil {
		t.Fatal("shutdown store issued a new session")
	}
}
