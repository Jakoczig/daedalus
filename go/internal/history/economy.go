package history

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Every observation is attributed before persistence. Upload permissions are
// independent: local station history is useful with every provider disabled.
func saveMarket(tx *sql.Tx, fid string, event int64, kind string, raw []byte, provider, fetched, expires string) error {
	if !moduleEnabled(tx, "economy") {
		return nil
	}
	v := object(raw)
	market := identifier(v["MarketID"])
	at := text(v["timestamp"])
	if market == "" || text(v["StationName"]) == "" || text(v["StarSystem"]) == "" || at == "" {
		return errors.New("Station identity missing")
	}
	stamp, e := time.Parse(time.RFC3339Nano, at)
	if e != nil {
		return e
	}
	at = stamp.UTC().Format("2006-01-02T15:04:05.000000000Z")
	items, ok := v["Items"].([]any)
	if kind == "Market" && (!ok || len(items) > 2048) {
		return errors.New("Bounded commodity array required")
	}
	seen := map[string]bool{}
	for _, item := range items {
		x, ok := item.(map[string]any)
		name := symbol(x["Name"])
		if !ok || name == "" || seen[name] {
			return errors.New("Duplicate or invalid commodity")
		}
		seen[name] = true
	}
	res, e := tx.Exec("INSERT OR IGNORE INTO market_observations(commander_fid,market_id,system_name,station_name,kind,source_event_id,observed_at,fetched_at,expires_at,source,content_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?)", fid, market, text(v["StarSystem"]), text(v["StationName"]), kind, nullEvent(event), at, fetched, expires, provider, digest(raw))
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}
	id, _ := res.LastInsertId()
	if kind != "Market" {
		return nil
	}
	for _, item := range items {
		x, _ := item.(map[string]any)
		if _, e = tx.Exec("INSERT INTO commodity_quotes VALUES(?,?,?,?,?,?,?)", id, symbol(x["Name"]), unsigned(x["BuyPrice"]), unsigned(x["SellPrice"]), unsigned(x["MeanPrice"]), unsigned(x["Demand"]), unsigned(x["Stock"])); e != nil {
			return e
		}
	}
	return nil
}
func nullEvent(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// Exact marker and most recent docking boundary must agree; no browser identity.
func validateStationSnapshot(tx *sql.Tx, fid string, event int64, kind string, raw []byte) error {
	v := object(raw)
	if text(v["event"]) != kind {
		return errors.New("Station sidecar event mismatch")
	}
	var markerRaw, session, source string
	var offset int64
	if e := tx.QueryRow("SELECT raw_relevant_json,coalesce(session_id,''),source_id,byte_offset FROM journal_events WHERE event_id=? AND commander_fid=? AND type=?", event, fid, kind).Scan(&markerRaw, &session, &source, &offset); e != nil {
		return e
	}
	m := object([]byte(markerRaw))
	if session == "" || identifier(v["MarketID"]) == "" || identifier(m["MarketID"]) != identifier(v["MarketID"]) || (text(m["StationName"]) != "" && text(v["StationName"]) != text(m["StationName"])) || (text(m["StarSystem"]) != "" && text(v["StarSystem"]) != text(m["StarSystem"])) || text(v["timestamp"]) != text(m["timestamp"]) {
		return errors.New("Station marker identity or timestamp mismatch")
	}
	var dockRaw, boundary string
	if e := tx.QueryRow(`SELECT type,raw_relevant_json FROM journal_events WHERE source_id=? AND session_id=? AND byte_offset<=? AND type IN ('Docked','Undocked','StartJump','Shutdown','LoadGame') ORDER BY byte_offset DESC LIMIT 1`, source, session, offset).Scan(&boundary, &dockRaw); e != nil {
		return e
	}
	d := object([]byte(dockRaw))
	if boundary != "Docked" || identifier(d["MarketID"]) != identifier(v["MarketID"]) || text(d["StationName"]) != text(v["StationName"]) || text(d["StarSystem"]) != text(v["StarSystem"]) {
		return errors.New("Docking session mismatch")
	}
	return nil
}

// Existing EDSM reference client supplies verified fixed-host, read-consented
// responses. This persists the selected station only, never initiates a request.
func (s *Store) SaveExternalMarket(fid string, station StationReference, data map[string]any, fetched, expires string) error {
	if fid == "" {
		return errors.New("Commander context required")
	}
	items, ok := data["commodities"].([]any)
	if !ok || len(items) > 2048 {
		return errors.New("Bounded external inventory required")
	}
	out := []any{}
	for _, item := range items {
		x, _ := item.(map[string]any)
		out = append(out, map[string]any{"Name": x["name"], "BuyPrice": x["buyPrice"], "SellPrice": x["sellPrice"], "Demand": x["demand"], "Stock": x["stock"]})
	}
	// Provider observation time remains distinct; fetch time is not proof of age.
	if station.Observed == "" {
		return nil
	}
	raw, _ := json.Marshal(map[string]any{"MarketID": station.MarketID, "StarSystem": station.SystemName, "StationName": station.Name, "timestamp": station.Observed, "Items": out})
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.writer.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = saveMarket(tx, fid, 0, "Market", raw, "edsm-reference", fetched, expires); e != nil {
		return e
	}
	return tx.Commit()
}
