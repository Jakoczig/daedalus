package gamestate

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
)

// Summary is the entire public transport projection. Never add paths, identity,
// raw records, inventories or free-form diagnostic messages to this type.
type Summary struct {
	SourceState      string `json:"sourceState"`
	CurrentSystem    string `json:"currentSystem"`
	LastEventType    string `json:"lastEventType"`
	LastEventAt      string `json:"lastEventAt"`
	ImportInProgress bool   `json:"importInProgress"`
	EventsRead       uint64 `json:"eventsRead"`
}
type Options struct {
	Directory    string
	Discovery    journal.DiscoveryOptions
	PollInterval time.Duration
	ConfigError  string
}
type SourceDetails struct {
	Directory, Override, Error string
	ReaderStarts               uint64
	ActiveReaders              int
	Sidecars                   map[string]string
}
type commanderState struct{ system, systemAddress, marketID string }
type selection struct {
	directory string
	reply     chan error
}
type Service struct {
	mu           sync.Mutex
	views        projectionState
	opts         Options
	summary      Summary
	details      SourceDetails
	commanders   map[string]commanderState
	order        []string
	unknown      commanderState
	active       string
	stopped      bool
	subscribers  map[chan Summary]struct{}
	selectSource chan selection
	done         chan struct{}
	closed       bool
	running      atomic.Bool
}

