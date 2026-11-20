package shipstate

import (
	"github.com/Jakoczig/eridion/go/internal/journal"
	"testing"
)

func TestG2NRenameOnlyMatchingWitnessedShip(t *testing.T) {
	m := NewModel()
	lines := []string{`{"event":"LoadGame","timestamp":"2026-01-01T00:00:00Z","FID":"SYNTHETIC","Ship":"asp","ShipID":1}`, `{"event":"SetUserShipName","timestamp":"2026-01-01T00:00:01Z","ShipID":1,"UserShipName":"Fixture Voyager"}`, `{"event":"SetUserShipName","timestamp":"2026-01-01T00:00:02Z","ShipID":2,"UserShipName":"Wrong Ship"}`}
	for i, raw := range lines {
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{Path: "synthetic", FileID: "fixture", Offset: int64(i * 500), Sequence: uint64(i + 1)})
		if err != nil {
			t.Fatal(err)
		}
		m.ApplyEvent("SYNTHETIC", e)
	}
	if m.View("SYNTHETIC").ShipName != "Fixture Voyager" {
		t.Fatal("rename missing or cross-ship applied")
	}
}
