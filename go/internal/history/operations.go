package history

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

//go:embed operations.sql
var operationsSchema string

func operationKind(k string) bool {
	return k == "Cargo" || k == "CollectCargo" || k == "EjectCargo" || k == "CargoTransfer" || k == "CargoDepot" || k == "MissionCompleted" || operationCategory(k) != "" || k == "Loadout" || k == "SetUserShipName" || k == "StoredShips" || k == "StoredModules" || strings.HasPrefix(k, "Shipyard") || strings.HasPrefix(k, "Module") || k == "MassModuleStore" || k == "FetchRemoteModule" || k == "SellShipOnRebuy" || k == "MarketBuy" || k == "MarketSell" || k == "MiningRefined"
}
func operationCategory(k string) string {
	if strings.HasPrefix(k, "Module") || k == "MassModuleStore" || k == "FetchRemoteModule" {
		return "modules"
	}
	if strings.HasPrefix(k, "Carrier") {
		return "carrier"
	}
	if strings.HasPrefix(k, "CommunityGoal") {
		return "goals"
	}
	if strings.HasPrefix(k, "Powerplay") {
		return "powerplay"
	}
	if strings.Contains(k, "Squadron") {
		return "squadron"
	}
	switch k {
	case "CrewHire", "CrewAssign", "CrewFire":
		return "crew"
	case "WingJoin", "WingAdd", "WingLeave", "WingInvite", "JoinACrew", "QuitACrew", "ChangeCrewRole":
		return "wing"
	}
	return ""
}
func encoded(v any) string { b, _ := json.Marshal(exactFactValue(v)); return string(b) }
func subset(v map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if x, ok := v[k]; ok {
			out[k] = x
		}
	}
	return out
}
func list(v any) []any {
	a, _ := v.([]any)
	if len(a) > 2048 {
		return nil
	}
	return a
}
func unsigned(v any) any {
	s := identifier(v)
	if s == "" {
		return nil
	}
	return s
}
func symbol(v any) string {
	s := strings.ToLower(text(v))
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimSuffix(s, "_name;")
	if len(s) > 128 {
		return ""
	}
	return s
}

