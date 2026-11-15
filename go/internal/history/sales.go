package history

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
)

func linkSale(tx *sql.Tx, fid string, sale int64, kind, stamp string, v map[string]any) error {
	names := map[string]bool{}
	if kind == "SellExplorationData" {
		a, _ := v["Systems"].([]any)
		for _, x := range a {
			if text(x) != "" {
				names[text(x)] = true
			}
		}
	}
	if kind == "MultiSellExplorationData" {
		a, _ := v["Discovered"].([]any)
		for _, x := range a {
			m, _ := x.(map[string]any)
			if text(m["SystemName"]) != "" {
				names[text(m["SystemName"])] = true
			}
		}
	}
	for name := range names {
		var address string
		// Identical names at different witnessed addresses cannot establish a sale.
		err := tx.QueryRow("SELECT min(system_address) FROM system_aliases WHERE name=? HAVING count(DISTINCT system_address)=1", name).Scan(&address)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT OR IGNORE INTO sale_item_links SELECT ?,e.event_id,'proven','Unique witnessed system name in exploration sale; prior scan only' FROM journal_events e WHERE e.commander_fid=? AND e.system_address=? AND e.type IN ('Scan','SAAScanComplete') AND e.event_id<? AND (?='' OR e.event_utc<=?)`, sale, fid, address, sale, stamp, stamp)
		if err != nil {
			return err
		}
	}
	if kind != "SellOrganicData" {
		return nil
	}
	data, _ := v["BioData"].([]any)
	for _, x := range data {
		item, _ := x.(map[string]any)
		genus, species := text(item["Genus"]), text(item["Species"])
		if genus == "" || species == "" {
			continue
		}
		// Even a single candidate is not proof that old/missing journal files contained
		// no second matching sample. Preserve ambiguity rather than infer a sale.
		if _, err := tx.Exec(`INSERT OR IGNORE INTO sale_item_links SELECT ?,b.source_event_id,'ambiguous','Organic sale omits source system/body; owner must confirm matching sample' FROM bio_completed b WHERE b.commander_fid=? AND b.source_event_id<? AND json_extract(b.raw_json,'$.Genus')=? AND json_extract(b.raw_json,'$.Species')=? AND (?='' OR b.observed_at<=?)`, sale, fid, sale, genus, species, stamp, stamp); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) ConfirmOrganicSale(fid, sale, observation string, ack bool) error {
	if !ack || fid == "" {
		return errors.New("Explicit matching sample confirmation required")
	}
	for _, id := range []string{sale, observation} {
		n, e := strconv.ParseInt(id, 10, 64)
		if e != nil || n < 1 || strconv.FormatInt(n, 10) != id {
			return errors.New("Invalid event reference")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.writer.Exec(`UPDATE sale_item_links SET confidence='owner_confirmed',reason='Owner explicitly matched organic sale to this completed sample' WHERE sale_event_id=? AND observation_event_id=? AND confidence='ambiguous' AND EXISTS(SELECT 1 FROM journal_events a JOIN journal_events b ON b.event_id=observation_event_id WHERE a.event_id=sale_event_id AND a.commander_fid=? AND b.commander_fid=a.commander_fid)`, sale, observation, fid)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("No ambiguous matching sample for this commander")
	}
	return nil
}

type SaleLink struct {
	Sale        string          `json:"sale"`
	Observation string          `json:"observation"`
	Confidence  string          `json:"confidence"`
	Reason      string          `json:"reason"`
	Sample      json.RawMessage `json:"sample"`
}

func (s *Store) SaleLinks(fid string) ([]SaleLink, error) {
	rows, err := s.reader.Query(`SELECT cast(l.sale_event_id AS TEXT),cast(l.observation_event_id AS TEXT),l.confidence,l.reason,e.raw_relevant_json FROM sale_item_links l JOIN journal_events e ON e.event_id=l.observation_event_id WHERE e.commander_fid=? ORDER BY l.sale_event_id DESC LIMIT 100`, fid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SaleLink{}
	for rows.Next() {
		var v SaleLink
		var raw string
		if err = rows.Scan(&v.Sale, &v.Observation, &v.Confidence, &v.Reason, &raw); err != nil {
			return nil, err
		}
		if len(raw) <= 16384 {
			v.Sample = json.RawMessage(raw)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
