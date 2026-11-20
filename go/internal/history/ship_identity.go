package history

import "context"

func (s *Store) WitnessedShipName(ctx context.Context, fid, id string, name, at *string) error {
	return s.reader.QueryRowContext(ctx, "SELECT name,observed_at FROM fleet_ships WHERE commander_fid=? AND ship_id=? AND state<>'sold'", fid, id).Scan(name, at)
}
