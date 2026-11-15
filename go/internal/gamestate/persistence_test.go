package gamestate

import (
	"context"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func durableService(t *testing.T, profile, dir string) (*Service, func()) {
	t.Helper()
	s, err := NewService(Options{StorageDir: profile, Directory: dir, EnableParity: true, Discovery: journal.DiscoveryOptions{Roots: []string{}}, PollInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	return s, func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("persistent service did not stop")
		}
	}
}
func TestG2JDurableRestartAndCatchup(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	appendEvent(t, dir, `{"timestamp":"2026-01-01T00:00:00Z","event":"Fileheader","gameversion":"4.0"}`)
	appendEvent(t, dir, `{"timestamp":"2026-01-01T00:00:01Z","event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture"}`)
	appendEvent(t, dir, `{"timestamp":"2026-01-01T00:00:02Z","event":"Location","StarSystem":"Fixture Alpha","SystemAddress":18446744073709551615}`)
	appendEvent(t, dir, `{"timestamp":"2026-01-01T00:00:03Z","event":"Loadout","ShipID":7,"Ship":"asp","ShipName":"PRIVATE_FIXTURE","UnladenMass":300,"Modules":[{"Slot":"PowerPlant","Item":"fixture_plant","On":true,"Engineering":{"Modifiers":[{"Label":"PowerCapacity","Value":30,"OriginalValue":20,"LessIsGood":false}]}}]}`)
	s, stop := durableService(t, profile, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 4 })
	before := s.Workspace()
	db, _ := s.History()
	if db.Stats().Events != 4 {
		t.Fatal("history incomplete")
	}
	stop()
	s, stop = durableService(t, profile, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 4 })
	if s.Snapshot().CurrentSystem != "Fixture Alpha" || len(s.Workspace().Parity.Modules) != len(before.Parity.Modules) {
		t.Fatalf("cache not restored: %+v", s.Snapshot())
	}
	if s.Details().ReaderBytes != 0 {
		t.Fatal("unchanged source reread")
	}
	if s.Snapshot().Stage != "WAITING_FOR_GAME" {
		t.Fatalf("unchanged cache falsely live: %+v", s.Snapshot())
	}
	stop()
	appendEvent(t, dir, `{"timestamp":"2026-01-02T00:00:00Z","event":"FSDJump","StarSystem":"Fixture Beta","SystemAddress":9007199254740993}`)
	s, stop = durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.CurrentSystem == "Fixture Beta" })
	db, _ = s.History()
	if db.Stats().Events != 5 || s.Snapshot().EventsRead != 5 {
		t.Fatalf("catch-up duplicated: %+v %+v", db.Stats(), s.Snapshot())
	}
}
func TestG2JFailedSourceValidationPreservesHistory(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	s, stop := durableService(t, profile, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 1 })
	stop()
	if err := os.WriteFile(sourceFile(dir), []byte("broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s, stop = durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return v.Stage == "DB_ERROR" })
	db, _ := s.History()
	if db.Stats().Events != 1 {
		t.Fatal("quarantine deleted history")
	}
}
func BenchmarkG2JLifecycle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dir := b.TempDir()
		profile := b.TempDir()
		f, e := os.Create(filepath.Join(dir, "Journal.20260101.01.log"))
		if e != nil {
			b.Fatal(e)
		}
		for j := 0; j < 100000; j++ {
			if j%50000 == 0 {
				fmt.Fprintf(f, "{\"event\":\"LoadGame\",\"FID\":\"SYNTHETIC-%d\",\"timestamp\":\"2026-01-01T00:00:00Z\"}\n", j/50000)
			} else {
				fmt.Fprintf(f, "{\"event\":\"MaterialCollected\",\"timestamp\":\"2026-01-01T00:00:00Z\",\"Name\":\"iron\",\"Category\":\"Raw\",\"Count\":1}\n")
			}
		}
		f.Close()
		s, _ := NewService(Options{StorageDir: profile, Directory: dir, EnableParity: true, PollInterval: time.Millisecond})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		started := time.Now()
		go func() { s.Run(ctx); close(done) }()
		for {
			v := s.Snapshot()
			if v.Stage == "DB_ERROR" {
				b.Fatal(v)
			}
			if !v.ImportInProgress && v.EventsRead == 100000 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		elapsed := time.Since(started)
		cancel()
		<-done
		b.ReportMetric(100000/elapsed.Seconds(), "events/s")
		b.ReportMetric(float64(elapsed.Microseconds())/1000, "cold-ms")
		warm, _ := NewService(Options{StorageDir: profile, Directory: dir, EnableParity: true, PollInterval: time.Millisecond})
		warmCtx, warmCancel := context.WithCancel(context.Background())
		warmDone := make(chan struct{})
		warmStarted := time.Now()
		go func() { warm.Run(warmCtx); close(warmDone) }()
		for {
			v := warm.Snapshot()
			if v.Stage == "DB_ERROR" {
				b.Fatal(v)
			}
			if !v.ImportInProgress && v.EventsRead == 100000 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		b.ReportMetric(float64(time.Since(warmStarted).Microseconds())/1000, "warm-ms")
		b.ReportMetric(float64(warm.Details().ReaderBytes), "warm-journal-bytes")
		warmCancel()
		<-warmDone

	}
}
