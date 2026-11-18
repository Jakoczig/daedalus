package gamestate

import (
	"testing"
)

func TestSelectedCommanderDoesNotFollowOtherJournalSession(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture A","SystemAddress":1}`)
	s := startService(t, dir)
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 2 })
	if e := s.SetAccounts(true, "SYNTHETIC-A"); e != nil {
		t.Fatal(e)
	}
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-B"}`)
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture B","SystemAddress":2}`)
	// The reader keeps ingesting B, while the selected projection remains A.
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 4 })
	if s.FieldCommander() != "SYNTHETIC-A" || s.Snapshot().CurrentSystem != "Fixture A" {
		t.Fatal("silent account switch", s.Snapshot())
	}
	if e := s.SetAccounts(true, "NOT-VERIFIED"); e == nil {
		t.Fatal("browser invented FID accepted")
	}
	if e := s.SetAccounts(true, "SYNTHETIC-B"); e != nil {
		t.Fatal(e)
	}
	if s.FieldCommander() != "SYNTHETIC-B" {
		t.Fatal("explicit switch failed")
	}
}
