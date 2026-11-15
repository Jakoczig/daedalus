package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type ConstructionResource struct {
	Name     string   `json:"name"`
	Required *float64 `json:"required"`
	Provided *float64 `json:"provided"`
	Payment  *float64 `json:"payment"`
}
type ConstructionProject struct {
	MarketID      string                 `json:"marketId"`
	SystemAddress string                 `json:"systemAddress"`
	Progress      *float64               `json:"progress"`
	Complete      *bool                  `json:"complete"`
	Failed        *bool                  `json:"failed"`
	Observed      string                 `json:"observedAt"`
	Resources     []ConstructionResource `json:"resources"`
}

func (s *Store) Construction(ctx context.Context, fid string) ([]ConstructionProject, error) {
	if fid == "" || len(fid) > 128 {
		return nil, errors.New("Select commander")
	}
	rows, err := s.reader.QueryContext(ctx, `SELECT p.market_id,coalesce(p.system_address,''),p.construction_progress,p.complete,p.failed,coalesce(p.observed_at,''),e.raw_relevant_json FROM colonisation_projects p JOIN journal_events e ON e.event_id=p.source_event_id WHERE p.commander_fid=? ORDER BY p.observed_at DESC LIMIT 50`, fid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConstructionProject{}
	for rows.Next() {
		var v ConstructionProject
		var raw string
		var progress sql.NullFloat64
		var complete, failed sql.NullBool
		if err = rows.Scan(&v.MarketID, &v.SystemAddress, &progress, &complete, &failed, &v.Observed, &raw); err != nil {
			return nil, err
		}
		if progress.Valid {
			v.Progress = &progress.Float64
		}
		if complete.Valid {
			v.Complete = &complete.Bool
		}
		if failed.Valid {
			v.Failed = &failed.Bool
		}
		v.Resources = []ConstructionResource{}
		items, _ := object([]byte(raw))["ResourcesRequired"].([]any)
		if len(items) > 128 {
			items = items[:128]
		}
		for _, x := range items {
			item, _ := x.(map[string]any)
			name := text(item["Name"])
			if name == "" {
				continue
			}
			resource := ConstructionResource{Name: name}
			for k, target := range map[string]**float64{"RequiredAmount": &resource.Required, "ProvidedAmount": &resource.Provided, "Payment": &resource.Payment} {
				if n, ok := item[k].(json.Number); ok {
					f, e := n.Float64()
					if e == nil && f >= 0 {
						*target = &f
					}
				}
			}
			v.Resources = append(v.Resources, resource)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
