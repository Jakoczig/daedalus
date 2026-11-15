package history

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"
)

// ExternalFacts contains normalized allowlisted facts, never provider responses or secrets.
type ExternalFacts struct {
	Provider, Entity, EntityID, FID string
	Observed, Fetched, Expires      time.Time
	IdentityVerified                bool
	Fields                          map[string]any
}
type FactEvidence struct {
	Source    string          `json:"source"`
	Reference string          `json:"reference"`
	Observed  string          `json:"observedAt"`
	Fetched   string          `json:"fetchedAt"`
	Stale     bool            `json:"stale"`
	Value     json.RawMessage `json:"value"`
}
type EffectiveFact struct {
	Field string `json:"field"`
	FactEvidence
	Alternatives []FactEvidence `json:"alternatives"`
}

var allowedFacts = map[string]bool{"name": true, "credits": true, "debt": true, "ranks": true, "squadron": true, "ships": true, "ship": true, "carrier": true, "inventory": true, "communityGoals": true, "market": true, "shipyard": true, "system": true, "station": true, "coordinates": true}

func (s *Store) SaveExternal(ctx context.Context, x ExternalFacts) error {
	if (x.Provider != "frontier" && x.Provider != "inara") || x.Entity == "" || len(x.Entity) > 64 || len(x.EntityID) > 128 || len(x.FID) > 128 || !x.IdentityVerified || x.Fetched.IsZero() || !x.Expires.After(x.Fetched) || len(x.Fields) > 32 {
		return errors.New("Verified bounded external identity and freshness required")
	}
	for k := range x.Fields {
		if !allowedFacts[k] {
			return errors.New("Unsupported external field")
		}
	}
	raw, err := json.Marshal(x.Fields)
	if err != nil || len(raw) > 256<<10 {
		return errors.New("External facts exceed limit")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := func(v time.Time) string {
		if v.IsZero() {
			return ""
		}
		return v.UTC().Format(time.RFC3339Nano)
	}
	observed := stamp(x.Observed)
	result, err := tx.ExecContext(ctx, "INSERT INTO external_snapshots(provider,entity_type,entity_id,commander_fid,source_observed_at,fetched_at,expires_at,identity_verified,content_hash,normalized_fields) VALUES(?,?,?,?,?,?,?,?,?,?)", x.Provider, x.Entity, x.EntityID, null(x.FID), observed, stamp(x.Fetched), stamp(x.Expires), true, digest(raw), string(raw))
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	for k, v := range x.Fields {
		b, _ := json.Marshal(v)
		if _, err = tx.ExecContext(ctx, "INSERT INTO field_provenance(entity,field,commander_fid,source,source_ref,observed_at,fetched_at,confidence,stale,value_json) VALUES(?,?,?,?,?,?,?,?,?,?)", x.Entity, k, null(x.FID), x.Provider, strconv.FormatInt(id, 10), observed, stamp(x.Fetched), "verified identity; remote snapshot", false, string(b)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) EffectiveFacts(ctx context.Context, fid, entity string, now time.Time) ([]EffectiveFact, error) {
	if len(fid) > 128 || len(entity) > 64 {
		return nil, errors.New("Invalid fact query")
	}
	rows, err := s.reader.QueryContext(ctx, `SELECT p.field,p.source,p.source_ref,coalesce(p.observed_at,''),coalesce(p.fetched_at,''),p.value_json,coalesce(e.expires_at,'') FROM field_provenance p LEFT JOIN external_snapshots e ON e.id=cast(p.source_ref AS INTEGER) AND p.source=e.provider WHERE p.id IN (SELECT max(id) FROM field_provenance WHERE coalesce(commander_fid,'')=? AND entity=? GROUP BY field,source) ORDER BY p.id DESC LIMIT 256`, fid, entity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string][]FactEvidence{}
	seen := map[string]bool{}
	for rows.Next() {
		var field, expires, raw string
		var v FactEvidence
		if err = rows.Scan(&field, &v.Source, &v.Reference, &v.Observed, &v.Fetched, &raw, &expires); err != nil {
			return nil, err
		}
		v.Value = json.RawMessage(raw)
		key := field + "/" + v.Source
		if seen[key] {
			continue
		}
		seen[key] = true
		expiry, e := time.Parse(time.RFC3339Nano, expires)
		v.Stale = e != nil || !now.Before(expiry)
		if v.Source == "journal" {
			at, e := time.Parse(time.RFC3339Nano, v.Observed)
			v.Stale = e != nil || at.After(now.Add(5*time.Second)) || now.Sub(at) > 5*time.Minute
		}
		values[field] = append(values[field], v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	score := func(v FactEvidence) int {
		n := 0
		if !v.Stale {
			n = 10
		}
		switch v.Source {
		case "journal":
			n += 3
		case "frontier":
			n += 2
		case "inara":
			n++
		}
		return n
	}
	out := []EffectiveFact{}
	for field, a := range values {
		sort.SliceStable(a, func(i, j int) bool { return score(a[i]) > score(a[j]) })
		out = append(out, EffectiveFact{Field: field, FactEvidence: a[0], Alternatives: a[1:]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out, nil
}
func (s *Store) NeedsInara(ctx context.Context, fid, entity string, fields []string, now time.Time) bool {
	facts, err := s.EffectiveFacts(ctx, fid, entity, now)
	if err != nil {
		return false
	}
	for _, name := range fields {
		found := false
		for _, f := range facts {
			if f.Field == name && !f.Stale {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}
