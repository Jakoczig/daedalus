// Package history owns the private SQLite history and the only serialized writer.
package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"path/filepath"
	"sync"
	"time"
)

//go:embed schema.sql
var schema string

const schemaVersion = 3
const maxCache = 32 << 20

var ErrQuarantined = errors.New("Journal identity changed; cached history retained. Review quarantined source in Data/History")

type Store struct {
	mu             sync.Mutex
	writer, reader *sql.DB
	dir, path      string
	closed         bool
}
type Stats struct {
	Events, Files, Bytes, LastOffset int64
	Schema                           int
	LastEvent                        string
	State                            string
}

func utc() string            { return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") }
func digest(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func dsn(path string) string {
	u := url.URL{Scheme: "file", Path: databaseURIPath(path)}
	return u.String() + "?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL&_txlock=immediate"
}
func Open(dir string) (*Store, error) {
	if err := privatefs.Prepare(dir); err != nil {
		return nil, err
	}
	if err := localFilesystem(dir); err != nil {
		return nil, err
	}
	if err := consumeRestore(dir); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "daedalus.sqlite")
	f, err := privatefs.PrivateFile(path, false)
	if err != nil {
		return nil, err
	}
	f.Close()
	db, err := sql.Open("sqlite3", dsn(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	fail := func(e error) (*Store, error) { db.Close(); return nil, e }
	if err = db.Ping(); err != nil {
		return fail(err)
	}
	var integrity string
	if err = db.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		return fail(errors.New("Database integrity failed; original preserved"))
	}
	if err := migrate(db, dir); err != nil {
		return fail(err)
	}
	read, err := sql.Open("sqlite3", dsn(path)+"&_query_only=on")
	if err != nil {
		return fail(err)
	}
	read.SetMaxOpenConns(2)
	read.SetMaxIdleConns(2)
	return &Store{writer: db, reader: read, dir: dir, path: path}, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.reader.Close()
	_, err := s.writer.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	e := s.writer.Close()
	if err != nil {
		return err
	}
	return e
}
func (s *Store) Resume(src journal.Source) (journal.Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := journal.Checkpoint{Source: src}
	var state string
	err := s.writer.QueryRow("SELECT committed_offset,committed_sequence,fingerprint_head,fingerprint_anchor,discard,state FROM journal_sources WHERE source_id=?", src.ID).Scan(&cp.Offset, &cp.Sequence, &cp.Head, &cp.Anchor, &cp.Discard, &state)
	if err == nil {
		if state == "replaced" {
			return s.resumedGeneration(src)
		}
		if state != "verified" {
			return cp, ErrQuarantined
		}
		return cp, nil
	}
	if err != sql.ErrNoRows {
		return cp, err
	}
	_, err = s.writer.Exec("INSERT INTO journal_sources(source_id,canonical_path,file_identity,observed_size,first_seen_at,last_seen_at,state) VALUES(?,?,?,?,?,?,'verified')", src.ID, src.Path, src.Identity, src.Size, utc(), utc())
	return cp, err
}
func (s *Store) Quarantine(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, e := s.writer.Exec("UPDATE journal_sources SET state='quarantined' WHERE source_id=?", id)
	return e
}
func (s *Store) Cache() ([]byte, error) {
	var data []byte
	var hash string
	err := s.reader.QueryRow("SELECT value,content_hash FROM app_state WHERE key='reducers-v1' AND length(value)<=?", maxCache).Scan(&data, &hash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if digest(data) != hash {
		return nil, errors.New("Cached state checksum mismatch")
	}
	return data, nil
}
func (s *Store) Commit(ctx context.Context, cp journal.Checkpoint, records []journal.Record, cache []byte) error {
	if len(records) > 256 || len(cache) > maxCache {
		return errors.New("Transaction budget exceeded")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("Database closed")
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.normalize(tx, cp.Source.ID, records); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE journal_sources SET canonical_path=?,observed_size=?,committed_offset=?,committed_sequence=?,fingerprint_head=?,fingerprint_anchor=?,discard=?,last_seen_at=? WHERE source_id=? AND state='verified'", cp.Source.Path, cp.Source.Size, cp.Offset, cp.Sequence, cp.Head, cp.Anchor, cp.Discard, utc(), cp.Source.ID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrQuarantined
	}
	if cache != nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO app_state VALUES('reducers-v1',?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,content_hash=excluded.content_hash,updated_at=excluded.updated_at", cache, digest(cache), utc())
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Stats() Stats {
	v := Stats{Schema: schemaVersion, State: "verified"}
	if err := s.reader.QueryRow("SELECT count(*),coalesce(max(event_utc),'') FROM journal_events").Scan(&v.Events, &v.LastEvent); err != nil {
		v.State = "DB_ERROR"
	}
	if err := s.reader.QueryRow("SELECT count(*),coalesce(sum(committed_offset),0),coalesce(max(committed_offset),0) FROM journal_sources").Scan(&v.Files, &v.Bytes, &v.LastOffset); err != nil {
		v.State = "DB_ERROR"
	}
	return v
}
func null(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func (s *Store) String() string { return fmt.Sprintf("private SQLite schema %d", schemaVersion) }

func databaseURIPath(path string) string {
	v := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" {
		return "/" + v
	}
	return v
}
