package providers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/history"
	"sort"
	"strings"
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
			for _, k := range []string{"combat", "trade", "explore", "cqc", "empire", "federation", "power", "soldier", "exobiologist", "builder", "learner"} {
				if n, ok := ranks[k].(json.Number); ok {
					safe[k] = string(n)
				}
			}
			if len(safe) > 0 {
				x.Fields["ranks"] = safe
			}
		}
		ships := []CAPIShip{}
		currentID := capiID(commander["currentShipId"])
		current, hasCurrent := normalizeCAPIShip(data["ship"])
		currentVerified := hasCurrent && currentID != "" && current.ID == currentID
		seen := map[string]bool{}
		appendShip := func(raw any, key string) {
			v, ok := raw.(map[string]any)
			if !ok {
				return
			}
			if key != "" {
				actual := capiID(v["id"])
				if actual != "" && actual != key {
					return
				}
				if actual == "" {
					copy := map[string]any{}
					for k, x := range v {
						copy[k] = x
					}
					copy["id"] = json.Number(key)
					v = copy
				}
			}
			if ship, ok := normalizeCAPIShip(v); ok && !seen[ship.ID] {
				ship.CurrentReference = currentVerified && ship.ID == currentID
				if ship.CurrentReference {
					ship.Modules = current.Modules
				}
				ships = append(ships, ship)
				seen[ship.ID] = true
			}
		}
		switch values := data["ships"].(type) {
		case []any:
			if len(values) > 512 {
				return x, errors.New("CAPI ship limit")
			}
			for _, raw := range values {
				appendShip(raw, "")
			}
			x.Fields["ships"] = ships
		case map[string]any:
			if len(values) > 512 {
				return x, errors.New("CAPI ship limit")
			}
			keys := make([]string, 0, len(values))
			for key := range values {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if canonicalID(key) {
					appendShip(values[key], key)
				}
			}
			x.Fields["ships"] = ships
		}
		if hasCurrent {
			current.CurrentReference = currentVerified
			x.Fields["ship"] = current
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
	ID               string       `json:"id"`
	Model            string       `json:"model"`
	Name             string       `json:"name"`
	Ident            string       `json:"ident"`
	SystemAddress    string       `json:"systemAddress"`
	SystemName       string       `json:"systemName"`
	Station          string       `json:"station"`
	Ownership        string       `json:"ownership"`
	LoadoutComplete  bool         `json:"loadoutComplete"`
	CurrentReference bool         `json:"currentReference"`
	Value            string       `json:"value"`
	HullValue        string       `json:"hullValue"`
	ModulesValue     string       `json:"modulesValue"`
	CargoValue       string       `json:"cargoValue"`
	Modules          []CAPIModule `json:"modules,omitempty"`
}

func normalizeCAPIShip(raw any) (CAPIShip, bool) {
	v, ok := raw.(map[string]any)
	if !ok {
		return CAPIShip{}, false
	}
	id := capiID(v["id"])
	if id == "" {
		return CAPIShip{}, false
	}
	ship := CAPIShip{ID: id, Model: capiText(v["name"], 128), Name: capiText(v["shipName"], 80), Ident: capiText(v["shipID"], 32), Ownership: "owned according to authenticated CAPI profile; may lag", LoadoutComplete: false}
	system, _ := v["starsystem"].(map[string]any)
	address, ok := system["systemaddress"].(json.Number)
	if ok && canonicalID(string(address)) {
		ship.SystemAddress = string(address)
		ship.SystemName = capiText(system["name"], 128)
	}
	station, _ := v["station"].(map[string]any)
	ship.Station = capiText(station["name"], 128)
	value, _ := v["value"].(map[string]any)
	ship.Value = capiAmount(value["total"])
	ship.HullValue = capiAmount(value["hull"])
	ship.ModulesValue = capiAmount(value["modules"])
	ship.CargoValue = capiAmount(value["cargo"])
	ship.Modules = normalizeCAPIModules(v["modules"])
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

func capiID(v any) string {
	switch x := v.(type) {
	case json.Number:
		if canonicalID(string(x)) {
			return string(x)
		}
	case string:
		if canonicalID(x) {
			return x
		}
	}
	return ""
}
func capiAmount(v any) string {
	n, ok := v.(json.Number)
	if ok && canonicalID(string(n)) {
		return string(n)
	}
	return ""
}

type CAPIModifier struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Display string `json:"display"`
}
type CAPIModule struct {
	Slot      string         `json:"slot"`
	Item      string         `json:"item"`
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Value     string         `json:"value"`
	On        *bool          `json:"on,omitempty"`
	Priority  string         `json:"priority"`
	Blueprint string         `json:"blueprint"`
	Grade     string         `json:"grade"`
	Modifiers []CAPIModifier `json:"modifiers,omitempty"`
	Notice    string         `json:"notice"`
}

func normalizeCAPIModules(raw any) []CAPIModule {
	slots, ok := raw.(map[string]any)
	if !ok || len(slots) > 128 {
		return nil
	}
	keys := make([]string, 0, len(slots))
	for k := range slots {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []CAPIModule{}
	for _, slot := range keys {
		if len(slot) > 80 {
			continue
		}
		v, _ := slots[slot].(map[string]any)
		module, _ := v["module"].(map[string]any)
		if module == nil {
			continue
		}
		item := text(module["name"])
		if len(item) > 128 || item == "" {
			continue
		}
		m := CAPIModule{Slot: slot, Item: item, ID: capiID(module["id"]), Name: text(module["locName"]), Value: capiAmount(module["value"]), Priority: capiAmount(module["priority"]), Notice: "Supplemental CAPI fitted slot. Fetch time is not observation time; journal Loadout remains authoritative. CAPI modifier units/absolute semantics are not validated and are not used for ship calculations."}
		if on, ok := module["on"].(bool); ok {
			m.On = &on
		}
		engineer, _ := v["engineer"].(map[string]any)
		m.Blueprint = text(engineer["recipeName"])
		m.Grade = capiAmount(engineer["recipeLevel"])
		mods, _ := v["WorkInProgress_modifications"].(map[string]any)
		if len(mods) <= 64 {
			names := make([]string, 0, len(mods))
			for key := range mods {
				names = append(names, key)
			}
			sort.Strings(names)
			for _, key := range names {
				if !strings.HasPrefix(key, "OutfittingFieldType_") || len(key) > 128 {
					continue
				}
				a, _ := mods[key].(map[string]any)
				n, ok := a["value"].(json.Number)
				if ok && len(string(n)) <= 64 {
					display := text(a["displayValue"])
					if len(display) > 80 {
						display = ""
					}
					m.Modifiers = append(m.Modifiers, CAPIModifier{key, string(n), display})
				}
			}
		}
		if len(m.Name) > 160 {
			m.Name = ""
		}
		if len(m.Blueprint) > 128 {
			m.Blueprint = ""
		}
		out = append(out, m)
	}
	return out
}
