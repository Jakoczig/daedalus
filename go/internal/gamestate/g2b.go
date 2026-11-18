package gamestate

import (
	"context"
	"github.com/Jakoczig/eridion/go/internal/activitystate"
	"github.com/Jakoczig/eridion/go/internal/explorestate"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/navcontext"
	"github.com/Jakoczig/eridion/go/internal/navstate"
	"github.com/Jakoczig/eridion/go/internal/paritystate"
	"github.com/Jakoczig/eridion/go/internal/shipstate"
	"github.com/Jakoczig/eridion/go/internal/sidecar"
	"reflect"
	"sync"
)

// Projections is an internal, immutable, coalesced notification. Transport sends
// only the separately typed allowlisted DTOs, never this service container.
type Projections struct {
	Workspace   *WorkspaceView
	Ship        shipstate.View
	Source      Summary
	Navigation  navstate.NavigationView
	Exploration explorestate.ExplorationView
}
type WorkspaceView struct {
	Parity   paritystate.View   `json:"parity"`
	Activity activitystate.View `json:"activity"`
}

type projectionState struct {
	parity      *paritystate.Model
	activity    *activitystate.Model
	ship        *shipstate.Model
	navigation  *navstate.Model
	exploration *explorestate.Model
	subscribers map[chan Projections]struct{}
	last        Projections
	hasLast     bool
	dirty       bool
}

