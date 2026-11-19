package history

import (
	"context"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"os"
	"os/exec"
	"testing"
)

func TestG2MBContextFiltersBeforeLimitAndKeepsCommanderAndCursor(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	src := journal.Source{ID: "context", Identity: "synthetic"}
	cp, _ := s.Resume(src)
	add := func(raw string) {
		t.Helper()
		ev, e := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: src.ID, Sequence: cp.Sequence + 1, Offset: cp.Offset, Length: 100})
		if e != nil {
			t.Fatal(e)
		}
		cp.Offset += 100
		cp.Sequence++
		if e = s.Commit(context.Background(), cp, []journal.Record{{Event: ev, Phase: journal.Live}}, nil); e != nil {
			t.Fatal(e)
		}
	}
	add(`{"event":"LoadGame","FID":"SYNTHETIC_A","timestamp":"2026-11-18T00:00:00Z"}`)
	add(`{"event":"MissionCompleted","MissionID":1,"Reward":42,"Faction":"Home","timestamp":"2026-11-18T00:00:01Z"}`)
	add(`{"event":"MissionFailed","MissionID":2,"timestamp":"2026-11-18T00:00:02Z"}`)
	add(`{"event":"PowerplayCollect","Merits":5,"timestamp":"2026-11-18T00:00:03Z"}`)
	for i := 0; i < 110; i++ {
		add(fmt.Sprintf(`{"event":"MarketBuy","Count":1,"Type":"gold","timestamp":"2026-11-18T00:01:%02dZ"}`, i%60))
	}
	rows, e := s.FieldTallyCursor(context.Background(), "SYNTHETIC_A", "", "", "", 1, false, "missions")
	if e != nil || len(rows) != 1 || rows[0].Category != "MissionFailed" {
		t.Fatal("context must filter before LIMIT", rows, e)
	}
	cursor := rows[0].Event + ":" + fmt.Sprint(rows[0].Ordinal)
	rows, e = s.FieldTallyCursor(context.Background(), "SYNTHETIC_A", "", "", cursor, 1, false, "missions")
	if e != nil || len(rows) != 1 || rows[0].Category != "mission_completed" || rows[0].Amount != "42" {
		t.Fatal("context cursor", rows, e)
	}
	rows, e = s.FieldTallyCursor(context.Background(), "SYNTHETIC_A", "", "", "", 10, false, "powerplay")
	if e != nil || len(rows) != 1 || rows[0].Category != "PowerplayCollect" {
		t.Fatal(rows, e)
	}
	rows, e = s.FieldTallyCursor(context.Background(), "SYNTHETIC_B", "", "", "", 10, false, "missions")
	if e != nil || len(rows) != 0 {
		t.Fatal("cross commander", rows, e)
	}
	if _, e = s.FieldTallyCursor(context.Background(), "SYNTHETIC_A", "", "", "", 10, false, "missions' OR 1=1"); e == nil {
		t.Fatal("invalid context accepted")
	}
}

// Exit without closing SQLite to exercise committed WAL recovery, not only a clean restart.
func TestG2MBRouteHistoryAfterAbruptExit(t *testing.T) {
	if dir := os.Getenv("ERIDION_SYNTHETIC_CRASH_HISTORY"); dir != "" {
		s, e := Open(dir)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.writer.Exec("INSERT INTO commander VALUES('SYNTHETIC','Fixture',NULL)"); e != nil {
			t.Fatal(e)
		}
		first, e := s.ImportItinerary("SYNTHETIC", "First route", []byte("system\nAlpha\nBeta\n"))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.ImportItinerary("SYNTHETIC", "Second route", []byte("system\nGamma\nDelta\n")); e != nil {
			t.Fatal(e)
		}
		if _, e = s.EditItinerary("SYNTHETIC", first.ID, 1, true); e != nil {
			t.Fatal(e)
		}
		os.Exit(0)
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestG2MBRouteHistoryAfterAbruptExit$")
	cmd.Env = append(os.Environ(), "ERIDION_SYNTHETIC_CRASH_HISTORY="+dir)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	plans, e := s.Itineraries("SYNTHETIC")
	if e != nil || len(plans) != 2 {
		t.Fatal("history lost", plans, e)
	}
	current, e := s.Itinerary("SYNTHETIC", "")
	if e != nil || current.Name != "First route" || current.Next != 1 || !current.Active {
		t.Fatal("active progress lost", current, e)
	}
	if _, e = s.ImportItinerary("SYNTHETIC", "Broken", []byte("system\nAlpha\nAlpha\n")); e == nil {
		t.Fatal("ambiguous import accepted")
	}
	after, e := s.Itinerary("SYNTHETIC", "")
	if e != nil || after.ID != current.ID || after.Next != 1 {
		t.Fatal("failed import changed active plan", after, e)
	}
	if p, e := s.Itineraries("OTHER"); e != nil || len(p) != 0 {
		t.Fatal("cross commander history", p, e)
	}
}
