package paritystate

import (
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
	"strings"
	"testing"
)

func event(t *testing.T, seq uint64, kind, fields string) journal.Event {
	t.Helper()
	e, err := journal.ParseLine([]byte(fmt.Sprintf(`{"timestamp":"2026-11-14T10:00:%02dZ","event":%q%s}`, seq, kind, fields)), journal.SourcePosition{FileID: "f", Offset: int64(seq) * 100, Sequence: seq})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestStatusCannotRegressOrRestoreCommander(t *testing.T) {
	m := session(t)
	status := func(seq uint64, fuel int) sidecar.Change {
		stamp := event(t, seq, "Anything", "").Timestamp
		return sidecar.Change{Kind: "Status", Valid: true, Timestamp: stamp, Raw: []byte(fmt.Sprintf(`{"timestamp":%q,"event":"Status","Flags":16777216,"Fuel":{"FuelMain":%d}}`, stamp.Format("2006-01-02T15:04:05Z"), fuel))}
	}
	m.ApplySidecar("a", status(4, 4))
	m.ApplySidecar("a", status(3, 3))
	if m.View("a").Status.FuelMainT != nil {
		t.Fatal("older snapshot restored current trusted state")
	}
	m.ApplyEvent("b", event(t, 5, "LoadGame", `,"FID":"other","Ship":"adder"`))
	m.ApplySidecar("a", status(6, 6))
	if m.View("a").Status.FuelMainT != nil {
		t.Fatal("inactive commander sidecar accepted")
	}
}

func TestStatusFlags2ControlsVehicleOwnership(t *testing.T) {
	m := session(t)
	stamp := event(t, 2, "Anything", "").Timestamp
	c := sidecar.Change{Kind: "Status", Valid: true, Timestamp: stamp, Raw: []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Status","Flags":2164260864,"Flags2":4097,"Fuel":{"FuelMain":2}}`)}
	m.ApplySidecar("a", c)
	v := m.View("a")
	if v.Status.State != "known" || v.Status.ObservedAt != "2026-11-14T10:00:02Z" {
		t.Fatal("status observation provenance missing")
	}
	if v.Status.FuelMainT != nil || v.Status.Flags.InMainShip || !v.Status.Flags.GlideMode {
		t.Fatalf("flags2 ownership and glide incorrect: %+v", v.Status)
	}
}
func TestExplicitCraftAndTradeMaterialDeltas(t *testing.T) {
	m := session(t)
	m.ApplyEvent("a", event(t, 2, "Materials", `,"Raw":[{"Name":"iron","Count":10},{"Name":"nickel","Count":3}],"Encoded":[],"Manufactured":[]`))
	m.ApplyEvent("a", event(t, 3, "EngineerCraft", `,"Ingredients":[{"Name":"iron","Count":2}]`))
	v := m.View("a")
	if v.Materials.State != "known" || v.Materials.Items[0].Count != 8 {
		t.Fatal("explicit craft ingredients not applied")
	}
	m.ApplyEvent("a", event(t, 4, "MaterialTrade", `,"Paid":{"Material":"iron","Quantity":2},"Received":{"Material":"nickel","Quantity":1}`))
	v = m.View("a")
	if v.Materials.State != "known" || v.Materials.Items[0].Count != 6 || v.Materials.Items[1].Count != 4 {
		t.Fatal("explicit trade not applied")
	}
	m.ApplyEvent("a", event(t, 5, "EngineerCraft", `,"Ingredients":[{"Name":"iron","Count":7}]`))
	if m.View("a").Materials.State == "known" {
		t.Fatal("negative material stock trusted")
	}
}
func TestDuplicateOrPartialMaterialSnapshotNeverKnown(t *testing.T) {
	m := session(t)
	m.ApplyEvent("a", event(t, 2, "Materials", `,"Raw":[{"Name":"iron","Count":1},{"Name":"IRON","Count":2}],"Encoded":[],"Manufactured":[]`))
	if m.View("a").Materials.State == "known" {
		t.Fatal("duplicate material stock trusted")
	}
	m.ApplyEvent("a", event(t, 3, "Materials", `,"Raw":[],"Encoded":[]`))
	if m.View("a").Materials.State == "known" {
		t.Fatal("missing material category trusted as empty")
	}
}
func TestResetMalformedStatusAndSidecarBarrier(t *testing.T) {
	m := session(t)
	stamp := event(t, 2, "Anything", "").Timestamp
	c := sidecar.Change{Kind: "Status", Valid: true, Timestamp: stamp, Raw: []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Status","Flags":"16777216"}`)}
	m.ApplySidecar("a", c)
	if m.View("a").Status.State != "invalid" {
		t.Fatal("string flags coerced")
	}
	c.Raw = []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Status","Flags":16777216,"Fuel":{"FuelMain":"0"}}`)
	m.ApplySidecar("a", c)
	if m.View("a").Status.FuelMainT != nil {
		t.Fatal("string fuel coerced")
	}
	m.ApplyEvent("a", event(t, 2, "ShipyardSwap", `,"ShipType":"adder"`))
	m.ApplySidecar("a", c)
	if m.View("a").Status.Flags != nil {
		t.Fatal("same timestamp status crossed ship barrier")
	}
	m.ResetSource()
	if m.View("a").Evidence != "unknown" {
		t.Fatal("reset retained session")
	}
	m.ApplyEvent("a", event(t, 3, "Materials", `,"Raw":[],"Encoded":[],"Manufactured":[]`))
	if m.View("a").Materials.State == "known" {
		t.Fatal("unverified session accepted snapshot")
	}
}
func TestMaterialConsumptionInvalidatesAndBounds(t *testing.T) {
	m := session(t)
	m.ApplyEvent("a", event(t, 2, "Materials", `,"Raw":[{"Name":"iron","Count":3}],"Encoded":[],"Manufactured":[]`))
	m.ApplyEvent("a", event(t, 3, "MaterialCollected", `,"Category":"Raw","Name":"iron","Count":2`))
	if m.View("a").Materials.Items[0].Count != 5 {
		t.Fatal("explicit delta missing")
	}
	m.ApplyEvent("a", event(t, 4, "EngineerCraft", ``))
	if m.View("a").Materials.State == "known" {
		t.Fatal("craft leaves stale known materials")
	}
	mods := make([]string, 140)
	for i := range mods {
		mods[i] = fmt.Sprintf(`{"Slot":"S%d","Item":"int_powerplant_size2_class1"}`, i)
	}
	m.ApplyEvent("a", event(t, 5, "Loadout", `,"Ship":"anaconda","ShipID":18446744073709551614,"Modules":[`+strings.Join(mods, ",")+`]`))
	v := m.View("a")
	if len(v.Modules) != 128 || v.ModulesState != "truncated" {
		t.Fatal("module limit dishonest")
	}
	encoded, _ := json.Marshal(v)
	for _, secret := range []string{"private", "18446744073709551614", "Latitude", "FID", "ShipID"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("private detail leaked: %s", secret)
		}
	}
	for i := 0; i < 10; i++ {
		m.ApplyEvent(fmt.Sprint(i), event(t, 6, "LoadGame", `,"FID":"other"`))
	}
	if m.View("a").Evidence != "unknown" {
		t.Fatal("high-detail sessions unbounded")
	}
}
func session(t *testing.T) *Model {
	m := NewModel()
	m.ApplyEvent("a", event(t, 1, "LoadGame", `,"FID":"private","Ship":"anaconda","ShipID":18446744073709551614`))
	return m
}
func TestExplicitSnapshotsAndDetachedView(t *testing.T) {
	m := session(t)
	m.ApplyEvent("a", event(t, 2, "Loadout", `,"Ship":"anaconda","ShipID":18446744073709551614,"Modules":[{"Slot":"PowerPlant","Item":"int_powerplant_size2_class1","Health":0.9,"On":true,"Priority":1,"Engineering":{"BlueprintName":"Armour_HeavyDuty","Level":3,"Quality":0.7,"Modifiers":[{"Label":"Mass","Value":3,"OriginalValue":2,"LessIsGood":true}]}}]`))
	v := m.View("a")
	if v.ModulesState != "journal_last_known" || len(v.Modules) != 1 || v.Modules[0].Health == nil {
		t.Fatalf("missing loadout: %+v", v)
	}
	*v.Modules[0].Health = 0
	v.Modules[0].Engineering.Modifiers[0].Label = "changed"
	if *m.View("a").Modules[0].Health != 0.9 || m.View("a").Modules[0].Engineering.Modifiers[0].Label != "Mass" {
		t.Fatal("view aliases model")
	}
	m.ApplyEvent("a", event(t, 3, "Materials", `,"Raw":[{"Name":"iron","Count":3}],"Encoded":[],"Manufactured":[]`))
	if len(m.View("a").Materials.Items) != 1 {
		t.Fatal("materials absent")
	}
	m.ApplyEvent("a", event(t, 4, "ShipLocker", `,"Items":[{"Name":"healthpack","Count":2}],"Components":[],"Data":[],"Consumables":[]`))
	m.ApplyEvent("a", event(t, 5, "Backpack", `,"Items":[],"Components":[],"Data":[],"Consumables":[{"Name":"healthpack","Count":1}]`))
	if len(m.View("a").Inventory.Items) != 2 {
		t.Fatal("locker and backpack mixed")
	}
}
func TestSessionGapAndSwapClearFacts(t *testing.T) {
	m := session(t)
	m.ApplyEvent("a", event(t, 2, "Loadout", `,"Ship":"anaconda","ShipID":18446744073709551614,"Modules":[]`))
	m.ApplyEvent("a", event(t, 4, "Materials", `,"Raw":[],"Encoded":[],"Manufactured":[]`))
	if m.View("a").Evidence != "unverified" || m.View("a").ModulesState != "unknown" {
		t.Fatal("source gap retained trusted evidence")
	}
	m.ApplyEvent("a", event(t, 5, "LoadGame", `,"FID":"private","Ship":"anaconda","ShipID":18446744073709551614`))
	m.ApplyEvent("a", event(t, 6, "Loadout", `,"Ship":"anaconda","ShipID":18446744073709551614,"Modules":[{"Slot":"A","Item":"one"}]`))
	m.ApplyEvent("a", event(t, 7, "ShipyardSwap", `,"ShipType":"anaconda","ShipID":18446744073709551615`))
	if len(m.View("a").Modules) != 0 {
		t.Fatal("same type swap retained modules")
	}
}
func TestCargoMarkerOrderAndAmbiguousReplacement(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(fmt.Sprint(before), func(t *testing.T) {
			m := session(t)
			e := event(t, 2, "Cargo", `,"Vessel":"Ship","Count":2`)
			c := sidecar.Change{Kind: "Cargo", Valid: true, Timestamp: e.Timestamp, Raw: []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Cargo","Vessel":"Ship","Count":2,"Inventory":[{"Name":"gold","Count":2,"Stolen":0}]}`)}
			if before {
				m.ApplySidecar("a", c)
			}
			m.ApplyEvent("a", e)
			if !before {
				m.ApplySidecar("a", c)
			}
			if m.View("a").Cargo.State != "known" || len(m.View("a").Cargo.Items) != 1 {
				t.Fatalf("not correlated: %+v", m.View("a").Cargo)
			}
			c.Raw = []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Cargo","Vessel":"Ship","Count":2,"Inventory":[{"Name":"silver","Count":2}]}`)
			m.ApplySidecar("a", c)
			if len(m.View("a").Cargo.Items) != 0 {
				t.Fatal("replacement inherited marker")
			}
		})
	}
}

func TestNewCargoFileWaitsForItsOwnMarker(t *testing.T) {
	m := session(t)
	e := event(t, 2, "Cargo", `,"Vessel":"Ship","Count":1`)
	m.ApplyEvent("a", e)
	c := sidecar.Change{Kind: "Cargo", Valid: true, Timestamp: e.Timestamp, Raw: []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Cargo","Vessel":"Ship","Count":1,"Inventory":[{"Name":"gold","Count":1}]}`)}
	m.ApplySidecar("a", c)
	c.Timestamp = event(t, 3, "Anything", "").Timestamp
	c.Raw = []byte(`{"timestamp":"2026-11-14T10:00:03Z","event":"Cargo","Vessel":"Ship","Count":1,"Inventory":[{"Name":"silver","Count":1}]}`)
	m.ApplySidecar("a", c)
	if m.View("a").Cargo.State == "known" || len(m.View("a").Cargo.Items) > 0 {
		t.Fatal("old cargo remains trusted after newer unclaimed file")
	}
}
func TestStatusMainShipGateAndMissing(t *testing.T) {
	m := session(t)
	stamp := event(t, 2, "Anything", "").Timestamp
	c := sidecar.Change{Kind: "Status", Valid: true, Timestamp: stamp, Raw: []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Status","Flags":16777224,"Pips":[4,2,6],"Fuel":{"FuelMain":12,"FuelReservoir":0.5},"Cargo":3,"GuiFocus":0}`)}
	m.ApplySidecar("a", c)
	v := m.View("a")
	if v.Status.FuelMainT == nil || v.Status.Pips.Sys != 2 || !v.Status.Flags.ShieldsUp {
		t.Fatalf("missing status %+v", v.Status)
	}
	c.Raw = []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Status","Flags":67108864,"Fuel":{"FuelMain":1},"Cargo":1}`)
	m.ApplySidecar("a", c)
	if m.View("a").Status.FuelMainT != nil {
		t.Fatal("SRV fuel presented as main ship")
	}
	m.ApplySidecar("a", sidecar.Change{Kind: "Status", State: "missing"})
	if m.View("a").Status.Flags != nil {
		t.Fatal("missing sidecar retains trusted flags")
	}
}

func TestLockerKnowledgeDoesNotDependOnBackpackSnapshot(t *testing.T) {
	m := session(t)
	m.ApplyEvent("a", event(t, 2, "ShipLocker", `,"Items":[],"Components":[],"Data":[],"Consumables":[]`))
	v := m.View("a").Inventory
	if v.ShipLockerState != "known" || v.BackpackState == "known" || v.State == "known" {
		t.Fatal("separate inventory evidence lost", v)
	}
}
