package sidecar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Jakoczig/daedalus/go/internal/journal"
)

type Options struct{ MaxBytes int }
type Change struct {
	Kind      string
	Timestamp time.Time
	Valid     bool
	Raw       json.RawMessage
	State     string
}
type Reader struct {
	dir    string
	limit  int
	mu     sync.Mutex
	good   map[string]Change
	hashes map[string][32]byte
	states map[string]string
}

func NewReader(dir string, opts Options) (*Reader, error) {
	if dir != "" && !filepath.IsAbs(dir) {
		return nil, errors.New("sidecar directory must be absolute")
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 4 << 20
	}
	if opts.MaxBytes < 1 || opts.MaxBytes > 4<<20 {
		return nil, errors.New("invalid sidecar limit")
	}
	return &Reader{dir: dir, limit: opts.MaxBytes, good: map[string]Change{}, hashes: map[string][32]byte{}, states: map[string]string{}}, nil
}
func clone(c Change) Change { c.Raw = append(json.RawMessage(nil), c.Raw...); return c }
func (r *Reader) LastGood(kind string) Change {
	r.mu.Lock()
	defer r.mu.Unlock()
	return clone(r.good[kind])
}
func (r *Reader) Poll(ctx context.Context) ([]Change, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var changes []Change
	for _, kind := range []string{"Status", "Cargo", "Materials", "NavRoute"} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		change, hash := r.read(kind)
		if change.Valid && !change.Timestamp.IsZero() && change.Timestamp.Before(r.good[kind].Timestamp) {
			change.Valid = false
			change.State = "stale"
			change.Raw = nil
		}
		if r.states[kind] == change.State && r.hashes[kind] == hash {
			continue
		}
		r.states[kind] = change.State
		r.hashes[kind] = hash
		if change.Valid {
			r.good[kind] = clone(change)
		} else {
			change.Timestamp = r.good[kind].Timestamp
		}
		changes = append(changes, change)
	}
	return changes, nil
}
func (r *Reader) read(kind string) (Change, [32]byte) {
	c := Change{Kind: kind, State: "missing"}
	var hash [32]byte
	if r.dir == "" {
		return c, hash
	}
	path := filepath.Join(r.dir, kind+".json")
	before, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			c.State = "permission_error"
		}
		return c, hash
	}
	if !before.Mode().IsRegular() {
		c.State = "unsafe"
		return c, hash
	}
	if before.Size() > int64(r.limit) {
		c.State = "too_large"
		return c, hash
	}
	f, err := journal.OpenReadOnly(path)
	if err != nil {
		c.State = "permission_error"
		return c, hash
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, int64(r.limit)+1))
	after, statErr := f.Stat()
	f.Close()
	if readErr != nil || statErr != nil {
		c.State = "unreadable"
		return c, hash
	}
	hash = sha256.Sum256(raw)
	if len(raw) > r.limit {
		c.State = "too_large"
		return c, hash
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || !os.SameFile(after, current) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		c.State = "changing"
		return c, hash
	}
	trimmed := bytes.TrimSpace(raw)
	c.State = "invalid"
	if len(trimmed) == 0 || trimmed[0] != '{' || !utf8.Valid(raw) {
		return c, hash
	}
	var envelope struct {
		Timestamp string
		Event     string
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&envelope) != nil || decoder.Decode(new(any)) != io.EOF {
		return c, hash
	}
	if envelope.Event != "" && envelope.Event != kind {
		return c, hash
	}
	if envelope.Timestamp != "" {
		c.Timestamp, err = time.Parse(time.RFC3339Nano, envelope.Timestamp)
		if err != nil {
			return c, hash
		}
	}
	c.Valid = true
	c.State = "valid"
	c.Raw = append(json.RawMessage(nil), raw...)
	return c, hash
}
