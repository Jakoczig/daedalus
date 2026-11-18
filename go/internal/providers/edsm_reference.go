package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/history"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (m *Manager) edsmReferenceEndpoint(ctx context.Context, kind, query string) (string, error) {
	if m.history == nil {
		return "", errors.New("Verified database references required")
	}
	if kind == "nearby" {
		parts := strings.Split(query, "|")
		if len(parts) > 2 {
			return "", errors.New("Use system name | radius LY")
		}
		name := strings.TrimSpace(parts[0])
		radius := 50.0
		if len(parts) == 2 {
			var err error
			radius, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil {
				return "", err
			}
		}
		if radius < 1 || radius > 100 || math.IsNaN(radius) || math.IsInf(radius, 0) {
			return "", errors.New("Radius must be 1 through 100 LY")
		}
		origin, err := m.history.SystemReference(ctx, name, m.opts.Now())
		if err != nil {
			return "", err
		}
		_ = origin
		return "https://www.edsm.net/api-v1/sphere-systems?" + url.Values{"systemName": {name}, "radius": {strconv.FormatFloat(radius, 'g', -1, 64)}, "showId": {"1"}, "showCoordinates": {"1"}}.Encode(), nil
	}
	if kind == "stations" {
		origin, err := m.history.SystemReference(ctx, query, m.opts.Now())
		if err != nil {
			return "", err
		}
		return "https://www.edsm.net/api-system-v1/stations?" + url.Values{"systemName": {origin.Name}}.Encode(), nil
	}
	if kind == "market" || kind == "shipyard" || kind == "outfitting" {
		station, err := m.history.StationByMarket(ctx, query)
		if err != nil {
			return "", err
		}
		_ = station
		return "https://www.edsm.net/api-system-v1/stations/" + kind + "?" + url.Values{"marketId": {query}}.Encode(), nil
	}
	return "", errors.New("Unsupported EDSM reference")
}
func numberText(v any) string {
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
func (m *Manager) normalizeEDSMReference(ctx context.Context, kind, query string, raw []byte, now time.Time, commander ...string) (*Snapshot, error) {
	snapshot := &Snapshot{Source: "edsm-reference / " + kind, Facts: []Fact{}}
	fetched, expires := now.UTC().Format(time.RFC3339), now.Add(6*time.Hour).UTC().Format(time.RFC3339)
	add := func(name, value string) {
		if len(snapshot.Facts) < 100 && len(value) <= 160 {
			snapshot.Facts = append(snapshot.Facts, Fact{Name: name, Value: value})
		}
	}
	if kind == "nearby" {
		parts := strings.Split(query, "|")
		origin, err := m.history.SystemReference(ctx, strings.TrimSpace(parts[0]), now)
		if err != nil {
			return nil, err
		}
		radius := 50.0
		if len(parts) == 2 {
			radius, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		var records []map[string]any
		if decoder.Decode(&records) != nil || len(records) > 4096 {
			return nil, errors.New("Nearby response exceeds bounded system list")
		}
		type nearby struct {
			System   history.SystemReference
			Distance float64
		}
		systems := []nearby{}
		for _, v := range records {
			locked, _ := v["coordsLocked"].(bool)
			address := numberText(v["id64"])
			name := text(v["name"])
			coords, _ := v["coords"].(map[string]any)
			a := []any{coords["x"], coords["y"], coords["z"]}
			if !locked || address == "" || address == "0" || name == "" || !validPosition(a) {
				continue
			}
			r := history.SystemReference{Address: address, Name: name, EDSMID: numberText(v["id"])}
			distance := 0.0
			for i, n := range a {
				f, _ := n.(json.Number).Float64()
				r.Coordinates[i] = f
				distance += math.Pow(f-origin.Coordinates[i], 2)
			}
			distance = math.Sqrt(distance)
			if distance <= radius {
				systems = append(systems, nearby{r, distance})
			}
		}
		sort.Slice(systems, func(i, j int) bool { return systems[i].Distance < systems[j].Distance })
		if len(systems) > 32 {
			systems = systems[:32]
		}
		for _, v := range systems {
			encoded, _ := json.Marshal(v.System)
			if err = m.history.SaveReference(ctx, "edsm-reference", "system/"+v.System.Name, encoded, now, now.Add(6*time.Hour)); err != nil {
				return nil, err
			}
			add(v.System.Name, fmt.Sprintf("%s · %.2f LY · locked coordinates", v.System.Address, v.Distance))
		}
		add("Scope", "At most 32 verified candidates. Select a system and fetch its stations; no automatic network fan-out.")
		return snapshot, nil
	}
	data := object(raw)
	if data == nil {
		return nil, errors.New("Invalid EDSM reference envelope")
	}
	if kind == "stations" {
		origin, err := m.history.SystemReference(ctx, query, now)
		if err != nil {
			return nil, err
		}
		if text(data["name"]) != origin.Name || numberText(data["id64"]) != origin.Address {
			return nil, errors.New("Station list system identity mismatch")
		}
		items, ok := data["stations"].([]any)
		if !ok || len(items) > 512 {
			return nil, errors.New("Station response exceeds limit")
		}
		stationBatch := []history.StationReference{}
		for _, item := range items {
			r, _ := item.(map[string]any)
			market := numberText(r["marketId"])
			name := text(r["name"])
			if market == "" || name == "" {
				continue
			}
			v := history.StationReference{Provider: "edsm-reference", MarketID: market, SystemAddress: origin.Address, SystemName: origin.Name, Name: name, Type: text(r["type"]), Coordinates: origin.Coordinates, Services: map[string]bool{}, Fetched: fetched, Expires: expires}
			for k, label := range map[string]string{"haveOutfitting": "outfitting", "haveShipyard": "shipyard"} {
				if value, ok := r[k].(bool); ok {
					v.Services[label] = value
				}
			}
			if n, ok := r["distanceToArrival"].(json.Number); ok {
				if f, e := n.Float64(); e == nil && f >= 0 {
					v.ArrivalLS = &f
				}
			}
			if services, ok := r["otherServices"].([]any); ok && len(services) < 128 {
				for _, service := range services {
					for name, key := range map[string]string{"Repair": "repair", "Material Trader": "material_trader", "Technology Broker": "technology_broker"} {
						if text(service) == name {
							v.Services[key] = true
						}
					}
				}
			}
			dates, _ := r["updateTime"].(map[string]any)
			v.Observed = text(dates["information"])
			stationBatch = append(stationBatch, v)
			add(name, market+" · "+v.Type+" · observed "+v.Observed)
		}
		if err = m.history.SaveStations(stationBatch); err != nil {
			return nil, err
		}
		return snapshot, nil
	}
	station, err := m.history.StationByMarket(ctx, query)
	if err != nil {
		return nil, err
	}
	if numberText(data["marketId"]) != station.MarketID || numberText(data["id64"]) != station.SystemAddress || text(data["name"]) != station.SystemName || text(data["sName"]) != station.Name {
		return nil, errors.New("Station inventory identity mismatch")
	}
	add("Station", station.Name)
	add("System", station.SystemName)
	add("MarketID", station.MarketID)
	key := map[string]string{"market": "commodities", "shipyard": "ships", "outfitting": "outfitting"}[kind]
	items, ok := data[key].([]any)
	if !ok || len(items) > 2048 {
		return nil, errors.New("Station inventory exceeds limit")
	}
	if kind == "market" && len(commander) > 0 {
		if err := m.history.SaveExternalMarket(commander[0], station, data, fetched, expires); err != nil {
			return nil, err
		}
	}
	for i, item := range items {
		if i >= 32 {
			add("Preview", "First 32 entries of bounded inventory; not a full market feed")
			break
		}
		v, _ := item.(map[string]any)
		name := text(v["name"])
		if name == "" {
			continue
		}
		details := []string{}
		for _, k := range []string{"buyPrice", "sellPrice", "stock", "demand"} {
			if n, ok := v[k].(json.Number); ok {
				details = append(details, k+" "+string(n))
			}
		}
		add(name, strings.Join(details, " · "))
	}
	return snapshot, nil
}
