package explorestate

import (
	"bytes"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"math"
	"time"
)

type ParentView struct {
	Kind   string `json:"kind"`
	BodyID string `json:"bodyId"`
}
type FractionView struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}
type RingView struct {
	Name    string   `json:"name"`
	Class   string   `json:"class"`
	InnerKM *float64 `json:"innerKM"`
	OuterKM *float64 `json:"outerKM"`
}
type BodyDetails struct {
	IsStar                bool           `json:"isStar"`
	RadiusKM              *float64       `json:"radiusKM"`
	TemperatureK          *float64       `json:"temperatureK"`
	PressureATM           *float64       `json:"pressureATM"`
	MassEarths            *float64       `json:"massEarths"`
	MassSolar             *float64       `json:"massSolar"`
	AgeMY                 *float64       `json:"ageMY"`
	OrbitalDays           *float64       `json:"orbitalDays"`
	RotationHours         *float64       `json:"rotationHours"`
	SemiMajorAU           *float64       `json:"semiMajorAU"`
	Eccentricity          *float64       `json:"eccentricity"`
	AxialTiltDegrees      *float64       `json:"axialTiltDegrees"`
	TidalLock             *bool          `json:"tidalLock"`
	Luminosity            string         `json:"luminosity"`
	Parents               []ParentView   `json:"parents"`
	Rings                 []RingView     `json:"rings"`
	Composition           []FractionView `json:"composition"`
	AtmosphereComposition []FractionView `json:"atmosphereComposition"`
}
type OrganismView struct {
	Genus          string `json:"genus"`
	Species        string `json:"species"`
	Variant        string `json:"variant"`
	Samples        *int   `json:"samples"`
	Stage          string `json:"stage"`
	Complete       bool   `json:"complete"`
	LastObservedAt string `json:"lastObservedAt"`
}
type organismState struct {
	view          OrganismView
	key, lastKind string
}
type surveyCache struct {
	key, address string
	view         ExplorationView
}

