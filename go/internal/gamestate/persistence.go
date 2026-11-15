package gamestate

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/activitystate"
	"github.com/Jakoczig/daedalus/go/internal/explorestate"
	"github.com/Jakoczig/daedalus/go/internal/history"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/navcontext"
	"github.com/Jakoczig/daedalus/go/internal/navstate"
	"github.com/Jakoczig/daedalus/go/internal/paritystate"
	"github.com/Jakoczig/daedalus/go/internal/shipstate"
	"time"
)

const reducerFormat = shipstate.CacheFormat + paritystate.CacheFormat + activitystate.CacheFormat + navstate.CacheFormat + navcontext.CacheFormat + explorestate.CacheFormat

type cacheCommander struct{ System, SystemAddress, MarketID string }
type reducerCache struct {
	ActiveSource string
	Format       string
	Summary      Summary
	Active       string
	Stopped      bool
	Order        []string
	Unknown      cacheCommander
	Commanders   map[string]cacheCommander
	Ship         *shipstate.Model
	Parity       *paritystate.Model
	Activity     *activitystate.Model
	Navigation   *navstate.Model
	Exploration  *explorestate.Model
	Context      *navcontext.Model
}

func cacheState(v commanderState) cacheCommander {
	return cacheCommander{v.system, v.systemAddress, v.marketID}
}
func uncached(v cacheCommander) commanderState {
	return commanderState{v.System, v.SystemAddress, v.MarketID}
}
func (s *Service) cacheLocked() ([]byte, error) {
	c := reducerCache{ActiveSource: s.activeSource, Format: reducerFormat, Summary: s.summary, Active: s.active, Stopped: s.stopped, Order: s.order, Unknown: cacheState(s.unknown), Commanders: map[string]cacheCommander{}, Ship: s.views.ship, Parity: s.views.parity, Activity: s.views.activity, Navigation: s.views.navigation, Exploration: s.views.exploration, Context: s.navigationContext}
	for k, v := range s.commanders {
		c.Commanders[k] = cacheState(v)
	}
	c.Summary.CachedAt = time.Now().UTC().Format(time.RFC3339Nano)
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(c)
	return b.Bytes(), err
}
func (s *Service) restoreCacheLocked(raw []byte) error {
	var c reducerCache
	if len(raw) > 32<<20 {
		return errors.New("Cache budget exceeded")
	}
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&c); err != nil {
		return err
	}
	if c.Format != reducerFormat || c.Ship == nil || c.Parity == nil || c.Activity == nil || c.Navigation == nil || c.Exploration == nil || c.Context == nil || len(c.Commanders) > 64 {
		return errors.New("Derived cache needs an explicit rebuild")
	}
	s.activeSource = c.ActiveSource
	s.summary = c.Summary
	s.active = c.Active
	s.stopped = c.Stopped
	s.order = c.Order
	s.unknown = uncached(c.Unknown)
	s.commanders = map[string]commanderState{}
	for k, v := range c.Commanders {
		s.commanders[k] = uncached(v)
	}
	s.views.ship = c.Ship
	s.views.parity = c.Parity
	s.views.activity = c.Activity
	s.views.navigation = c.Navigation
	c.Exploration.RestoreLinks()
	s.views.exploration = c.Exploration
	s.navigationContext = c.Context
	s.views.dirty = true
	return nil
}
func (s *Service) Resume(src journal.Source) (journal.Checkpoint, error) { return s.store.Resume(src) }
func (s *Service) RejectSource(src journal.Source) error                 { return s.store.Quarantine(src.ID) }
func (s *Service) CommitRecords(records []journal.Record, cp journal.Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, err := s.cacheLocked()
	if err != nil {
		return err
	}
	live := false
	applyRecords := records
	if s.activeSource != "" && s.activeSource != cp.Source.ID {
		applyRecords = nil
		for i, r := range records {
			if r.Event.Type == "LoadGame" && !r.Event.Timestamp.IsZero() {
				last, _ := time.Parse(time.RFC3339Nano, s.summary.LastEventAt)
				if !r.Event.Timestamp.Before(last) {
					applyRecords = records[i:]
					break
				}
			}
		}
	}
	if len(applyRecords) > 0 {
		s.activeSource = cp.Source.ID
	}
	for _, r := range applyRecords {
		s.applyEventLocked(r.Event, r.Phase)
		if r.Phase == journal.Live {
			live = true
			s.summary.Cached = false
			s.hasFreshLive = true
		}
	}
	if !s.hasFreshLive {
		s.summary.Cached = true
	}
	s.summary.CachedAt = time.Now().UTC().Format(time.RFC3339Nano)
	after, err := s.cacheLocked()
	if err == nil {
		err = s.store.Commit(context.Background(), cp, records, after)
	}
	if err != nil {
		_ = s.restoreCacheLocked(before)
		return err
	}
	if s.opts.ProviderEvent != nil {
		for _, r := range applyRecords {
			s.opts.ProviderEvent(r.Event, r.Phase)
		}
	}
	if live || time.Since(s.lastPersistPublish) > 100*time.Millisecond {
		s.lastPersistPublish = time.Now()
		s.publishLocked()
	}
	return nil
}
func (s *Service) History() (*history.Store, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store, s.databaseError
}
