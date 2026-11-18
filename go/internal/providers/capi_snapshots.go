package providers

import (
	"errors"
	"github.com/Jakoczig/eridion/go/internal/history"
	"sort"
	"strings"
	"time"
)

func capiText(v any, limit int) string {
	s, ok := v.(string)
	if !ok || len(s) > limit || strings.ContainsAny(s, "\r\n\x00") {
		return ""
	}
	return s
}
func normalizeCAPISnapshot(kind, fid string, data map[string]any, now time.Time) (history.ExternalFacts, error) {
	x := history.ExternalFacts{Provider: "frontier", Entity: "commander", EntityID: fid, FID: fid, IdentityVerified: true, Fetched: now, Expires: now.Add(5 * time.Minute), Fields: map[string]any{}}
	if fid == "" || data == nil {
		return x, errors.New("Verified Frontier account and supported response required")
	}
	switch kind {
	case "communitygoals":
		entries, ok := data["activeCommunityGoals"].([]any)
		if !ok || len(entries) > 256 {
			return x, errors.New("Unsupported Frontier Community Goals response; journal participation retained")
		}
		goals := []map[string]any{}
		for _, raw := range entries {
			v, ok := raw.(map[string]any)
			if !ok || capiID(v["id"]) == "" {
				return x, errors.New("Invalid Frontier goal identity")
			}
			goals = append(goals, map[string]any{"id": capiID(v["id"]), "title": capiText(v["title"], 256), "objective": capiText(v["objective"], 512), "system": capiText(v["starsystem_name"], 128), "station": capiText(v["market_name"], 128), "expiry": capiText(v["expiry"], 80), "activityType": capiText(v["activityType"], 80), "globalTotal": capiAmount(v["qty"]), "targetTotal": capiAmount(v["target_qty"])})
		}
		x.Fields["communityGoals"] = map[string]any{"global": goals, "personalParticipationVerified": false, "notice": "Global goal reference. This verified response contains no commander participation/contribution fields. Journal subscription, contribution and received rewards remain authoritative."}
	case "market":
		entries, ok := data["commodities"].([]any)
		if !ok || len(entries) > 2048 || capiID(data["id"]) == "" {
			return x, errors.New("Unsupported Frontier last-station market response")
		}
		quotes := []map[string]string{}
		for _, raw := range entries {
			v, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			quotes = append(quotes, map[string]string{"id": capiID(v["id"]), "name": capiText(v["name"], 128), "label": capiText(v["locName"], 160), "buyPrice": capiAmount(v["buyPrice"]), "sellPrice": capiAmount(v["sellPrice"]), "meanPrice": capiAmount(v["meanPrice"]), "stock": capiAmount(v["stock"]), "demand": capiAmount(v["demand"])})
		}
		x.Fields["market"] = map[string]any{"stationId": capiID(data["id"]), "station": capiText(data["name"], 128), "commodities": quotes, "notice": "Last docked station according to CAPI; observation time unknown. Not a global/live market feed, witnessed docking, or guaranteed opportunity."}
	case "shipyard":
		ships, _ := data["ships"].(map[string]any)
		stock, ok := ships["shipyard_list"].(map[string]any)
		if !ok || len(stock) > 256 || capiID(data["id"]) == "" {
			return x, errors.New("Unsupported Frontier station shipyard response")
		}
		keys := make([]string, 0, len(stock))
		for k := range stock {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		available := []map[string]string{}
		for _, key := range keys {
			v, _ := stock[key].(map[string]any)
			available = append(available, map[string]string{"id": capiID(v["id"]), "model": capiText(v["name"], 128), "baseValue": capiAmount(v["basevalue"]), "stock": capiAmount(v["stock"])})
		}
		modules, _ := data["modules"].(map[string]any)
		x.Fields["shipyard"] = map[string]any{"stationId": capiID(data["id"]), "station": capiText(data["name"], 128), "availableShips": available, "moduleStockCount": len(modules), "notice": "Station sale stock, never the commander's owned fleet or fitted/stored modules. Observation time unknown."}
	case "fleetcarrier":
		if owned, ok := data["owned"].(bool); ok && !owned {
			x.Fields["carrier"] = map[string]any{"owned": false, "notice": "Frontier returned HTTP 204: no owned carrier as fetched. Does not override later witnessed carrier evidence."}
		} else {
			return x, errors.New("Carrier-owner response association/normalization requires verified owner fixture")
		}
	default:
		return x, errors.New("Unsupported Frontier account snapshot")
	}
	return x, nil
}
