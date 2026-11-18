package gamestate

import (
	"context"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"time"
)

func (s *Service) RebuildDerived(ctx context.Context, source string) error {
	kind := "rebuild"
	if source != "" {
		kind = "reimport"
	}
	request := maintenanceRequest{kind: kind, source: source, reply: make(chan error, 1)}
	select {
	case s.maintenance <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return errors.New("Journal service stopped")
	}
	select {
	case err := <-request.reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return errors.New("Journal service stopped")
	}
}
func (s *Service) rebuildDerived(ctx context.Context) error {
	s.mu.Lock()
	if s.store == nil {
		s.mu.Unlock()
		return errors.New("Database not ready")
	}
	before, err := s.cacheLocked()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	previous := s.projectionLocked()
	s.holdProjectionLocked(previous)
	s.resetProjectionsLocked()
	s.commanders = map[string]commanderState{}
	s.order = nil
	s.unknown = commanderState{}
	s.active = ""
	s.activeSource = ""
	s.stopped = false
	s.hasFreshLive = false
	s.summary = Summary{Stage: "CATCHING_UP", SourceState: "replaying", ImportInProgress: true, Cached: true}
	s.publishLocked()
	s.mu.Unlock()
	last := time.Now()
	err = s.store.Walk(ctx, func(e journal.Event) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.applyEventLocked(e, journal.Replay)
		s.activeSource = e.Position.FileID
		if time.Since(last) > 100*time.Millisecond {
			last = time.Now()
			s.publishLocked()
		}
		return ctx.Err()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.summary.Cached = true
		s.summary.CachedAt = time.Now().UTC().Format(time.RFC3339Nano)
		s.summary.ImportInProgress = false
		s.summary.Stage = "WAITING_FOR_GAME"
		s.summary.SourceState = "waiting_game"
		var raw []byte
		raw, err = s.cacheLocked()
		if err == nil {
			err = s.store.SaveCache(ctx, raw)
		}
	}
	if err != nil {
		_ = s.restoreCacheLocked(before)
		s.databaseError = "Derived rebuild failed; original events and committed cache retained"
		s.summary.Stage = "DB_ERROR"
	} else {
		s.databaseError = ""
		s.restored = true
		s.cachedProjection = nil
	}
	s.publishLocked()
	return err
}
