package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type SystemReference struct {
	Address     string     `json:"address"`
	Name        string     `json:"name"`
	EDSMID      string     `json:"edsmId"`
	Coordinates [3]float64 `json:"coordinates"`
}

func (s *Store) SystemReference(ctx context.Context, name string, now time.Time) (SystemReference, error) {
	var v SystemReference
	var x, y, z sql.NullFloat64
	err := s.reader.QueryRowContext(ctx, "SELECT min(system_address),name,x,y,z FROM systems WHERE name=? COLLATE NOCASE HAVING count(*)=1", name).Scan(&v.Address, &v.Name, &x, &y, &z)
	if err == nil && x.Valid && y.Valid && z.Valid {
		v.Coordinates = [3]float64{x.Float64, y.Float64, z.Float64}
		return v, nil
	}
	raw, err := s.CachedReference("edsm-reference", "system/"+name, now)
	if err != nil {
		return v, err
	}
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil || identifier(v.Address) == "" || v.Name != name {
		return v, errors.New("System needs witnessed coordinates or a verified nearby-system cache entry")
	}
	return v, nil
}
func (s *Store) StationByMarket(ctx context.Context, market string) (StationReference, error) {
	var v StationReference
	var services string
	if identifier(market) == "" {
		return v, errors.New("Canonical MarketID required")
	}
	err := s.reader.QueryRowContext(ctx, "SELECT provider,market_id,system_address,system_name,name,coalesce(station_type,''),arrival_ls,x,y,z,services_json,coalesce(observed_at,''),fetched_at,expires_at FROM station_reference WHERE market_id=? ORDER BY fetched_at DESC LIMIT 1", market).Scan(&v.Provider, &v.MarketID, &v.SystemAddress, &v.SystemName, &v.Name, &v.Type, &v.ArrivalLS, &v.Coordinates[0], &v.Coordinates[1], &v.Coordinates[2], &services, &v.Observed, &v.Fetched, &v.Expires)
	if err != nil {
		return v, errors.New("Fetch the verified station list before its inventory")
	}
	json.Unmarshal([]byte(services), &v.Services)
	return v, nil
}
