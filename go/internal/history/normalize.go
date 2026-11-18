package history

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"math"
	"strconv"
	"strings"
)

func object(raw []byte) map[string]any {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var v map[string]any
	if d.Decode(&v) != nil {
		return nil
	}
	return v
}
func text(v any) string {
	s, _ := v.(string)
	if len(s) > 512 || strings.ContainsAny(s, "\x00\r\n") {
		return ""
	}
	return s
}
func identifier(v any) string {
	var s string
	switch x := v.(type) {
	case json.Number:
		s = string(x)
	case string:
		s = x
	default:
		return ""
	}
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil || strconv.FormatUint(n, 10) != s {
		return ""
	}
	return s
}
func number(v any) any {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	f, e := n.Float64()
	if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	return f
}
func boolean(v any) any {
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return b
}

var domain = map[string]string{
	"Cargo": "cargo_snapshots", "MarketBuy": "transactions", "MarketSell": "transactions", "BuyDrones": "transactions", "SellDrones": "transactions", "RedeemVoucher": "transactions", "PayFines": "transactions", "PayBounties": "transactions", "BuyTradeData": "transactions", "RefuelAll": "transactions", "RefuelPartial": "transactions", "RepairAll": "transactions", "Repair": "transactions", "Statistics": "transactions",
	"Materials": "materials_snapshots", "MaterialCollected": "materials_snapshots", "MaterialDiscarded": "materials_snapshots", "MaterialTrade": "materials_snapshots", "EngineerCraft": "materials_snapshots", "Synthesis": "materials_snapshots",
	"ShipLocker": "inventory_snapshots", "Backpack": "inventory_snapshots", "BackpackChange": "inventory_snapshots", "TransferMicroResources": "inventory_snapshots", "BuyMicroResources": "inventory_snapshots", "SellMicroResources": "inventory_snapshots",
	"StoredShips": "stored_ships", "StoredModules": "stored_modules", "Scan": "exploration_scans", "SAAScanComplete": "exploration_scans", "FSSBodySignals": "exploration_scans", "SAASignalsFound": "exploration_scans", "FSSDiscoveryScan": "exploration_scans", "FSSAllBodiesFound": "exploration_scans", "ScanOrganic": "bio_samples",
	"SellExplorationData": "exploration_sales", "MultiSellExplorationData": "exploration_sales", "SellOrganicData": "organic_sales",
	"MissionAccepted": "mission_history", "MissionCompleted": "mission_history", "MissionFailed": "mission_history", "MissionAbandoned": "mission_history", "Missions": "mission_history", "CommunityGoal": "mission_history", "CommunityGoalJoin": "mission_history", "CommunityGoalReward": "transactions",
	"SquadronStartup": "squadron_history", "SquadronPromotion": "squadron_history", "SquadronDemotion": "squadron_history", "JoinedSquadron": "squadron_history", "LeftSquadron": "squadron_history",
	"ColonisationConstructionDepot": "colonisation_commodity_snapshots", "ColonisationContribution": "colonisation_contributions",
}

