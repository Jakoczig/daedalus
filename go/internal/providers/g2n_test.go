package providers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestG2NFrontierSuppressesOverlappingInaraReads(t *testing.T) {
	calls := 0
	m := New(context.Background(), t.TempDir(), Options{Vault: &testVault{}, Manual: true, InaraAppName: "fixture", Client: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
	})}})
	defer m.Close()
	session(t, m)
	m.mu.Lock()
	m.states["inara"].config = Config{Read: true, CommunityGoals: true}
	m.states["frontier"].config = Config{Read: true}
	m.secrets["frontier"] = Credential{Commander: "Fixture CMDR", JournalFID: "SYNTHETIC-A", AccessToken: "SYNTHETIC", RefreshToken: "SYNTHETIC"}
	m.mu.Unlock()
	for _, kind := range []string{"profile", "communitygoals"} {
		_, err := m.Read(context.Background(), "inara", kind, "")
		if err == nil || !strings.Contains(err.Error(), "Frontier") {
			t.Fatalf("overlapping Inara %s not blocked: %v", kind, err)
		}
	}
	if calls != 0 {
		t.Fatal("redundant provider request", calls)
	}
}
