package providers

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type testVault struct{ v map[string]Credential }

func (v *testVault) Load() (map[string]Credential, error) { return v.v, nil }
func (v *testVault) Save(c map[string]Credential) error   { v.v = c; return nil }
func event(t *testing.T, raw string) journal.Event {
	t.Helper()
	e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture", Offset: 0, Sequence: 0})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func session(t *testing.T, m *Manager) {
	m.Observe(event(t, `{"event":"Fileheader","gameversion":"4.1.0.0","build":"fixture"}`), journal.Replay)
	m.Observe(event(t, `{"event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture CMDR"}`), journal.Replay)
}
func TestG2HNoDefaultOrReplayUploadsAndDurableDedupe(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	calls := 0
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "PRIVATE") {
			t.Fatal("private fields escaped")
		}
		var env map[string]any
		if json.Unmarshal(raw, &env) != nil {
			t.Fatal("invalid envelope")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}}, nil
	})}
	dir := t.TempDir()
	opts := Options{Client: client, Vault: &testVault{}, Now: func() time.Time { return now }, Manual: true}
	m := New(context.Background(), dir, opts)
	defer m.Close()
	session(t, m)
	e := event(t, `{"timestamp":"2026-11-14T12:00:00Z","event":"FSDJump","StarSystem":"Fixture","SystemAddress":9007199254740993,"StarPos":[1,2,3],"Commander":"PRIVATE","Latitude":1,"FuelLevel":5}`)
	m.Observe(e, journal.Live)
	m.dispatch("eddn")
	if calls != 0 {
		t.Fatal("default upload")
	}
	if err := m.Configure("eddn", Config{Upload: true, Categories: []string{"travel"}}); err != nil {
		t.Fatal(err)
	}
	m.Observe(e, journal.Replay)
	m.dispatch("eddn")
	if calls != 0 {
		t.Fatal("replay upload")
	}
	m.Observe(e, journal.Live)
	m.dispatch("eddn")
	if calls != 1 {
		t.Fatalf("live calls %d", calls)
	}
	m.Observe(e, journal.Live)
	m.dispatch("eddn")
	if calls != 1 {
		t.Fatal("duplicate")
	}
	m.Close()
	m = New(context.Background(), dir, opts)
	session(t, m)
	m.Observe(e, journal.Live)
	m.dispatch("eddn")
	if calls != 1 {
		t.Fatal("restart duplicate")
	}
}
func TestG2HApprovalGateAndSafeCredentialStatus(t *testing.T) {
	m := New(context.Background(), t.TempDir(), Options{Vault: &testVault{}, Manual: true})
	defer m.Close()
	session(t, m)
	if err := m.SetCredential("inara", Credential{APIKey: "SECRET", Commander: "Fixture CMDR"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Configure("inara", Config{Read: true, Upload: true, Categories: []string{"travel"}}); err == nil {
		t.Fatal("unapproved application enabled")
	}
	raw, _ := json.Marshal(m.Status())
	if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "Fixture CMDR") {
		t.Fatal("credential exposed")
	}
}
func TestG2HDisableCancelsReservedOutbound(t *testing.T) {
	started := make(chan struct{})
	done := make(chan struct{})
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		close(done)
		return nil, r.Context().Err()
	})}
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	m := New(context.Background(), t.TempDir(), Options{Client: client, Vault: &testVault{}, Manual: true, Now: func() time.Time { return now }})
	defer m.Close()
	session(t, m)
	if err := m.Configure("eddn", Config{Upload: true, Categories: []string{"travel"}}); err != nil {
		t.Fatal(err)
	}
	m.Observe(event(t, `{"timestamp":"2026-11-14T12:00:00Z","event":"FSDJump","StarSystem":"Fixture","SystemAddress":1,"StarPos":[1,2,3]}`), journal.Live)
	go m.dispatch("eddn")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("not dispatched")
	}
	if err := m.Configure("eddn", Config{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request not cancelled")
	}
	if m.Status()[3].Queue != 0 {
		t.Fatal("disabled queue retained")
	}
}