func relevant(kind string) bool {
	if operationKind(kind) {
		return true
	}
	if kind == "Bounty" || kind == "FactionKillBond" || kind == "JetConeBoost" || kind == "JetConeDamage" || kind == "CodexEntry" {
		return true
	}
	if domain[kind] != "" {
		return true
	}
	if strings.HasPrefix(kind, "Powerplay") {
		return true
	}
	switch kind {
	case "Market", "Shipyard", "Outfitting", "Fileheader", "LoadGame", "Commander", "Shutdown", "Location", "FSDJump", "CarrierJump", "Docked", "Undocked", "StartJump", "Loadout", "SetUserShipName", "ShipyardBuy", "ShipyardSell", "ShipyardSwap", "ShipyardTransfer", "ModuleBuy", "ModuleSell", "ModuleSwap", "NavRoute", "NavRouteClear", "FSDTarget", "Rank", "Progress", "Reputation", "EngineerProgress", "Died", "SupercruiseEntry", "SupercruiseExit", "ApproachBody", "LeaveBody", "Touchdown", "Liftoff", "FuelScoop", "ReservoirReplenished", "HullDamage", "HeatDamage", "ReceiveText", "CarrierStats", "CarrierJumpRequest":
		return kind != "ReceiveText"
	}
	return false
}
func (s *Store) normalize(tx *sql.Tx, source string, records []journal.Record, checkpointSequence uint64) error {
	var fid, session, system sql.NullString
	var previous uint64
	if err := tx.QueryRow("SELECT context_fid,context_session,context_system,committed_sequence FROM journal_sources WHERE source_id=?", source).Scan(&fid, &session, &system, &previous); err != nil {
		return err
	}
	insertEvent, err := tx.Prepare("INSERT OR IGNORE INTO journal_events(source_id,byte_offset,byte_length,record_sequence,line_hash,event_utc,type,commander_fid,session_id,system_address,raw_relevant_json,ingested_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer insertEvent.Close()
	domainStatements := map[string]*sql.Stmt{}
	defer func() {
		for _, stmt := range domainStatements {
			stmt.Close()
		}
	}()
	for _, r := range records {
		e := r.Event
		if previous != 0 && e.Position.Sequence != previous+1 {
			fid = sql.NullString{}
			session = sql.NullString{}
			system = sql.NullString{}
		}
		previous = e.Position.Sequence
		v := object(e.Raw)
		stamp := ""
		if !e.Timestamp.IsZero() {
			stamp = e.Timestamp.UTC().Format("2006-01-02T15:04:05.000000000Z")
		}
		if e.Type == "Fileheader" {
			fid = sql.NullString{}
			session = sql.NullString{}
			system = sql.NullString{}
		}
		if e.Type == "LoadGame" && text(e.FID) != "" {
			fid = sql.NullString{String: e.FID, Valid: true}
			session = sql.NullString{String: fmt.Sprintf("%s:%d", source, e.Position.Offset), Valid: true}
			system = sql.NullString{}
			if _, err := tx.Exec("INSERT INTO commander VALUES(?,?,?) ON CONFLICT(fid) DO UPDATE SET latest_name=excluded.latest_name,updated_at=excluded.updated_at WHERE excluded.updated_at>=coalesce(commander.updated_at,'')", e.FID, text(v["Commander"]), stamp); err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO commander_names VALUES(?,?,?,NULL)", e.FID, text(v["Commander"]), stamp); err != nil {
				return err
			}
			version := e.GameVersion
			var header string
			if tx.QueryRow("SELECT raw_relevant_json FROM journal_events WHERE source_id=? AND type='Fileheader' ORDER BY byte_offset DESC LIMIT 1", source).Scan(&header) == nil {
				version = text(object([]byte(header))["gameversion"])
			}
			galaxy := "unknown"
			if strings.HasPrefix(version, "4.") && !strings.ContainsAny(strings.ToLower(version), "abcdefghijklmnopqrstuvwxyz") {
				galaxy = "live"
			} else if strings.HasPrefix(version, "3.") || strings.HasPrefix(version, "2.") {
				galaxy = "legacy"
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO game_sessions VALUES(?,?,?,?,NULL,?,?)", session.String, e.FID, source, stamp, version, galaxy); err != nil {
				return err
			}
		}
		if fid.Valid && e.SystemAddress != "" && (e.Type == "Location" || e.Type == "FSDJump" || e.Type == "CarrierJump") {
			system = sql.NullString{String: e.SystemAddress, Valid: true}
		}
		retainedRaw := string(e.Raw)
		if !relevant(e.Type) {
			retainedRaw = "{}"
		} // Keep continuity metadata, never unrelated verbose payloads.
		address := e.SystemAddress
		if address == "" && system.Valid {
			address = system.String
		}
		res, err := insertEvent.Exec(source, e.Position.Offset, e.Position.Length, e.Position.Sequence, digest(e.Raw), null(stamp), e.Type, fid, session, null(address), retainedRaw, utc())
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		eventID, _ := res.LastInsertId()
		if !fid.Valid {
			continue
		}
		if err = normalizeOperations(tx, eventID, fid.String, session.String, address, e.Type, stamp, v); err != nil {
			return err
		}
		if err = fieldTally(tx, eventID, fid.String, address, e.Type, stamp, v); err != nil {
			return err
		}
		if err = advanceItinerary(tx, fid.String, eventID, e.Type, e.SystemName, e.SystemAddress, stamp); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO publication_policy VALUES(?,'Never',?)", fid.String, utc()); err != nil {
			return err
		}
		table := domain[e.Type]
		if strings.HasPrefix(e.Type, "Powerplay") {
			table = "powerplay_history"
		}
		if table != "" {
			entity := identifier(v["MarketID"])
			if entity == "" {
				entity = identifier(v["ShipID"])
			}
			if entity == "" {
				entity = identifier(v["BodyID"])
			}
			if e.Type == "ScanOrganic" {
				entity = identifier(v["Body"])
			}
			stmt := domainStatements[table]
			if stmt == nil {
				stmt, err = tx.Prepare("INSERT INTO " + table + "(commander_fid,source_event_id,entity_id,system_address,observed_at,raw_json) VALUES(?,?,?,?,?,?)")
				if err != nil {
					return err
				}
				domainStatements[table] = stmt
			}
			if _, err = stmt.Exec(fid.String, eventID, null(entity), null(address), null(stamp), string(e.Raw)); err != nil {
				return err
			}
		}
		facts := map[string]any{}
		switch e.Type {
		case "LoadGame":
			facts["name"] = text(v["Commander"])
			if n, ok := v["Credits"].(json.Number); ok {
				facts["credits"] = string(n)
			}
		case "Rank", "Progress":
			facts["ranks"] = v
		case "SquadronStartup", "JoinedSquadron":
			facts["squadron"] = v
		case "CommunityGoal":
			facts["communityGoals"] = v
		}
		for field, value := range facts {
			raw, _ := json.Marshal(exactFactValue(value))
			if _, err = tx.Exec("INSERT INTO field_provenance(entity,field,commander_fid,source,source_ref,observed_at,fetched_at,confidence,stale,value_json) VALUES('commander',?,?,'journal',?,?,?,'witnessed local event',0,?)", field, fid.String, strconv.FormatInt(eventID, 10), stamp, utc(), string(raw)); err != nil {
				return err
			}
		}
		if e.Type == "SellExplorationData" || e.Type == "MultiSellExplorationData" || e.Type == "SellOrganicData" {
			if err = linkSale(tx, fid.String, eventID, e.Type, stamp, v); err != nil {
				return err
			}
		}
		if e.Type == "Shutdown" && session.Valid {
			if _, err = tx.Exec("UPDATE game_sessions SET end_utc=? WHERE session_id=?", stamp, session.String); err != nil {
				return err
			}
		}
		if address != "" && e.SystemName != "" && (e.Type == "Location" || e.Type == "FSDJump" || e.Type == "CarrierJump") {
			var xyz [3]any
			if a, ok := v["StarPos"].([]any); ok && len(a) == 3 {
				for i, x := range a {
					xyz[i] = number(x)
				}
			}
			if _, err = tx.Exec("INSERT INTO systems VALUES(?,?,?,?,?) ON CONFLICT(system_address) DO UPDATE SET name=excluded.name,x=coalesce(excluded.x,x),y=coalesce(excluded.y,y),z=coalesce(excluded.z,z)", address, e.SystemName, xyz[0], xyz[1], xyz[2]); err != nil {
				return err
			}
			if _, err = tx.Exec("INSERT OR IGNORE INTO system_aliases VALUES(?,?)", address, e.SystemName); err != nil {
				return err
			}
			if _, err = tx.Exec("INSERT INTO system_visits(commander_fid,system_address,event_utc,source_event_id) VALUES(?,?,?,?)", fid.String, address, stamp, eventID); err != nil {
				return err
			}
			if a, ok := v["Factions"].([]any); ok && len(a) > 0 {
				if _, err = tx.Exec("INSERT INTO bgs_observations(commander_fid,source_event_id,system_address,observed_at,raw_json) VALUES(?,?,?,?,?)", fid.String, eventID, address, stamp, string(e.Raw)); err != nil {
					return err
				}
			}
		}
		if e.Type == "Loadout" {
			if err = normalizeLoadout(tx, fid.String, eventID, stamp, v, string(e.Raw)); err != nil {
				return err
			}
		}
		if e.Type == "Scan" && address != "" && identifier(v["BodyID"]) != "" {
			if _, err = tx.Exec("INSERT INTO body_observations VALUES(?,?,?,?,?,?)", address, identifier(v["BodyID"]), fid.String, eventID, stamp, string(e.Raw)); err != nil {
				return err
			}
		}
		if e.Type == "ScanOrganic" && text(v["ScanType"]) == "Analyse" {
			if _, err = tx.Exec("INSERT INTO bio_completed(commander_fid,source_event_id,entity_id,system_address,observed_at,raw_json) VALUES(?,?,?,?,?,?)", fid.String, eventID, null(identifier(v["Body"])), null(address), stamp, string(e.Raw)); err != nil {
				return err
			}
		}
		if e.Type == "ColonisationConstructionDepot" && identifier(v["MarketID"]) != "" {
			if _, err = tx.Exec("INSERT INTO colonisation_projects VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(commander_fid,market_id) DO UPDATE SET system_address=excluded.system_address,construction_progress=excluded.construction_progress,complete=excluded.complete,failed=excluded.failed,observed_at=excluded.observed_at,source_event_id=excluded.source_event_id", fid.String, identifier(v["MarketID"]), null(address), number(v["ConstructionProgress"]), boolean(v["ConstructionComplete"]), boolean(v["ConstructionFailed"]), stamp, eventID); err != nil {
				return err
			}
		}
	}
	if previous != checkpointSequence {
		fid = sql.NullString{}
		session = sql.NullString{}
		system = sql.NullString{}
	}
	_, err = tx.Exec("UPDATE journal_sources SET context_fid=?,context_session=?,context_system=? WHERE source_id=?", fid, session, system, source)
	return err
}
func normalizeLoadout(tx *sql.Tx, fid string, eventID int64, stamp string, v map[string]any, raw string) error {
	ship := identifier(v["ShipID"])
	if ship == "" {
		return nil
	}
	_, err := tx.Exec("INSERT INTO ship_instances VALUES(?,?,?,?,?,?,?) ON CONFLICT(commander_fid,ship_id) DO UPDATE SET model=excluded.model,custom_name=excluded.custom_name,last_seen=excluded.last_seen WHERE excluded.last_seen>=coalesce(ship_instances.last_seen,'')", fid, ship, text(v["Ship"]), text(v["ShipName"]), "witnessed", nil, stamp)
	if err != nil {
		return err
	}
	result, err := tx.Exec("INSERT INTO ship_loadout_snapshots(commander_fid,ship_id,source_event_id,observed_at,reported_mass,reported_jump_range,fuel,rebuy,raw_json) VALUES(?,?,?,?,?,?,?,?,?)", fid, ship, eventID, stamp, number(v["UnladenMass"]), number(v["MaxJumpRange"]), nil, number(v["Rebuy"]), raw)
	if err != nil {
		return err
	}
	loadout, _ := result.LastInsertId()
	modules, _ := v["Modules"].([]any)
	if len(modules) > 512 {
		return nil
	}
	for _, x := range modules {
		m, ok := x.(map[string]any)
		if !ok {
			continue
		}
		slot, item := text(m["Slot"]), text(m["Item"])
		if slot == "" || item == "" {
			continue
		}
		eng, _ := m["Engineering"].(map[string]any)
		result, err = tx.Exec("INSERT OR IGNORE INTO ship_module_snapshots(loadout_id,slot,module_symbol,on_state,priority,engineering_blueprint,engineering_grade,experimental) VALUES(?,?,?,?,?,?,?,?)", loadout, slot, item, boolean(m["On"]), number(m["Priority"]), text(eng["BlueprintName"]), number(eng["Level"]), text(eng["ExperimentalEffect"]))
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			continue
		}
		module, _ := result.LastInsertId()
		mods, _ := eng["Modifiers"].([]any)
		if len(mods) > 128 {
			continue
		}
		for _, y := range mods {
			d, ok := y.(map[string]any)
			if !ok || text(d["Label"]) == "" {
				continue
			}
			if _, err = tx.Exec("INSERT OR IGNORE INTO ship_module_modifiers VALUES(?,?,?,?,?,?)", module, text(d["Label"]), number(d["Value"]), number(d["OriginalValue"]), boolean(d["LessIsGood"]), fmt.Sprintf("%T", d["Value"])); err != nil {
				return err
			}
		}
	}
	return nil
}