// Parse only named public physical observations. Raw metadata and materials never
// enter the DTO. The limits below apply before allocating public array entries.
func physical(raw []byte, body *BodyView) bool {
	present := false
	for _, field := range []string{`"Radius"`, `"SurfaceTemperature"`, `"SurfacePressure"`, `"MassEM"`, `"StellarMass"`, `"Age_MY"`, `"OrbitalPeriod"`, `"RotationPeriod"`, `"SemiMajorAxis"`, `"Eccentricity"`, `"AxialTilt"`, `"StarType"`, `"TidalLock"`, `"Luminosity"`, `"Parents"`, `"Rings"`, `"Composition"`, `"AtmosphereComposition"`} {
		if bytes.Contains(raw, []byte(field)) {
			present = true
			break
		}
	}
	if !present {
		return true
	}

	var fields map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&fields) != nil {
		return false
	}
	details := body.Details
	valid := true
	ensure := func() *BodyDetails {
		if details == nil {
			details = &BodyDetails{}
		}
		return details
	}
	scalars := []struct {
		field         string
		min, max, div float64
		set           func(*BodyDetails, *float64)
	}{
		{"Radius", 0, 1e14, 1000, func(d *BodyDetails, v *float64) { d.RadiusKM = v }},
		{"SurfaceTemperature", 0, 1e9, 1, func(d *BodyDetails, v *float64) { d.TemperatureK = v }},
		{"SurfacePressure", 0, 1e18, 101325, func(d *BodyDetails, v *float64) { d.PressureATM = v }},
		{"MassEM", 0, 1e12, 1, func(d *BodyDetails, v *float64) { d.MassEarths = v }},
		{"StellarMass", 0, 1e9, 1, func(d *BodyDetails, v *float64) { d.MassSolar = v }},
		{"Age_MY", 0, 1e9, 1, func(d *BodyDetails, v *float64) { d.AgeMY = v }},
		{"OrbitalPeriod", 0, 1e18, 86400, func(d *BodyDetails, v *float64) { d.OrbitalDays = v }},
		{"RotationPeriod", -1e18, 1e18, 3600, func(d *BodyDetails, v *float64) { d.RotationHours = v }},
		{"SemiMajorAxis", 0, 1e20, 149597870700, func(d *BodyDetails, v *float64) { d.SemiMajorAU = v }},
		{"Eccentricity", 0, 100, 1, func(d *BodyDetails, v *float64) { d.Eccentricity = v }},
		{"AxialTilt", -2 * math.Pi, 2 * math.Pi, math.Pi / 180, func(d *BodyDetails, v *float64) { d.AxialTiltDegrees = v }},
	}
	for _, f := range scalars {
		if value, present := fields[f.field]; present {
			n, ok := value.(json.Number)
			x, good := number(n, f.min, f.max)
			if !ok || !good {
				valid = false
				continue
			}
			x /= f.div
			f.set(ensure(), &x)
		}
	}
	if v, ok := fields["StarType"].(string); ok && text(v) {
		ensure().IsStar = true
	}
	if v, present := fields["TidalLock"]; present {
		if b, ok := v.(bool); ok {
			ensure().TidalLock = &b
		} else {
			valid = false
		}
	}
	if v, present := fields["Luminosity"]; present {
		if name, ok := v.(string); ok && text(name) {
			ensure().Luminosity = name
		} else {
			valid = false
		}
	}
	if v, present := fields["Parents"]; present {
		parents, ok := v.([]any)
		if !ok || len(parents) > 32 {
			valid = false
		} else {
			out := []ParentView{}
			for _, p := range parents {
				obj, ok := p.(map[string]any)
				if !ok || len(obj) != 1 {
					valid = false
					continue
				}
				for kind, value := range obj {
					n, ok := value.(json.Number)
					id, good := identifier(n)
					if !ok || !good || (kind != "Star" && kind != "Planet" && kind != "Null") {
						valid = false
						continue
					}
					out = append(out, ParentView{kind, id})
				}
			}
			ensure().Parents = out
		}
	}
	if v, present := fields["Rings"]; present {
		rings, ok := v.([]any)
		if !ok || len(rings) > 16 {
			valid = false
		} else {
			out := []RingView{}
			for _, value := range rings {
				obj, ok := value.(map[string]any)
				if !ok {
					valid = false
					continue
				}
				name, _ := obj["Name"].(string)
				class, _ := obj["RingClass"].(string)
				if !text(name) || !text(class) {
					valid = false
					continue
				}
				ring := RingView{Name: name, Class: class}
				for _, field := range []string{"InnerRad", "OuterRad"} {
					if raw, present := obj[field]; present {
						n, ok := raw.(json.Number)
						x, good := number(n, 0, 1e15)
						if !ok || !good {
							valid = false
							continue
						}
						x /= 1000
						if field == "InnerRad" {
							ring.InnerKM = &x
						} else {
							ring.OuterKM = &x
						}
					}
				}
				if ring.InnerKM != nil && ring.OuterKM != nil && *ring.InnerKM > *ring.OuterKM {
					valid = false
					continue
				}
				out = append(out, ring)
			}
			ensure().Rings = out
		}
	}
	if v, present := fields["Composition"]; present {
		obj, ok := v.(map[string]any)
		if !ok || len(obj) > 16 {
			valid = false
		} else {
			out := []FractionView{}
			for _, name := range []string{"Ice", "Rock", "Metal"} {
				if value, present := obj[name]; present {
					n, ok := value.(json.Number)
					x, good := number(n, 0, 1)
					if !ok || !good {
						valid = false
						continue
					}
					out = append(out, FractionView{name, x * 100})
				}
			}
			ensure().Composition = out
		}
	}
	if v, present := fields["AtmosphereComposition"]; present {
		items, ok := v.([]any)
		if !ok || len(items) > 32 {
			valid = false
		} else {
			out := []FractionView{}
			for _, value := range items {
				obj, ok := value.(map[string]any)
				if !ok {
					valid = false
					continue
				}
				name, _ := obj["Name"].(string)
				n, ok := obj["Percent"].(json.Number)
				x, good := number(n, 0, 100)
				if !text(name) || !ok || !good {
					valid = false
					continue
				}
				out = append(out, FractionView{name, x})
			}
			ensure().AtmosphereComposition = out
		}
	}
	body.Details = details
	return valid
}
func (m *Model) organic(target *systemState, context *contextState, e journal.Event) {
	var data struct {
		Body                                                                                     any
		ScanType, Genus, Genus_Localised, Species, Species_Localised, Variant, Variant_Localised string
	}
	d := json.NewDecoder(bytes.NewReader(e.Raw))
	d.UseNumber()
	if d.Decode(&data) != nil || !text(data.Genus) || !text(data.Species) || e.Timestamp.IsZero() {
		target.partial = true
		return
	}
	bodyNumber, numeric := data.Body.(json.Number)
	id, ok := identifier(bodyNumber)
	ok = ok && numeric
	if !ok {
		target.partial = true
		return
	}
	if data.ScanType != "Log" && data.ScanType != "Sample" && data.ScanType != "Analyse" {
		target.partial = true
		return
	}
	body := target.bodies[id]
	if body == nil {
		if len(target.bodies) >= MaxBodies {
			target.overflow = true
			return
		}
		body = &bodyState{view: BodyView{BodyID: id, Genuses: []string{}}}
		target.bodies[id] = body
	}
	key := data.Genus + "\x00" + data.Species
	var organism *organismState
	for _, o := range body.organisms {
		if o.key == key {
			organism = o
			break
		}
	}
	if organism == nil {
		if len(body.organisms) >= 32 {
			target.partial = true
			return
		}
		organism = &organismState{key: key}
		body.organisms = append(body.organisms, organism)
	}
	// Completed observations never become an active sampler again.
	if organism.view.Complete {
		return
	}
	stamp := e.Timestamp.UTC().Format(time.RFC3339Nano)
	// Reject reordered events and duplicate phase observations without counting them.
	if organism.view.LastObservedAt != "" {
		last, _ := time.Parse(time.RFC3339Nano, organism.view.LastObservedAt)
		if e.Timestamp.Before(last) || stamp == organism.view.LastObservedAt && organism.lastKind == data.ScanType {
			return
		}
	}
	name := func(local, raw string) string {
		if text(local) {
			return local
		}
		if local != "" {
			target.partial = true
		}
		if text(raw) {
			return raw
		}
		return ""
	}
	organism.view.Genus = name(data.Genus_Localised, data.Genus)
	organism.view.Species = name(data.Species_Localised, data.Species)
	organism.view.Variant = name(data.Variant_Localised, data.Variant)
	if context.activeOrganic != nil && context.activeOrganic != organism && !context.activeOrganic.view.Complete {
		context.activeOrganic.view.Stage = "interrupted"
		context.activeOrganic.view.Samples = nil
	}
	switch data.ScanType {
	case "Log":
		if !organism.view.Complete {
			n := 1
			organism.view.Samples = &n
			organism.view.Stage = "logged"
		}
	case "Sample":
		if !organism.view.Complete {
			if organism.view.Stage == "logged" || organism.view.Stage == "sampled" {
				n := 2
				organism.view.Samples = &n
				organism.view.Stage = "sampled"
			} else {
				organism.view.Samples = nil
				organism.view.Stage = "partial_history"
			}
		}
	case "Analyse":
		n := 3
		organism.view.Samples = &n
		organism.view.Stage = "complete"
		organism.view.Complete = true
	}
	organism.lastKind = data.ScanType
	organism.view.LastObservedAt = stamp
	if organism.view.Complete {
		context.activeOrganic = nil
	} else {
		context.activeOrganic = organism
	}
}

