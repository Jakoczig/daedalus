package paritystate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/shipstate"
	"github.com/Jakoczig/eridion/go/internal/sidecar"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type state struct {
	v                                     View
	verified                              bool
	session, barrier, marker, statusStamp time.Time
	file, id                              string
	offset                                int64
	sequence                              uint64
	locker, backpack                      bool
}
type cargoCandidate struct {
	stamp     time.Time
	hash      [32]byte
	view      CargoView
	ambiguous bool
	owner     string
}
type Model struct {
	states     map[string]*state
	order      []string
	ship       *shipstate.Model
	active     string
	candidate  *cargoCandidate
	claimed    time.Time
	cargoValid bool
}

func NewModel() *Model { m := &Model{}; m.ResetSource(); return m }
func blank() View {
	return View{Evidence: "unknown", Ship: shipstate.View{Evidence: "unknown"}, Modules: []Module{}, ModulesState: "unknown", Cargo: CargoView{State: "unknown", Items: []CargoItem{}}, Inventory: InventoryView{State: "unknown", Items: []InventoryItem{}}, Materials: MaterialView{State: "unknown", Items: []MaterialItem{}}, Status: StatusView{State: "unknown"}}
}
func (m *Model) ResetSource() {
	m.states = map[string]*state{}
	m.order = nil
	m.ship = shipstate.NewModel()
	m.active = ""
	m.candidate = nil
	m.claimed = time.Time{}
	m.cargoValid = false
}
func (m *Model) get(key string) *state {
	if s := m.states[key]; s != nil {
		return s
	}
	if len(m.order) >= 8 {
		delete(m.states, m.order[0])
		m.order = m.order[1:]
	}
	s := &state{v: blank()}
	m.states[key] = s
	m.order = append(m.order, key)
	return s
}
func (m *Model) View(key string) View {
	if s := m.states[key]; s != nil {
		return Clone(s.v)
	}
	return blank()
}
func object(raw []byte) map[string]any {
	if len(raw) > 4<<20 {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v map[string]any
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		return nil
	}
	return v
}
func text(v any) string {
	s, ok := v.(string)
	if !ok || len(s) > 256 {
		return ""
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return ""
		}
	}
	return s
}
func number(v any, min, max float64) *float64 {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	f, err := n.Float64()
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < min || f > max {
		return nil
	}
	return &f
}
func integer(v any, max int) *int {
	n, ok := v.(json.Number)
	if !ok || strings.ContainsAny(string(n), ".+-eE") {
		return nil
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	if err != nil || i < 0 || i > int64(max) {
		return nil
	}
	o := int(i)
	return &o
}
func ident(v any) string {
	n, ok := v.(json.Number)
	if !ok || strings.ContainsAny(string(n), ".+-eE") {
		return ""
	}
	i, err := strconv.ParseUint(string(n), 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(i, 10)
}
func boolean(v any) *bool {
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return &b
}
func (m *Model) ApplyEvent(key string, e journal.Event) {
	m.active = key
	s := m.get(key)
	gap := s.file != "" && (e.Position.FileID != s.file || e.Position.Offset <= s.offset || (s.sequence != 0 && e.Position.Sequence != 0 && e.Position.Sequence != s.sequence+1))
	if gap {
		*s = state{v: blank()}
		s.v.Evidence = "unverified"
	}
	defer func() { s.file = e.Position.FileID; s.offset = e.Position.Offset; s.sequence = e.Position.Sequence }()
	if e.Type == "LoadGame" || e.Type == "Fileheader" {
		*s = state{v: blank(), session: e.Timestamp, barrier: e.Timestamp, verified: e.Type == "LoadGame" && key != "" && e.FID != "" && !e.Timestamp.IsZero()}
		m.cargoValid = false
	}
	if !s.verified {
		return
	}
	if e.Type != "LoadGame" && (e.Timestamp.IsZero() || e.Timestamp.Before(s.session) || e.Timestamp.Before(s.barrier)) {
		return
	}
	s.v.Evidence = "journal_last_known"
	if e.Type == "Shutdown" {
		// Account-scoped inventory is last-known history, not live telemetry.
		// Keep only this verified snapshot; a new header/session or source gap
		// still clears it, and shutdown cannot authorize later deltas/sidecars.
		materials := s.v.Materials
		*s = state{v: blank()}
		s.v.Materials = materials
		s.v.Evidence = "waiting_game"
		return
	}
	d := object(e.Raw)
	if d == nil || len(e.Raw) > journal.MaxRecordBytes {
		s.v.Evidence = "partial"
		if e.Type == "Loadout" {
			s.v.Modules = []Module{}
			s.v.ModulesState = "invalid"
		}
		return
	}
	m.ship.ApplyEvent(key, e)
	s.v.Ship = m.ship.View(key)
	if e.Type == "LoadGame" {
		s.id = ident(d["ShipID"])
	}
	if e.Type != "LoadGame" && (e.Timestamp.IsZero() || e.Timestamp.Before(s.session)) {
		return
	}
	switch e.Type {
	case "Loadout":
		id := ident(d["ShipID"])
		if id == "" {
			s.v.Modules = nil
			s.v.ModulesState = "invalid"
			return
		}
		if id != s.id {
			s.v.Cargo = blank().Cargo
			s.v.Status = StatusView{State: "unknown"}
			s.barrier = e.Timestamp
			s.marker = time.Time{}
		}
		s.id = id
		s.v.Modules, s.v.ModulesState = parseModules(d)
	case "ShipyardSwap", "ShipyardBuy":
		s.id = ident(d["ShipID"])
		s.barrier = e.Timestamp
		s.marker = time.Time{}
		s.v.Modules = []Module{}
		s.v.ModulesState = "pending_loadout"
		s.v.Cargo = blank().Cargo
		s.v.Status = StatusView{State: "unknown"}
	case "ModuleBuy", "ModuleSell", "ModuleStore", "ModuleRetrieve", "ModuleSwap", "MassModuleStore", "EngineerCraft", "EngineerLegacyConvert", "ModuleSellRemote":
		s.v.Modules = []Module{}
		s.v.ModulesState = "pending_loadout"
		if e.Type == "EngineerCraft" {
			applyConsumption(&s.v.Materials, d["Ingredients"])
		}
		if e.Type == "EngineerLegacyConvert" {
			s.v.Materials = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
		}
	case "Cargo":
		if text(d["Vessel"]) != "Ship" {
			return
		}
		s.marker = e.Timestamp
		s.v.Cargo = CargoView{State: "pending_snapshot", Count: integer(d["Count"], 1000000), Items: []CargoItem{}}
		if _, ok := d["Inventory"]; ok {
			s.v.Cargo = parseCargo(d)
		} else {
			m.correlate(key, s)
		}
	case "MarketBuy", "MarketSell", "BuyDrones", "SellDrones", "CollectCargo", "EjectCargo", "MiningRefined", "CargoDepot", "CargoTransfer", "SearchAndRescue":
		s.v.Cargo = CargoView{State: "pending_snapshot", Items: []CargoItem{}}
		s.marker = time.Time{}
	case "Materials":
		s.v.Materials = parseMaterials(d)
	case "MaterialCollected", "MaterialDiscarded":
		applyMaterialDelta(&s.v.Materials, d, e.Type == "MaterialDiscarded")
	case "MaterialTrade":
		applyTrade(&s.v.Materials, d)
	case "Synthesis":
		applyConsumption(&s.v.Materials, d["Materials"])
	case "EngineerContribution", "TechnologyBroker":
		s.v.Materials = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
	case "ShipLocker", "Backpack":
		applyInventory(s, d, e.Type)
	case "BackpackChange", "TransferMicroResources", "TradeMicroResources", "BuyMicroResources", "SellMicroResources", "CollectItems", "DropItems", "UseConsumable":
		s.v.Inventory = InventoryView{State: "pending_snapshot", Items: []InventoryItem{}}
		s.locker = false
		s.backpack = false
	}
}
func (m *Model) correlate(key string, s *state) {
	c := m.candidate
	if !m.cargoValid || c == nil || c.ambiguous || !s.verified || !c.stamp.Equal(s.marker) || !c.stamp.After(s.session) || c.stamp.Before(s.barrier) || (c.owner != "" && c.owner != key) {
		return
	}
	c.owner = key
	m.claimed = c.stamp
	s.v.Cargo = c.view
}
func (m *Model) ApplySidecar(key string, c sidecar.Change) {
	if c.Kind != "Status" && c.Kind != "Cargo" {
		return
	}
	s := m.states[key]
	d := object(c.Raw)
	valid := c.Valid && d != nil && !c.Timestamp.IsZero()
	if valid {
		stamp, err := time.Parse(time.RFC3339Nano, text(d["timestamp"]))
		valid = err == nil && stamp.Equal(c.Timestamp) && text(d["event"]) == c.Kind
	}
	if c.Kind == "Status" {
		if s == nil {
			return
		}
		s.v.Status = StatusView{State: "unverified"}
		if !valid || !s.verified || key != m.active || !c.Timestamp.After(s.session) || !c.Timestamp.After(s.barrier) || c.Timestamp.Before(s.statusStamp) {
			return
		}
		s.statusStamp = c.Timestamp
		s.v.Status = parseStatus(d)
		if s.v.Status.State == "known" {
			s.v.Status.ObservedAt = c.Timestamp.UTC().Format(time.RFC3339Nano)
		}
		return
	}
	continuous := m.cargoValid
	m.cargoValid = valid
	if !valid {
		if s != nil {
			s.v.Cargo = CargoView{State: "unverified", Items: []CargoItem{}}
		}
		return
	}
	v := parseCargo(d)
	if v.State != "known" {
		m.cargoValid = false
		if s != nil {
			s.v.Cargo = v
		}
		return
	}
	candidate := &cargoCandidate{stamp: c.Timestamp, hash: sha256.Sum256(c.Raw), view: v}
	if old := m.candidate; old != nil {
		if continuous && old.hash == candidate.hash {
			candidate = old
		} else if old.stamp.Equal(candidate.stamp) {
			candidate.ambiguous = true
		}
	}
	if candidate != m.candidate && !candidate.stamp.After(m.claimed) {
		candidate.ambiguous = true
	}
	m.candidate = candidate
	if s != nil {
		if candidate.ambiguous {
			s.v.Cargo = CargoView{State: "unverified", Items: []CargoItem{}}
		} else if key == m.active {
			// Observing replacement bytes revokes the old file's attestation,
			// even when the matching new journal marker has not arrived yet.
			if !candidate.stamp.Equal(s.marker) {
				s.v.Cargo = CargoView{State: "pending_snapshot", Items: []CargoItem{}}
			}
			m.correlate(key, s)
		}
	}
}
