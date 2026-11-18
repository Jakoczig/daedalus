package history

import (
	"errors"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"time"
)

type SourceRow struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	State    string `json:"state"`
	Offset   int64  `json:"offset"`
	Size     int64  `json:"size"`
	LastSeen string `json:"lastSeen"`
}

func (s *Store) Sources() ([]SourceRow, error) {
	rows, err := s.reader.Query("SELECT source_id,canonical_path,state,committed_offset,observed_size,last_seen_at FROM journal_sources ORDER BY last_seen_at DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	v := []SourceRow{}
	for rows.Next() {
		var x SourceRow
		if err = rows.Scan(&x.ID, &x.Path, &x.State, &x.Offset, &x.Size, &x.LastSeen); err != nil {
			return nil, err
		}
		v = append(v, x)
	}
	return v, rows.Err()
}

// ReimportSource retains the original immutable generation and requires an
// explicit owner action. Only the selected source is restarted at offset zero.
func (s *Store) ReimportSource(id string) error {
	if len(id) < 1 || len(id) > 128 {
		return errors.New("Invalid source identity")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var path, identity, state string
	var size int64
	if err = tx.QueryRow("SELECT canonical_path,file_identity,observed_size,state FROM journal_sources WHERE source_id=?", id).Scan(&path, &identity, &size, &state); err != nil {
		return err
	}
	if state != "quarantined" {
		return errors.New("Only a quarantined source can be explicitly reimported")
	}
	if _, err = tx.Exec("UPDATE journal_sources SET state='replaced' WHERE source_id=?", id); err != nil {
		return err
	}
	next := digest([]byte(fmt.Sprintf("%s:%d", id, time.Now().UnixNano())))
	if _, err = tx.Exec("INSERT INTO journal_sources(source_id,canonical_path,file_identity,observed_size,first_seen_at,last_seen_at,state) VALUES(?,?,?,?,?,?,'verified')", next, path, identity, size, utc(), utc()); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) resumedGeneration(src journal.Source) (journal.Checkpoint, error) {
	cp := journal.Checkpoint{Source: src}
	var id string
	err := s.writer.QueryRow("SELECT source_id,committed_offset,committed_sequence,fingerprint_head,fingerprint_anchor,discard FROM journal_sources WHERE file_identity=? AND state='verified' ORDER BY first_seen_at DESC LIMIT 1", src.Identity).Scan(&id, &cp.Offset, &cp.Sequence, &cp.Head, &cp.Anchor, &cp.Discard)
	if err != nil {
		return cp, err
	}
	cp.Source.ID = id
	return cp, nil
}
