package history

import (
	"database/sql"
	"strings"
)

func normalizeFleet(tx *sql.Tx, event int64, fid, kind, at string, v map[string]any) error {
	ordinal := 0
	put := func(id, model, name, state, system, market string, data map[string]any) error {
		if id == "" {
			return nil
		}
		ordinal++
		raw := encoded(data)
		if _, e := tx.Exec("INSERT OR IGNORE INTO fleet_ship_events VALUES(?,?,?,?,?,?,?)", event, ordinal, fid, id, kind, at, raw); e != nil {
			return e
		}
		_, e := tx.Exec(`INSERT INTO fleet_ships VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(commander_fid,ship_id) DO UPDATE SET model=coalesce(nullif(excluded.model,''),model),name=coalesce(nullif(excluded.name,''),name),state=excluded.state,system_name=excluded.system_name,market_id=excluded.market_id,observed_at=excluded.observed_at,event_id=excluded.event_id,metadata_json=excluded.metadata_json WHERE excluded.observed_at>fleet_ships.observed_at OR (excluded.observed_at=fleet_ships.observed_at AND excluded.event_id>fleet_ships.event_id)`, fid, id, model, name, state, system, market, at, event, raw)
		return e
	}
	data := subset(v, "ShipID", "ShipType", "Ship", "Name", "UserShipName", "UserShipId", "Value", "Hot", "TransferPrice", "TransferTime", "Distance", "MarketID", "ShipMarketID", "StarSystem", "System", "InTransit")
	state := "last_known"
	id := identifier(v["ShipID"])
	model := text(v["ShipType"])
	if model == "" {
		model = text(v["Ship"])
	}
	switch kind {
	case "Loadout", "ShipyardSwap", "ShipyardNew":
		state = "known_active"
		var newer int
		if e := tx.QueryRow("SELECT count(*) FROM fleet_ships WHERE commander_fid=? AND state='known_active' AND observed_at>?", fid, at).Scan(&newer); e != nil {
			return e
		}
		if newer > 0 {
			state = "last_known"
		}
		if kind == "ShipyardNew" {
			id = identifier(v["NewShipID"])
		}
		if id != "" {
			if _, e := tx.Exec("UPDATE fleet_ships SET state='last_known' WHERE commander_fid=? AND state='known_active' AND observed_at<=? AND ship_id<>?", fid, at, id); e != nil {
				return e
			}
		}
	case "ShipyardSell", "SellShipOnRebuy":
		id = identifier(v["SellShipID"])
		if id == "" {
			id = identifier(v["SellShipId"])
		}
		state = "sold"
	case "ShipyardTransfer":
		state = "in_transit"
	case "SetUserShipName":
		_, e := tx.Exec("UPDATE fleet_ships SET name=?,observed_at=?,event_id=? WHERE commander_fid=? AND ship_id=? AND observed_at<=?", text(v["UserShipName"]), at, event, fid, id, at)
		return e
	case "StoredShips":
		for _, key := range []string{"ShipsHere", "ShipsRemote"} {
			for _, item := range list(v[key]) {
				x, _ := item.(map[string]any)
				sys, market := text(x["StarSystem"]), identifier(x["ShipMarketID"])
				state := "last_known"
				if key == "ShipsHere" {
					sys, market = text(v["StarSystem"]), identifier(v["MarketID"])
				}
				if b, _ := x["InTransit"].(bool); b {
					state = "in_transit"
					sys, market = "", ""
				}
				if e := put(identifier(x["ShipID"]), text(x["ShipType"]), text(x["Name"]), state, sys, market, subset(x, "ShipID", "ShipType", "Name", "Value", "Hot", "InTransit", "StarSystem", "ShipMarketID", "TransferPrice", "TransferTime")); e != nil {
					return e
				}
			}
		}
		return nil
	case "StoredModules":
		if _, e := tx.Exec("INSERT OR IGNORE INTO stored_module_snapshots VALUES(?,?,?)", event, fid, at); e != nil {
			return e
		}
		for i, item := range list(v["Items"]) {
			x, _ := item.(map[string]any)
			if _, e := tx.Exec("INSERT OR IGNORE INTO stored_module_observations VALUES(?,?,?,?,?,?,?)", event, i, fid, identifier(x["StorageSlot"]), text(x["Name"]), at, encoded(subset(x, "Name", "Name_Localised", "StorageSlot", "StarSystem", "MarketID", "TransferCost", "TransferTime", "BuyPrice", "Hot", "InTransit", "EngineerModifications", "Level", "Quality"))); e != nil {
				return e
			}
		}
		return nil
	default:
		if !strings.HasPrefix(kind, "Shipyard") {
			return nil
		}
	}
	// Store/sale of the previous ship is explicit and independent of new identity.
	if old := identifier(v["StoreShipID"]); old != "" {
		if e := put(old, text(v["StoreOldShip"]), "", "last_known", text(v["StarSystem"]), identifier(v["MarketID"]), map[string]any{"evidence": "explicitly stored at ship change"}); e != nil {
			return e
		}
	}
	if old := identifier(v["SellShipID"]); old != "" && old != id {
		if e := put(old, text(v["SellOldShip"]), "", "sold", "", "", map[string]any{"evidence": "explicitly sold at ship change"}); e != nil {
			return e
		}
	}
	if kind == "ShipyardBuy" {
		return nil
	} // New ID is not present until ShipyardNew.
	return put(id, model, text(v["ShipName"]), state, text(v["StarSystem"]), identifier(v["MarketID"]), data)
}
