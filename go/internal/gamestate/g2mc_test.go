package gamestate

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Jakoczig/eridion/go/internal/activitystate"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/paritystate"
)

func startupFixture(t *testing.T) []journal.Event {
	t.Helper()
	raw, err := os.ReadFile("../../../tests/testdata/g2mc/startup.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var out []journal.Event
	var offset int64
	for i, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		e, err := journal.ParseLine([]byte(line), journal.SourcePosition{Path: "/synthetic/Journal.log", FileID: "synthetic-file", Offset: offset, Length: int64(len(line) + 1), Sequence: uint64(i + 1)})
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
		offset += int64(len(line) + 1)
	}
	return out
}
func startupService(t *testing.T, events []journal.Event) *Service {
	t.Helper()
	s, err := NewService(Options{EnableParity: true})
	if err != nil {
		t.Fatal(err)
	}
	s.summary.ImportInProgress = false
	s.summary.SourceState = "live"
	for _, e := range events {
		s.applyEventLocked(e, journal.Replay)
	}
	return s
}
func requireStartup(t *testing.T, s *Service) {
	t.Helper()
	v := s.Workspace()
	if len(v.Activity.Commander.Ranks) != 3 || len(v.Activity.Commander.Reputation) != 3 || len(v.Activity.Commander.Engineers) != 1 || v.Parity.Materials.State != "known" || len(v.Parity.Materials.Items) != 1 || v.Parity.Materials.Items[0].Count != 91 {
		t.Fatalf("stored startup snapshots missing: ranks=%+v reputation=%+v engineers=%+v materials=%+v", v.Activity.Commander.Ranks, v.Activity.Commander.Reputation, v.Activity.Commander.Engineers, v.Parity.Materials)
	}
	if v.Activity.Commander.Session.Jumps != 0 || v.Activity.Commander.Session.BoughtUnits != 0 {
		t.Fatal("snapshot reconstruction counted activity")
	}
}
func TestG2MCObservedStartupOrdering(t *testing.T) {
	events := startupFixture(t)
	pending := startupService(t, events[:8])
	if len(pending.Workspace().Activity.Commander.Ranks) != 0 || pending.Workspace().Parity.Materials.State == "known" {
		t.Fatal("prelude published before verified LoadGame")
	}
	s := startupService(t, events)
	requireStartup(t, s)
	v := s.Workspace()
	if *v.Activity.Commander.Ranks[0].Progress != 27 {
		t.Fatal("progress not retained")
	}
	// The subsequent genuine change is applied once, not replayed as a snapshot.
	e := events[9]
	e.Type = "MaterialCollected"
	e.Timestamp = e.Timestamp.Add(1)
	e.Position.Sequence++
	e.Position.Offset += e.Position.Length
	e.Raw = []byte(`{"event":"MaterialCollected","Category":"Raw","Name":"iron","Count":2}`)
	s.applyEventLocked(e, journal.Live)
	if s.Workspace().Parity.Materials.Items[0].Count != 93 {
		t.Fatal("live material delta not applied once")
	}
	var collector journal.Startup
	var batch journal.StartupBatch
	for _, e := range events[:9] {
		batch = collector.Observe(e)
	}
	s.views.parity.RestoreStartup(s.active, events[8], batch)
	if s.Workspace().Parity.Materials.Items[0].Count != 93 {
		t.Fatal("old startup batch overwrote newer live evidence")
	}
}
func TestG2MCStartupRejectsAmbiguousEvidence(t *testing.T) {
	for _, name := range []string{"foreign-load", "foreign-file", "foreign-path", "gap", "duplicate-sequence", "duplicate-snapshot", "backward-time", "expired", "missing-commander", "second-commander", "truncated-start", "byte-gap"} {
		t.Run(name, func(t *testing.T) {
			e := startupFixture(t)
			switch name {
			case "foreign-load":
				e[8].FID = "SYNTHETIC-B"
			case "foreign-file":
				e[8].Position.FileID = "other"
			case "foreign-path":
				e[8].Position.Path = "/other/Journal.log"
			case "gap":
				e[5].Position.Sequence++
			case "duplicate-sequence":
				e[5].Position.Sequence--
			case "duplicate-snapshot":
				e[6].Type = e[5].Type
				e[6].Raw = e[5].Raw
			case "backward-time":
				e[5].Timestamp = e[0].Timestamp.Add(-1)
			case "expired":
				e[8].Timestamp = e[8].Timestamp.Add(3 * 60 * 1000000000)
			case "missing-commander":
				e[2].Type = "Music"
				e[2].FID = ""
				e[2].Raw = []byte(`{"event":"Music"}`)
			case "second-commander":
				e[5].Type = "Commander"
				e[5].FID = "SYNTHETIC-B"
				e[5].Raw = []byte(`{"event":"Commander","FID":"SYNTHETIC-B"}`)
			case "truncated-start":
				e = e[2:]
			case "byte-gap":
				e[6].Position.Offset++
			}
			s := startupService(t, e)
			v := s.Workspace()
			if len(v.Activity.Commander.Ranks) > 0 || v.Parity.Materials.State == "known" {
				t.Fatal("ambiguous startup restored private snapshots")
			}
		})
	}
}
func TestG2MCStartupDoesNotCrossCommanders(t *testing.T) {
	a := startupFixture(t)
	s := startupService(t, a)
	requireStartup(t, s)
	b := startupAccount(t, "SYNTHETIC-B", false)
	// B has no startup snapshots. A's batch must not fill them.
	for _, e := range b {
		s.applyEventLocked(e, journal.Replay)
	}
	if s.Workspace().Parity.Materials.State == "known" || len(s.Workspace().Activity.Commander.Ranks) > 0 {
		t.Fatal("A snapshots assigned to B")
	}
	// Valid B snapshots must be B's own values, not A's cached evidence.
	for _, e := range startupAccount(t, "SYNTHETIC-B", true) {
		s.applyEventLocked(e, journal.Replay)
	}
	v := s.Workspace()
	if len(v.Parity.Materials.Items) != 1 || v.Parity.Materials.Items[0].Count != 17 || len(v.Activity.Commander.Ranks) == 0 || *v.Activity.Commander.Ranks[0].Rank != 1 {
		t.Fatal("B did not receive its own verified snapshots")
	}
}

