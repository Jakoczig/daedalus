package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"
)

func (s *Store) CachedReference(provider, key string, now time.Time) (json.RawMessage, error) {
	var raw, expiry string
	err := s.reader.QueryRow("SELECT normalized_fields,expires_at FROM external_snapshots WHERE provider=? AND entity_type='public_reference' AND entity_id=? ORDER BY id DESC LIMIT 1", provider, key).Scan(&raw, &expiry)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil || !now.Before(at) {
		return nil, nil
	}
	return json.RawMessage(raw), nil
}
func (s *Store) SaveReference(ctx context.Context, provider, key string, raw json.RawMessage, now, expires time.Time) error {
	if len(provider) > 32 || len(key) > 256 || len(raw) > 256<<10 || !json.Valid(raw) || !expires.After(now) {
		return errors.New("Invalid bounded reference cache")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Public reference cache is bounded separately from indefinitely retained journals.
	_, err = tx.ExecContext(ctx, "DELETE FROM external_snapshots WHERE provider=? AND entity_type='public_reference' AND (entity_id=? OR expires_at<?)", provider, key, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO external_snapshots(provider,entity_type,entity_id,fetched_at,expires_at,identity_verified,content_hash,normalized_fields) VALUES(?,'public_reference',?,?,?,?,?,?)", provider, key, now.UTC().Format(time.RFC3339Nano), expires.UTC().Format(time.RFC3339Nano), true, digest(raw), string(raw)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM external_snapshots WHERE provider=? AND entity_type='public_reference' AND id NOT IN (SELECT id FROM external_snapshots WHERE provider=? AND entity_type='public_reference' ORDER BY id DESC LIMIT 128)", provider, provider); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) AuthorizeRead(provider, kind string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var last string
	err = tx.QueryRow("SELECT requested_at FROM provider_requests WHERE provider=? ORDER BY id DESC LIMIT 1", provider).Scan(&last)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if at, e := time.Parse(time.RFC3339Nano, last); e == nil && now.Before(at.Add(time.Minute)) {
		return errors.New("Persistent provider cooldown active")
	}
	if _, err = tx.Exec("INSERT INTO provider_requests(provider,kind,requested_at,outcome) VALUES(?,?,?,'requested')", provider, kind, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM provider_requests WHERE id NOT IN (SELECT id FROM provider_requests ORDER BY id DESC LIMIT 1024)"); err != nil {
		return err
	}
	return tx.Commit()
}

type StationReference struct {
	Provider      string          `json:"source"`
	MarketID      string          `json:"marketId"`
	SystemAddress string          `json:"systemAddress"`
	SystemName    string          `json:"systemName"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	ArrivalLS     *float64        `json:"arrivalLS"`
	Coordinates   [3]float64      `json:"coordinates"`
	Services      map[string]bool `json:"services"`
	Observed      string          `json:"observedAt"`
	Fetched       string          `json:"fetchedAt"`
	Expires       string          `json:"expiresAt"`
}

func (s *Store) SaveStation(x StationReference) error {
	if identifier(x.MarketID) == "" || identifier(x.SystemAddress) == "" || x.Name == "" || x.SystemName == "" || len(x.Services) > 32 {
		return errors.New("Exact station/system identity required")
	}
	for _, v := range x.Coordinates {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e6 {
			return errors.New("Invalid station coordinates")
		}
	}
	services, _ := json.Marshal(x.Services)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.writer.Exec(`INSERT INTO station_reference VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(provider,market_id) DO UPDATE SET system_address=excluded.system_address,system_name=excluded.system_name,name=excluded.name,station_type=excluded.station_type,arrival_ls=excluded.arrival_ls,x=excluded.x,y=excluded.y,z=excluded.z,services_json=excluded.services_json,observed_at=excluded.observed_at,fetched_at=excluded.fetched_at,expires_at=excluded.expires_at`, x.Provider, x.MarketID, x.SystemAddress, x.SystemName, x.Name, x.Type, x.ArrivalLS, x.Coordinates[0], x.Coordinates[1], x.Coordinates[2], string(services), x.Observed, x.Fetched, x.Expires)
	return err
}

type NearestStation struct {
	StationReference
	DistanceLY float64 `json:"distanceLY"`
	Stale      bool    `json:"stale"`
}

func (s *Store) Nearest(ctx context.Context, address, service string, radius float64, now time.Time) ([]NearestStation, error) {
	supported := map[string]bool{"repair": true, "outfitting": true, "shipyard": true, "large_pad": true, "material_trader": true, "technology_broker": true}
	if !supported[service] || radius <= 0 || radius > 100 || identifier(address) == "" {
		return nil, errors.New("Choose known system, service and radius <=100 LY")
	}
	var x, y, z sql.NullFloat64
	if err := s.reader.QueryRowContext(ctx, "SELECT x,y,z FROM systems WHERE system_address=?", address).Scan(&x, &y, &z); err != nil || !x.Valid || !y.Valid || !z.Valid {
		return nil, errors.New("Origin coordinates unknown")
	}
	rows, err := s.reader.QueryContext(ctx, `SELECT provider,market_id,coalesce(system_address,''),system_name,name,coalesce(station_type,''),arrival_ls,x,y,z,services_json,coalesce(observed_at,''),fetched_at,expires_at FROM station_reference WHERE x BETWEEN ? AND ? AND y BETWEEN ? AND ? AND z BETWEEN ? AND ? LIMIT 512`, x.Float64-radius, x.Float64+radius, y.Float64-radius, y.Float64+radius, z.Float64-radius, z.Float64+radius)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NearestStation{}
	for rows.Next() {
		var v NearestStation
		var services string
		if err = rows.Scan(&v.Provider, &v.MarketID, &v.SystemAddress, &v.SystemName, &v.Name, &v.Type, &v.ArrivalLS, &v.Coordinates[0], &v.Coordinates[1], &v.Coordinates[2], &services, &v.Observed, &v.Fetched, &v.Expires); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(services), &v.Services) != nil || !v.Services[service] {
			continue
		}
		v.DistanceLY = math.Sqrt(math.Pow(v.Coordinates[0]-x.Float64, 2) + math.Pow(v.Coordinates[1]-y.Float64, 2) + math.Pow(v.Coordinates[2]-z.Float64, 2))
		if v.DistanceLY > radius {
			continue
		}
		expiry, e := time.Parse(time.RFC3339Nano, v.Expires)
		v.Stale = e != nil || !now.Before(expiry)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceLY < out[j].DistanceLY })
	if len(out) > 50 {
		out = out[:50]
	}
	return out, rows.Err()
}
