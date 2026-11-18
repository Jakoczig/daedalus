package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"io"
	"os"
)

func verifiedFileHash(path string, size int64) (string, error) {
	f, err := journal.OpenReadOnly(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return verifiedOpenHash(f, size)
}
func verifiedOpenHash(f *os.File, size int64) (string, error) {
	before, err := f.Stat()
	if err != nil || before.Size() != size {
		return "", errors.New("Source changed")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.NewSectionReader(f, 0, size+1))
	after, e := f.Stat()
	if err != nil || e != nil || n != size || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return "", errors.New("Source changed while verifying copy")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ResolveCopy runs only for a new source whose complete size/head/anchor match
// an already committed file. Both whole files are then streamed and SHA-256
// compared. A timestamp, payload or sampled fingerprint alone never deduplicates.
// This is called synchronously by the single journal pipeline, not a new scanner.
func (s *Store) ResolveCopy(src journal.Source, head, anchor []byte, current *os.File) (journal.Checkpoint, bool, error) {
	cp := journal.Checkpoint{Source: src}
	if src.Size == 0 || len(head) == 0 {
		return cp, false, nil
	}
	rows, err := s.reader.Query(`SELECT source_id,canonical_path FROM journal_sources WHERE source_id!=? AND state='verified' AND committed_offset=? AND observed_size=? AND fingerprint_head=? AND fingerprint_anchor=? ORDER BY first_seen_at LIMIT 4`, src.ID, src.Size, src.Size, head, anchor)
	if err != nil {
		return cp, false, err
	}
	type candidate struct{ id, path string }
	candidates := []candidate{}
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.path); err != nil {
			rows.Close()
			return cp, false, err
		}
		candidates = append(candidates, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(candidates) == 0 {
		return cp, false, err
	}
	currentHash, err := verifiedOpenHash(current, src.Size)
	if err != nil {
		return cp, false, nil
	}
	for _, c := range candidates {
		originalHash, err := verifiedFileHash(c.path, src.Size)
		if err != nil || currentHash != originalHash {
			continue
		}
		s.mu.Lock()
		tx, err := s.writer.BeginTx(context.Background(), nil)
		if err != nil {
			s.mu.Unlock()
			return cp, false, err
		}
		var fid, session, system any
		err = tx.QueryRow("SELECT committed_offset,committed_sequence,fingerprint_head,fingerprint_anchor,discard,context_fid,context_session,context_system FROM journal_sources WHERE source_id=? AND state='verified' AND committed_offset=? AND fingerprint_head=? AND fingerprint_anchor=?", c.id, src.Size, head, anchor).Scan(&cp.Offset, &cp.Sequence, &cp.Head, &cp.Anchor, &cp.Discard, &fid, &session, &system)
		if err == nil {
			var result interface{ RowsAffected() (int64, error) }
			result, err = tx.Exec("UPDATE journal_sources SET committed_offset=?,committed_sequence=?,fingerprint_head=?,fingerprint_anchor=?,discard=?,context_fid=?,context_session=?,context_system=? WHERE source_id=? AND committed_offset=0 AND committed_sequence=0", cp.Offset, cp.Sequence, cp.Head, cp.Anchor, cp.Discard, fid, session, system, src.ID)
			if err == nil {
				n, _ := result.RowsAffected()
				if n != 1 {
					err = errors.New("Copy checkpoint changed")
				}
			}
		}
		if err == nil {
			raw, _ := json.Marshal(map[string]string{"canonicalSource": c.id, "wholeFileSHA256": currentHash})
			_, err = tx.Exec("INSERT OR REPLACE INTO app_state VALUES(?,?,?,?)", "source-alias/"+src.ID, raw, digest(raw), utc())
		}
		if err == nil {
			err = tx.Commit()
		} else {
			tx.Rollback()
		}
		s.mu.Unlock()
		if err != nil {
			return cp, false, err
		}
		return cp, true, nil
	}
	return cp, false, nil
}
