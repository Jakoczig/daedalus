package gamestate

import (
	"context"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
func TestG2JGameLaunchAfterEmptyStartupBecomesFreshLive(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	s, stop := durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.Stage == "WAITING_FOR_GAME" })
	stamp := time.Now().UTC().Add(time.Millisecond).Format(time.RFC3339Nano)
	raw := fmt.Sprintf("{\"event\":\"Fileheader\",\"timestamp\":%q,\"gameversion\":\"4.0\"}\n{\"event\":\"LoadGame\",\"timestamp\":%q,\"FID\":\"SYNTHETIC\"}\n{\"event\":\"Location\",\"timestamp\":%q,\"StarSystem\":\"Fixture Fresh\",\"SystemAddress\":9007199254740993}\n", stamp, stamp, stamp)
	if err := os.WriteFile(filepath.Join(dir, "Journal.20261115.01.log"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, func(v Summary) bool { return v.CurrentSystem == "Fixture Fresh" && v.Stage == "LIVE" && !v.Cached })
}
func TestG2JWholeFileCopiesDoNotReplayButDifferentInteriorIsRetained(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	raw := `{"event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture"}` + "\n"
	for i := 0; i < 10; i++ {
		raw += fmt.Sprintf("{\"event\":\"MaterialCollected\",\"Name\":\"iron\",\"Category\":\"Raw\",\"Count\":%d}\n", i)
	}
	first := filepath.Join(dir, "Journal.20260101.01.log")
	if err := os.WriteFile(first, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	s, stop := durableService(t, profile, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 11 })
	stop()
	if err := os.WriteFile(filepath.Join(dir, "Journal.20260101.02.log"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(raw, `"Count":5`, `"Count":6`, 1)
	if changed == raw {
		t.Fatal("fixture did not change")
	}
	if err := os.WriteFile(filepath.Join(dir, "Journal.20260101.03.log"), []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	s, stop = durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 22 })
	db, _ := s.History()
	if db.Stats().Events != 22 || db.Stats().Files != 3 {
		t.Fatalf("copy handling %+v", db.Stats())
	}
}
func TestG2JActiveSourceSameSizeAndTimestampStillVerifiesFingerprint(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	s, stop := durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 1 })
	path := sourceFile(dir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(raw), "SYNTHETIC-A", "SYNTHETIC-B", 1)
	if err = os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, func(v Summary) bool { return v.Stage == "DB_ERROR" })
	db, _ := s.History()
	if db.Stats().Events != 1 {
		t.Fatal("changed source silently reimported")
	}
}
func TestG2JAcceptedRouteSidecarSurvivesOfflineRestartWithoutAnotherJournalEvent(t *testing.T) {
	dir := g2bSource(t)
	profile := t.TempDir()
	s, stop := durableService(t, profile, dir)
	waitProjection(t, s, func() bool { return len(s.Navigation().Hops) == 2 })
	stop()
	if err := os.Rename(dir, dir+"-moved"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(dir+"-moved", dir) })
	s, stop = durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return v.Cached && !v.ImportInProgress })
	route := s.Navigation()
	if len(route.Hops) != 2 || route.RouteState != "cached" || route.TargetEvidence == "journal" {
		t.Fatalf("offline cached route lost or claimed live: %+v", route)
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
		if os.Getenv("G2J_BASELINE") == "1" {
			profile = ""
		}
		f, e := os.Create(filepath.Join(dir, "Journal.20260101.01.log"))
		if e != nil {
			b.Fatal(e)
		}
		for j := 0; j < 100000; j++ {
			if j%50000 == 0 {
				fmt.Fprintf(f, "{\"event\":\"LoadGame\",\"FID\":\"SYNTHETIC-%d\",\"timestamp\":\"2026-01-01T00:00:00Z\"}\n", j/50000)
			} else if j%50000 == 1 {
				fmt.Fprintf(f, "{\"event\":\"Location\",\"timestamp\":\"2026-01-01T00:00:01Z\",\"StarSystem\":\"Fixture %d\",\"SystemAddress\":%d}\n", j/50000, uint64(9007199254740993)+uint64(j/50000))
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
		f, e = os.OpenFile(filepath.Join(dir, "Journal.20260101.01.log"), os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			b.Fatal(e)
		}
		for j := 0; j < 1000; j++ {
			fmt.Fprintln(f, `{"event":"MaterialCollected","timestamp":"2026-01-08T00:00:00Z","Name":"iron","Category":"Raw","Count":1}`)
		}
		f.Close()
		catchup, _ := NewService(Options{StorageDir: profile, Directory: dir, EnableParity: true, PollInterval: time.Millisecond})
		catchCtx, catchCancel := context.WithCancel(context.Background())
		catchDone := make(chan struct{})
		catchStarted := time.Now()
		go func() { catchup.Run(catchCtx); close(catchDone) }()
		cacheAt := time.Duration(0)
		deadline := time.Now().Add(2 * time.Minute)
		for {
			v := catchup.Snapshot()
			if v.Stage == "DB_ERROR" || time.Now().After(deadline) {
				catchCancel()
				b.Fatal(v)
			}
			if cacheAt == 0 && v.Cached && v.CurrentSystem != "" {
				cacheAt = time.Since(catchStarted)
			}
			if !v.ImportInProgress && v.EventsRead == 101000 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		b.ReportMetric(float64(time.Since(catchStarted).Microseconds())/1000, "catchup-1000-ms")
		b.ReportMetric(float64(cacheAt.Microseconds())/1000, "cached-view-ms")
		b.ReportMetric(float64(catchup.Details().ReaderBytes), "catchup-journal-bytes")
		catchCancel()
		<-catchDone
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		b.ReportMetric(float64(memory.HeapInuse), "Go-heap-inuse-bytes")
		b.ReportMetric(float64(peakRSS()), "process-peak-RSS-bytes")

	}
}