func newProjectionState() projectionState {
	return projectionState{parity: paritystate.NewModel(), activity: activitystate.NewModel(), ship: shipstate.NewModel(), navigation: navstate.NewModel(), exploration: explorestate.NewModel(), subscribers: map[chan Projections]struct{}{}, dirty: true}
}
func (s *Service) projectionLocked() Projections {
	fid := s.displayFIDLocked()
	if s.cachedProjection != nil && s.summary.ImportInProgress && fid == s.active {
		p := cloneProjection(*s.cachedProjection)
		p.Source = s.selectedSummaryLocked()
		p.Ship.Evidence = "cached"
		p.Navigation.RouteState = "cached"
		p.Exploration.SurveyState = "cached"
		if p.Workspace != nil {
			p.Workspace.Parity.Evidence = "cached"
			p.Workspace.Parity.Status.State = "stale_source"
		}
		return p
	}

	p := Projections{Ship: s.views.ship.View(fid), Source: s.selectedSummaryLocked(), Navigation: s.views.navigation.View(fid)}
	address := s.unknown.systemAddress
	if fid != "" {
		address = s.commanders[fid].systemAddress
	}
	p.Exploration = s.views.exploration.View(fid, address)
	if s.opts.EnableParity {
		w := WorkspaceView{Parity: s.views.parity.View(fid), Activity: s.views.activity.View(fid)}
		if s.summary.ImportInProgress {
			w = WorkspaceView{Parity: paritystate.View{Evidence: "replaying"}, Activity: activitystate.View{Evidence: "replaying"}}
		} else if s.summary.SourceState != "live" && s.summary.SourceState != "partial" && s.summary.SourceState != "parse_warning" {
			w.Parity.Evidence = "stale_source"
			w.Parity.Status = paritystate.StatusView{State: "stale_source"}
			w.Activity.Evidence = "stale_source"
		}
		p.Workspace = &w
	}
	if s.summary.ImportInProgress {
		p.Ship = shipstate.View{Evidence: "replaying"}
		p.Navigation = navstate.NavigationView{RouteState: "replaying", TargetEvidence: "unknown", Hops: []navstate.Hop{}}
		p.Exploration = explorestate.ExplorationView{SurveyState: "replaying", Bodies: []explorestate.BodyView{}}
	} else if s.summary.SourceState != "live" && s.summary.SourceState != "replaying" && s.summary.SourceState != "partial" && s.summary.SourceState != "parse_warning" {
		p.Ship.Evidence = "stale_source"
		if !s.summary.Cached && (len(p.Navigation.Hops) > 0 || p.Navigation.TargetSystem != "") {
			p.Navigation.Hops = []navstate.Hop{}
			p.Navigation.DestinationSystem = ""
			p.Navigation.TargetSystem = ""
			p.Navigation.RemainingJumps = nil
			p.Navigation.ReportedRemainingJumps = nil
			p.Navigation.RouteState = "stale"
			p.Navigation.TargetEvidence = "unknown"
		}
		if len(p.Exploration.Bodies) > 0 {
			p.Exploration.SurveyState = "stale_source"
		}
	}
	if fid != s.active && p.Workspace != nil {
		p.Workspace.Parity.Status = paritystate.StatusView{State: "different_account"}
	}
	if s.summary.Cached {
		if p.Navigation.RouteState == "verified_last_known" || p.Navigation.RouteState == "verified" {
			p.Navigation.RouteState = "cached"
			p.Navigation.TargetEvidence = "cached"
		}
		p.Ship.Evidence = "cached"
		if p.Workspace != nil {
			p.Workspace.Parity.Evidence = "cached"
			p.Workspace.Activity.Evidence = "cached"
		}
		if p.Exploration.SystemName != "" {
			p.Exploration.SurveyState = "cached"
		}
	}
	return p
}
func (s *Service) Navigation() navstate.NavigationView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectionLocked().Navigation
}
func (s *Service) Exploration() explorestate.ExplorationView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectionLocked().Exploration
}
func (s *Service) applyProjectionsLocked(e journal.Event, phase journal.Phase) {
	s.views.ship.ApplyEvent(s.active, e)
	if s.opts.EnableParity {
		s.navigationContext.Apply(s.active, e, phase)
	}
	if s.opts.EnableParity {
		s.views.parity.ApplyEvent(s.active, e)
		s.views.activity.ApplyEvent(s.active, e)
	}
	s.views.navigation.ApplyEventPhase(s.active, e, phase)
	s.views.exploration.ApplyEvent(s.active, e)
	s.views.dirty = true
}
func (s *Service) sidecarProjectionLocked(c sidecar.Change) {
	if c.Kind == "Status" {
		s.bioStatus = nil
		if c.Valid && !s.summary.ImportInProgress && s.summary.SourceState == "live" && !s.stopped {
			s.bioStatus = append([]byte(nil), c.Raw...)
			s.captureBioLocked()
		}
	}
	if s.opts.EnableParity {
		s.views.parity.ApplySidecar(s.active, c)
		if c.Kind == "Status" && !s.summary.ImportInProgress && s.summary.SourceState == "live" {
			s.navigationContext.Status(s.active, s.views.parity.View(s.active).Status)
		}
		s.views.dirty = true
	}
	if c.Kind == "NavRoute" {
		s.views.navigation.ApplySidecar(c)
		s.views.dirty = true
	}
}
func (s *Service) resetProjectionsLocked() {
	s.views.ship.ResetSource()
	s.navigationContext = navcontext.New()
	s.views.parity.ResetSource()
	s.views.activity.ResetSource()
	s.views.navigation.ResetSource()
	s.views.exploration.ResetSource()
	s.views.hasLast = false
	s.views.dirty = true
}
func (s *Service) publishProjectionsLocked() {
	if !s.views.dirty && s.views.hasLast && s.views.last.Source.SourceState == s.summary.SourceState && s.views.last.Source.ImportInProgress == s.summary.ImportInProgress {
		return
	}
	p := s.projectionLocked()
	s.views.dirty = false
	if s.views.hasLast && reflect.DeepEqual(p.Workspace, s.views.last.Workspace) && reflect.DeepEqual(p.Ship, s.views.last.Ship) && reflect.DeepEqual(p.Navigation, s.views.last.Navigation) && reflect.DeepEqual(p.Exploration, s.views.last.Exploration) && p.Source.SourceState == s.views.last.Source.SourceState && p.Source.ImportInProgress == s.views.last.Source.ImportInProgress {
		return
	}
	s.views.last = p
	s.views.hasLast = true
	for ch := range s.views.subscribers {
		select {
		case ch <- cloneProjection(p):
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- cloneProjection(p):
			default:
			}
		}
	}
}
func (s *Service) SubscribeProjections(ctx context.Context) (<-chan Projections, func()) {
	ch := make(chan Projections, 1)
	s.mu.Lock()
	if s.closed || len(s.views.subscribers) >= 128 {
		s.mu.Unlock()
		close(ch)
		return ch, func() {}
	}
	s.views.subscribers[ch] = struct{}{}
	ch <- s.projectionLocked()
	s.mu.Unlock()
	var once sync.Once
	finished := make(chan struct{})
	stop := func() {
		once.Do(func() {
			s.mu.Lock()
			if _, ok := s.views.subscribers[ch]; ok {
				delete(s.views.subscribers, ch)
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
		case <-s.done:
			stop()
		case <-finished:
		}
	}()
	return ch, stop
}
func (s *Service) closeProjectionsLocked() {
	for ch := range s.views.subscribers {
		delete(s.views.subscribers, ch)
		close(ch)
	}
}

func scalarCopy[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func cloneProjection(p Projections) Projections {
	q := p
	if p.Workspace != nil {
		w := *p.Workspace
		w.Parity = paritystate.Clone(w.Parity)
		w.Activity = activitystate.Clone(w.Activity)
		q.Workspace = &w
	}
	q.Ship = shipstate.Clone(p.Ship)
	q.Navigation.RemainingJumps = scalarCopy(p.Navigation.RemainingJumps)
	q.Navigation.ReportedRemainingJumps = scalarCopy(p.Navigation.ReportedRemainingJumps)
	q.Navigation.Hops = append([]navstate.Hop{}, p.Navigation.Hops...)
	for i := range q.Navigation.Hops {
		q.Navigation.Hops[i].DistanceFromPreviousLY = scalarCopy(p.Navigation.Hops[i].DistanceFromPreviousLY)
	}
	q.Exploration.ReportedBodyCount = scalarCopy(p.Exploration.ReportedBodyCount)
	q.Exploration.ScannerBodyCount = scalarCopy(p.Exploration.ScannerBodyCount)
	q.Exploration.FSSProgress = scalarCopy(p.Exploration.FSSProgress)
	q.Exploration.Bodies = append([]explorestate.BodyView{}, p.Exploration.Bodies...)
	for i := range q.Exploration.Bodies {
		q.Exploration.Bodies[i] = explorestate.CloneBody(p.Exploration.Bodies[i])
	}
	return q
}

func (s *Service) Ship() shipstate.View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectionLocked().Ship
}

func (s *Service) Workspace() WorkspaceView {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.projectionLocked()
	if p.Workspace == nil {
		return WorkspaceView{}
	}
	return *p.Workspace
}
