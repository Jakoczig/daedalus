package history

import (
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/journal"
)

func (s *Store) SaveSidecar(fid string, e journal.Event, kind string, raw json.RawMessage) error {
	if len(raw) > 1<<20 || !json.Valid(raw) || (kind != "Market" && kind != "Shipyard" && kind != "Outfitting") {
		return errors.New("Unsupported station sidecar")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRow("SELECT event_id FROM journal_events WHERE source_id=? AND byte_offset=? AND line_hash=? AND commander_fid=? AND type=?", e.Position.FileID, e.Position.Offset, digest(e.Raw), fid, kind).Scan(&id); err != nil {
		return err
	}
	if err = validateStationSnapshot(tx, fid, id, kind, raw); err != nil {
		return err
	}
	_, err = tx.Exec("INSERT OR IGNORE INTO sidecar_snapshots(source_event_id,commander_fid,kind,content_hash,observed_at,raw_json) VALUES(?,?,?,?,?,?)", id, fid, kind, digest(raw), e.Timestamp.UTC().Format("2006-01-02T15:04:05.000000000Z"), string(raw))
	if err != nil {
		return err
	}
	if err = saveMarket(tx, fid, id, kind, raw, "journal-sidecar", utc(), ""); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) SourceHeader(source string) (journal.Event, error) {
	var raw string
	var offset int64
	var sequence uint64
	err := s.reader.QueryRow("SELECT raw_relevant_json,byte_offset,record_sequence FROM journal_events WHERE source_id=? AND type='Fileheader' ORDER BY byte_offset DESC LIMIT 1", source).Scan(&raw, &offset, &sequence)
	if err != nil {
		return journal.Event{}, err
	}
	return journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: source, Offset: offset, Sequence: sequence})
}
