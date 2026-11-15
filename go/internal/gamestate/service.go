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

	"github.com/Jakoczig/daedalus/go/internal/history"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/navcontext"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
)

// Summary is the entire public transport projection. Never add paths, identity,
// raw records, inventories or free-form diagnostic messages to this type.
type Summary struct {
	Stage            string `json:"stage,omitempty"`
	Cached           bool   `json:"cached,omitempty"`
	CachedAt         string `json:"cachedAt,omitempty"`
	SourceState      string `json:"sourceState"`
	CurrentSystem    string `json:"currentSystem"`
	LastEventType    string `json:"lastEventType"`
	LastEventAt      string `json:"lastEventAt"`
	ImportInProgress bool   `json:"importInProgress"`
	EventsRead       uint64 `json:"eventsRead"`
}
type Options struct {
	ProviderSidecar func(sidecar.Change)
	OnHistoryReady  func(*history.Store, history.ProviderSeed)
	StorageDir      string
	ProviderRestore func(string, string)
	ProviderEvent   func(journal.Event, journal.Phase)
	EnableParity    bool
	Directory       string
	Discovery       journal.DiscoveryOptions
	PollInterval    time.Duration
	ConfigError     string
}
type SourceDetails struct {
	ReaderBytes                uint64
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
type maintenanceRequest struct {
	kind, source string
	reply        chan error
}
type Service struct {
	maintenance        chan maintenanceRequest
	hasFreshLive       bool
	activeSource       string
	store              *history.Store
	databaseError      string
	lastPersistPublish time.Time
	restored           bool
	cachedProjection   *Projections
	navigationContext  *navcontext.Model
	mu                 sync.Mutex
	views              projectionState
	opts               Options
	summary            Summary
	details            SourceDetails
	commanders         map[string]commanderState
	order              []string
	unknown            commanderState
	active             string
	stopped            bool
	subscribers        map[chan Summary]struct{}
	selectSource       chan selection
	done               chan struct{}
	closed             bool
	running            atomic.Bool
}

func NewService(opts Options) (*Service, error) {
	if opts.PollInterval <= 0 {
		opts.PollInterval = 500 * time.Millisecond
	}
	initialStage := ""
	if opts.StorageDir != "" {
		initialStage = "RESTORING_CACHE"
	}
	return &Service{maintenance: make(chan maintenanceRequest), navigationContext: navcontext.New(), views: newProjectionState(), opts: opts, summary: Summary{Stage: initialStage, SourceState: "waiting", ImportInProgress: true}, details: SourceDetails{Override: opts.Directory, Sidecars: map[string]string{}}, commanders: map[string]commanderState{}, subscribers: map[chan Summary]struct{}{}, selectSource: make(chan selection), done: make(chan struct{})}, nil
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
	defer func() {
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
		if s.store != nil {
			_ = s.store.Close()
		}
	}()
	defer stopReader()

	if s.opts.StorageDir != "" {
		db, err := history.Open(s.opts.StorageDir)
		if err == nil {
			s.mu.Lock()
			s.store = db
			s.summary.Stage = "RESTORING_CACHE"
			s.mu.Unlock()
			raw, e := db.Cache()
			if e == nil && len(raw) > 0 {
				s.mu.Lock()
				e = s.restoreCacheLocked(raw)
				if e == nil {
					s.restored = true
					s.summary.Cached = true
					s.summary.Stage = "CATCHING_UP"
					savedSummary := s.summary
					s.summary.ImportInProgress = false
					s.summary.SourceState = "live"
					v := s.projectionLocked()
					s.summary = savedSummary
					v.Source = savedSummary
					s.cachedProjection = &v
				}
				s.mu.Unlock()
			}
			err = e
		}
		if err != nil {

			s.mu.Lock()
			s.databaseError = "Database could not be verified. Original files retained; use Data/History recovery."
			s.details.Error = s.databaseError
			s.summary.Stage = "DB_ERROR"
			s.summary.SourceState = "db_error"
			s.summary.ImportInProgress = false
			s.publishLocked()
			s.mu.Unlock()
			for err != nil {
				select {
				case request := <-s.maintenance:
					if s.store == nil {
						request.reply <- errors.New("A verified database is required; restore a backup and restart")
						continue
					}
					err = s.rebuildDerived(ctx)
					request.reply <- err
				case <-ctx.Done():
					return nil
				case request := <-s.selectSource:
					request.reply <- errors.New("database recovery required")
				}
			}
		}
		if s.opts.OnHistoryReady != nil {
			seed, e := db.ProviderSeed(s.activeSource)
			if e == nil {
				s.opts.OnHistoryReady(db, seed)
			}
		}
	}

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
			if s.store == nil {
				s.resetProjectionsLocked()
				s.commanders = map[string]commanderState{}
				s.order = nil
				s.unknown = commanderState{}
				s.active = ""
				s.stopped = false
				s.summary = Summary{SourceState: "waiting", ImportInProgress: directory != ""}
			} else {
				s.summary.ImportInProgress = directory != ""
				s.summary.Cached = s.restored
				if s.restored {
					s.summary.Stage = "CATCHING_UP"
				} else {
					s.summary.Stage = "INITIAL_IMPORT"
				}
			}
			s.details.Directory = directory
			s.details.Override = opts.Directory
			s.details.Sidecars = map[string]string{}
			s.publishLocked()
			s.mu.Unlock()
			if directory != "" && opts.ConfigError == "" {
				var durable journal.Durable
				if s.store != nil {
					durable = s
				}
				reader, _ = journal.NewReader(directory, journal.ReaderOptions{PollInterval: opts.PollInterval, Durable: durable})
				snapshots, _ = sidecar.NewReader(directory, sidecar.Options{StationData: s.store != nil})
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
					go func() {
						defer close(completed)
						if err := activeReader.Run(readerCtx, s.apply); err != nil {
							s.mu.Lock()
							s.databaseError = "Source/database commit failed; committed history retained. Review Data/History."
							s.summary.Stage = "DB_ERROR"
							s.summary.Cached = true
							s.publishLocked()
							s.mu.Unlock()
						}
					}()
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
		case request := <-s.maintenance:
			if s.store == nil {
				request.reply <- errors.New("Database unavailable")
				continue
			}
			stopReader()
			var err error
			if request.kind == "reimport" {
				err = s.store.ReimportSource(request.source)
			}
			if err == nil {
				err = s.rebuildDerived(ctx)
			}
			if err == nil {
				configure(true)
			}
			request.reply <- err
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
					if s.opts.ProviderSidecar != nil {
						s.opts.ProviderSidecar(change)
					}
				}
				s.mu.Unlock()
			}
			s.mu.Lock()
			before := s.summary
			if reader != nil {
				stats := reader.Stats()
				s.details.ReaderBytes = stats.BytesRead
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
				if s.databaseError != "" {
					s.summary.Stage = "DB_ERROR"
					s.details.Error = s.databaseError
				} else {
					s.details.Error = stats.LastError
				}
			} else {
				s.summary.ImportInProgress = false
			}
			if s.store != nil && s.databaseError == "" {
				if s.summary.ImportInProgress {
					if s.restored {
						s.summary.Stage = "CATCHING_UP"
					} else {
						s.summary.Stage = "INITIAL_IMPORT"
					}
				} else if s.summary.SourceState == "source_error" || s.summary.SourceState == "permission_error" {
					s.summary.Stage = "SOURCE_UNAVAILABLE"
				} else if s.summary.Cached || s.stopped || reader == nil || reader.Stats().SourceState == "waiting" {
					s.summary.Stage = "WAITING_FOR_GAME"
				} else {
					s.summary.Stage = "LIVE"
				}
				if !s.summary.ImportInProgress {
					s.cachedProjection = nil
				}
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
	s.applyEventLocked(event, phase)
	if s.opts.ProviderEvent != nil {
		s.opts.ProviderEvent(event, phase)
	}
	if phase == journal.Live {
		s.publishLocked()
	}
	return nil
}
func (s *Service) applyEventLocked(event journal.Event, phase journal.Phase) {
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
}

// SystemIdentity is used only by the fixed-host opt-in community lookup.
// It never exposes commander identity and is unavailable during replay/gaps.
func (s *Service) SystemIdentity() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.summary.ImportInProgress || s.summary.SourceState != "live" || s.active == "" {
		return "", ""
	}
	c := s.commanders[s.active]
	v := s.views.exploration.View(s.active, c.systemAddress)
	if v.SystemName != c.system || (v.SurveyState == "unverified" || v.SurveyState == "in_transit_last_known") {
		return "", ""
	}
	return c.system, c.systemAddress
}
