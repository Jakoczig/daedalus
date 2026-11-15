package fieldops

import (
	"math"
	"testing"
	"time"
)

func TestPinnedCriteriaRemainPossibleOrUnknownNeverConfirmed(t *testing.T) {
	scan := map[string]any{"AtmosphereType": "CarbonDioxide", "PlanetClass": "High metal content body", "SurfaceTemperature": 180.0, "SurfaceGravity": 1.0, "Volcanism": "No volcanism"}
	find := func() string {
		t.Helper()
		for _, c := range Infer(scan, []string{"$Codex_Ent_Stratum_Genus_Name;"}) {
			if c.Name == "Stratum Tectonicas" {
				return c.State
			}
		}
		t.Fatal("pinned species missing")
		return ""
	}
	if find() != "possible by known criteria" {
		t.Fatal(find())
	}
	delete(scan, "SurfaceTemperature")
	if find() != "unknown criteria" {
		t.Fatal(find())
	}
	scan["SurfaceTemperature"] = 500.0
	if find() != "excluded by a witnessed condition" {
		t.Fatal(find())
	}
	if MinimumDistance("unknown") != nil {
		t.Fatal("invented distance")
	}
}

func TestSurfaceDistanceAndCorrelation(t *testing.T) {
	a := Position{Latitude: 0, Longitude: 179.9, Radius: 1e6}
	b := Position{Latitude: 0, Longitude: -179.9, Radius: 1e6}
	d, ok := SurfaceDistance(a, b)
	if !ok || math.Abs(d-3490.6585) > 0.1 {
		t.Fatal(d, ok)
	}
	b.Latitude = 91
	if _, ok = SurfaceDistance(a, b); ok {
		t.Fatal("impossible latitude")
	}
	b = Position{Latitude: 90, Longitude: 20, Radius: 1e6}
	a = Position{Latitude: 90, Longitude: -140, Radius: 1e6}
	if d, ok = SurfaceDistance(a, b); !ok || d > 0.01 {
		t.Fatal(d)
	}
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	raw := []byte(`{"event":"Status","timestamp":"2026-11-15T12:00:00Z","Flags":2097152,"BodyName":"Fixture 1","Latitude":2,"Longitude":3,"PlanetRadius":1000000}`)
	if _, ok := StatusPosition(raw, "Fixture 1", at, at); !ok {
		t.Fatal("valid status rejected")
	}
	for _, x := range []struct {
		name string
		at   time.Time
	}{{"Other", at}, {"Fixture 1", at.Add(10 * time.Second)}} {
		if _, ok := StatusPosition(raw, x.name, x.at, x.at); ok {
			t.Fatal("uncorrelated status")
		}
	}
}
