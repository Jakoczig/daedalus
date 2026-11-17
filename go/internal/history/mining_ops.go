package history

import (
	"database/sql"
	"strconv"
)

// Deliberately conservative provenance: a witnessed empty Ship Cargo baseline,
// continuous same-session records and exclusively refined input are required.
func normalizeMining(tx *sql.Tx, event int64, fid, session, kind, at string, v map[string]any) error {
	if session == "" {
		return nil
	}
	commodity := symbol(v["Type"])
	invalidate := func(all bool) error {
		if !all {
			if commodity == "" {
				return nil
			}
			_, e := tx.Exec("INSERT INTO mining_inventory VALUES(?,?,?,0,0) ON CONFLICT(commander_fid,session_id,commodity) DO UPDATE SET trusted=0", fid, session, commodity)
			return e
		}
		_, e := tx.Exec("UPDATE mining_inventory SET trusted=0 WHERE commander_fid=? AND session_id=?", fid, session)
		return e
	}
	switch kind {
	case "Cargo":
		inventory, ok := v["Inventory"].([]any)
		if !ok || text(v["Vessel"]) != "Ship" {
			return nil
		}
		if identifier(v["Count"]) == "0" && len(inventory) == 0 {
			if _, e := tx.Exec("DELETE FROM mining_inventory WHERE commander_fid=? AND session_id=?", fid, session); e != nil {
				return e
			}
			_, e := tx.Exec("INSERT INTO mining_sessions VALUES(?,?,1) ON CONFLICT(commander_fid,session_id) DO UPDATE SET empty_baseline=1", fid, session)
			return e
		}
		// Reconcile every known mining lot with a witnessed cargo total; differences
		// or new sources invalidate attribution rather than creating FIFO estimates.
		totals := map[string]int64{}
		for _, item := range list(inventory) {
			x, _ := item.(map[string]any)
			n, e := strconv.ParseInt(identifier(x["Count"]), 10, 64)
			if e != nil {
				return invalidate(true)
			}
			totals[symbol(x["Name"])] = n
		}
		rows, e := tx.Query("SELECT commodity,units FROM mining_inventory WHERE commander_fid=? AND session_id=? AND trusted=1", fid, session)
		if e != nil {
			return e
		}
		bad := []string{}
		for rows.Next() {
			var c string
			var n int64
			if e = rows.Scan(&c, &n); e != nil {
				rows.Close()
				return e
			}
			if totals[c] != n {
				bad = append(bad, c)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, c := range bad {
			if _, e = tx.Exec("UPDATE mining_inventory SET trusted=0 WHERE commander_fid=? AND session_id=? AND commodity=?", fid, session, c); e != nil {
				return e
			}
		}
		return nil
	case "MarketBuy", "CollectCargo":
		return invalidate(false)
	case "CargoTransfer", "CargoDepot", "MissionCompleted":
		// No claim about reward/transfer composition when the cargo pipeline is mixed.
		if _, e := tx.Exec("UPDATE mining_sessions SET empty_baseline=0 WHERE commander_fid=? AND session_id=?", fid, session); e != nil {
			return e
		}
		return invalidate(true)
	case "MiningRefined":
		var baseline int
		_ = tx.QueryRow("SELECT empty_baseline FROM mining_sessions WHERE commander_fid=? AND session_id=?", fid, session).Scan(&baseline)
		if commodity == "" {
			return nil
		}
		_, e := tx.Exec("INSERT INTO mining_inventory VALUES(?,?,?,1,?) ON CONFLICT(commander_fid,session_id,commodity) DO UPDATE SET units=units+1", fid, session, commodity, baseline)
		return e
	case "EjectCargo", "MarketSell":
		n, e := strconv.ParseInt(identifier(v["Count"]), 10, 64)
		if e != nil || n <= 0 {
			return invalidate(false)
		}
		var units int64
		var trusted bool
		e = tx.QueryRow("SELECT units,trusted FROM mining_inventory WHERE commander_fid=? AND session_id=? AND commodity=?", fid, session, commodity).Scan(&units, &trusted)
		if e == sql.ErrNoRows {
			return nil
		}
		if e != nil {
			return e
		}
		if units < n || !trusted {
			return invalidate(false)
		}
		if kind == "MarketSell" && identifier(v["TotalSale"]) != "" {
			if _, e = tx.Exec("INSERT OR IGNORE INTO trade_mining_summary VALUES(?,?,?,?,?,?,?,?)", event, fid, session, commodity, identifier(v["Count"]), identifier(v["TotalSale"]), at, "Witnessed empty cargo baseline and exclusively refined same-session input; receipts, overhead cost unknown"); e != nil {
				return e
			}
		}
		_, e = tx.Exec("UPDATE mining_inventory SET units=units-? WHERE commander_fid=? AND session_id=? AND commodity=?", n, fid, session, commodity)
		return e
	}
	return nil
}
