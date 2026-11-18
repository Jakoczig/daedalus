package history

import (
	"context"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFieldOperationsAtomicProgressAndMissionEffects(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	src := journal.Source{ID: "field", Identity: "synthetic"}
	cp, _ := s.Resume(src)
	add := func(raw string) {
		t.Helper()
		if !strings.Contains(raw, `"timestamp"`) {
			raw = raw[:len(raw)-1] + `,"timestamp":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`
		}
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
	add(`{"event":"LoadGame","FID":"SYNTHETIC"}`)
	p, e := s.ImportItinerary("SYNTHETIC", "Fixture", []byte("system\nAlpha\nBeta\n"))
	if e != nil {
		t.Fatal(e)
	}
	add(`{"event":"FSDTarget","Name":"Alpha"}`)
	p, _ = s.Itinerary("SYNTHETIC", p.ID)
	if p.Next != 0 {
		t.Fatal("target advanced", p)
	}
	add(`{"event":"Location","StarSystem":"Alpha","SystemAddress":1}`)
	p, _ = s.Itinerary("SYNTHETIC", p.ID)
	if p.Next != 1 {
		t.Fatal(p)
	}
	add(`{"event":"FSDJump","StarSystem":"Elsewhere","SystemAddress":2}`)
	p, _ = s.Itinerary("SYNTHETIC", p.ID)
	if p.State != "off_route" || p.Next != 1 {
		t.Fatal(p)
	}
	add(`{"event":"MissionCompleted","timestamp":"2026-11-15T23:59:59Z","MissionID":18446744073709551615,"Faction":"Home","Reward":100,"FactionEffects":[{"Faction":"A","Reputation":"++","Influence":[{"SystemAddress":9007199254740993,"Influence":"+++"},{"SystemAddress":9007199254740994,"Influence":"+"}]}]}`)
	rows, e := s.FieldTally(context.Background(), "SYNTHETIC", "", "", 0, 100, false)
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, r := range rows {
		if r.Category == "mission_influence" {
			found++
			if r.System == "2" || r.Amount != "" || r.Qualifier == "" {
				t.Fatal("invented percent/system", r)
			}
		}
	}
	if found != 2 {
		t.Fatal(rows)
	}
	if e = s.SaveFieldTicks(context.Background(), []byte(`[{"time":"2026-11-15T23:00:00Z"},{"time":"2026-11-16T01:00:00Z"}]`), time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC)); e != nil {
		t.Fatal(e)
	}
	grouped, e := s.FieldTally(context.Background(), "SYNTHETIC", "", "", 0, 100, true)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range grouped {
		if r.Category == "mission_influence" && !strings.Contains(r.Period, "2026-11-15T23:00:00Z verified BGS tick") {
			t.Fatal("UTC day confused with tick", r)
		}
	}
	page, e := s.FieldTallyCursor(context.Background(), "SYNTHETIC", "", "", "", 1, false)
	if e != nil || len(page) != 1 {
		t.Fatal(page, e)
	}
	page2, e := s.FieldTallyCursor(context.Background(), "SYNTHETIC", "", "", page[0].Event+":"+strconv.Itoa(page[0].Ordinal), 1, false)
	if e != nil || len(page2) != 1 || page2[0].Ordinal == page[0].Ordinal {
		t.Fatal("effect lost between pages", page2, e)
	}
	if e = s.SetBuiltinModule("bgs", false); e != nil {
		t.Fatal(e)
	}
	add(`{"event":"MissionFailed","MissionID":99}`)
	if e = s.SetBuiltinModule("bgs", true); e != nil {
		t.Fatal(e)
	}
	if e = s.SetBuiltinModule("bgs", true); e != nil {
		t.Fatal(e)
	}
	all, e := s.FieldTally(context.Background(), "SYNTHETIC", "", "", 0, 100, false)
	if e != nil || len(all) != len(rows)+1 {
		t.Fatal("module catch-up duplicated/lost activity", all, e)
	}
	loaded, e := s.Itinerary("SYNTHETIC", p.ID)
	if e != nil || loaded.Next != 1 {
		t.Fatal("saved progress", loaded, e)
	}
	other, _ := s.FieldTally(context.Background(), "OTHER", "", "", 0, 100, false)
	if len(other) != 0 {
		t.Fatal("commander leak")
	}
	if _, e = s.Itinerary("OTHER", p.ID); e == nil {
		t.Fatal("other commander plan")
	}
}
