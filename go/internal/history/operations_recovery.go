package history

import (
	"bytes"
	"context"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"os"
)

// Recover only previously indexed records, at their recorded offsets, with exact
// original line-hash proof. No cursor reset, new journal copy, or provider event.
func (s *Store) RecoverOperations(ctx context.Context, source string) (int, error) {
	if len(source) == 0 || len(source) > 128 {
		return 0, errors.New("Select a verified original source")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var path, state string
	if e = tx.QueryRow("SELECT canonical_path,state FROM journal_sources WHERE source_id=?", source).Scan(&path, &state); e != nil {
		return 0, e
	}
	if state != "verified" {
		return 0, errors.New("Use existing quarantine recovery before backfill")
	}
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() {
		return 0, errors.New("Original journal unavailable; indexed history preserved")
	}
	file, e := os.Open(path)
	if e != nil {
		return 0, errors.New("Original journal unavailable")
	}
	defer file.Close()
	cursor := int64(0)
	count := 0
	for {
		rows, e := tx.QueryContext(ctx, `SELECT event_id,byte_offset,byte_length,line_hash,commander_fid,coalesce(session_id,''),coalesce(system_address,''),type,coalesce(event_utc,'') FROM journal_events WHERE source_id=? AND event_id>? AND raw_relevant_json='{}' AND commander_fid IS NOT NULL ORDER BY event_id LIMIT 128`, source, cursor)
		if e != nil {
			return 0, e
		}
		type row struct {
			id, offset, length                   int64
			hash, fid, session, system, kind, at string
		}
		batch := []row{}
		for rows.Next() {
			var r row
			if e = rows.Scan(&r.id, &r.offset, &r.length, &r.hash, &r.fid, &r.session, &r.system, &r.kind, &r.at); e != nil {
				rows.Close()
				return 0, e
			}
			batch = append(batch, r)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return 0, e
		}
		if len(batch) == 0 {
			break
		}
		for _, r := range batch {
			cursor = r.id
			if !operationKind(r.kind) {
				continue
			}
			if r.length <= 0 || r.length > journal.MaxRecordBytes+2 {
				return 0, errors.New("Original record bounds invalid")
			}
			raw := make([]byte, r.length)
			if _, e = file.ReadAt(raw, r.offset); e != nil {
				return 0, errors.New("Original record missing; no backfill applied")
			}
			raw = bytes.TrimSpace(raw)
			if digest(raw) != r.hash {
				return 0, errors.New("Original journal changed; no backfill applied")
			}
			ev, e := journal.ParseLine(raw, journal.SourcePosition{})
			if e != nil || ev.Type != r.kind {
				return 0, errors.New("Original event contract mismatch")
			}
			if _, e = tx.Exec("UPDATE journal_events SET raw_relevant_json=? WHERE event_id=? AND raw_relevant_json='{}'", string(raw), r.id); e != nil {
				return 0, e
			}
			if e = normalizeOperations(tx, r.id, r.fid, r.session, r.system, r.kind, r.at, object(raw)); e != nil {
				return 0, e
			}
			count++
		}
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return count, nil
}
