package gamestate

import (
	"context"
	"github.com/Jakoczig/daedalus/go/internal/explorestate"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/navstate"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
	"reflect"
	"sync"
)

// Projections is an internal, immutable, coalesced notification. Transport sends
// only the separately typed allowlisted DTOs, never this service container.
type Projections struct {
	Source      Summary
	Navigation  navstate.NavigationView
	Exploration explorestate.ExplorationView
}
type projectionState struct {
	navigation  *navstate.Model
	exploration *explorestate.Model
	subscribers map[chan Projections]struct{}
	last        Projections
	hasLast     bool
	dirty       bool
}

func newProjectionState() projectionState {
	return projectionState{navigation: navstate.NewModel(), exploration: explorestate.NewModel(), subscribers: map[chan Projections]struct{}{}, dirty: true}
}
func (s *Service) projectionLocked() Projections {
	p := Projections{Source: s.summary, Navigation: s.views.navigation.View(s.active)}
	address := s.unknown.systemAddress
	if s.active != "" {
		address = s.commanders[s.active].systemAddress
	}
	p.Exploration = s.views.exploration.View(s.active, address)
	if s.summary.ImportInProgress {
		p.Navigation = navstate.NavigationView{RouteState: "replaying", TargetEvidence: "unknown", Hops: []navstate.Hop{}}
		p.Exploration = explorestate.ExplorationView{SurveyState: "replaying", Bodies: []explorestate.BodyView{}}
	} else if s.summary.SourceState != "live" && s.summary.SourceState != "replaying" && s.summary.SourceState != "partial" && s.summary.SourceState != "parse_warning" {
		if len(p.Navigation.Hops) > 0 || p.Navigation.TargetSystem != "" {
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
	s.views.navigation.ApplyEventPhase(s.active, e, phase)
	s.views.exploration.ApplyEvent(s.active, e)
	s.views.dirty = true
}
func (s *Service) sidecarProjectionLocked(c sidecar.Change) {
	if c.Kind == "NavRoute" {
		s.views.navigation.ApplySidecar(c)
		s.views.dirty = true
	}
}
func (s *Service) resetProjectionsLocked() {
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
	if s.views.hasLast && reflect.DeepEqual(p.Navigation, s.views.last.Navigation) && reflect.DeepEqual(p.Exploration, s.views.last.Exploration) && p.Source.SourceState == s.views.last.Source.SourceState && p.Source.ImportInProgress == s.views.last.Source.ImportInProgress {
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
		src := p.Exploration.Bodies[i]
		b := &q.Exploration.Bodies[i]
		b.Landable = scalarCopy(src.Landable)
		b.Terraformable = scalarCopy(src.Terraformable)
		b.GravityG = scalarCopy(src.GravityG)
		b.DistanceLS = scalarCopy(src.DistanceLS)
		b.BioSignals = scalarCopy(src.BioSignals)
		b.GeoSignals = scalarCopy(src.GeoSignals)
		b.PreviouslyDiscovered = scalarCopy(src.PreviouslyDiscovered)
		b.PreviouslyMapped = scalarCopy(src.PreviouslyMapped)
		b.Genuses = append([]string{}, src.Genuses...)
	}
	return q
}
