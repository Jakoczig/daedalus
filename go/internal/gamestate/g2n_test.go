package gamestate

import (
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/shipstate"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestG2NWitnessedRenameRecoversOlderClosedGameCache(t *testing.T) {
	lines := []string{`{"event":"LoadGame","timestamp":"2026-01-01T00:00:00Z","FID":"SYNTHETIC","Ship":"asp","ShipID":42}`, `{"event":"Loadout","timestamp":"2026-01-01T00:00:01Z","Ship":"asp","ShipID":42,"ShipName":"Old name","Modules":[]}`, `{"event":"SetUserShipName","timestamp":"2026-01-01T00:00:02Z","ShipID":42,"UserShipName":"Witnessed new name"}`, `{"event":"Shutdown","timestamp":"2026-01-01T00:00:03Z"}`}
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "Journal.20260101.01.log"), []byte(strings.Join(lines, "\n")+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	s, stop := durableService(t, t.TempDir(), dir)
	defer stop()
	waitState(t, s, func(v Summary) bool { return !v.ImportInProgress && v.EventsRead == 4 })
	old := shipstate.NewModel()
	for i, raw := range lines {
		if i == 2 {
			raw = `{"event":"Music","timestamp":"2026-01-01T00:00:02Z"}`
		}
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{Path: "synthetic", FileID: "synthetic", Offset: int64(i * 500), Sequence: uint64(i + 1)})
		if err != nil {
			t.Fatal(err)
		}
		old.ApplyEvent("SYNTHETIC", e)
	}
	s.mu.Lock()
	s.views.ship = old
	s.mu.Unlock()
	if v := s.OwnerShipIdentity(); v["shipName"] != "Witnessed new name" || v["shipType"] != "Asp Explorer" {
		t.Fatal("cached rename not recovered", v)
	}
	var name, at string
	if e := s.store.WitnessedShipName(t.Context(), "FOREIGN", "42", &name, &at); e == nil {
		t.Fatal("rename crossed commanders")
	}
}
