package gamestate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/journal"
)

func sourceFile(dir string) string { return filepath.Join(dir, "Journal.20260101.01.log") }
func appendEvent(t *testing.T, dir, text string) {
	t.Helper()
	f, err := os.OpenFile(sourceFile(dir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.WriteString(text + "\n"); err != nil {
		t.Fatal(err)
	}
}
func waitState(t *testing.T, s *Service, p func(Summary) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if p(s.Snapshot()) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("state timed out: %+v", s.Snapshot())
}
func startService(t *testing.T, dir string) *Service {
	t.Helper()
	s, err := NewService(Options{Directory: dir, Discovery: journal.DiscoveryOptions{Roots: []string{}}, PollInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("service failed to stop")
		}
	})
	return s
}
func TestGameStateTwoFIDSessionsDoNotMix(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture A","SystemAddress":9223372036854775807}`)
	s := startService(t, dir)
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture A" })
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-B"}`)
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 3 && v.CurrentSystem == "" })
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture B"}`)
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture B" })
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture A" })
	appendEvent(t, dir, `{"event":"LoadGame"}`)
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 6 && v.CurrentSystem == "" })
	raw, _ := json.Marshal(s.Snapshot())
	if strings.Contains(string(raw), "SYNTHETIC") || strings.Contains(string(raw), dir) || strings.Contains(string(raw), "FID") {
		t.Fatal("private state escaped summary")
	}
	s.mu.Lock()
	id := s.commanders["SYNTHETIC-A"].systemAddress
	s.mu.Unlock()
	if id != "9223372036854775807" {
		t.Fatal("state lost exact identifier")
	}
}
func TestCurrentSystemOnlyUpdatesAfterArrival(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture Alpha"}`)
	s := startService(t, dir)
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture Alpha" })
	appendEvent(t, dir, `{"event":"StartJump","StarSystem":"Fixture Beta"}`)
	waitState(t, s, func(v Summary) bool { return v.LastEventType == "StartJump" })
	if s.Snapshot().CurrentSystem != "Fixture Alpha" {
		t.Fatal("departure treated as arrival")
	}
	appendEvent(t, dir, `{"event":"CarrierJump","StarSystem":"Fixture Gamma"}`)
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture Gamma" })
	appendEvent(t, dir, `{"event":"ReceiveText","Message":"DO_NOT_EXPORT_RAW_CHAT","FID":"SYNTHETIC-SECRET","StarSystem":"NOT_AN_ARRIVAL"}`)
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 4 })
	if v := s.Snapshot(); v.CurrentSystem != "Fixture Gamma" || v.LastEventType != "CarrierJump" {
		t.Fatal("unsafe event altered public status", v)
	}
	appendEvent(t, dir, `{"event":"Shutdown"}`)
	waitState(t, s, func(v Summary) bool { return v.SourceState == "waiting_game" })
}
func TestMissingSourceGivesHonestStatus(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-yet-created")
	s := startService(t, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.SourceState == "waiting" })
	if s.Snapshot().CurrentSystem != "" {
		t.Fatal("invented system without game")
	}
	os.MkdirAll(dir, 0700)
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture Late"}`)
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture Late" })
}
func TestGameServiceDirectorySwitchAndSingleReader(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	appendEvent(t, a, `{"event":"Location","StarSystem":"Fixture A"}`)
	appendEvent(t, b, `{"event":"Location","StarSystem":"Fixture B"}`)
	s := startService(t, a)
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture A" })
	if err := s.SetDirectory(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture B" })
	appendEvent(t, a, `{"event":"Location","StarSystem":"WRONG_OLD_READER"}`)
	time.Sleep(30 * time.Millisecond)
	if s.Snapshot().CurrentSystem != "Fixture B" || s.Details().ReaderStarts != 2 || s.Details().ActiveReaders != 1 {
		t.Fatal("directory change left a duplicate reader")
	}
	if err := s.SetDirectory(context.Background(), filepath.Join(b, "missing")); err == nil {
		t.Fatal("invalid owner directory accepted")
	}
}
func TestGameServiceReplayAndSubscribersAreBounded(t *testing.T) {
	dir := t.TempDir()
	var text strings.Builder
	for range 2000 {
		text.WriteString("{\"event\":\"Location\",\"StarSystem\":\"Fixture Replay\"}\n")
	}
	os.WriteFile(sourceFile(dir), []byte(text.String()), 0600)
	s := startService(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, unsubscribe := s.Subscribe(ctx)
	defer unsubscribe()
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 2000 && !v.ImportInProgress })
	if len(events) > 1 {
		t.Fatal("history flooded subscriber")
	}
	if s.Details().ReaderStarts != 1 {
		t.Fatal("subscriber created source reader")
	}
	for range 127 {
		_, stop := s.Subscribe(ctx)
		defer stop()
	}
	full, stop := s.Subscribe(ctx)
	defer stop()
	select {
	case _, ok := <-full:
		if ok {
			t.Fatal("subscriber bound not enforced")
		}
	default:
		t.Fatal("overflow subscription was not rejected")
	}
}

func TestGameConfigurationErrorReachesEarlySubscriber(t *testing.T) {
	s, _ := NewService(Options{ConfigError: "damaged preference", PollInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes, stop := s.Subscribe(ctx)
	defer stop()
	<-changes
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	defer func() { cancel(); <-done }()
	waitState(t, s, func(v Summary) bool { return v.SourceState == "source_error" })
	select {
	case v := <-changes:
		if v.SourceState != "source_error" {
			t.Fatal("subscriber retained misleading waiting status", v)
		}
	case <-time.After(time.Second):
		t.Fatal("source error was not broadcast")
	}
}

func TestGameRecoveredReaderClearsOwnerDiagnostic(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture Recovered"}`)
	s := startService(t, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.SourceState == "live" })
	// Model the retained diagnostic from a previous failed poll, then let the
	// real healthy reader's next poll reconcile it.
	s.mu.Lock()
	s.details.Error = "journal source unreadable"
	s.mu.Unlock()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.Details().Error == "" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("recovered source retained obsolete owner error")
}
