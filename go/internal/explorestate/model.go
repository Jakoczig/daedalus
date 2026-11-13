// Package explorestate retains only bounded, journal-attested survey observations.
// The enclosing ingestion service serializes access. There is no IO or inference.
package explorestate

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Jakoczig/daedalus/go/internal/journal"
)

const (
	MaxSystems   = 8
	MaxBodies    = 512
	earthGravity = 9.80665
)

type BodyView struct {
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	BodyID               string   `json:"bodyId"`
	Atmosphere           string   `json:"atmosphere"`
	Volcanism            string   `json:"volcanism"`
	Landable             *bool    `json:"landable"`
	Terraformable        *bool    `json:"terraformable"`
	GravityG             *float64 `json:"gravityG"`
	DistanceLS           *float64 `json:"distanceLS"`
	BioSignals           *int     `json:"bioSignals"`
	GeoSignals           *int     `json:"geoSignals"`
	PreviouslyDiscovered *bool    `json:"previouslyDiscovered"`
	PreviouslyMapped     *bool    `json:"previouslyMapped"`
	DSSMapped            bool     `json:"dssMapped"`
	Genuses              []string `json:"genuses"`
}
type ExplorationView struct {
	SystemName        string     `json:"systemName"`
	SurveyState       string     `json:"surveyState"`
	ReportedBodyCount *int       `json:"reportedBodyCount"`
	FSSProgress       *float64   `json:"fssProgress"`
	AllBodiesFound    bool       `json:"allBodiesFound"`
	ScannerBodyCount  *int       `json:"scannerBodyCount"`
	Bodies            []BodyView `json:"bodies"`
	Truncated         bool       `json:"truncated"`
}
type contextState struct {
	verified, arrived, pending, continuous, seen bool
	address, name, fileID                        string
	offset                                       int64
}
type systemKey struct{ commander, address string }
type bodyState struct {
	view    BodyView
	scanned bool
}
type systemState struct {
	name                   string
	count, scannerCount    *int
	progress               *float64
	all, partial, overflow bool
	bodies                 map[string]*bodyState
}
type Model struct {
	sessions map[string]*contextState
	order    []string
	systems  map[systemKey]*systemState
	lru      []systemKey
}

