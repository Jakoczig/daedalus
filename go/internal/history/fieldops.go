package history

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/fieldops"
	"strconv"
	"strings"
	"time"
)

//go:embed fieldops.sql
var fieldSchema string

type Itinerary struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Hash      string              `json:"hash"`
	State     string              `json:"state"`
	Origin    string              `json:"origin"`
	Next      int                 `json:"next"`
	Active    bool                `json:"active"`
	Waypoints []fieldops.Waypoint `json:"waypoints"`
}
type ItineraryProgress struct {
	Plan           Itinerary
	Next, Previous *fieldops.Waypoint
}

func (s *Store) ItinerarySize(id string) (int, error) {
	var n int
	e := s.reader.QueryRow("SELECT count(*) FROM itinerary_waypoints WHERE itinerary_id=?", id).Scan(&n)
	return n, e
}
func (s *Store) ItineraryProgress(fid string) (ItineraryProgress, error) {
	var out ItineraryProgress
	p := &out.Plan
	err := s.reader.QueryRow("SELECT cast(id AS TEXT),name,source_hash,state,origin,next_hop,active FROM itineraries WHERE commander_fid=? AND active=1", fid).Scan(&p.ID, &p.Name, &p.Hash, &p.State, &p.Origin, &p.Next, &p.Active)
	if err != nil {
		return out, err
	}
	rows, err := s.reader.Query("SELECT ordinal,name,address,neutron,refuel FROM itinerary_waypoints WHERE itinerary_id=? AND ordinal BETWEEN ? AND ? ORDER BY ordinal", p.ID, p.Next-1, p.Next)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var i int
		var w fieldops.Waypoint
		if err = rows.Scan(&i, &w.Name, &w.Address, &w.Neutron, &w.Refuel); err != nil {
			return out, err
		}
		if i == p.Next {
			out.Next = &w
		} else {
			out.Previous = &w
		}
	}
	return out, rows.Err()
}