// CloneBody is the single deep-copy boundary used by cache, model views and
// each independently delivered service subscription.
func CloneBody(b BodyView) BodyView {
	b.Landable = copyPtr(b.Landable)
	b.Terraformable = copyPtr(b.Terraformable)
	b.GravityG = copyPtr(b.GravityG)
	b.DistanceLS = copyPtr(b.DistanceLS)
	b.BioSignals = copyPtr(b.BioSignals)
	b.GeoSignals = copyPtr(b.GeoSignals)
	b.PreviouslyDiscovered = copyPtr(b.PreviouslyDiscovered)
	b.PreviouslyMapped = copyPtr(b.PreviouslyMapped)
	b.Genuses = append([]string{}, b.Genuses...)
	b.Organisms = append([]OrganismView(nil), b.Organisms...)
	for i := range b.Organisms {
		b.Organisms[i].Samples = copyPtr(b.Organisms[i].Samples)
	}
	if b.Details != nil {
		d := *b.Details
		d.RadiusKM = copyPtr(d.RadiusKM)
		d.TemperatureK = copyPtr(d.TemperatureK)
		d.PressureATM = copyPtr(d.PressureATM)
		d.MassEarths = copyPtr(d.MassEarths)
		d.MassSolar = copyPtr(d.MassSolar)
		d.AgeMY = copyPtr(d.AgeMY)
		d.OrbitalDays = copyPtr(d.OrbitalDays)
		d.RotationHours = copyPtr(d.RotationHours)
		d.SemiMajorAU = copyPtr(d.SemiMajorAU)
		d.Eccentricity = copyPtr(d.Eccentricity)
		d.AxialTiltDegrees = copyPtr(d.AxialTiltDegrees)
		d.TidalLock = copyPtr(d.TidalLock)
		d.Parents = append([]ParentView(nil), d.Parents...)
		d.Rings = append([]RingView(nil), d.Rings...)
		for i := range d.Rings {
			d.Rings[i].InnerKM = copyPtr(d.Rings[i].InnerKM)
			d.Rings[i].OuterKM = copyPtr(d.Rings[i].OuterKM)
		}
		d.Composition = append([]FractionView(nil), d.Composition...)
		d.AtmosphereComposition = append([]FractionView(nil), d.AtmosphereComposition...)
		b.Details = &d
	}
	return b
}
func cloneView(v ExplorationView) ExplorationView {
	v.ReportedBodyCount = copyPtr(v.ReportedBodyCount)
	v.ScannerBodyCount = copyPtr(v.ScannerBodyCount)
	v.FSSProgress = copyPtr(v.FSSProgress)
	v.Bodies = append([]BodyView{}, v.Bodies...)
	for i := range v.Bodies {
		v.Bodies[i] = CloneBody(v.Bodies[i])
	}
	return v
}
func (m *Model) View(key, address string) ExplorationView {
	if c := m.cached; c != nil && c.key == key && c.address == address {
		return cloneView(c.view)
	}
	v := m.buildView(key, address)
	m.cached = &surveyCache{key, address, v}
	return cloneView(v)
}
