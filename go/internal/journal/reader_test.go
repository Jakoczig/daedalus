package journal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type observed struct {
	Event Event
	Phase Phase
}

func startReader(t *testing.T, dir string) (*Reader, chan observed, context.CancelFunc, chan error) {
	t.Helper()
	return startReaderWhen(t, dir, nil)
}

// A gate lets startup tests control the first poll without scheduler assumptions.
func startReaderWhen(t *testing.T, dir string, gate <-chan struct{}) (*Reader, chan observed, context.CancelFunc, chan error) {
	t.Helper()
	r, err := NewReader(dir, ReaderOptions{PollInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan observed, 200)
	done := make(chan error, 1)
	go func() {
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
				done <- nil
				return
			}
		}
		done <- r.Run(ctx, func(e Event, p Phase) error {
			select {
			case events <- observed{e, p}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("reader leaked after cancel")
		}
	})
	return r, events, cancel, done
}
func waitReader(t *testing.T, predicate func() bool) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if predicate() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("journal condition timed out")
}
func nextEvent(t *testing.T, events <-chan observed) observed {
	t.Helper()
	select {
	case e := <-events:
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("event timed out")
		return observed{}
	}
}
func journalPath(dir string) string { return filepath.Join(dir, "Journal.20260101.01.log") }
func appendJournal(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func TestReaderStartsBeforeGameDirectoryExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "later")
	gate := make(chan struct{})
	r, events, _, _ := startReaderWhen(t, dir, gate)
	close(gate)
	// The constructor also says waiting. ImportInProgress becomes false only
	// after Run has completed its first poll and established the replay boundary.
	waitReader(t, func() bool {
		s := r.Stats()
		return s.SourceState == "waiting" && !s.ImportInProgress
	})
	makeDir(t, dir)
	os.WriteFile(journalPath(dir), []byte("{\"event\":\"Fileheader\"}\n"), 0600)
	if e := nextEvent(t, events); e.Event.Type != "Fileheader" || e.Phase != Live {
		t.Fatal("late source did not become live")
	}
}
func TestReaderDirectoryAppearsBeforeFirstPollReplays(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "before-first-poll")
	gate := make(chan struct{})
	r, events, _, _ := startReaderWhen(t, dir, gate)
	if !r.Stats().ImportInProgress {
		t.Fatal("reader claims first poll completed before Run")
	}
	makeDir(t, dir)
	if err := os.WriteFile(journalPath(dir), []byte("{\"event\":\"Fileheader\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	close(gate)
	if e := nextEvent(t, events); e.Event.Type != "Fileheader" || e.Phase != Replay {
		t.Fatalf("source present at first poll must replay: %+v", e)
	}
}
func TestReaderWaitsForNewline(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(journalPath(dir), []byte(`{"event":"Location","StarSystem":"Fixture`), 0600)
	r, events, _, _ := startReader(t, dir)
	waitReader(t, func() bool { return r.Stats().Pending })
	select {
	case <-events:
		t.Fatal("published partial JSON")
	default:
	}
	appendJournal(t, journalPath(dir), " Alpha\"}\n")
	if e := nextEvent(t, events); e.Event.SystemName != "Fixture Alpha" || e.Phase != Live || e.Event.Position.Offset != 0 {
		t.Fatal("partial record lost")
	}
}
func TestReaderAppendNoDuplicatesAndNoRescan(t *testing.T) {
	dir := t.TempDir()
	line := "{\"event\":\"Fileheader\"}\r\n"
	os.WriteFile(journalPath(dir), []byte(line), 0600)
	r, events, _, _ := startReader(t, dir)
	if nextEvent(t, events).Phase != Replay {
		t.Fatal("history reported live")
	}
	waitReader(t, func() bool { return !r.Stats().ImportInProgress && r.Stats().EventsRead == 1 })
	before := r.Stats().BytesRead
	time.Sleep(35 * time.Millisecond)
	if r.Stats().BytesRead != before {
		t.Fatal("stable journal re-scanned")
	}
	appendJournal(t, journalPath(dir), "{\"event\":\"FSDJump\",\"StarSystem\":\"Fixture Beta\"}\n")
	e := nextEvent(t, events)
	if e.Phase != Live || e.Event.Position.Offset != int64(len(line)) {
		t.Fatal("append source offset/phase wrong")
	}
	time.Sleep(30 * time.Millisecond)
	select {
	case <-events:
		t.Fatal("completed line repeated")
	default:
	}
}
func TestReaderRotationAndSamePathReplacement(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(journalPath(dir), []byte("{\"event\":\"Fileheader\"}\n"), 0600)
	_, events, _, _ := startReader(t, dir)
	a := nextEvent(t, events)
	next := filepath.Join(dir, "Journal.20260101.02.log")
	os.WriteFile(next, []byte("{\"event\":\"Fileheader\"}\n{\"event\":\"LoadGame\",\"FID\":\"SYNTHETIC-B\"}\n"), 0600)
	if e := nextEvent(t, events); e.Event.Type != "Fileheader" || e.Event.Position.Offset != 0 || e.Phase != Live {
		t.Fatal("rotation lost first line")
	}
	nextEvent(t, events)
	replacement := filepath.Join(dir, "replacement.tmp")
	os.WriteFile(replacement, []byte("{\"event\":\"Location\",\"StarSystem\":\"Fixture Replaced\"}\n"), 0600)
	if err := os.Remove(journalPath(dir)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, journalPath(dir)); err != nil {
		t.Fatal(err)
	}
	if e := nextEvent(t, events); e.Event.SystemName != "Fixture Replaced" || e.Event.Position.FileID == a.Event.Position.FileID {
		t.Fatal("same-name replacement missed")
	}
}
func TestReaderTruncateRecovery(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(journalPath(dir), []byte("{\"event\":\"Location\",\"StarSystem\":\"Fixture Old Long Name\"}\n"), 0600)
	r, events, _, _ := startReader(t, dir)
	nextEvent(t, events)
	os.WriteFile(journalPath(dir), []byte("{\"event\":\"Fileheader\"}\n"), 0600)
	if nextEvent(t, events).Event.Type != "Fileheader" {
		t.Fatal("truncate not recovered")
	}
	waitReader(t, func() bool { return r.Stats().Resets > 0 })
}
func TestReaderMalformedLineContinues(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(journalPath(dir), []byte("bad\n"+strings.Repeat("x", MaxRecordBytes+1)+"\n{\"event\":\"Fileheader\"}\n"), 0600)
	r, events, _, _ := startReader(t, dir)
	nextEvent(t, events)
	waitReader(t, func() bool { return r.Stats().MalformedLines == 2 })
}
func TestReaderRestartReconstructsWithoutLiveDuplicate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(journalPath(dir), []byte("{\"event\":\"Fileheader\"}\n"), 0600)
	for range 2 {
		_, events, cancel, _ := startReader(t, dir)
		if nextEvent(t, events).Phase != Replay {
			t.Fatal("restart history emitted live")
		}
		cancel()
	}
}
func TestReaderCancellationStopsPolling(t *testing.T) {
	r, _, cancel, done := startReader(t, t.TempDir())
	waitReader(t, func() bool { return r.Stats().SourceState == "waiting" })
	cancel()
	// Cleanup also consumes done; put result back after proving prompt completion.
	select {
	case err := <-done:
		done <- err
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop reader")
	}
}

func TestReaderAllowsRotationWhileReadHandleIsOpen(t *testing.T) {
	dir := t.TempDir()
	path := journalPath(dir)
	os.WriteFile(path, []byte("{\"event\":\"Fileheader\"}\n"), 0600)
	r, _ := NewReader(dir, ReaderOptions{PollInterval: time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rotated := false
	sawNew := false
	err := r.Run(ctx, func(e Event, _ Phase) error {
		if !rotated {
			rotated = true
			if err := os.Rename(path, path+".retired"); err != nil {
				return err
			}
			return os.WriteFile(path, []byte("{\"event\":\"Location\",\"StarSystem\":\"Fixture After Rotation\"}\n"), 0600)
		}
		if e.SystemName == "Fixture After Rotation" {
			sawNew = true
			cancel()
		}
		return nil
	})
	if err != nil || !sawNew {
		t.Fatal("reader blocked game file rotation", err)
	}
}
