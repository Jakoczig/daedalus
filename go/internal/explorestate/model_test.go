package explorestate

import (
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"strings"
	"sync/atomic"
	"testing"
)

var offset atomic.Int64

func apply(t *testing.T, m *Model, key, raw string) {
	t.Helper()
	e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture-1", Offset: offset.Add(1)})
	if err != nil {
		t.Fatal(err)
	}
	m.ApplyEvent(key, e)
}
func start(t *testing.T) *Model {
	t.Helper()
	m := NewModel()
	apply(t, m, "A", `{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	apply(t, m, "A", `{"event":"Location","StarSystem":"Fixture Alpha","SystemAddress":9007199254740993}`)
	return m
}

const scan = `{"event":"Scan","SystemAddress":9007199254740993,"BodyID":9007199254740997,"BodyName":"Fixture Alpha 1","PlanetClass":"Rocky body","WasMapped":true}`

func TestExplorationFSSUnknownVsReportedCounts(t *testing.T) {
	m := start(t)
	if v := m.View("A", "9007199254740993"); v.ReportedBodyCount != nil || len(v.Bodies) != 0 || v.AllBodiesFound {
		t.Fatal("invented honk or bodies", v)
	}
	apply(t, m, "A", `{"event":"FSSDiscoveryScan","BodyCount":12,"Progress":0.25}`)
	v := m.View("A", "9007199254740993")
	if v.ReportedBodyCount == nil || *v.ReportedBodyCount != 12 || *v.FSSProgress != 0.25 || len(v.Bodies) != 0 || v.AllBodiesFound {
		t.Fatal("honk truth lost", v)
	}
	apply(t, m, "A", `{"event":"FSSAllBodiesFound","SystemAddress":9007199254740993,"Count":12}`)
	if !m.View("A", "9007199254740993").AllBodiesFound {
		t.Fatal("scanner completion missing")
	}
}
func TestExplorationDeduplicatesBodiesByAddressAndBodyID(t *testing.T) {
	m := start(t)
	apply(t, m, "A", scan)
	apply(t, m, "A", strings.Replace(scan, `"WasMapped":true`, `"Landable":true`, 1))
	v := m.View("A", "9007199254740993")
	if len(v.Bodies) != 1 || v.Bodies[0].Landable == nil || !*v.Bodies[0].Landable || v.Bodies[0].PreviouslyMapped == nil || !*v.Bodies[0].PreviouslyMapped {
		t.Fatal("body update/dedup lost", v)
	}
	*v.Bodies[0].Landable = false
	if !*m.View("A", "9007199254740993").Bodies[0].Landable {
		t.Fatal("mutable view aliases state")
	}
}
func TestExplorationIgnoresOutOfSystemScan(t *testing.T) {
	m := start(t)
	for _, raw := range []string{strings.Replace(scan, "9007199254740993", "9007199254740992", 1), strings.Replace(scan, `"SystemAddress":9007199254740993,`, "", 1)} {
		apply(t, m, "A", raw)
	}
	if v := m.View("A", "9007199254740993"); len(v.Bodies) != 0 {
		t.Fatal("wrong/missing address guessed", v)
	}
	apply(t, m, "A", scan)
	apply(t, m, "A", `{"event":"FSDJump","StarSystem":"Fixture Beta","SystemAddress":2}`)
	if v := m.View("A", "2"); len(v.Bodies) != 0 || v.SystemName != "Fixture Beta" {
		t.Fatal("old system bodies displayed", v)
	}
}
func TestExplorationKnownVsMissingLandability(t *testing.T) {
	m := start(t)
	apply(t, m, "A", scan)
	b := m.View("A", "9007199254740993").Bodies[0]
	if b.Landable != nil || b.GravityG != nil || b.Terraformable != nil {
		t.Fatal("unknown became false/zero", b)
	}
	apply(t, m, "A", strings.Replace(scan, `"WasMapped":true`, `"Landable":false,"SurfaceGravity":9.80665,"TerraformState":"Terraformable","Atmosphere":"thin argon atmosphere","Volcanism":"No volcanism"`, 1))
	b = m.View("A", "9007199254740993").Bodies[0]
	if b.Landable == nil || *b.Landable || b.GravityG == nil || *b.GravityG != 1 || b.Terraformable == nil || !*b.Terraformable || b.Atmosphere != "thin argon atmosphere" {
		t.Fatal("explicit observations missing", b)
	}
}
func TestExplorationDSSMappingVsPreviouslyMapped(t *testing.T) {
	m := start(t)
	apply(t, m, "A", scan)
	if m.View("A", "9007199254740993").Bodies[0].DSSMapped {
		t.Fatal("WasMapped treated as own DSS")
	}
	apply(t, m, "A", `{"event":"SAAScanComplete","SystemAddress":9007199254740993,"BodyID":9007199254740997}`)
	if !m.View("A", "9007199254740993").Bodies[0].DSSMapped {
		t.Fatal("DSS completion lost")
	}
}
func TestExplorationSignalsDoNotInventSpecies(t *testing.T) {
	m := start(t)
	apply(t, m, "A", scan)
	apply(t, m, "A", `{"event":"SAASignalsFound","SystemAddress":9007199254740993,"BodyID":9007199254740997,"Signals":[{"Type":"$SAA_SignalType_Biological;","Count":3},{"Type":"$SAA_SignalType_Geological;","Count":0}],"Genuses":[{"Genus":"$Codex_Ent_Bacterial_Genus_Name;","Genus_Localised":"Bacterium"}],"Species":"PRIVATE_PREDICTION","Latitude":23,"Materials":["PRIVATE_MATERIAL"]}`)
	v := m.View("A", "9007199254740993")
	b := v.Bodies[0]
	if b.BioSignals == nil || *b.BioSignals != 3 || b.GeoSignals == nil || *b.GeoSignals != 0 || len(b.Genuses) != 1 || b.Genuses[0] != "Bacterium" {
		t.Fatal("signal source facts lost", b)
	}
	raw, _ := json.Marshal(v)
	for _, private := range []string{"PRIVATE_", "Latitude", "Materials", "SystemAddress", "SYNTHETIC", "Species"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("private/predicted field leaked", string(raw))
		}
	}
}
func TestExplorationLargeIDsAndBoundedRows(t *testing.T) {
	m := start(t)
	apply(t, m, "A", scan)
	if id := m.View("A", "9007199254740993").Bodies[0].BodyID; id != "9007199254740997" {
		t.Fatal("rounded BodyID", id)
	}
	for i := 0; i < 513; i++ {
		apply(t, m, "A", fmt.Sprintf(`{"event":"Scan","SystemAddress":9007199254740993,"BodyID":%d,"BodyName":"Fixture Body %d","PlanetClass":"Rocky body"}`, i, i))
	}
	if v := m.View("A", "9007199254740993"); len(v.Bodies) != 512 || !v.Truncated || v.SurveyState != "partial" {
		t.Fatal("body bound not explicit", len(v.Bodies), v.SurveyState)
	}
	for i := 0; i < 10; i++ {
		apply(t, m, "A", fmt.Sprintf(`{"event":"FSDJump","SystemAddress":%d,"StarSystem":"Fixture System %d"}`, i+10, i))
	}
	if len(m.systems) > 8 {
		t.Fatal("unbounded systems")
	}
}
func TestExplorationDifferentFIDCannotShareBodies(t *testing.T) {
	m := start(t)
	apply(t, m, "A", scan)
	apply(t, m, "B", `{"event":"LoadGame","FID":"SYNTHETIC-B"}`)
	apply(t, m, "B", `{"event":"Location","StarSystem":"Fixture Alpha","SystemAddress":9007199254740993}`)
	if len(m.View("B", "9007199254740993").Bodies) != 0 {
		t.Fatal("cross commander body leak")
	}
	apply(t, m, "", `{"event":"LoadGame"}`)
	apply(t, m, "", `{"event":"Location","StarSystem":"Fixture Alpha","SystemAddress":9007199254740993}`)
	apply(t, m, "", scan)
	if len(m.View("", "9007199254740993").Bodies) != 0 {
		t.Fatal("unverified session attributed scan")
	}
}
func TestExplorationAddresslessFSSRequiresContinuousArrivalContext(t *testing.T) {
	for _, breakContext := range []string{`{"event":"StartJump","JumpType":"Hyperspace"}`, `{"event":"Fileheader"}`, `{"event":"Shutdown"}`, `{"event":"LoadGame","FID":"SYNTHETIC-A"}`} {
		m := start(t)
		apply(t, m, "A", breakContext)
		apply(t, m, "A", `{"event":"FSSDiscoveryScan","BodyCount":99,"Progress":1}`)
		if v := m.View("A", "9007199254740993"); v.ReportedBodyCount != nil {
			t.Fatal("unattested addressless FSS attributed", breakContext, v)
		}
	}
	m := start(t)
	e, _ := journal.ParseLine([]byte(`{"event":"FSSDiscoveryScan","BodyCount":99}`), journal.SourcePosition{FileID: "different-file", Offset: 0})
	m.ApplyEvent("A", e)
	if m.View("A", "9007199254740993").ReportedBodyCount != nil {
		t.Fatal("discontinuous source attributed")
	}
}

func TestExplorationSupercruiseDoesNotPretendSystemChange(t *testing.T) {
	m := start(t)
	apply(t, m, "A", `{"event":"StartJump","JumpType":"Supercruise"}`)
	apply(t, m, "A", `{"event":"FSSDiscoveryScan","BodyCount":12,"Progress":0.5}`)
	if v := m.View("A", "9007199254740993"); v.ReportedBodyCount == nil || *v.ReportedBodyCount != 12 {
		t.Fatal("supercruise blocked known-system FSS", v)
	}
}

func TestExplorationPublicPayloadHasByteBudget(t *testing.T) {
	m := start(t)
	g := `{"Genus_Localised":"` + strings.Repeat("🚀", 128) + `"}`
	for i := 0; i < 512; i++ {
		apply(t, m, "A", fmt.Sprintf(`{"event":"Scan","SystemAddress":9007199254740993,"BodyID":%d,"BodyName":"Fixture Body %d"}`, i, i))
		apply(t, m, "A", fmt.Sprintf(`{"event":"SAASignalsFound","SystemAddress":9007199254740993,"BodyID":%d,"Genuses":[%s]}`, i, g))
	}
	v := m.View("A", "9007199254740993")
	raw, _ := json.Marshal(v)
	if len(raw) > 256<<10 || !v.Truncated || v.SurveyState != "partial" {
		t.Fatal("unbounded public survey", len(raw), v.Truncated)
	}
}
