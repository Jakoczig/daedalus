package history

import (
	"database/sql"
	"encoding/json"
	"errors"
)

type ProviderDisk struct {
	Providers map[string]json.RawMessage `json:"providers"`
	Ledger    map[string]int64           `json:"ledger"`
	Uploader  string                     `json:"uploader"`
}

func (s *Store) LoadProviderConsent(initial []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var marker int
	err := s.writer.QueryRow("SELECT count(*) FROM app_state WHERE key='provider-consent-migrated-v1'").Scan(&marker)
	if err != nil {
		return nil, err
	}
	if marker == 0 {
		if err = s.saveConsentLocked(initial); err != nil {
			return nil, err
		}
	}
	disk := ProviderDisk{Providers: map[string]json.RawMessage{}, Ledger: map[string]int64{}}
	rows, err := s.writer.Query("SELECT provider,config_json FROM provider_consent ORDER BY provider LIMIT 33")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		disk.Providers[id] = json.RawMessage(raw)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(disk.Providers) > 32 {
		return nil, errors.New("Invalid provider consent database")
	}
	var b []byte
	var hash string
	if err = s.writer.QueryRow("SELECT value,content_hash FROM app_state WHERE key='provider-ledger'").Scan(&b, &hash); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if len(b) > 0 {
		if digest(b) != hash {
			return nil, errors.New("Provider ledger checksum mismatch")
		}
		var ledger struct {
			Ledger   map[string]int64 `json:"ledger"`
			Uploader string           `json:"uploader"`
		}
		if err = json.Unmarshal(b, &ledger); err != nil {
			return nil, err
		}
		disk.Ledger = ledger.Ledger
		disk.Uploader = ledger.Uploader
	}
	return json.Marshal(disk)
}
func (s *Store) SaveProviderConsent(raw []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveConsentLocked(raw)
}
func (s *Store) saveConsentLocked(raw []byte) error {
	if len(raw) > 1<<20 {
		return errors.New("Consent budget exceeded")
	}
	var disk ProviderDisk
	if json.Unmarshal(raw, &disk) != nil || len(disk.Providers) > 32 || len(disk.Ledger) > 4096 {
		return errors.New("Invalid provider state")
	}
	tx, err := s.writer.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for id, cfg := range disk.Providers {
		if len(id) > 32 || len(cfg) > 16384 || !json.Valid(cfg) {
			return errors.New("Invalid provider configuration")
		}
		if _, err = tx.Exec("INSERT INTO provider_consent VALUES(?,?,?) ON CONFLICT(provider) DO UPDATE SET config_json=excluded.config_json,updated_at=excluded.updated_at", id, string(cfg), utc()); err != nil {
			return err
		}
	}
	disk.Providers = nil
	b, _ := json.Marshal(disk)
	for key, value := range map[string][]byte{"provider-ledger": b, "provider-consent-migrated-v1": []byte("1")} {
		if _, err = tx.Exec("INSERT INTO app_state VALUES(?,?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,content_hash=excluded.content_hash,updated_at=excluded.updated_at", key, value, digest(value), utc()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) SingleConsent(id string, initial []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b []byte
	err := s.writer.QueryRow("SELECT config_json FROM provider_consent WHERE provider=?", id).Scan(&b)
	if err == sql.ErrNoRows {
		if !json.Valid(initial) {
			return nil, errors.New("Invalid consent")
		}
		_, err = s.writer.Exec("INSERT INTO provider_consent VALUES(?,?,?)", id, string(initial), utc())
		return initial, err
	}
	return b, err
}
func (s *Store) SetSingleConsent(id string, raw []byte) error {
	if len(id) > 32 || len(raw) > 16384 || !json.Valid(raw) {
		return errors.New("Invalid consent")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.writer.Exec("INSERT INTO provider_consent VALUES(?,?,?) ON CONFLICT(provider) DO UPDATE SET config_json=excluded.config_json,updated_at=excluded.updated_at", id, string(raw), utc())
	return err
}
