package history

import (
	"database/sql"
	"strings"
)

func normalizeCarrier(tx *sql.Tx, event int64, fid, kind, at string, v map[string]any) error {
	id := identifier(v["CarrierID"])
	if kind == "CarrierJump" && id == "" {
		id = identifier(v["MarketID"])
	}
	if id == "" {
		return nil
	}
	data := subset(v, "CarrierID", "MarketID", "Name", "Callsign", "DockingAccess", "AllowNotorious", "FuelLevel", "JumpRangeCurr", "JumpRangeMax", "PendingDecommission", "SpaceUsage", "Finance", "Crew", "ShipPacks", "ModulePacks", "CarrierBalance", "ReserveBalance", "AvailableBalance", "ReservePercent", "TaxRate", "Deposit", "Withdraw", "PlayerBalance", "Amount", "Total", "Commodity", "PurchaseOrder", "SaleOrder", "CancelTrade", "Price", "BlackMarket", "CrewRole", "Operation", "PackTheme", "PackTier", "Cost", "Refund", "SystemName", "SystemID", "SystemAddress", "StarSystem", "Body", "BodyID", "DepartureTime", "ScrapTime", "ScrapRefund")
	if _, e := tx.Exec("INSERT OR IGNORE INTO carrier_observations VALUES(?,?,?,?,?,?)", event, fid, id, kind, at, encoded(data)); e != nil {
		return e
	}
	if kind == "CarrierBuy" || kind == "CarrierStats" {
		if _, e := tx.Exec("INSERT OR IGNORE INTO carrier_owned VALUES(?,?,?,?)", fid, id, event, at); e != nil {
			return e
		}
	}
	var owned int
	if e := tx.QueryRow("SELECT count(*) FROM carrier_owned WHERE commander_fid=? AND carrier_id=?", fid, id).Scan(&owned); e != nil || owned == 0 {
		return e
	}
	put := func(field string, value any) error {
		_, e := tx.Exec(`INSERT INTO carrier_facts VALUES(?,?,?,?,?,?) ON CONFLICT(commander_fid,carrier_id,field) DO UPDATE SET value_json=excluded.value_json,observed_at=excluded.observed_at,event_id=excluded.event_id WHERE excluded.observed_at>carrier_facts.observed_at OR (excluded.observed_at=carrier_facts.observed_at AND excluded.event_id>carrier_facts.event_id)`, fid, id, field, encoded(value), at, event)
		return e
	}
	switch kind {
	case "CarrierStats":
		for _, key := range []string{"Name", "Callsign", "DockingAccess", "AllowNotorious", "FuelLevel", "JumpRangeCurr", "JumpRangeMax", "PendingDecommission", "SpaceUsage", "ShipPacks", "ModulePacks"} {
			if value, ok := data[key]; ok {
				if e := put(key, value); e != nil {
					return e
				}
			}
		}
		if finance, ok := v["Finance"].(map[string]any); ok {
			for k, value := range subset(finance, "CarrierBalance", "ReserveBalance", "AvailableBalance", "ReservePercent", "TaxRate") {
				if e := put("finance."+k, value); e != nil {
					return e
				}
			}
		}
		for _, x := range list(v["Crew"]) {
			r, _ := x.(map[string]any)
			if role := text(r["CrewRole"]); role != "" {
				if e := put("service."+role, subset(r, "Activated", "Enabled")); e != nil {
					return e
				}
			}
		}
		return nil
	case "CarrierFinance", "CarrierBankTransfer":
		for k, value := range subset(v, "CarrierBalance", "ReserveBalance", "AvailableBalance", "ReservePercent", "TaxRate") {
			if e := put("finance."+k, value); e != nil {
				return e
			}
		}
		return put("lastTransfer", data)
	case "CarrierJumpRequest":
		return put("jumpRequest", map[string]any{"state": "scheduled_not_arrived", "request": data})
	case "CarrierJumpCancelled":
		return put("jumpRequest", map[string]any{"state": "cancelled"})
	case "CarrierJump":
		if e := put("location", subset(v, "StarSystem", "SystemAddress", "Body", "BodyID")); e != nil {
			return e
		}
		var requestRaw, requestAt string
		if tx.QueryRow("SELECT value_json,observed_at FROM carrier_facts WHERE commander_fid=? AND carrier_id=? AND field='jumpRequest'", fid, id).Scan(&requestRaw, &requestAt) == nil && at >= requestAt {
			request, _ := object([]byte(requestRaw))["request"].(map[string]any)
			want := identifier(request["SystemAddress"])
			if want == "" {
				want = identifier(request["SystemID"])
			}
			matches := want != "" && want == identifier(v["SystemAddress"])
			if want == "" {
				matches = text(request["SystemName"]) != "" && text(request["SystemName"]) == text(v["StarSystem"])
			}
			if matches {
				if e := put("jumpRequest", map[string]any{"state": "confirmed_matching_owned_arrival", "request": request, "arrivedAt": at}); e != nil {
					return e
				}
			}
		}
		return put("lastMovement", data) // A prior request is not evidence of a subsequent arrival.
	case "CarrierDecommission":
		return put("decommission", map[string]any{"state": "requested_not_completed", "details": data})
	case "CarrierCancelDecommission":
		return put("decommission", map[string]any{"state": "cancelled"})
	case "CarrierTradeOrder":
		return put("order."+symbol(v["Commodity"]), data)
	case "CarrierCrewServices":
		return put("serviceChange."+text(v["CrewRole"]), data)
	case "CarrierDepositFuel":
		return put("lastFuelDeposit", data) // Do not invent a new total.
	default:
		if strings.HasPrefix(kind, "Carrier") {
			return put("last."+kind, data)
		}
	}
	return nil
}
