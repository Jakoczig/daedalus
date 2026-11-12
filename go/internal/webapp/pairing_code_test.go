package webapp

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReplacementCannotReissueCurrentCodeOnRandomCollision(t *testing.T) {
	s, owner, client, remoteURL, _ := remoteFixture(t)
	s.remote.mu.Lock()
	s.remote.pairing.Hash = tokenHash("7K4M9P")
	s.remote.mu.Unlock()
	// First draw repeats the current code; the second is independently sampled.
	previous := rand.Reader
	defer func() { rand.Reader = previous }()
	rand.Reader = bytes.NewReader([]byte{7, 19, 4, 20, 9, 22, 8, 19, 4, 20, 9, 22})
	issued := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	rand.Reader = previous
	var code string
	json.Unmarshal(issued["code"], &code)
	if code != "8K4M9P" {
		t.Fatal("replacement reused the pending code")
	}
	status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": "7K4M9P"})
	if status != 403 {
		t.Fatal("old code survived replacement", status)
	}
}

func TestHumanPairingCodeAndUnchangedViewerSecret(t *testing.T) {
	s, owner, _, _, _ := remoteFixture(t)
	for range 64 {
		before := time.Now()
		issued := ownerAction(t, s, owner, map[string]any{"action": "pair"})
		var code string
		var expires time.Time
		json.Unmarshal(issued["code"], &code)
		json.Unmarshal(issued["expires"], &expires)
		if len(code) != 6 || strings.Trim(code, "0123456789ABCDEFGHJKMNPQRSTVWXYZ") != "" {
			t.Fatalf("not a six-character typo-resistant uppercase code (length %d)", len(code))
		}
		if expires.Before(before.Add(5*time.Minute)) || expires.After(time.Now().Add(5*time.Minute)) {
			t.Fatal("pairing expiry is not five minutes")
		}
		s.remote.mu.Lock()
		hash := s.remote.pairing.Hash
		s.remote.mu.Unlock()
		if hash != tokenHash(code) || hash == code {
			t.Fatal("pending code is not hash-only")
		}
	}
}

func TestHumanPairingLowercaseOneUseAndViewerSecret(t *testing.T) {
	s, _, client, remoteURL, _ := remoteFixture(t)
	status, body, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil)
	if status != 401 || !strings.Contains(string(body), `"pairingCodeLength":6`) || strings.Contains(string(body), `"role"`) {
		t.Fatal("anonymous bootstrap lacks safe short-code hint", status)
	}
	// A known mixed letter/digit code makes lowercase admission deterministic.
	s.remote.mu.Lock()
	s.remote.pairing.Hash = tokenHash("7K4M9P")
	s.remote.mu.Unlock()
	status, body, cookies := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": "7k4m9p", "name": "Laptop", "role": "owner"})
	if status != 200 || !strings.Contains(string(body), `"role":"viewer"`) || len(cookies) != 1 {
		t.Fatal("lowercase code did not create a read-only viewer", status)
	}
	secret, err := hex.DecodeString(cookies[0].Value)
	if err != nil || len(secret) != 32 {
		t.Fatal("viewer cookie lost its 256-bit token")
	}
	status, _, _ = remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": "7K4M9P", "name": "Replay"})
	if status != 403 {
		t.Fatal("normalized code was reusable", status)
	}
}

func TestHumanPairingRejectsInvalidAlphabetAndLength(t *testing.T) {
	for _, code := range []string{"", "7K4M9", "7K4M9PP", "7I4M9P", "7L4M9P", "7O4M9P", "7U4M9P", "7K4M P", "7K4M-P", "７K4M9P"} {
		t.Run(code, func(t *testing.T) {
			s, _, client, remoteURL, _ := remoteFixture(t)
			// Even a matching hash must not bypass canonical alphabet/length validation.
			s.remote.mu.Lock()
			s.remote.pairing.Hash = tokenHash(strings.ToUpper(code))
			s.remote.mu.Unlock()
			status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code, "name": "Invalid"})
			if status != 403 {
				t.Fatal("invalid manual code accepted", status)
			}
		})
	}
}

func TestHumanPairingReplacementAndExactExpiry(t *testing.T) {
	s, owner, client, remoteURL, old := remoteFixture(t)
	issued := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	var code string
	var expires time.Time
	json.Unmarshal(issued["code"], &code)
	json.Unmarshal(issued["expires"], &expires)
	status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": old})
	if status != 403 {
		t.Fatal("replaced code survived", status)
	}
	s.remote.mu.Lock()
	s.remote.now = func() time.Time { return expires }
	s.remote.mu.Unlock()
	status, _, _ = remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code})
	if status != 403 {
		t.Fatal("code survived exact five-minute deadline", status)
	}
}

func TestPairingBudgetSurvivesIssuanceAndDifferentPeers(t *testing.T) {
	s, owner, client, remoteURL, _ := remoteFixture(t)
	current := time.Now()
	s.remote.mu.Lock()
	s.remote.now = func() time.Time { return current }
	s.remote.mu.Unlock()
	var code string
	for attempt := range 11 {
		issued := ownerAction(t, s, owner, map[string]any{"action": "pair"})
		json.Unmarshal(issued["code"], &code)
		submitted := "!!!!!!"
		if attempt == 10 {
			submitted = code
		}
		req := httptest.NewRequest("POST", remoteURL+"/auth/pair", strings.NewReader(`{"code":"`+submitted+`","name":"Guess"}`))
		req.TLS = &tls.ConnectionState{}
		req.RemoteAddr = []string{"192.168.1.20:12000", "192.168.1.21:12001"}[attempt%2]
		req.Header.Set("Origin", remoteURL)
		result := httptest.NewRecorder()
		s.remote.serve(s.remote.epoch, result, req)
		if result.Code != 403 {
			t.Fatal("global attempt cap bypassed by issuance or another peer", attempt, result.Code)
		}
	}
	s.remote.mu.Lock()
	current = current.Add(time.Minute)
	s.remote.mu.Unlock()
	status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code})
	if status != 200 {
		t.Fatal("global budget did not recover after one minute", status)
	}
}
