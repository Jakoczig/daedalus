// Package shipstate projects only explicit journal ship facts. No status positions,
// custom identifiers, modules, credits, file IO or game commands are exposed.
package shipstate

import (
	"bytes"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"math"
	"strconv"
	"strings"
)

type View struct {
	ShipType       string   `json:"shipType"`
	Evidence       string   `json:"evidence"`
	HullHealth     *float64 `json:"hullHealth"`
	FuelMainT      *float64 `json:"fuelMainT"`
	FuelCapacityT  *float64 `json:"fuelCapacityT"`
	CargoCapacityT *float64 `json:"cargoCapacityT"`
	MaxJumpRangeLY *float64 `json:"maxJumpRangeLY"`
	UnladenMassT   *float64 `json:"unladenMassT"`
	Docked         *bool    `json:"docked"`
	Landed         *bool    `json:"landed"`
}
type state struct {
	v        View
	verified bool
	file, id string
	offset   int64
	sequence uint64
}
type Model struct {
	states map[string]*state
	order  []string
}

func NewModel() *Model        { return &Model{states: map[string]*state{}} }
func (m *Model) ResetSource() { m.states = map[string]*state{}; m.order = nil }
func copyPtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func Clone(v View) View {
	v.HullHealth = copyPtr(v.HullHealth)
	v.FuelMainT = copyPtr(v.FuelMainT)
	v.FuelCapacityT = copyPtr(v.FuelCapacityT)
	v.CargoCapacityT = copyPtr(v.CargoCapacityT)
	v.MaxJumpRangeLY = copyPtr(v.MaxJumpRangeLY)
	v.UnladenMassT = copyPtr(v.UnladenMassT)
	v.Docked = copyPtr(v.Docked)
	v.Landed = copyPtr(v.Landed)
	return v
}
func (m *Model) View(key string) View {
	if s := m.states[key]; s != nil {
		return Clone(s.v)
	}
	return View{Evidence: "unknown"}
}
func shipType(value any) string {
	s, ok := value.(string)
	if !ok || len(s) > 64 {
		return ""
	}
	s = strings.ToLower(s)
	for _, c := range s {
		if c != '_' && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return ""
		}
	}
	return s
}
func shipID(value any) string {
	n, ok := value.(json.Number)
	if !ok || strings.ContainsAny(string(n), "+-.eE") {
		return ""
	}
	v, err := strconv.ParseUint(string(n), 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(v, 10)
}
func (m *Model) ApplyEvent(key string, e journal.Event) {
	s := m.states[key]
	if s == nil {
		if len(m.order) >= 64 {
			delete(m.states, m.order[0])
			m.order = m.order[1:]
		}
		s = &state{v: View{Evidence: "unknown"}}
		m.states[key] = s
		m.order = append(m.order, key)
	}
	if s.file != "" && (e.Position.FileID != s.file || e.Position.Offset <= s.offset || (s.sequence != 0 && e.Position.Sequence != 0 && e.Position.Sequence != s.sequence+1)) {
		s.verified = false
		s.v = View{Evidence: "unverified"}
	}
	defer func() { s.file = e.Position.FileID; s.offset = e.Position.Offset; s.sequence = e.Position.Sequence }()
	if e.Type == "LoadGame" || e.Type == "Fileheader" {
		*s = state{v: View{Evidence: "unknown"}, verified: e.Type == "LoadGame" && key != "" && e.FID != ""}
	}
	if !s.verified {
		return
	}
	switch e.Type {
	case "LoadGame", "Loadout", "FSDJump", "FuelScoop", "HullDamage", "RefuelAll", "RefuelPartial", "Docked", "Undocked", "Touchdown", "Liftoff", "Location", "ShipyardSwap", "ShipyardBuy", "Shutdown":
	default:
		return
	}
	if len(e.Raw) > journal.MaxRecordBytes {
		s.v.Evidence = "partial"
		return
	}
	var data map[string]any
	d := json.NewDecoder(bytes.NewReader(e.Raw))
	d.UseNumber()
	if d.Decode(&data) != nil {
		s.v.Evidence = "partial"
		return
	}
	s.v.Evidence = "journal_last_known"
	numeric := func(obj map[string]any, name string, min, max float64, out **float64) {
		value, present := obj[name]
		if !present {
			return
		}
		n, ok := value.(json.Number)
		v, err := n.Float64()
		if !ok || err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
			*out = nil
			s.v.Evidence = "partial"
			return
		}
		*out = &v
	}
	switch e.Type {
	case "LoadGame":
		s.v.ShipType = shipType(data["Ship"])
		s.id = shipID(data["ShipID"])
		numeric(data, "FuelLevel", 0, 1e6, &s.v.FuelMainT)
		numeric(data, "FuelCapacity", 0, 1e6, &s.v.FuelCapacityT)
	case "Loadout":
		kind, id := shipType(data["Ship"]), shipID(data["ShipID"])
		fuel := s.v.FuelMainT
		if kind == "" || id == "" || kind != s.v.ShipType || id != s.id {
			fuel = nil
		}
		s.v = View{ShipType: kind, Evidence: "journal_last_known", FuelMainT: fuel}
		s.id = id
		numeric(data, "HullHealth", 0, 1, &s.v.HullHealth)
		numeric(data, "CargoCapacity", 0, 1e6, &s.v.CargoCapacityT)
		numeric(data, "UnladenMass", 0, 1e9, &s.v.UnladenMassT)
		numeric(data, "MaxJumpRange", 0, 1e6, &s.v.MaxJumpRangeLY)
		if raw, exists := data["FuelCapacity"]; exists {
			if cap, ok := raw.(map[string]any); ok {
				numeric(cap, "Main", 0, 1e6, &s.v.FuelCapacityT)
			} else {
				s.v.Evidence = "partial"
			}
		}
	case "FSDJump":
		numeric(data, "FuelLevel", 0, 1e6, &s.v.FuelMainT)
	case "FuelScoop":
		numeric(data, "Total", 0, 1e6, &s.v.FuelMainT)
	case "HullDamage":
		fighter, _ := data["Fighter"].(bool)
		srv, _ := data["SRV"].(bool)
		if !fighter && !srv {
			numeric(data, "Health", 0, 1, &s.v.HullHealth)
		}
	case "RefuelAll", "RefuelPartial":
		s.v.FuelMainT = nil
	case "ShipyardSwap", "ShipyardBuy":
		s.v = View{ShipType: shipType(data["ShipType"]), Evidence: "unknown"}
		s.id = ""
	case "Docked":
		v := true
		s.v.Docked = &v
	case "Undocked":
		v := false
		s.v.Docked = &v
	case "Touchdown":
		v := true
		s.v.Landed = &v
	case "Liftoff":
		v := false
		s.v.Landed = &v
	case "Location":
		if v, ok := data["Docked"].(bool); ok {
			s.v.Docked = &v
		}
	case "Shutdown":
		s.v.Evidence = "waiting_game"
		s.verified = false
	}
}
