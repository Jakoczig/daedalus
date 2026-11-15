package providers

import (
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
	"regexp"
	"strings"
	"time"
)

type stationSession struct {
	Market, Name, System string
	Markers              map[string]journal.Event
	Files                map[string]sidecar.Change
}

func (m *Manager) stationEventLocked(e journal.Event, phase journal.Phase) {
	if e.Type == "Fileheader" || e.Type == "LoadGame" || e.Type == "Shutdown" || e.Type == "Undocked" || e.Type == "StartJump" {
		m.station = stationSession{Markers: map[string]journal.Event{}, Files: map[string]sidecar.Change{}}
		return
	}
	v := object(e.Raw)
	if e.Type == "Docked" && phase == journal.Live && m.identity != "" && validID(v["MarketID"]) && text(v["StationName"]) != "" && text(v["StarSystem"]) == m.system {
		n, _ := v["MarketID"].(json.Number)
		m.station = stationSession{Market: string(n), Name: text(v["StationName"]), System: m.system, Markers: map[string]journal.Event{}, Files: map[string]sidecar.Change{}}
	}
	if e.Type != "Market" && e.Type != "Shipyard" && e.Type != "Outfitting" {
		return
	}
	age := m.opts.Now().Sub(e.Timestamp)
	if phase != journal.Live || e.Timestamp.IsZero() || age < -5*time.Second || age > 2*time.Minute || m.station.Market == "" || m.station.Markers == nil {
		return
	}
	m.station.Markers[e.Type] = e
	m.publishStationLocked(e.Type)
}
func (m *Manager) ObserveSidecar(c sidecar.Change) {
	if (c.Kind != "Market" && c.Kind != "Shipyard" && c.Kind != "Outfitting") || len(c.Raw) > 1<<20 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.history == nil || m.station.Files == nil {
		return
	}
	if !c.Valid {
		delete(m.station.Files, c.Kind)
		return
	}
	m.station.Files[c.Kind] = c
	m.publishStationLocked(c.Kind)
}
func (m *Manager) publishStationLocked(kind string) {
	s := m.states["eddn"]
	c, ok := m.station.Files[kind]
	e, marked := m.station.Markers[kind]
	if !ok || !marked || !c.Valid || c.Timestamp.IsZero() || !c.Timestamp.Equal(e.Timestamp) || m.history == nil || s == nil || !s.config.Upload || s.config.Paused || s.config.CommanderHash != m.identity || m.identity == "" || !strings.HasPrefix(m.gameVersion, "4.") || strings.ContainsAny(strings.ToLower(m.gameVersion), "abcdefghijklmnopqrstuvwxyz") {
		return
	}
	cat := map[string]string{"Market": "commodities", "Shipyard": "shipyard", "Outfitting": "outfitting"}[kind]
	permitted := false
	for _, v := range s.config.Categories {
		if v == cat {
			permitted = true
		}
	}
	if !permitted {
		return
	}
	age := m.opts.Now().Sub(c.Timestamp)
	if age < -5*time.Second || age > 2*time.Minute {
		return
	}
	v := object(c.Raw)
	n, valid := v["MarketID"].(json.Number)
	if !valid || string(n) != m.station.Market || text(v["StationName"]) != m.station.Name || text(v["StarSystem"]) != m.station.System {
		return
	}
	candidate, err := m.history.Candidate(m.fid, "eddn", cat, e, journal.Live, false, m.opts.Now())
	if err != nil || candidate.Status != "eligible" {
		return
	}
	payload, err := m.stationPayload(kind, v)
	if err != nil {
		s.lastError = "Station message lacked verified public fields; not queued"
		return
	}
	key := hash("eddn/station/" + m.identity + "/" + string(c.Raw))
	if _, seen := m.ledger[key]; seen {
		return
	}
	for _, q := range s.queue {
		if q.key == key || q.candidateID == candidate.ID {
			return
		}
	}
	total := len(payload)
	for _, q := range s.queue {
		total += len(q.body)
	}
	if len(s.queue) >= 64 || total > 2<<20 {
		return
	}
	if err = m.history.SaveSidecar(m.fid, e, kind, c.Raw); err != nil {
		s.lastError = "Station source persistence failed"
		return
	}
	if err = m.history.CandidateOutcome(candidate.ID, "queued", "Fresh dock-matched sidecar authorized under exact recipient/category policy"); err != nil {
		return
	}
	s.queue = append(s.queue, queued{candidateID: candidate.ID, key: key, category: cat, body: payload, endpoint: m.cap("eddn").Endpoint, contentType: "application/json", generation: s.generation})
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

var publicOutfitting = regexp.MustCompile(`(?i)^int_(engine|powerplant|hyperdrive|powerdistributor|lifesupport|sensors|fueltank|cargorack)_size[1-8]_class[1-5]$`)

func (m *Manager) stationPayload(kind string, v map[string]any) ([]byte, error) {
	fail := func() ([]byte, error) { return nil, errors.New("Unsupported station snapshot") }
	if !validID(v["MarketID"]) || text(v["StationName"]) == "" || text(v["StarSystem"]) == "" {
		return fail()
	}
	msg := map[string]any{"systemName": v["StarSystem"], "stationName": v["StationName"], "marketId": v["MarketID"], "timestamp": v["timestamp"]}
	schema := ""
	items, _ := v["Items"].([]any)
	if len(items) > 2048 {
		return fail()
	}
	switch kind {
	case "Market":
		schema = "commodity/3"
		out := []any{}
		for _, item := range items {
			o, _ := item.(map[string]any)
			if illegal, _ := o["Illegal"].(bool); illegal {
				continue
			}
			if text(o["Name"]) == "" {
				return fail()
			}
			row := map[string]any{"name": o["Name"]}
			for key, target := range map[string]string{"MeanPrice": "meanPrice", "BuyPrice": "buyPrice", "Stock": "stock", "StockBracket": "stockBracket", "SellPrice": "sellPrice", "Demand": "demand", "DemandBracket": "demandBracket"} {
				n, ok := o[key].(json.Number)
				if !ok || !validID(n) {
					return fail()
				}
				row[target] = n
			}
			out = append(out, row)
		}
		if len(out) == 0 {
			return fail()
		}
		msg["commodities"] = out
	case "Shipyard":
		schema = "shipyard/2"
		items, _ = v["PriceList"].([]any)
		if len(items) > 256 {
			return fail()
		}
		ships := []string{}
		seen := map[string]bool{}
		safe := map[string]bool{"sidewinder": true, "adder": true, "anaconda": true, "asp": true, "asp_scout": true, "cobramkiii": true, "diamondback": true, "diamondbackxl": true, "eagle": true, "ferdelance": true, "hauler": true, "independant_trader": true, "orca": true, "python": true, "type6": true, "type7": true, "type9": true, "viper": true, "viper_mkiv": true, "vulture": true}
		for _, item := range items {
			o, _ := item.(map[string]any)
			name := text(o["ShipType"])
			if safe[strings.ToLower(name)] && !seen[name] {
				ships = append(ships, name)
				seen[name] = true
			}
		}
		if len(ships) == 0 {
			return fail()
		}
		msg["ships"] = ships
	case "Outfitting":
		schema = "outfitting/2"
		out := []string{}
		seen := map[string]bool{}
		for _, item := range items {
			o, _ := item.(map[string]any)
			name := text(o["Name"])
			if publicOutfitting.MatchString(name) && !seen[name] {
				out = append(out, name)
				seen[name] = true
			}
		}
		if len(out) == 0 {
			return fail()
		}
		msg["modules"] = out
	default:
		return fail()
	}
	payload := map[string]any{"$schemaRef": "https://eddn.edcd.io/schemas/" + schema, "header": map[string]any{"uploaderID": m.uploader, "softwareName": software, "softwareVersion": version, "gameversion": m.gameVersion, "gamebuild": m.gameBuild}, "message": msg}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(raw) > 256<<10 {
		return fail()
	}
	if err = validateEDDNStation(schema, object(raw)); err != nil {
		return nil, err
	}
	return raw, nil
}