func readPlan(q interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}, fid, id string) (Itinerary, error) {
	p := Itinerary{Waypoints: []fieldops.Waypoint{}}
	err := q.QueryRow("SELECT cast(id AS TEXT),name,source_hash,state,origin,next_hop,active FROM itineraries WHERE commander_fid=? AND ((?='' AND active=1) OR cast(id AS TEXT)=?)", fid, id, id).Scan(&p.ID, &p.Name, &p.Hash, &p.State, &p.Origin, &p.Next, &p.Active)
	if err != nil {
		return p, err
	}
	rows, err := q.Query("SELECT name,address,neutron,refuel,coalesce(coordinates,'') FROM itinerary_waypoints WHERE itinerary_id=? ORDER BY ordinal LIMIT 2048", p.ID)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var w fieldops.Waypoint
		var pos string
		if err = rows.Scan(&w.Name, &w.Address, &w.Neutron, &w.Refuel, &pos); err != nil {
			return p, err
		}
		if pos != "" {
			json.Unmarshal([]byte(pos), &w.Coordinates)
		}
		p.Waypoints = append(p.Waypoints, w)
	}
	return p, rows.Err()
}
func (s *Store) Itinerary(fid, id string) (Itinerary, error) { return readPlan(s.reader, fid, id) }
func (s *Store) Itineraries(fid string) ([]Itinerary, error) {
	rows, e := s.reader.Query("SELECT cast(id AS TEXT),name,source_hash,state,origin,next_hop,active FROM itineraries WHERE commander_fid=? ORDER BY id DESC LIMIT 16", fid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Itinerary{}
	for rows.Next() {
		var p Itinerary
		if e = rows.Scan(&p.ID, &p.Name, &p.Hash, &p.State, &p.Origin, &p.Next, &p.Active); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) ImportItinerary(fid, name string, raw []byte) (Itinerary, error) {
	p := Itinerary{}
	points, e := fieldops.ParseCSV(raw)
	if e != nil {
		return p, e
	}
	if !fieldops.SafeName(name) || fid == "" {
		return p, errors.New("Choose a verified commander and unique plan name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.writer.Begin()
	if e != nil {
		return p, e
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRow("SELECT count(*) FROM itineraries WHERE commander_fid=?", fid).Scan(&n); e != nil {
		return p, e
	}
	if n >= 16 {
		return p, errors.New("16 saved plans per commander maximum")
	}
	if _, e = tx.Exec("UPDATE itineraries SET active=0 WHERE commander_fid=?", fid); e != nil {
		return p, e
	}
	r, e := tx.Exec("INSERT INTO itineraries(commander_fid,name,source_hash,created_at,active,last_event) VALUES(?,?,?,?,1,coalesce((SELECT max(event_id) FROM journal_events),0))", fid, name, digest(raw), utc())
	if e != nil {
		return p, errors.New("Plan name already exists or commander unavailable")
	}
	id, _ := r.LastInsertId()
	for i, w := range points {
		pos := ""
		if w.Coordinates != nil {
			b, _ := json.Marshal(w.Coordinates)
			pos = string(b)
		}
		if _, e = tx.Exec("INSERT INTO itinerary_waypoints VALUES(?,?,?,?,?,?,?)", id, i, w.Name, w.Address, w.Neutron, w.Refuel, pos); e != nil {
			return p, e
		}
	}
	var arrival, address string
	var event int64
	if tx.QueryRow("SELECT event_id,raw_relevant_json,coalesce(system_address,'') FROM journal_events WHERE commander_fid=? AND type IN ('FSDJump','Location') ORDER BY event_id DESC LIMIT 1", fid).Scan(&event, &arrival, &address) == nil {
		if next, state := fieldops.Advance(points, 0, "Location", text(object([]byte(arrival))["StarSystem"]), address); next == 1 {
			if _, e = tx.Exec("UPDATE itineraries SET next_hop=1,state=?,origin='initial last-known location' WHERE id=?", state, id); e != nil {
				return p, e
			}
		}
	}
	if e = tx.Commit(); e != nil {
		return p, e
	}
	return readPlan(s.writer, fid, strconv.FormatInt(id, 10))
}
func (s *Store) EditItinerary(fid, id string, next int, ack bool) (Itinerary, error) {
	if !ack {
		return Itinerary{}, errors.New("Preview the waypoint and acknowledge this manual correction")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := readPlan(s.writer, fid, id)
	if e != nil {
		return p, e
	}
	if next < 0 || next > len(p.Waypoints) {
		return p, errors.New("Invalid waypoint")
	}
	tx, e := s.writer.Begin()
	if e != nil {
		return p, e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE itineraries SET active=0 WHERE commander_fid=?", fid); e != nil {
		return p, e
	}
	if _, e = tx.Exec("UPDATE itineraries SET active=1,next_hop=?,state='manual',origin='owner correction',last_event=coalesce((SELECT max(event_id) FROM journal_events),0) WHERE id=?", next, id); e != nil {
		return p, e
	}
	if _, e = tx.Exec("INSERT INTO itinerary_edits(itinerary_id,next_hop,origin,at) VALUES(?,?,'owner correction',?)", id, next, utc()); e != nil {
		return p, e
	}
	if e = tx.Commit(); e != nil {
		return p, e
	}
	return readPlan(s.writer, fid, id)
}
func advanceItinerary(tx *sql.Tx, fid string, eventID int64, kind, name, address, stamp string) error {
	if kind != "FSDJump" && kind != "Location" {
		return nil
	}
	if !moduleEnabled(tx, "itinerary") {
		return nil
	}
	p, e := readPlan(tx, fid, "")
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	var last int64
	var created string
	if e = tx.QueryRow("SELECT last_event,coalesce((SELECT at FROM itinerary_edits WHERE itinerary_id=itineraries.id AND origin='owner correction' ORDER BY id DESC LIMIT 1),created_at) FROM itineraries WHERE id=?", p.ID).Scan(&last, &created); e != nil {
		return e
	}
	at, err := time.Parse(time.RFC3339Nano, stamp)
	imported, _ := time.Parse(time.RFC3339Nano, created)
	if err != nil || at.Before(imported.Add(-time.Second)) || at.After(time.Now().Add(5*time.Second)) {
		return nil
	}
	if eventID <= last || p.State == "off_route" {
		return nil
	}
	next, state := fieldops.Advance(p.Waypoints, p.Next, kind, name, address)
	if state == "unchanged" {
		return nil
	}
	if _, e = tx.Exec("UPDATE itineraries SET next_hop=?,state=?,origin='journal arrival',last_event=? WHERE id=?", next, state, eventID, p.ID); e != nil {
		return e
	}
	_, e = tx.Exec("INSERT INTO itinerary_edits(itinerary_id,next_hop,origin,source_event,at) VALUES(?,?,'journal arrival',?,?)", p.ID, next, eventID, utc())
	return e
}

type TallyRow struct {
	Ordinal   int    `json:"ordinal"`
	Event     string `json:"event"`
	System    string `json:"system"`
	Faction   string `json:"faction"`
	Category  string `json:"category"`
	At        string `json:"at"`
	Amount    string `json:"amount"`
	Unit      string `json:"unit"`
	Qualifier string `json:"qualifier"`
	Mission   string `json:"mission"`
	Period    string `json:"period"`
}

func (s *Store) FieldTally(ctx context.Context, fid, system, faction string, before int64, limit int, ticks bool) ([]TallyRow, error) {
	return s.fieldTallyPage(ctx, fid, system, faction, before, -1, limit, ticks)
}
func (s *Store) FieldTallyCursor(ctx context.Context, fid, system, faction, cursor string, limit int, ticks bool) ([]TallyRow, error) {
	before := int64(0)
	ordinal := -1
	if cursor != "" {
		a := strings.Split(cursor, ":")
		if len(a) != 2 {
			return nil, fmt.Errorf("Invalid tally cursor")
		}
		var e error
		before, e = strconv.ParseInt(a[0], 10, 64)
		if e != nil || before < 1 {
			return nil, fmt.Errorf("Invalid tally cursor")
		}
		ordinal, e = strconv.Atoi(a[1])
		if e != nil || ordinal < 0 {
			return nil, fmt.Errorf("Invalid tally cursor")
		}
	}
	return s.fieldTallyPage(ctx, fid, system, faction, before, ordinal, limit, ticks)
}
func (s *Store) fieldTallyPage(ctx context.Context, fid, system, faction string, before int64, ordinal, limit int, ticks bool) ([]TallyRow, error) {
	if limit < 1 || limit > 200 {
		return nil, errors.New("Choose 1–200 rows")
	}
	if before == 0 {
		before = 1<<63 - 1
	}
	rows, e := s.reader.QueryContext(ctx, "SELECT cast(source_event AS TEXT),ordinal,system_address,faction,category,observed_at,amount,unit,qualifier,mission_id FROM field_tally WHERE commander_fid=? AND (?='' OR system_address=?) AND (?='' OR faction=?) AND (source_event<? OR (source_event=? AND ordinal>?)) ORDER BY source_event DESC,ordinal LIMIT ?", fid, system, system, faction, faction, before, before, ordinal, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []TallyRow{}
	for rows.Next() {
		var r TallyRow
		if e = rows.Scan(&r.Event, &r.Ordinal, &r.System, &r.Faction, &r.Category, &r.At, &r.Amount, &r.Unit, &r.Qualifier, &r.Mission); e != nil {
			return nil, e
		}
		at, e := time.Parse(time.RFC3339Nano, r.At)
		r.Period = "unknown date"
		if e == nil {
			r.Period = at.UTC().Format("2006-01-02") + " UTC day (not BGS tick)"
		}
		out = append(out, r)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	if ticks {
		bounds, e := s.reader.QueryContext(ctx, "SELECT at FROM field_ticks ORDER BY at DESC LIMIT 128")
		if e != nil {
			return nil, e
		}
		defer bounds.Close()
		var b []time.Time
		for bounds.Next() {
			var v string
			if e = bounds.Scan(&v); e != nil {
				return nil, e
			}
			if at, e := time.Parse(time.RFC3339Nano, v); e == nil {
				b = append(b, at)
			}
		}
		for i := range out {
			at, e := time.Parse(time.RFC3339Nano, out[i].At)
			if e != nil {
				continue
			}
			for j := 1; j < len(b); j++ {
				if !at.Before(b[j]) && at.Before(b[j-1]) {
					out[i].Period = b[j].UTC().Format(time.RFC3339) + " verified BGS tick"
					break
				}
			}
		}
	}
	return out, nil
}
func fieldTally(tx *sql.Tx, id int64, fid, system, kind, stamp string, v map[string]any) error {
	switch kind {
	case "MissionCompleted", "MissionFailed", "MissionAbandoned", "MarketBuy", "MarketSell", "Bounty", "RedeemVoucher", "FactionKillBond", "ColonisationContribution":
	default:
		if !strings.HasPrefix(kind, "Powerplay") {
			return nil
		}
	}
	if !moduleEnabled(tx, "bgs") {
		return nil
	}
	ordinal := 0
	add := func(sys, faction, category string, amount any, unit, qualifier string) error {
		if sys == "" {
			sys = "unattributed"
		}
		if faction == "" {
			faction = "unattributed"
		}
		n := ""
		if x, ok := amount.(json.Number); ok {
			n = string(x)
		}
		_, e := tx.Exec("INSERT OR IGNORE INTO field_tally VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", id, ordinal, fid, sys, faction, category, stamp, n, unit, qualifier, identifier(v["MissionID"]), "journal-effects-v1")
		ordinal++
		return e
	}
	faction := text(v["Faction"])
	switch kind {
	case "MissionCompleted":
		if e := add(system, faction, "mission_completed", v["Reward"], "CR", "Observed reward; no inferred influence"); e != nil {
			return e
		}
		effects, _ := v["FactionEffects"].([]any)
		for _, raw := range effects {
			x, _ := raw.(map[string]any)
			f := text(x["Faction"])
			inf, _ := x["Influence"].([]any)
			for _, raw := range inf {
				n, _ := raw.(map[string]any)
				if e := add(identifier(n["SystemAddress"]), f, "mission_influence", nil, "ordinal label (not percent)", text(n["Influence"])); e != nil {
					return e
				}
			}
			if rep := text(x["Reputation"]); rep != "" {
				if e := add("", f, "mission_reputation", nil, "ordinal label", rep); e != nil {
					return e
				}
			}
		}
	case "MissionFailed", "MissionAbandoned":
		return add(system, faction, kind, nil, "event", "Outcome only; impact unknown")
	case "MarketBuy", "MarketSell":
		return add(system, "", kind, v["Count"], "units", "Trade volume; faction impact unknown")
	case "Bounty":
		rewards, _ := v["Rewards"].([]any)
		for _, raw := range rewards {
			x, _ := raw.(map[string]any)
			if e := add(system, text(x["Faction"]), "bounty_awarded", x["Reward"], "CR", "Awarded, not redeemed; no percent change"); e != nil {
				return e
			}
		}
		if len(rewards) == 0 {
			return add(system, "", "bounty_awarded", v["TotalReward"], "CR", "Faction unattributed")
		}
	case "RedeemVoucher":
		if factions, ok := v["Factions"].([]any); ok && len(factions) > 0 {
			for _, raw := range factions {
				x, _ := raw.(map[string]any)
				if e := add(system, text(x["Faction"]), "voucher_redeemed", x["Amount"], "CR", text(v["Type"])+"; impact unknown"); e != nil {
					return e
				}
			}
			return nil
		}
		return add(system, faction, "voucher_redeemed", v["Amount"], "CR", text(v["Type"])+"; impact unknown")
	case "FactionKillBond":
		return add(system, text(v["AwardingFaction"]), "combat_bond_awarded", v["Reward"], "CR", "Awarded, not redeemed")
	case "ColonisationContribution":
		if a, ok := v["Contributions"].([]any); ok {
			for _, raw := range a {
				x, _ := raw.(map[string]any)
				if e := add(system, "", "colonisation_commodity", x["Amount"], "units", text(x["Name"])+"; observed contribution, influence unknown"); e != nil {
					return e
				}
			}
			return nil
		}
		return add(system, "", "colonisation_contribution", v["Contribution"], "reported contribution", "No inferred BGS influence")
	default:
		if len(kind) >= 9 && kind[:9] == "Powerplay" {
			return add(system, "", kind, v["Merits"], "merits if reported", "No inferred influence")
		}
	}
	return nil
}
func backfillFieldTally(tx *sql.Tx) error {
	var cursor int64
	for {
		rows, e := tx.Query("SELECT event_id,commander_fid,coalesce(system_address,''),type,coalesce(event_utc,''),raw_relevant_json FROM journal_events WHERE event_id>? AND commander_fid IS NOT NULL AND (type IN ('MissionCompleted','MissionFailed','MissionAbandoned','MarketBuy','MarketSell','Bounty','RedeemVoucher','FactionKillBond','ColonisationContribution','ScanOrganic') OR type LIKE 'Powerplay%') ORDER BY event_id LIMIT 256", cursor)
		if e != nil {
			return e
		}
		type item struct {
			id                      int64
			fid, sys, kind, at, raw string
		}
		batch := []item{}
		for rows.Next() {
			var x item
			if e = rows.Scan(&x.id, &x.fid, &x.sys, &x.kind, &x.at, &x.raw); e != nil {
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
			return nil
		}
		for _, x := range batch {
			if x.kind == "ScanOrganic" {
				if _, e = tx.Exec("UPDATE bio_samples SET entity_id=? WHERE source_event_id=? AND (entity_id IS NULL OR entity_id='')", identifier(object([]byte(x.raw))["Body"]), x.id); e != nil {
					return e
				}
			}

			if e = fieldTally(tx, x.id, x.fid, x.sys, x.kind, x.at, object([]byte(x.raw))); e != nil {
				return e
			}
			cursor = x.id
		}
	}
}
