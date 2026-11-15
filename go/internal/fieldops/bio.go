package fieldops

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"math"
	"strings"
	"time"
)

//go:embed bio-rules.json
var bioRules []byte

type Candidate struct {
	Genus     string           `json:"genus"`
	Symbol    string           `json:"symbol"`
	Name      string           `json:"name"`
	State     string           `json:"state"`
	Evidence  []string         `json:"evidence"`
	MinimumM  *float64         `json:"minimumM"`
	BaseValue *float64         `json:"baseValue"`
	Criteria  []map[string]any `json:"criteria"`
}
type criteriaSpecies struct {
	Genus, Symbol, Name string
	Value               *float64
	Rules               []map[string]any
}
type criteriaPack struct {
	GenusNames      map[string]string
	Version, Notice string
	Distances       map[string]float64
	Species         []criteriaSpecies
}

var pack = func() criteriaPack {
	var p criteriaPack
	if json.Unmarshal(bioRules, &p) != nil {
		panic("Invalid embedded BioScan criteria")
	}
	return p
}()

func BioRuleVersion() string { return pack.Version }
func MinimumDistance(genus string) *float64 {
	if v, ok := pack.Distances[genus]; ok && v > 0 {
		return &v
	}
	return nil
}

// Infer only evaluates witnessed environmental fields. Unsupported criteria
// remain unknown; an OR alternative cannot be dropped to produce a false match.
func Infer(scan map[string]any, genera []string) []Candidate {
	out := []Candidate{}
	for _, s := range pack.Species {
		if len(genera) > 0 {
			found := false
			for _, g := range genera {
				if g == s.Genus || strings.EqualFold(g, pack.GenusNames[s.Genus]) {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		c := Candidate{Genus: s.Genus, Symbol: s.Symbol, Name: s.Name, State: "unknown criteria", Evidence: []string{}, MinimumM: MinimumDistance(s.Genus), BaseValue: s.Value, Criteria: s.Rules}
		anyUnknown := len(s.Rules) == 0
		matched := false
		for _, rule := range s.Rules {
			unknown, excluded := false, false
			for k, want := range rule {
				field := map[string]string{"atmosphere": "AtmosphereType", "body_type": "PlanetClass", "volcanism": "Volcanism", "min_temperature": "SurfaceTemperature", "max_temperature": "SurfaceTemperature", "min_gravity": "SurfaceGravity", "max_gravity": "SurfaceGravity", "min_pressure": "SurfacePressure", "max_pressure": "SurfacePressure"}[k]
				if field == "" {
					unknown = true
					continue
				}
				if field == "Volcanism" && want != "None" {
					unknown = true
					continue
				}
				got, ok := scan[field]
				if !ok {
					unknown = true
					continue
				}
				if strings.HasPrefix(k, "min_") || strings.HasPrefix(k, "max_") {
					v, ok := floatValue(got)
					n, nok := want.(float64)
					if !ok || !nok {
						unknown = true
						continue
					}
					if field == "SurfaceGravity" {
						v /= 9.80665
					}
					if field == "SurfacePressure" {
						v /= 101325
					}
					if (strings.HasPrefix(k, "min_") && v < n) || (strings.HasPrefix(k, "max_") && v > n) {
						excluded = true
					}
				} else {
					v, ok := got.(string)
					if !ok {
						unknown = true
						continue
					}
					if a, ok := want.([]any); ok {
						found := false
						for _, x := range a {
							if x == v {
								found = true
							}
						}
						if !found {
							excluded = true
						}
					} else if want == "None" {
						if v != "" && !strings.EqualFold(v, "None") && !strings.EqualFold(v, "No volcanism") {
							excluded = true
						}
					} else {
						unknown = true
					}
				}
			}
			if !excluded {
				if unknown {
					anyUnknown = true
				} else {
					matched = true
				}
			}
		}
		if matched {
			c.State = "possible by known criteria"
		} else if !anyUnknown {
			c.State = "excluded by a witnessed condition"
		}
		c.Evidence = append(c.Evidence, "Pinned environmental rules "+pack.Version, "Regional, stellar, parent and nebula requirements are unknown when needed; never a probability")
		out = append(out, c)
		if len(out) >= 128 {
			break
		}
	}
	return out
}
func floatValue(x any) (float64, bool) {
	var v float64
	switch x := x.(type) {
	case float64:
		v = x
	case json.Number:
		var e error
		v, e = x.Float64()
		if e != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return v, !math.IsNaN(v) && !math.IsInf(v, 0)
}

type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    float64 `json:"radiusM"`
	At        string  `json:"at"`
}

func validPosition(p Position) bool {
	return !math.IsNaN(p.Latitude) && !math.IsInf(p.Latitude, 0) && math.Abs(p.Latitude) <= 90 && !math.IsNaN(p.Longitude) && !math.IsInf(p.Longitude, 0) && math.Abs(p.Longitude) <= 180 && p.Radius > 0 && p.Radius < 1e9
}
func SurfaceDistance(a, b Position) (float64, bool) {
	if !validPosition(a) || !validPosition(b) || math.Abs(a.Radius-b.Radius) > 1 {
		return 0, false
	}
	lat1, lat2 := a.Latitude*math.Pi/180, b.Latitude*math.Pi/180
	dlat, dlon := lat2-lat1, (b.Longitude-a.Longitude)*math.Pi/180
	x := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
	return 2 * a.Radius * math.Asin(math.Sqrt(math.Max(0, math.Min(1, x)))), true
}
func StatusPosition(raw []byte, body string, sample, now time.Time) (Position, bool) {
	var v map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&v) != nil || v["event"] != "Status" || body == "" || v["BodyName"] != body {
		return Position{}, false
	}
	flags, ok := v["Flags"].(json.Number)
	if !ok {
		return Position{}, false
	}
	n, e := flags.Int64()
	if e != nil || n&(1<<21) == 0 {
		return Position{}, false
	}
	at, e := time.Parse(time.RFC3339Nano, stringValue(v["timestamp"]))
	if e != nil || math.Abs(at.Sub(sample).Seconds()) > 2 || math.Abs(now.Sub(at).Seconds()) > 5 {
		return Position{}, false
	}
	lat, a := floatValue(v["Latitude"])
	lon, b := floatValue(v["Longitude"])
	radius, c := floatValue(v["PlanetRadius"])
	p := Position{lat, lon, radius, at.UTC().Format(time.RFC3339Nano)}
	return p, a && b && c && validPosition(p)
}
func stringValue(v any) string { s, _ := v.(string); return s }