func NewModel() *Model { m := &Model{}; m.ResetSource(); return m }
func (m *Model) ResetSource() {
	m.sessions = map[string]*contextState{}
	m.order = nil
	m.systems = map[systemKey]*systemState{}
	m.lru = nil
}
func (m *Model) context(key string) *contextState {
	if s := m.sessions[key]; s != nil {
		return s
	}
	if len(m.order) >= 64 {
		old := m.order[0]
		delete(m.sessions, old)
		m.order = m.order[1:]
	}
	s := &contextState{}
	m.sessions[key] = s
	m.order = append(m.order, key)
	return s
}
func (m *Model) system(key systemKey, name string) *systemState {
	s := m.systems[key]
	for i, k := range m.lru {
		if k == key {
			m.lru = append(m.lru[:i], m.lru[i+1:]...)
			break
		}
	}
	if s == nil {
		if len(m.lru) >= MaxSystems {
			delete(m.systems, m.lru[0])
			m.lru = m.lru[1:]
		}
		s = &systemState{name: name, bodies: map[string]*bodyState{}}
		m.systems[key] = s
	}
	m.lru = append(m.lru, key)
	return s
}
func (m *Model) ApplyEvent(key string, e journal.Event) {
	s := m.context(key)
	// Only a new explicit session restores identity after unordered source data.
	if e.Type == "LoadGame" || e.Type == "Fileheader" {
		*s = contextState{verified: e.Type == "LoadGame" && key != "" && e.FID != "", continuous: true, seen: true, fileID: e.Position.FileID, offset: e.Position.Offset}
		return
	}
	if s.seen && (e.Position.FileID == "" || e.Position.FileID != s.fileID || e.Position.Offset <= s.offset) {
		s.continuous = false
		s.verified = false
		s.arrived = false
	}
	s.seen = true
	s.fileID = e.Position.FileID
	s.offset = e.Position.Offset
	switch e.Type {
	case "Shutdown":
		s.verified = false
		s.arrived = false
		return
	case "StartJump":
		var jump struct{ JumpType string }
		if json.Unmarshal(e.Raw, &jump) == nil && jump.JumpType == "Supercruise" {
			return
		}
		s.pending = true
		return
	case "Location", "FSDJump", "CarrierJump":
		s.pending = false
		s.address = ""
		s.name = ""
		s.arrived = false
		if !s.verified || !text(e.SystemName) || e.SystemAddress == "" {
			return
		}
		s.address = e.SystemAddress
		s.name = e.SystemName
		s.arrived = true
		s.continuous = true
		m.system(systemKey{key, s.address}, s.name)
		return
	}
	if !s.verified || !s.arrived {
		return
	}
	switch e.Type {
	case "Scan", "SAAScanComplete", "FSSBodySignals", "SAASignalsFound", "FSSDiscoveryScan", "FSSAllBodiesFound":
	default:
		return
	}
	if e.Type == "FSSDiscoveryScan" {
		if !s.continuous || s.pending || (e.SystemAddress != "" && e.SystemAddress != s.address) {
			return
		}
	} else if e.SystemAddress == "" || e.SystemAddress != s.address {
		return
	}
	target := m.system(systemKey{key, s.address}, s.name)
	var data struct {
		BodyID                                json.Number
		BodyName, StarType, PlanetClass       string
		Atmosphere, Volcanism                 *string
		Landable, WasDiscovered, WasMapped    *bool
		TerraformState                        *string
		SurfaceGravity, DistanceFromArrivalLS *json.Number
		BodyCount, Progress, Count            *json.Number
		Signals                               []struct {
			Type  string
			Count json.Number
		}
		Genuses []struct{ Genus, Genus_Localised string }
	}
	d := json.NewDecoder(bytes.NewReader(e.Raw))
	d.UseNumber()
	if len(e.Raw) > journal.MaxRecordBytes || !utf8.Valid(e.Raw) || d.Decode(&data) != nil {
		target.partial = true
		return
	}
	switch e.Type {
	case "FSSDiscoveryScan":
		if data.BodyCount != nil {
			if n, ok := count(*data.BodyCount, 100000); ok {
				target.count = &n
			} else {
				target.partial = true
			}
		}
		if data.Progress != nil {
			if n, ok := number(*data.Progress, 0, 1); ok {
				target.progress = &n
			} else {
				target.partial = true
			}
		}
		return
	case "FSSAllBodiesFound":
		if data.Count == nil {
			target.partial = true
			return
		}
		n, ok := count(*data.Count, 100000)
		if !ok {
			target.partial = true
			return
		}
		target.all = true
		target.scannerCount = &n
		return
	}
	id, ok := identifier(data.BodyID)
	if !ok {
		target.partial = true
		return
	}
	if len(data.Signals) > 32 || len(data.Genuses) > 32 {
		target.partial = true
		return
	}
	if e.Type == "Scan" && !text(data.BodyName) {
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
	b := &body.view
	switch e.Type {
	case "SAAScanComplete":
		b.DSSMapped = true
	case "Scan":
		body.scanned = true
		b.Name = data.BodyName
		if data.StarType != "" {
			if text(data.StarType) {
				b.Kind = data.StarType
			} else {
				target.partial = true
			}
		}
		if data.PlanetClass != "" {
			if text(data.PlanetClass) {
				b.Kind = data.PlanetClass
			} else {
				target.partial = true
			}
		}
		if data.Atmosphere != nil {
			if *data.Atmosphere == "" || text(*data.Atmosphere) {
				b.Atmosphere = *data.Atmosphere
			} else {
				target.partial = true
			}
		}
		if data.Volcanism != nil {
			if *data.Volcanism == "" || text(*data.Volcanism) {
				b.Volcanism = *data.Volcanism
			} else {
				target.partial = true
			}
		}
		if data.Landable != nil {
			b.Landable = copyPtr(data.Landable)
		}
		if data.WasDiscovered != nil {
			b.PreviouslyDiscovered = copyPtr(data.WasDiscovered)
		}
		if data.WasMapped != nil {
			b.PreviouslyMapped = copyPtr(data.WasMapped)
		}
		if data.TerraformState != nil {
			switch *data.TerraformState {
			case "":
				x := false
				b.Terraformable = &x
			case "Terraformable", "Terraforming", "Terraformed":
				x := true
				b.Terraformable = &x
			default:
				target.partial = true
			}
		}
		if data.SurfaceGravity != nil {
			if n, ok := number(*data.SurfaceGravity, 0, 1e6); ok {
				g := n / earthGravity
				b.GravityG = &g
			} else {
				target.partial = true
			}
		}
		if data.DistanceFromArrivalLS != nil {
			if n, ok := number(*data.DistanceFromArrivalLS, 0, 1e10); ok {
				b.DistanceLS = &n
			} else {
				target.partial = true
			}
		}
	case "SAASignalsFound", "FSSBodySignals":
		for _, signal := range data.Signals {
			n, ok := count(signal.Count, 100000)
			if !ok {
				target.partial = true
				continue
			}
			switch signal.Type {
			case "$SAA_SignalType_Biological;":
				b.BioSignals = &n
			case "$SAA_SignalType_Geological;":
				b.GeoSignals = &n
			}
		}
		if data.Genuses != nil {
			genuses := []string{}
			seen := map[string]bool{}
			for _, g := range data.Genuses {
				name := g.Genus_Localised
				if name == "" {
					name = g.Genus
				}
				if !text(name) {
					target.partial = true
					continue
				}
				if !seen[name] {
					genuses = append(genuses, name)
					seen[name] = true
				}
			}
			b.Genuses = genuses
		}
	}
}
func (m *Model) View(key, address string) ExplorationView {
	v := ExplorationView{SurveyState: "unknown", Bodies: []BodyView{}}
	s := m.sessions[key]
	if s == nil {
		return v
	}
	v.SystemName = s.name
	if !s.verified || !s.arrived || address == "" || address != s.address {
		v.SurveyState = "unverified"
		return v
	}
	state := m.systems[systemKey{key, address}]
	if state == nil {
		return v
	}
	v.ReportedBodyCount = copyPtr(state.count)
	v.FSSProgress = copyPtr(state.progress)
	v.ScannerBodyCount = copyPtr(state.scannerCount)
	v.AllBodiesFound = state.all
	v.Truncated = state.overflow
	ids := []string{}
	for id, b := range state.bodies {
		if b.scanned {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if len(ids[i]) != len(ids[j]) {
			return len(ids[i]) < len(ids[j])
		}
		return ids[i] < ids[j]
	})
	budget := 1024
	for _, id := range ids {
		b := state.bodies[id].view
		b.Landable = copyPtr(b.Landable)
		b.Terraformable = copyPtr(b.Terraformable)
		b.GravityG = copyPtr(b.GravityG)
		b.DistanceLS = copyPtr(b.DistanceLS)
		b.BioSignals = copyPtr(b.BioSignals)
		b.GeoSignals = copyPtr(b.GeoSignals)
		b.PreviouslyDiscovered = copyPtr(b.PreviouslyDiscovered)
		b.PreviouslyMapped = copyPtr(b.PreviouslyMapped)
		b.Genuses = append([]string{}, b.Genuses...)
		encoded, err := json.Marshal(b)
		if err != nil || budget+len(encoded)+1 > 256<<10 {
			v.Truncated = true
			break
		}
		budget += len(encoded) + 1
		v.Bodies = append(v.Bodies, b)
	}
	if len(v.Bodies) > 0 || state.count != nil || state.progress != nil {
		v.SurveyState = "observed"
	}
	if state.all {
		v.SurveyState = "scanner_complete"
	}
	if state.partial || v.Truncated {
		v.SurveyState = "partial"
	}
	if s.pending {
		v.SurveyState = "in_transit_last_known"
	}
	return v
}
func copyPtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func identifier(n json.Number) (string, bool) {
	s := string(n)
	if s == "" || strings.ContainsAny(s, "+-.eE") {
		return "", false
	}
	x, err := strconv.ParseUint(s, 10, 64)
	return strconv.FormatUint(x, 10), err == nil
}
func count(n json.Number, max int) (int, bool) {
	id, ok := identifier(n)
	if !ok {
		return 0, false
	}
	x, err := strconv.ParseUint(id, 10, 64)
	return int(x), err == nil && x <= uint64(max)
}
func number(n json.Number, min, max float64) (float64, bool) {
	x, err := n.Float64()
	return x, err == nil && !math.IsNaN(x) && !math.IsInf(x, 0) && x >= min && x <= max
}
func text(s string) bool {
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > 128 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
