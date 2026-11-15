package providers

import (
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/history"
	"net/url"
	"strconv"
	"strings"
)

func canonicalID(s string) bool {
	n, e := strconv.ParseUint(s, 10, 64)
	return e == nil && strconv.FormatUint(n, 10) == s
}
func referenceEndpoint(id, kind, query string) (string, error) {
	if id == "spansh" && canonicalID(query) && (kind == "dump" || kind == "system" || kind == "body" || kind == "station") {
		return "https://spansh.co.uk/api/" + kind + "/" + query, nil
	}
	return "", errors.New("Choose a documented lookup with a canonical decimal identifier")
}
func spanshPath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[1] != "api" {
		return false
	}
	_, err := referenceEndpoint("spansh", parts[2], parts[3])
	return err == nil
}
func normalizeSpansh(kind, query string, raw []byte) (*Snapshot, error) {
	data := object(raw)
	key := "record"
	if kind == "dump" {
		key = "system"
	}
	record, _ := data[key].(map[string]any)
	idKey := "id64"
	if kind == "station" {
		idKey = "market_id"
	}
	n, ok := record[idKey].(json.Number)
	if !ok || string(n) != query || !canonicalID(query) {
		return nil, errors.New("Lookup response entity does not match requested exact ID")
	}
	prefix := ""
	if kind == "body" || kind == "station" {
		prefix = "system_"
		id, ok := record["system_id64"].(json.Number)
		if !ok || !canonicalID(string(id)) {
			return nil, errors.New("Missing exact parent system ID")
		}
	}
	coords := record
	if kind == "dump" {
		coords, _ = record["coords"].(map[string]any)
	}
	pos := []any{coords[prefix+"x"], coords[prefix+"y"], coords[prefix+"z"]}
	if !validPosition(pos) {
		return nil, errors.New("Missing or invalid system coordinates")
	}
	snapshot := &Snapshot{Source: "spansh / " + kind, Facts: []Fact{{Name: idKey, Value: query}}}
	for _, k := range []string{"name", "system_name", "system_id64", "body_id", "updated_at", "date", "type", "subtype", "distance_to_arrival", "has_large_pad", "is_planetary", "has_shipyard", "has_outfitting"} {
		switch v := record[k].(type) {
		case string:
			if len(v) <= 160 {
				snapshot.Facts = append(snapshot.Facts, Fact{Name: k, Value: v})
			}
		case json.Number:
			snapshot.Facts = append(snapshot.Facts, Fact{Name: k, Value: string(v)})
		case bool:
			snapshot.Facts = append(snapshot.Facts, Fact{Name: k, Value: strconv.FormatBool(v)})
		}
	}
	for i, k := range []string{"x", "y", "z"} {
		snapshot.Facts = append(snapshot.Facts, Fact{Name: "system " + k, Value: string(pos[i].(json.Number))})
	}
	return snapshot, nil
}

// Public routes are fixed in code. Never accept a browser-supplied base URL.
func validReferenceURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Host == "spansh.co.uk" && u.Scheme == "https" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && spanshPath(u.Path)
}

func spanshStation(raw []byte, snapshot *Snapshot) (history.StationReference, error) {
	r, _ := object(raw)["record"].(map[string]any)
	v := history.StationReference{Provider: "spansh", Name: text(r["name"]), SystemName: text(r["system_name"]), Type: text(r["type"]), Fetched: snapshot.FetchedAt, Expires: snapshot.ExpiresAt, Observed: text(r["updated_at"]), Services: map[string]bool{}}
	for key, target := range map[string]*string{"market_id": &v.MarketID, "system_id64": &v.SystemAddress} {
		n, ok := r[key].(json.Number)
		if !ok || !canonicalID(string(n)) {
			return v, errors.New("Invalid station identity")
		}
		*target = string(n)
	}
	for i, k := range []string{"system_x", "system_y", "system_z"} {
		n, ok := r[k].(json.Number)
		if !ok {
			return v, errors.New("Unknown station coordinates")
		}
		f, e := n.Float64()
		if e != nil {
			return v, e
		}
		v.Coordinates[i] = f
	}
	for k, label := range map[string]string{"has_large_pad": "large_pad", "has_outfitting": "outfitting", "has_shipyard": "shipyard"} {
		if b, ok := r[k].(bool); ok {
			v.Services[label] = b
		}
	}
	services, _ := r["services"].([]any)
	if len(services) <= 128 {
		for _, item := range services {
			service, _ := item.(map[string]any)
			for name, key := range map[string]string{"Repair": "repair", "Material Trader": "material_trader", "Technology Broker": "technology_broker"} {
				if text(service["name"]) == name {
					v.Services[key] = true
				}
			}
		}
	}
	if n, ok := r["distance_to_arrival"].(json.Number); ok {
		if f, e := n.Float64(); e == nil && f >= 0 {
			v.ArrivalLS = &f
		}
	}
	return v, nil
}
