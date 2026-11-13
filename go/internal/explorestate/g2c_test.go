package explorestate

import (
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"strings"
	"testing"
)

func BenchmarkSurveySnapshot(b *testing.B) {
	m := NewModel()
	feed := func(raw string) {
		e, err := journalEvent(raw)
		if err != nil {
			b.Fatal(err)
		}
		m.ApplyEvent("A", e)
	}
	feed(`{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	feed(`{"event":"Location","StarSystem":"Fixture","SystemAddress":9007199254740993}`)
	for i := 0; i < 500; i++ {
		feed(fmt.Sprintf(`{"event":"Scan","SystemAddress":9007199254740993,"BodyID":%d,"BodyName":"Fixture %d","PlanetClass":"Rocky body","SurfaceGravity":9.8}`, i, i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		v := m.View("A", "9007199254740993")
		if len(v.Bodies) != 500 {
			b.Fatal("lost bodies")
		}
	}
}
func journalEvent(raw string) (journal.Event, error) {
	return journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "snapshot-fixture", Offset: offset.Add(1)})
}

func TestG2CPhysicalDetailsAndExactParents(t *testing.T) {
	m := start(t)
	apply(t, m, "A", `{"event":"Scan","SystemAddress":9007199254740993,"BodyID":9007199254740997,"BodyName":"Fixture 1","PlanetClass":"Rocky body","Radius":1000000,"SurfaceTemperature":250,"SurfacePressure":101325,"OrbitalPeriod":86400,"RotationPeriod":-3600,"SemiMajorAxis":149597870700,"Parents":[{"Planet":9007199254740999},{"Star":0}],"Rings":[{"Name":"Fixture ring","RingClass":"eRingClass_Rocky","InnerRad":2000000,"OuterRad":3000000}],"Composition":{"Ice":0.1,"Rock":0.9},"PRIVATE_SECRET":"never"}`)
	raw, _ := json.Marshal(m.View("A", "9007199254740993"))
	var v struct {
		Bodies []struct {
			Details struct {
				RadiusKM, TemperatureK, PressureATM, OrbitalDays, RotationHours, SemiMajorAU *float64
				Parents                                                                      []struct{ BodyID string }
				Rings                                                                        []struct{ InnerKM *float64 }
			}
		}
	}
	if json.Unmarshal(raw, &v) != nil || len(v.Bodies) != 1 {
		t.Fatal(string(raw))
	}
	d := v.Bodies[0].Details
	if d.RadiusKM == nil || *d.RadiusKM != 1000 || *d.TemperatureK != 250 || *d.PressureATM != 1 || *d.OrbitalDays != 1 || *d.RotationHours != -1 || *d.SemiMajorAU != 1 || d.Parents[0].BodyID != "9007199254740999" || *d.Rings[0].InnerKM != 2000 {
		t.Fatal("physical units/exact parent missing", string(raw))
	}
	if strings.Contains(string(raw), "PRIVATE_") {
		t.Fatal("raw data leaked")
	}
}
func TestG2COrganicPhasesAreNotAdditiveAndAreIsolated(t *testing.T) {
	m := start(t)
	organic := func(phase, stamp, species string) {
		apply(t, m, "A", fmt.Sprintf(`{"event":"ScanOrganic","timestamp":"2026-01-01T00:00:%02sZ","SystemAddress":9007199254740993,"Body":7,"Genus":"genus-1","Genus_Localised":"Bacterium","Species":"%s","Species_Localised":"Confirmed %s","ScanType":"%s","Latitude":23,"PRIVATE_SECRET":"never"}`, stamp, species, species, phase))
	}
	organic("Log", "01", "one")
	organic("Sample", "02", "one")
	organic("Sample", "02", "one")
	v := m.View("A", "9007199254740993")
	raw, _ := json.Marshal(v)
	if !strings.Contains(string(raw), `"samples":2`) || !strings.Contains(string(raw), `"stage":"sampled"`) || strings.Contains(string(raw), "Latitude") || strings.Contains(string(raw), "PRIVATE_") {
		t.Fatal(string(raw))
	}
	organic("Log", "03", "two")
	raw, _ = json.Marshal(m.View("A", "9007199254740993"))
	if !strings.Contains(string(raw), `"stage":"interrupted"`) {
		t.Fatal("old active sample retained", string(raw))
	}
	organic("Analyse", "04", "two")
	raw, _ = json.Marshal(m.View("A", "9007199254740993"))
	if !strings.Contains(string(raw), `"complete":true`) {
		t.Fatal(string(raw))
	}
	apply(t, m, "B", `{"event":"LoadGame","FID":"SYNTHETIC-B"}`)
	apply(t, m, "B", `{"event":"Location","StarSystem":"Fixture","SystemAddress":9007199254740993}`)
	if len(m.View("B", "9007199254740993").Bodies) != 0 {
		t.Fatal("commander leaked biology")
	}
	apply(t, m, "A", `{"event":"FSDJump","StarSystem":"Other","SystemAddress":2}`)
	if len(m.View("A", "2").Bodies) != 0 {
		t.Fatal("system leaked biology")
	}
}
func TestG2COrganicUnknownHistoryAndInvalidSource(t *testing.T) {
	m := start(t)
	apply(t, m, "A", `{"event":"ScanOrganic","timestamp":"2026-01-01T00:00:01Z","SystemAddress":9007199254740993,"Body":7,"Genus":"Bacterium","Species":"one","ScanType":"Sample"}`)
	raw, _ := json.Marshal(m.View("A", "9007199254740993"))
	if !strings.Contains(string(raw), `"samples":null`) {
		t.Fatal("invented start", string(raw))
	}
	apply(t, m, "A", `{"event":"ScanOrganic","SystemAddress":2,"Body":7,"Genus":"Bad","Species":"wrong-system","ScanType":"Analyse"}`)
	raw, _ = json.Marshal(m.View("A", "9007199254740993"))
	if strings.Contains(string(raw), "wrong-system") {
		t.Fatal("wrong system bio accepted")
	}
}

func TestG2CCompletedReplayCannotInterruptNewSampler(t *testing.T) {
	m := start(t)
	feed := func(phase, species, stamp string) {
		apply(t, m, "A", fmt.Sprintf(`{"event":"ScanOrganic","timestamp":"2026-01-01T00:00:%sZ","SystemAddress":9007199254740993,"Body":7,"Genus":"Bacterium","Species":"%s","ScanType":"%s"}`, stamp, species, phase))
	}
	feed("Analyse", "old", "01")
	feed("Log", "new", "02")
	feed("Analyse", "old", "03")
	raw, _ := json.Marshal(m.View("A", "9007199254740993"))
	if strings.Contains(string(raw), `"stage":"interrupted"`) {
		t.Fatal("completed organism interrupted active run", string(raw))
	}
}
func TestG2CDetailAndOrganismSnapshotCopiesAndInvalidation(t *testing.T) {
	m := start(t)
	apply(t, m, "A", `{"event":"Scan","SystemAddress":9007199254740993,"BodyID":7,"BodyName":"Fixture 7","Radius":1000,"Parents":[{"Star":0}],"Rings":[{"Name":"Ring","RingClass":"Rocky","InnerRad":1000,"OuterRad":2000}]}`)
	apply(t, m, "A", `{"event":"ScanOrganic","timestamp":"2026-01-01T00:00:01Z","SystemAddress":9007199254740993,"Body":7,"Genus":"Bacterium","Species":"one","ScanType":"Log"}`)
	v := m.View("A", "9007199254740993")
	v.Bodies[0].Details.Parents[0].BodyID = "evil"
	*v.Bodies[0].Details.RadiusKM = 42
	*v.Bodies[0].Details.Rings[0].InnerKM = 42
	*v.Bodies[0].Organisms[0].Samples = 42
	got := m.View("A", "9007199254740993")
	if *got.Bodies[0].Details.RadiusKM != 1 || got.Bodies[0].Details.Parents[0].BodyID != "0" || *got.Bodies[0].Details.Rings[0].InnerKM != 1 || *got.Bodies[0].Organisms[0].Samples != 1 {
		t.Fatal("cache aliases caller")
	}
	apply(t, m, "A", `{"event":"Scan","SystemAddress":9007199254740993,"BodyID":7,"BodyName":"Fixture 7","Radius":2000}`)
	if *m.View("A", "9007199254740993").Bodies[0].Details.RadiusKM != 2 {
		t.Fatal("cache did not invalidate")
	}
	apply(t, m, "A", `{"event":"Scan","SystemAddress":9007199254740993,"BodyID":7,"BodyName":"Fixture 7","Radius":"3000","SurfaceTemperature":null,"Parents":[{"Planet":1.5}]}`)
	got = m.View("A", "9007199254740993")
	if got.SurveyState != "partial" || *got.Bodies[0].Details.RadiusKM != 2 || got.Bodies[0].Details.TemperatureK != nil {
		t.Fatal("invalid physical data claimed", got)
	}
}
func TestG2COrganicBoundsAndExactBody(t *testing.T) {
	m := start(t)
	for i := 0; i < 40; i++ {
		apply(t, m, "A", fmt.Sprintf(`{"event":"ScanOrganic","timestamp":"2026-01-01T00:00:01Z","SystemAddress":9007199254740993,"Body":9007199254740999,"Genus":"Bacterium","Species":"species-%d","ScanType":"Analyse"}`, i))
	}
	v := m.View("A", "9007199254740993")
	if len(v.Bodies) != 1 || len(v.Bodies[0].Organisms) != 32 || v.Bodies[0].BodyID != "9007199254740999" || v.SurveyState != "partial" {
		t.Fatal("unbounded/rounded organic evidence", v)
	}
	raw, _ := json.Marshal(v)
	if len(raw) > 256<<10 {
		t.Fatal("budget exceeded")
	}
}
