package journal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Phase string

const (
	Replay Phase = "replay"
	Live   Phase = "live"
)

type ReaderOptions struct {
	PollInterval   time.Duration
	MaxRecordBytes int
}
type ReaderStats struct {
	SourceState                                   string
	LastError                                     string
	Files                                         int
	EventsRead, MalformedLines, BytesRead, Resets uint64
	Pending, ImportInProgress                     bool
}
type cursor struct {
	info                os.FileInfo
	offset, replayUntil int64
	id                  string
	prefix, anchor      []byte
	discard, pending    bool
}
type Reader struct {
	dir     string
	opts    ReaderOptions
	mu      sync.RWMutex
	stats   ReaderStats
	cursors map[string]*cursor
	serial  uint64
	started bool
	running atomic.Bool
}

func NewReader(dir string, opts ReaderOptions) (*Reader, error) {
	if dir != "" && !filepath.IsAbs(dir) {
		return nil, errors.New("journal directory must be absolute")
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 500 * time.Millisecond
	}
	if opts.MaxRecordBytes == 0 {
		opts.MaxRecordBytes = MaxRecordBytes
	}
	if opts.MaxRecordBytes < 1 || opts.MaxRecordBytes > MaxRecordBytes {
		return nil, errors.New("invalid journal record limit")
	}
	return &Reader{dir: dir, opts: opts, cursors: map[string]*cursor{}, stats: ReaderStats{SourceState: "waiting", ImportInProgress: true}}, nil
}
func (r *Reader) Stats() ReaderStats          { r.mu.RLock(); defer r.mu.RUnlock(); return r.stats }
func (r *Reader) change(f func(*ReaderStats)) { r.mu.Lock(); defer r.mu.Unlock(); f(&r.stats) }
func (r *Reader) Run(ctx context.Context, emit func(Event, Phase) error) error {
	if !r.running.CompareAndSwap(false, true) {
		return errors.New("journal reader already running")
	}
	defer r.running.Store(false)
	ticker := time.NewTicker(r.opts.PollInterval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := r.poll(ctx, emit); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		r.started = true
		r.change(func(s *ReaderStats) { s.ImportInProgress = false })
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func journalLess(a, b string) bool {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	if x[1] != y[1] {
		return x[1] < y[1]
	}
	p, q := strings.TrimLeft(x[2], "0"), strings.TrimLeft(y[2], "0")
	if len(p) != len(q) {
		return len(p) < len(q)
	}
	if p != q {
		return p < q
	}
	return a < b
}
func (r *Reader) poll(ctx context.Context, emit func(Event, Phase) error) error {
	if r.dir == "" {
		return nil
	}
	f, err := os.Open(r.dir)
	if err != nil {
		r.sourceError(err)
		return nil
	}
	entries, err := f.ReadDir(4097)
	f.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		r.sourceError(err)
		return nil
	}
	if len(entries) > 4096 {
		r.change(func(s *ReaderStats) { s.SourceState = "source_error"; s.LastError = "journal file count exceeds limit" })
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && journalName.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return journalLess(names[i], names[j]) })
	r.change(func(s *ReaderStats) {
		s.Files = len(names)
		s.Pending = false
		s.LastError = ""
		if len(names) == 0 {
			s.SourceState = "waiting"
		} else {
			s.SourceState = "live"
		}
		if !r.started && len(names) > 0 {
			s.SourceState = "replaying"
		}
	})
	current := map[string]bool{}
	for _, name := range names {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		current[name] = true
		if err := r.readFile(ctx, name, emit); err != nil {
			return err
		}
	}
	for name := range r.cursors {
		if !current[name] {
			delete(r.cursors, name)
		}
	}
	return nil
}
func (r *Reader) sourceError(err error) {
	r.change(func(s *ReaderStats) {
		s.Pending = false
		s.Files = 0
		s.SourceState = "source_error"
		s.LastError = "journal source unavailable"
		if errors.Is(err, os.ErrNotExist) {
			s.SourceState = "waiting"
			s.LastError = ""
		}
		if errors.Is(err, os.ErrPermission) {
			s.SourceState = "permission_error"
			s.LastError = "journal source unreadable"
		}
	})
}
func sample(f *os.File, offset int64, size int) []byte {
	if offset < 0 {
		return nil
	}
	b := make([]byte, size)
	n, _ := f.ReadAt(b, offset)
	return b[:n]
}
func (r *Reader) readFile(ctx context.Context, name string, emit func(Event, Phase) error) error {
	path := filepath.Join(r.dir, name)
	info, err := os.Lstat(path)
	if err != nil {
		r.sourceError(err)
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	c := r.cursors[name]
	if c != nil && os.SameFile(c.info, info) && c.info.Size() == info.Size() && c.info.ModTime() == info.ModTime() {
		if c.pending {
			r.change(func(s *ReaderStats) { s.Pending = true })
		}
		return nil
	}
	f, err := OpenReadOnly(path)
	if err != nil {
		r.sourceError(err)
		return nil
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !actual.Mode().IsRegular() {
		return nil
	}
	info = actual
	prefix := sample(f, 0, int(min(info.Size(), 128)))
	reset := c == nil
	if c != nil {
		reset = !os.SameFile(c.info, info) || info.Size() < c.offset || len(prefix) < len(c.prefix) || !bytes.Equal(prefix[:min(len(prefix), len(c.prefix))], c.prefix)
		if !reset && c.offset > 0 {
			anchor := sample(f, c.offset-int64(len(c.anchor)), len(c.anchor))
			reset = !bytes.Equal(anchor, c.anchor)
		}
	}
	if reset {
		r.serial++
		c = &cursor{id: strconv.FormatUint(r.serial, 10)}
		if !r.started {
			c.replayUntil = info.Size()
		}
		if r.cursors[name] != nil {
			r.change(func(s *ReaderStats) { s.Resets++ })
		}
		r.cursors[name] = c
	}
	c.prefix = prefix
	c.pending = false
	if _, err = f.Seek(c.offset, io.SeekStart); err != nil {
		r.sourceError(err)
		return nil
	}
	reader := bufio.NewReaderSize(io.LimitReader(f, max(0, info.Size()-c.offset)), 64<<10)
	position := c.offset
	start := c.offset
	var line []byte
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fragment, readErr := reader.ReadSlice('\n')
		position += int64(len(fragment))
		r.change(func(s *ReaderStats) { s.BytesRead += uint64(len(fragment)) })
		if !c.discard {
			if len(line)+len(fragment) > r.opts.MaxRecordBytes+1 {
				c.discard = true
				line = nil
				r.change(func(s *ReaderStats) { s.MalformedLines++ })
			} else {
				line = append(line, fragment...)
			}
		}
		complete := len(fragment) > 0 && fragment[len(fragment)-1] == '\n'
		if complete {
			if !c.discard {
				e, eventErr := ParseLine(bytes.TrimSuffix(line, []byte("\n")), SourcePosition{Path: path, FileID: c.id, Offset: start})
				if eventErr != nil {
					r.change(func(s *ReaderStats) { s.MalformedLines++ })
				} else {
					phase := Live
					if position <= c.replayUntil {
						phase = Replay
					}
					if err := emit(e, phase); err != nil {
						return err
					}
					r.change(func(s *ReaderStats) { s.EventsRead++ })
				}
			}
			c.offset = position
			start = position
			c.discard = false
			line = nil
		} else if c.discard {
			c.offset = position
		}
		if readErr != nil {
			if errors.Is(readErr, bufio.ErrBufferFull) {
				continue
			}
			if !errors.Is(readErr, io.EOF) {
				r.sourceError(readErr)
			}
			c.pending = position > start || c.discard
			if c.pending {
				r.change(func(s *ReaderStats) { s.Pending = true })
			}
			break
		}
	}
	c.info = info
	c.anchor = sample(f, max(0, c.offset-128), int(min(c.offset, 128)))
	return nil
}
