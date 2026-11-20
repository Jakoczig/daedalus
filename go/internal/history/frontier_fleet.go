package history

import (
	"context"
	"encoding/json"
	"strings"
)

// CAPI contributes explicit last-known gaps, never a replacement for witnessed
// ship identity, a completed transfer, a sale, or journal-derived engineering.
func frontierFleetFacts(facts []EffectiveFact) ([]map[string]any, *FactEvidence) {
	for _, f := range facts {
		if f.Field != "ships" {
			continue
		}
		sources := append([]FactEvidence{f.FactEvidence}, f.Alternatives...)
		for i := range sources {
			if sources[i].Source != "frontier" {
				continue
			}
			var ships []map[string]any
			if json.Unmarshal(sources[i].Value, &ships) == nil && len(ships) <= 512 {
				return ships, &sources[i]
			}
		}
	}
	return nil, nil
}
func (s *Store) supplementFrontierFleet(ctx context.Context, fid string, q OperationQuery, out *OperationPage) error {
	ships, evidence := frontierFleetFacts(out.Reference)
	if evidence == nil {
		return nil
	}
	known := map[string]bool{}
	rows, e := s.reader.QueryContext(ctx, "SELECT ship_id FROM fleet_ships WHERE commander_fid=?", fid)
	if e != nil {
		return e
	}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		known[id] = true
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, r := range out.Rows {
		seen[r.Entity] = true
	}
	candidate := 0
	for _, ship := range ships {
		id := text(ship["id"])
		if identifier(id) == "" {
			continue
		}
		source := map[string]any{"source": "frontier/profile.ships", "fetchedAt": evidence.Fetched, "observedAt": evidence.Observed, "stale": evidence.Stale, "loadoutComplete": false, "currentReference": ship["currentReference"]}
		fill := map[string]any{"model": ship["model"], "shipName": ship["name"], "system": ship["systemName"], "station": ship["station"], "Value": ship["value"], "HullValue": ship["hullValue"], "ModulesValue": ship["modulesValue"], "capiModules": ship["modules"]}
		if seen[id] {
			for i := range out.Rows {
				r := &out.Rows[i]
				if r.Entity != id || r.State == "sold" {
					continue
				}
				data := object(r.Details)
				if data == nil {
					continue
				}
				for key, value := range fill {
					if key == "system" || key == "station" {
						continue
					}
					if data[key] == nil || data[key] == "" {
						data[key] = value
					}
				}
				data["capiReference"] = source
				r.Details = json.RawMessage(encoded(data))
			}
			continue
		}
		// A journal row outside this page may contain a newer sale/transfer. Never
		// manufacture a CAPI-only row for a ship already represented in local history.
		if known[id] || out.Next != "" || (q.FleetState != "" && q.FleetState != "owned") {
			continue
		}

		if q.Commodity != "" && !strings.Contains(strings.ToLower(text(ship["name"])+" "+text(ship["model"])), strings.ToLower(q.Commodity)) {
			continue
		}
		candidate++
		if q.FleetAfter > 0 && candidate < q.FleetAfter {
			continue
		}
		if len(out.Rows) >= q.Limit {
			out.FleetNext = candidate
			break
		}
		fill["capiReference"] = source
		out.Rows = append(out.Rows, OperationRow{ID: "capi:" + id, Entity: id, Kind: "Fleet", State: "capi_last_known", Title: text(ship["model"]), Details: json.RawMessage(encoded(fill))})
		seen[id] = true
	}
	return nil
}

// Overview counts use the same union, excluding witnessed sales. Unknown CAPI
// observation times remain reference evidence rather than proof of recent ownership.
func (s *Store) ReconciledFleetCount(ctx context.Context, fid string, facts []EffectiveFact) (int, error) {
	rows, e := s.reader.QueryContext(ctx, "SELECT ship_id,state FROM fleet_ships WHERE commander_fid=?", fid)
	if e != nil {
		return 0, e
	}
	seen := map[string]bool{}
	count := 0
	for rows.Next() {
		var id, state string
		if e = rows.Scan(&id, &state); e != nil {
			rows.Close()
			return 0, e
		}
		seen[id] = true
		if state != "sold" {
			count++
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}

	ships, _ := frontierFleetFacts(facts)
	for _, ship := range ships {
		id := text(ship["id"])
		if identifier(id) != "" && !seen[id] {
			seen[id] = true
			count++
		}
	}
	return count, nil
}
