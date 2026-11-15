package history

import (
	"database/sql"
	"encoding/json"
	"strconv"
)

// ProviderSeed is private bootstrap context, never a browser projection.
type ProviderSeed struct {
	FID, Name, SourceID, GameVersion, GameBuild, System, Address string
	Sequence                                                     uint64
	Position                                                     []any
	Odyssey                                                      bool
}

func (s *Store) ProviderSeed(source string) (ProviderSeed, error) {
	v := ProviderSeed{SourceID: source}
	if source == "" {
		return v, nil
	}
	var fid, address sql.NullString
	if err := s.reader.QueryRow("SELECT context_fid,context_system,committed_sequence FROM journal_sources WHERE source_id=?", source).Scan(&fid, &address, &v.Sequence); err != nil {
		return v, err
	}
	if !fid.Valid {
		return v, nil
	}
	v.FID = fid.String
	v.Address = address.String
	if err := s.reader.QueryRow("SELECT coalesce(latest_name,'') FROM commander WHERE fid=?", v.FID).Scan(&v.Name); err != nil {
		return v, err
	}
	var raw string
	if s.reader.QueryRow("SELECT raw_relevant_json FROM journal_events WHERE source_id=? AND type='Fileheader' ORDER BY byte_offset DESC LIMIT 1", source).Scan(&raw) == nil {
		d := object([]byte(raw))
		v.GameVersion = text(d["gameversion"])
		v.GameBuild = text(d["build"])
	}
	if s.reader.QueryRow("SELECT raw_relevant_json FROM journal_events WHERE source_id=? AND commander_fid=? AND type='LoadGame' ORDER BY byte_offset DESC LIMIT 1", source, v.FID).Scan(&raw) == nil {
		v.Odyssey, _ = object([]byte(raw))["Odyssey"].(bool)
	}
	var x, y, z sql.NullFloat64
	if address.Valid && s.reader.QueryRow("SELECT name,x,y,z FROM systems WHERE system_address=?", address.String).Scan(&v.System, &x, &y, &z) == nil && x.Valid && y.Valid && z.Valid {
		v.Position = []any{json.Number(strconv.FormatFloat(x.Float64, 'g', -1, 64)), json.Number(strconv.FormatFloat(y.Float64, 'g', -1, 64)), json.Number(strconv.FormatFloat(z.Float64, 'g', -1, 64))}
	}
	return v, nil
}
