package shipstate

import (
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"strings"
	"testing"
)

func TestJournalShipFactsAndSourceIsolation(t *testing.T) {
	m := NewModel()
	seq := uint64(0)
	feed := func(key, raw string) {
		seq++
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture", Offset: int64(seq), Sequence: seq})
		if err != nil {
			t.Fatal(err)
		}
		m.ApplyEvent(key, e)
	}
	feed("A", `{"event":"LoadGame","FID":"SYNTHETIC-A","Ship":"asp","ShipID":7,"FuelLevel":12,"FuelCapacity":32,"ShipName":"PRIVATE_NAME","ShipIdent":"PRIVATE_IDENT"}`)
	feed("A", `{"event":"Loadout","Ship":"asp","ShipID":7,"HullHealth":0.82,"FuelCapacity":{"Main":32,"Reserve":0.5},"CargoCapacity":64,"UnladenMass":280,"MaxJumpRange":45,"Modules":[{"Item":"PRIVATE_MODULE"}]}`)
	v := m.View("A")
	if v.ShipType != "asp" || v.HullHealth == nil || *v.HullHealth != 0.82 || v.FuelCapacityT == nil || *v.FuelCapacityT != 32 || v.FuelMainT == nil || *v.FuelMainT != 12 || *v.MaxJumpRangeLY != 45 || *v.CargoCapacityT != 64 {
		t.Fatalf("facts lost: %+v", v)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "PRIVATE_") || strings.Contains(string(raw), "SYNTHETIC") {
		t.Fatal("private data leaked")
	}
	*v.FuelMainT = 99
	if *m.View("A").FuelMainT != 12 {
		t.Fatal("mutable view")
	}
	feed("A", `{"event":"FSDJump","StarSystem":"Fixture","SystemAddress":9007199254740993,"FuelLevel":9}`)
	if *m.View("A").FuelMainT != 9 {
		t.Fatal("jump fuel stale")
	}
	feed("A", `{"event":"HullDamage","Health":0.1,"Fighter":true}`)
	if *m.View("A").HullHealth != 0.82 {
		t.Fatal("fighter hull replaced ship")
	}
	feed("A", `{"event":"ShipyardSwap","ShipType":"anaconda"}`)
	if m.View("A").HullHealth != nil || m.View("A").FuelMainT != nil {
		t.Fatal("old ship carried across swap")
	}
	feed("B", `{"event":"LoadGame","FID":"SYNTHETIC-B"}`)
	if m.View("B").FuelMainT != nil || m.View("B").ShipType != "" {
		t.Fatal("commander leak")
	}
	seq++
	feed("A", `{"event":"FuelScoop","Total":20}`)
	if m.View("A").FuelMainT != nil {
		t.Fatal("source gap trusted")
	}
}
func TestShipUnknownAndInvalidFactsStayUnknown(t *testing.T) {
	m := NewModel()
	seq := int64(0)
	feed := func(raw string) {
		seq++
		e, _ := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture", Offset: seq})
		m.ApplyEvent("A", e)
	}
	feed(`{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	feed(`{"event":"Loadout","Ship":"asp","HullHealth":2,"FuelCapacity":{"Main":"32"},"MaxJumpRange":-1}`)
	v := m.View("A")
	if v.HullHealth != nil || v.FuelCapacityT != nil || v.MaxJumpRangeLY != nil || v.Evidence != "partial" {
		t.Fatalf("invalid facts invented: %+v", v)
	}
	feed(`{"event":"FuelScoop","Total":5}`)
	feed(`{"event":"RefuelAll","Amount":5}`)
	if m.View("A").FuelMainT != nil {
		t.Fatal("refuel inferred final quantity")
	}
	feed(`{"event":"Docked"}`)
	if m.View("A").Docked == nil || !*m.View("A").Docked {
		t.Fatal("docked evidence missing")
	}
	feed(`{"event":"Undocked"}`)
	if *m.View("A").Docked {
		t.Fatal("undock stale")
	}
}

func TestG2DShipIdentityDoesNotRoundOrCarryFuel(t *testing.T) {
	m := NewModel()
	var seq int64
	feed := func(raw string) {
		seq++
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "synthetic", Offset: seq})
		if err != nil {
			t.Fatal(err)
		}
		m.ApplyEvent("A", e)
	}
	feed(`{"event":"LoadGame","FID":"SYNTHETIC","Ship":"asp","ShipID":9007199254740992,"FuelLevel":12}`)
	feed(`{"event":"Loadout","Ship":"asp","ShipID":9007199254740993,"HullHealth":0.7}`)
	if m.View("A").FuelMainT != nil {
		t.Fatal("same-type distinct exact ID inherited fuel")
	}
	feed(`{"event":"FuelScoop","Total":8}`)
	feed(`{"event":"Loadout","Ship":"asp","ShipID":9007199254740993,"HullHealth":0.8}`)
	if m.View("A").FuelMainT == nil || *m.View("A").FuelMainT != 8 {
		t.Fatal("same ship fuel lost")
	}
}

func TestG2DMissingIdentityCannotCarryFuel(t *testing.T) {
	m := NewModel()
	for i, raw := range []string{`{"event":"LoadGame","FID":"SYNTHETIC","Ship":"asp","FuelLevel":12}`, `{"event":"Loadout","Ship":"asp","HullHealth":0.7}`} {
		e, _ := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture", Offset: int64(i + 1)})
		m.ApplyEvent("A", e)
	}
	if m.View("A").FuelMainT != nil {
		t.Fatal("unknown ship identity retained old fuel")
	}
}
