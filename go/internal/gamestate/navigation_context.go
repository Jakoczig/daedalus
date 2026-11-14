package gamestate

import (
	"github.com/Jakoczig/daedalus/go/internal/explorestate"
	"github.com/Jakoczig/daedalus/go/internal/navcontext"
	"strings"
	"time"
)

func (s *Service) NavigationContext() navcontext.View {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.navigationContext.View(s.active, time.Now())
	if s.summary.ImportInProgress || s.summary.SourceState != "live" {
		v.Fresh = false
		v.Mode = "idle"
	}
	seen := map[string]bool{}
	for _, p := range v.Points {
		seen[p.Address] = true
	}
	positions := s.views.navigation.VerifiedPositions(s.active)
	// Reserve the coordinate-verified destination before filling the bounded route context.
	for i := range positions {
		index := i - 1
		if i == 0 {
			index = len(positions) - 1
		}
		p := positions[index]
		if len(v.Points) >= 192 {
			break
		}
		if !seen[p.Address] {
			x := p.Coordinates
			v.Points = append(v.Points, navcontext.Point{Name: p.Name, Address: p.Address, Coordinates: &x, Source: "verified_route"})
			seen[p.Address] = true
		}
	}
	return v
}
func (s *Service) KnownSystem(name string) (navcontext.Point, explorestate.ExplorationView) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var p navcontext.Point
	for _, x := range s.navigationContext.View(s.active, time.Now()).Points {
		if strings.EqualFold(x.Name, name) {
			p = x
			break
		}
	}
	if p.Name == "" {
		for _, x := range s.views.navigation.VerifiedPositions(s.active) {
			if strings.EqualFold(x.Name, name) {
				coords := x.Coordinates
				p = navcontext.Point{Name: x.Name, Address: x.Address, Coordinates: &coords, Source: "verified_route"}
				break
			}
		}
	}
	if p.Name == "" {
		return p, explorestate.ExplorationView{}
	}
	return p, s.views.exploration.CachedSystem(s.active, p.Address)
}