func startupAccount(t *testing.T, fid string, snapshots bool) []journal.Event {
	t.Helper()
	var result []journal.Event
	var offset int64
	for _, e := range startupFixture(t) {
		if !snapshots && e.Position.Sequence >= 4 && e.Position.Sequence <= 8 {
			continue
		}
		var d map[string]any
		if err := json.Unmarshal(e.Raw, &d); err != nil {
			t.Fatal(err)
		}
		d["timestamp"] = e.Timestamp.Add(time.Hour).Format(time.RFC3339Nano)
		if e.FID != "" {
			d["FID"] = fid
		}
		if e.Type == "Rank" {
			d["Combat"] = 1
		}
		if e.Type == "Materials" {
			d["Raw"] = []map[string]any{{"Name": "iron", "Count": 17}}
		}
		raw, _ := json.Marshal(d)
		next, err := journal.ParseLine(raw, journal.SourcePosition{Path: "/synthetic/B.log", FileID: "file-b", Offset: offset, Length: int64(len(raw) + 1), Sequence: uint64(len(result) + 1)})
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, next)
		offset += next.Position.Length
	}
	return result
}
func TestG2MCStartupSurvivesRestartAndUpgradesLegacyCache(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	raw, err := os.ReadFile("../../../tests/testdata/g2mc/startup.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "Journal.20261119.01.log"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, stop := durableService(t, profile, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 10 })
	db, _ := s.History()
	before := db.Stats().Events
	// Simulate the reviewed cache, which retained journal history but lost these projections.
	s.mu.Lock()
	old, err := s.cacheLocked()
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var c reducerCache
	if err = gob.NewDecoder(bytes.NewReader(old)).Decode(&c); err != nil {
		t.Fatal(err)
	}
	c.Parity = paritystate.NewModel()
	c.Activity = activitystate.NewModel()
	if v := reflect.ValueOf(&c).Elem().FieldByName("StartupVersion"); v.IsValid() {
		v.SetUint(0)
	}
	var encoded bytes.Buffer
	if err = gob.NewEncoder(&encoded).Encode(c); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveCache(context.Background(), encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	stop()
	s, stop = durableService(t, profile, dir)
	// Register cleanup before assertions; a failing regression must release SQLite.
	var stopped bool
	defer func() {
		if !stopped {
			stop()
		}
	}()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 10 })
	requireStartup(t, s)
	db, _ = s.History()
	if db.Stats().Events != before {
		t.Fatal("cache upgrade duplicated history")
	}
	stop()
	stopped = true
	s, stop = durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress })
	requireStartup(t, s)
}