func NewService(opts Options) (*Service, error) {
	if opts.PollInterval <= 0 {
		opts.PollInterval = 500 * time.Millisecond
	}
	return &Service{views: newProjectionState(), opts: opts, summary: Summary{SourceState: "waiting", ImportInProgress: true}, details: SourceDetails{Override: opts.Directory, Sidecars: map[string]string{}}, commanders: map[string]commanderState{}, subscribers: map[chan Summary]struct{}{}, selectSource: make(chan selection), done: make(chan struct{})}, nil
}
func (s *Service) Snapshot() Summary { s.mu.Lock(); defer s.mu.Unlock(); return s.summary }
func (s *Service) Details() SourceDetails {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.details
	d.Sidecars = map[string]string{}
	for k, v := range s.details.Sidecars {
		d.Sidecars[k] = v
	}
	return d
}
func (s *Service) publishLocked() {
	s.publishProjectionsLocked()
	for ch := range s.subscribers {
		select {
		case ch <- s.summary:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- s.summary:
			default:
			}
		}
	}
}
func (s *Service) Subscribe(ctx context.Context) (<-chan Summary, func()) {
	ch := make(chan Summary, 1)
	s.mu.Lock()
	if s.closed || len(s.subscribers) >= 128 {
		s.mu.Unlock()
		close(ch)
		return ch, func() {}
	}
	s.subscribers[ch] = struct{}{}
	ch <- s.summary
	s.mu.Unlock()
	var once sync.Once
	finished := make(chan struct{})
	stop := func() {
		once.Do(func() {
			s.mu.Lock()
			if _, ok := s.subscribers[ch]; ok {
				delete(s.subscribers, ch)
				close(ch)
			}
			s.mu.Unlock()
			close(finished)
		})
	}
	go func() {
		select {
		case <-ctx.Done():
			stop()
		case <-finished:
		case <-s.done:
			stop()
		}
	}()
	return ch, stop
}
func (s *Service) SetDirectory(ctx context.Context, dir string) error {
	if dir != "" {
		if err := journal.ValidateDir(dir); err != nil {
			return err
		}
	}
	cmd := selection{directory: dir, reply: make(chan error, 1)}
	select {
	case s.selectSource <- cmd:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return errors.New("journal service stopped")
	}
	select {
	case err := <-cmd.reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return errors.New("journal service stopped")
	}
}
func (s *Service) Run(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("journal service already running")
	}
	var reader *journal.Reader
	var snapshots *sidecar.Reader
	var stop context.CancelFunc
	var finished chan struct{}
	stopReader := func() {
		if stop != nil {
			stop()
			<-finished
			stop = nil
		}
		s.mu.Lock()
		s.details.ActiveReaders = 0
		s.mu.Unlock()
	}
	defer func() {
		stopReader()
		s.mu.Lock()
		s.closed = true
		s.closeProjectionsLocked()
		s.summary.SourceState = "stopped"
		for ch := range s.subscribers {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
		close(s.done)
	}()
	opts := s.opts
	directory := ""
	found := false
	configure := func(reset bool) {
		discovery := opts.Discovery
		discovery.Override = opts.Directory
		result, err := journal.Discover(ctx, discovery)
		if opts.ConfigError != "" {
			result = journal.DiscoveryResult{}
			err = errors.New(opts.ConfigError)
		}
		found = result.Found
		if result.Directory != directory || reset {
			stopReader()
			directory = result.Directory
			reader = nil
			snapshots = nil
			s.mu.Lock()
			s.resetProjectionsLocked()
			s.commanders = map[string]commanderState{}
			s.order = nil
			s.unknown = commanderState{}
			s.active = ""
			s.stopped = false
			s.summary = Summary{SourceState: "waiting", ImportInProgress: directory != ""}
			s.details.Directory = directory
			s.details.Override = opts.Directory
			s.details.Sidecars = map[string]string{}
			s.publishLocked()
			s.mu.Unlock()
			if directory != "" && opts.ConfigError == "" {
				reader, _ = journal.NewReader(directory, journal.ReaderOptions{PollInterval: opts.PollInterval})
				snapshots, _ = sidecar.NewReader(directory, sidecar.Options{})
				if reader != nil {
					readerCtx, cancel := context.WithCancel(ctx)
					stop = cancel
					finished = make(chan struct{})
					activeReader := reader
					completed := finished
					s.mu.Lock()
					s.details.ReaderStarts++
					s.details.ActiveReaders = 1
					s.mu.Unlock()
					go func() { defer close(completed); _ = activeReader.Run(readerCtx, s.apply) }()
				}
			}
		}
		s.mu.Lock()
		s.details.Error = ""
		if err != nil {
			_, statErr := os.Stat(result.Directory)
			if opts.ConfigError != "" || !os.IsNotExist(statErr) {
				s.details.Error = err.Error()
				s.summary.SourceState = "source_error"
				s.summary.ImportInProgress = false
			}
		}
		s.publishLocked()
		s.mu.Unlock()
	}
	configure(true)
	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case cmd := <-s.selectSource:
			opts.Directory = cmd.directory
			opts.ConfigError = ""
			configure(true)
			cmd.reply <- nil
		case <-ticker.C:
			if opts.ConfigError == "" && (!found || reader == nil || reader.Stats().SourceState == "waiting") {
				configure(false)
			}
			if snapshots != nil {
				changes, _ := snapshots.Poll(ctx)
				s.mu.Lock()
				for _, change := range changes {
					s.details.Sidecars[change.Kind] = change.State
					s.sidecarProjectionLocked(change)
				}
				s.mu.Unlock()
			}
			s.mu.Lock()
			before := s.summary
			if reader != nil {
				stats := reader.Stats()
				s.summary.SourceState = stats.SourceState
				s.summary.ImportInProgress = stats.ImportInProgress
				if stats.SourceState == "live" || stats.SourceState == "replaying" {
					if stats.Pending {
						s.summary.SourceState = "partial"
					} else if stats.MalformedLines > 0 {
						s.summary.SourceState = "parse_warning"
					} else if s.stopped {
						s.summary.SourceState = "waiting_game"
					}
				}
				s.details.Error = stats.LastError
			} else {
				s.summary.ImportInProgress = false
			}
			s.publishProjectionsLocked()
			if s.summary != before {
				s.publishLocked()
			}
			s.mu.Unlock()
		}
	}
}
func safeText(text string) bool {
	if text == "" || utf8.RuneCountInString(text) > 128 {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (s *Service) apply(event journal.Event, phase journal.Phase) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary.EventsRead++
	switch event.Type {
	case "Fileheader":
		s.active = ""
		s.unknown = commanderState{}
		s.stopped = false
	case "LoadGame":
		s.active = ""
		s.unknown = commanderState{}
		s.stopped = false
		if safeText(event.FID) {
			s.active = event.FID
			if _, ok := s.commanders[s.active]; !ok {
				if len(s.order) >= 64 {
					delete(s.commanders, s.order[0])
					s.order = s.order[1:]
				}
				s.order = append(s.order, s.active)
				s.commanders[s.active] = commanderState{}
			}
		}
	case "Location", "FSDJump", "CarrierJump":
		name := strings.TrimSpace(event.SystemName)
		if safeText(name) {
			state := commanderState{system: name, systemAddress: event.SystemAddress, marketID: event.MarketID}
			if s.active == "" {
				s.unknown = state
			} else {
				s.commanders[s.active] = state
			}
			s.stopped = false
		}
	case "Shutdown":
		s.stopped = true
	}
	s.applyProjectionsLocked(event, phase)
	state := s.unknown
	if s.active != "" {
		state = s.commanders[s.active]
	}
	s.summary.CurrentSystem = state.system
	switch event.Type {
	case "Fileheader", "LoadGame", "Location", "FSDJump", "CarrierJump", "StartJump", "Shutdown":
		s.summary.LastEventType = event.Type
		s.summary.LastEventAt = ""
		if !event.Timestamp.IsZero() {
			s.summary.LastEventAt = event.Timestamp.UTC().Format(time.RFC3339Nano)
		}
	}
	if phase == journal.Live {
		s.publishLocked()
	}
	return nil
}