// Financial arithmetic uses exact decimals. A zero/missing cost basis is unknown.
func grossSpread(total, count, basis any) any {
	a, b, c := identifier(total), identifier(count), identifier(basis)
	if a == "" || b == "" || c == "" || c == "0" {
		return nil
	}
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	z, _ := new(big.Int).SetString(c, 10)
	return x.Sub(x, y.Mul(y, z)).String()
}
func normalizeOperations(tx *sql.Tx, id int64, fid, session, system, kind, at string, v map[string]any) error {
	if !operationKind(kind) || fid == "" || at == "" {
		return nil
	}
	if len(v) == 0 {
		return normalizeMining(tx, id, fid, session, "CargoTransfer", at, nil)
	}
	module := "fleet"
	switch operationCategory(kind) {
	case "modules":
		module = "fleet"
	case "carrier":
		module = "carrier"
	case "goals", "powerplay", "crew", "wing", "squadron":
		module = "commander-operations"
	}
	if kind == "Cargo" || kind == "CollectCargo" || kind == "EjectCargo" || kind == "CargoTransfer" || kind == "CargoDepot" || kind == "MissionCompleted" || kind == "MarketBuy" || kind == "MarketSell" || kind == "MiningRefined" {
		module = "economy"
	}
	if !moduleEnabled(tx, module) {
		return nil
	}
	// A copied/reimported journal has the same header, line hash and byte offset.
	// Retain both immutable sources, but account that physical record only once.
	var header, source, hash string
	var offset int64
	if e := tx.QueryRow(`SELECT e.source_id,e.byte_offset,e.line_hash,coalesce((SELECT h.line_hash FROM journal_events h WHERE h.source_id=e.source_id AND h.type='Fileheader' ORDER BY h.byte_offset LIMIT 1),'') FROM journal_events e WHERE e.event_id=?`, id).Scan(&source, &offset, &hash, &header); e != nil {
		return e
	}
	if header == "" {
		header = source
	}
	sig := digest([]byte(fmt.Sprintf("%s:%d:%s", header, offset, hash)))
	res, e := tx.Exec("INSERT OR IGNORE INTO operation_sources VALUES(?,?,?)", id, fid, sig)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}
	if e = normalizeMining(tx, id, fid, session, kind, at, v); e != nil {
		return e
	}
	if e = normalizeFleet(tx, id, fid, kind, at, v); e != nil {
		return e
	}
	category := operationCategory(kind)
	if category == "carrier" {
		return normalizeCarrier(tx, id, fid, kind, at, v)
	}
	if kind == "MiningRefined" {
		if symbol(v["Type"]) == "" {
			return nil
		}
		_, e = tx.Exec("INSERT OR IGNORE INTO mining_refined VALUES(?,?,?,?,?,?)", id, fid, session, system, symbol(v["Type"]), at)
		return e
	}
	if kind == "MarketBuy" || kind == "MarketSell" {
		price, total := v["BuyPrice"], v["TotalCost"]
		if kind == "MarketSell" {
			price, total = v["SellPrice"], v["TotalSale"]
		}
		var spread any
		if kind == "MarketSell" {
			spread = grossSpread(total, v["Count"], v["AvgPricePaid"])
		}
		_, e = tx.Exec("INSERT OR IGNORE INTO trade_ledger VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", id, fid, session, system, identifier(v["MarketID"]), symbol(v["Type"]), kind, unsigned(v["Count"]), unsigned(price), unsigned(total), unsigned(v["AvgPricePaid"]), spread, encoded(subset(v, "BlackMarket", "StolenGoods", "IllegalGoods")), at, "journal")
		return e
	}
	if category != "" {
		items := []any{v}
		if kind == "CommunityGoal" {
			items = list(v["CurrentGoals"])
		}
		for i, x := range items {
			row, _ := x.(map[string]any)
			entity := identifier(row["CGID"])
			if entity == "" {
				entity = identifier(row["CrewID"])
			}
			// Owner-only typed projection: explicit field allowlist, never raw journals.
			data := subset(row, "Module", "StoredItem", "StorageSlot", "Slot", "ToSlot", "FromSlot", "Ship", "ShipID", "MarketID", "TransferCost", "TransferTime", "BuyItem", "SellItem", "StoreItem", "StoredItem_Localised", "Others", "CombatRank", "Cost", "Telepresence", "System", "PlayerInTopRank", "CGID", "Title", "SystemName", "MarketName", "Expiry", "IsComplete", "CurrentTotal", "PlayerContribution", "NumContributors", "PlayerPercentileBand", "TierReached", "TopRankSize", "Bonus", "Reward", "Power", "FromPower", "ToPower", "Rank", "Merits", "Votes", "Count", "Type", "Amount", "Credits", "TimePledged", "SquadronName", "CurrentRank", "NewRank", "OldRank", "CrewID", "Name", "Role", "Captain", "Wing", "Members", "Faction", "SystemAddress")
			if e = commanderCurrent(tx, fid, category, entity, kind, at, id, data); e != nil {
				return e
			}
			if _, e = tx.Exec("INSERT OR IGNORE INTO commander_operations VALUES(?,?,?,?,?,?,?,?)", id, i, fid, category, entity, kind, at, encoded(data)); e != nil {
				return e
			}
		}
	}
	return nil
}
func backfillOperations(tx *sql.Tx) error {
	// A single migration/explicit-enable cursor, never a render-time journal scan.
	var cursor int64
	for {
		rows, e := tx.Query(`SELECT event_id,commander_fid,coalesce(session_id,''),coalesce(system_address,''),type,coalesce(event_utc,''),raw_relevant_json FROM journal_events WHERE event_id>? AND commander_fid IS NOT NULL ORDER BY event_id LIMIT 256`, cursor)
		if e != nil {
			return e
		}
		type r struct {
			id                                  int64
			fid, session, system, kind, at, raw string
		}
		batch := []r{}
		for rows.Next() {
			var x r
			if e = rows.Scan(&x.id, &x.fid, &x.session, &x.system, &x.kind, &x.at, &x.raw); e != nil {
				rows.Close()
				return e
			}
			batch = append(batch, x)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(batch) == 0 {
			break
		}
		for _, x := range batch {
			cursor = x.id
			if e = normalizeOperations(tx, x.id, x.fid, x.session, x.system, x.kind, x.at, object([]byte(x.raw))); e != nil {
				return e
			}
		}
	}
	// Existing validated station snapshots: keyset batches keep migrations bounded.
	var sideCursor int64
	for {
		rows, err := tx.Query("SELECT id,source_event_id,commander_fid,kind,raw_json FROM sidecar_snapshots WHERE id>? ORDER BY id LIMIT 128", sideCursor)
		if err != nil {
			return err
		}
		type side struct {
			cursor, id     int64
			fid, kind, raw string
		}
		batch := []side{}
		for rows.Next() {
			var r side
			if err = rows.Scan(&r.cursor, &r.id, &r.fid, &r.kind, &r.raw); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, r := range batch {
			sideCursor = r.cursor
			if moduleEnabled(tx, "economy") && validateStationSnapshot(tx, r.fid, r.id, r.kind, []byte(r.raw)) == nil {
				if err = saveMarket(tx, r.fid, r.id, r.kind, []byte(r.raw), "journal-sidecar", utc(), ""); err != nil {
					return err
				}
			}
		}
	}
	_, e := tx.Exec("INSERT OR REPLACE INTO operations_state VALUES('backfilled','retained records only; missing raw payloads require explicit verified original-journal reimport')")
	return e
}