func TestG2MCNewSourcePreludePersistsAcrossRestart(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	old := `{"event":"LoadGame","timestamp":"2026-11-18T10:00:00Z","FID":"SYNTHETIC-OLD"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "Journal.20261118.01.log"), []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	s, stop := durableService(t, profile, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 1 })
	raw, _ := os.ReadFile("../../../tests/testdata/g2mc/startup.jsonl")
	lines := bytes.SplitAfter(raw, []byte("\n"))
	prefix := bytes.Join(lines[:8], nil)
	suffix := raw[len(prefix):]
	file := filepath.Join(dir, "Journal.20261119.01.log")
	if err := os.WriteFile(file, prefix, 0600); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 9 })
	if s.Workspace().Parity.Materials.State == "known" {
		t.Fatal("unbound prefix published")
	}
	stop()
	s, stop = durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 9 })
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(suffix)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 11 })
	requireStartup(t, s)
}

func TestG2MCClosedGameRetainsOnlyLastKnownSnapshot(t *testing.T) {
	dir, profile := t.TempDir(), t.TempDir()
	raw, _ := os.ReadFile("../../../tests/testdata/g2mc/startup.jsonl")
	raw = append(raw, []byte("{\"event\":\"Shutdown\",\"timestamp\":\"2026-11-19T10:00:05Z\"}\n")...)
	if err := os.WriteFile(filepath.Join(dir, "Journal.20261119.01.log"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, stop := durableService(t, profile, dir)
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 11 })
	// Stop before assertions so a deliberately failing regression releases SQLite.
	stop()
	requireStartup(t, s)
	if s.Workspace().Parity.Status.State == "known" {
		t.Fatal("shutdown retained live Status")
	}
	s, stop = durableService(t, profile, dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 11 })
	requireStartup(t, s)
}

func TestG2MCMalformedSnapshotDoesNotBecomeKnown(t *testing.T) {
	for _, kind := range []string{"partial-materials", "duplicate-material", "duplicate-engineer"} {
		t.Run(kind, func(t *testing.T) {
			events := startupFixture(t)
			var offset int64
			for i := range events {
				var d map[string]any
				json.Unmarshal(events[i].Raw, &d)
				if events[i].Type == "Materials" {
					if kind == "partial-materials" {
						delete(d, "Encoded")
					}
					if kind == "duplicate-material" {
						d["Raw"] = []any{map[string]any{"Name": "iron", "Count": 91}, map[string]any{"Name": "iron", "Count": 91}}
					}
				}
				if events[i].Type == "EngineerProgress" && kind == "duplicate-engineer" {
					rows := d["Engineers"].([]any)
					d["Engineers"] = append(rows, rows[0])
				}
				raw, _ := json.Marshal(d)
				pos := events[i].Position
				pos.Offset = offset
				pos.Length = int64(len(raw) + 1)
				e, err := journal.ParseLine(raw, pos)
				if err != nil {
					t.Fatal(err)
				}
				events[i] = e
				offset += pos.Length
			}
			s := startupService(t, events)
			v := s.Workspace()
			if len(v.Activity.Commander.Ranks) != 3 {
				t.Fatal("independent valid ranks lost")
			}
			if kind == "duplicate-engineer" {
				if len(v.Activity.Commander.Engineers) != 0 {
					t.Fatal("duplicate engineer accepted")
				}
			} else if v.Parity.Materials.State == "known" {
				t.Fatal("incomplete/duplicate stock accepted")
			}
		})
	}
}
