package history

import (
	"database/sql"
	"encoding/json"
	"errors"
)

func (s *Store) DerivedStatistics(key, version string) (json.RawMessage, error) {
	var raw string
	err := s.reader.QueryRow("SELECT result_json FROM derived_statistics WHERE loadout_hash=? AND formula_version=?", key, version).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return json.RawMessage(raw), err
}
func (s *Store) SaveStatistics(key, version string, raw json.RawMessage) error {
	if len(key) != 64 || len(version) > 64 || len(raw) > 256<<10 || !json.Valid(raw) {
		return errors.New("Invalid derived statistics cache")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT OR REPLACE INTO derived_statistics VALUES(?,?,?,?)", key, version, string(raw), utc()); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM derived_statistics WHERE rowid NOT IN (SELECT rowid FROM derived_statistics ORDER BY computed_at DESC LIMIT 128)"); err != nil {
		return err
	}
	return tx.Commit()
}
