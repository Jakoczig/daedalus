package history

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Only the existing consented Elite BGS adapter calls this after validation.
func (s *Store) SaveFieldTicks(ctx context.Context, raw []byte, now time.Time) error {
	var ticks []struct {
		Time string `json:"time"`
	}
	if len(raw) > 64<<10 || json.Unmarshal(raw, &ticks) != nil || len(ticks) > 32 {
		return errors.New("Invalid bounded tick response")
	}
	bounds := []string{}
	for _, t := range ticks {
		at, e := time.Parse(time.RFC3339Nano, t.Time)
		if e != nil || at.After(now.Add(5*time.Minute)) {
			return errors.New("Unverified tick timestamp")
		}
		bounds = append(bounds, at.UTC().Format(time.RFC3339Nano))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, b := range bounds {
		if _, e = tx.Exec("INSERT OR REPLACE INTO field_ticks VALUES(?,'Elite BGS V5 /ticks',?)", b, now.UTC().Format(time.RFC3339Nano)); e != nil {
			return e
		}
	}
	return tx.Commit()
}
