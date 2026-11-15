package history

import (
	"database/sql"
	"encoding/json"
)

type NavigationEvidence struct {
	Boost     *float64 `json:"boost"`
	BoostAt   string   `json:"boostAt"`
	DamageAt  string   `json:"damageAt"`
	Scooped   *float64 `json:"scooped"`
	FuelTotal *float64 `json:"fuelTotal"`
	ScoopAt   string   `json:"scoopAt"`
}

func (s *Store) FieldNavigationEvidence(fid string) (NavigationEvidence, error) {
	out := NavigationEvidence{}
	for _, kind := range []string{"JetConeBoost", "JetConeDamage", "FuelScoop"} {
		var raw, at string
		e := s.reader.QueryRow("SELECT raw_relevant_json,coalesce(event_utc,'') FROM journal_events WHERE commander_fid=? AND type=? ORDER BY event_id DESC LIMIT 1", fid, kind).Scan(&raw, &at)
		if e == sql.ErrNoRows {
			continue
		}
		if e != nil {
			return out, e
		}
		v := object([]byte(raw))
		get := func(k string) *float64 {
			if n, ok := v[k].(json.Number); ok {
				if x, e := n.Float64(); e == nil && x >= 0 {
					return &x
				}
			}
			return nil
		}
		switch kind {
		case "JetConeBoost":
			out.Boost = get("BoostValue")
			out.BoostAt = at
		case "JetConeDamage":
			out.DamageAt = at
		case "FuelScoop":
			out.Scooped = get("Scooped")
			out.FuelTotal = get("Total")
			out.ScoopAt = at
		}
	}
	return out, nil
}
