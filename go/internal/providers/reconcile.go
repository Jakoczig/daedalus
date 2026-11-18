package providers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/history"
	"time"
)

// The numerical CAPI commander.id is not assumed to be a journal FID. The
// authenticated response must match the exact name of the current verified FID.
func normalizedCommander(id, kind, fid, name string, data map[string]any, now time.Time) (history.ExternalFacts, error) {
	// Current profile contracts do not provide a trustworthy per-field observed-at
	// timestamp. Keep Observed unknown; neither fetch time nor arbitrary response
	// timestamps prove that these values postdate witnessed journal facts.
	x := history.ExternalFacts{Provider: id, Entity: "commander", EntityID: fid, FID: fid, Fetched: now, Expires: now.Add(5 * time.Minute), Fields: map[string]any{}}
	if fid == "" || name == "" {
		return x, errors.New("Verified local commander required")
	}
	switch id {
	case "frontier":
		if kind != "profile" {
			return x, errors.New("Station/carrier identity needs separate verified association")
		}
		commander, _ := data["commander"].(map[string]any)
		if text(commander["name"]) != name {
			return x, errors.New("Frontier commander identity mismatch")
		}
		x.Fields["name"] = name
		for _, k := range []string{"credits", "debt"} {
			if n, ok := commander[k].(json.Number); ok {
				x.Fields[k] = string(n)
			}
		}
		if ranks, ok := commander["rank"].(map[string]any); ok {
			safe := map[string]string{}
			for _, k := range []string{"combat", "trade", "explore", "cqc", "empire", "federation", "power"} {
				if n, ok := ranks[k].(json.Number); ok {
					safe[k] = string(n)
				}
			}
			if len(safe) > 0 {
				x.Fields["ranks"] = safe
			}
		}
		ships := []CAPIShip{}
		switch values := data["ships"].(type) {
		case []any:
			if len(values) > 512 {
				return x, errors.New("CAPI ship limit")
			}
			for _, raw := range values {
				if ship, ok := normalizeCAPIShip(raw); ok {
					ships = append(ships, ship)
				}
			}
		case map[string]any:
			if len(values) > 512 {
				return x, errors.New("CAPI ship limit")
			}
			for _, raw := range values {
				if ship, ok := normalizeCAPIShip(raw); ok {
					ships = append(ships, ship)
				}
			}
		}
		if len(ships) > 0 {
			x.Fields["ships"] = ships
		}
		if ship, ok := normalizeCAPIShip(data["ship"]); ok {
			x.Fields["ship"] = ship
		}
		// No stored-ship loadout or current-station inference from incomplete CAPI dictionaries.
	case "inara":
		events, _ := data["events"].([]any)
		if len(events) != 1 {
			return x, errors.New("Expected one profile response")
		}
		event, _ := events[0].(map[string]any)
		d, _ := event["eventData"].(map[string]any)
		if text(d["commanderName"]) != name {
			return x, errors.New("Inara returned a different or fuzzy-matched commander")
		}
		x.Fields["name"] = name
		if ranks, ok := d["commanderRanksPilot"].([]any); ok && len(ranks) <= 32 {
			safe := []map[string]string{}
			for _, r := range ranks {
				v, _ := r.(map[string]any)
				if n, ok := v["rankValue"].(json.Number); ok && text(v["rankName"]) != "" {
					safe = append(safe, map[string]string{"name": text(v["rankName"]), "value": string(n)})
				}
			}
			x.Fields["ranks"] = safe
		}
	default:
		return x, errors.New("Unsupported commander source")
	}
	x.IdentityVerified = true
	return x, nil
}

type CAPIShip struct {
	ID              string `json:"id"`
	Model           string `json:"model"`
	Name            string `json:"name"`
	Ident           string `json:"ident"`
	SystemAddress   string `json:"systemAddress"`
	SystemName      string `json:"systemName"`
	Station         string `json:"station"`
	Ownership       string `json:"ownership"`
	LoadoutComplete bool   `json:"loadoutComplete"`
}

func normalizeCAPIShip(raw any) (CAPIShip, bool) {
	v, ok := raw.(map[string]any)
	if !ok {
		return CAPIShip{}, false
	}
	id, ok := v["id"].(json.Number)
	if !ok || !canonicalID(string(id)) {
		return CAPIShip{}, false
	}
	ship := CAPIShip{ID: string(id), Model: text(v["name"]), Name: text(v["shipName"]), Ident: text(v["shipID"]), Ownership: "owned according to authenticated CAPI profile; may lag", LoadoutComplete: false}
	system, _ := v["starsystem"].(map[string]any)
	address, ok := system["systemaddress"].(json.Number)
	if ok && canonicalID(string(address)) {
		ship.SystemAddress = string(address)
		ship.SystemName = text(system["name"])
	}
	station, _ := v["station"].(map[string]any)
	ship.Station = text(station["name"])
	return ship, true
}

func (m *Manager) persistCommanderRead(ctx context.Context, id, kind, fid, name string, data map[string]any, now time.Time) error {
	if id != "inara" && !(id == "frontier" && kind == "profile") {
		return nil
	}
	x, err := normalizedCommander(id, kind, fid, name, data, now)
	if err != nil {
		return err
	}
	if m.history != nil {
		return m.history.SaveExternal(ctx, x)
	}
	return nil
}
