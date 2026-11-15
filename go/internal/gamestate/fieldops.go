package gamestate

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/fieldops"
	"github.com/Jakoczig/daedalus/go/internal/history"
	"time"
)

func (s *Service) FieldCommander() string { s.mu.Lock(); defer s.mu.Unlock(); return s.active }
func (s *Service) captureBioLocked() {
	if s.store == nil || len(s.bioStatus) == 0 || s.active == "" || s.stopped {
		return
	}
	if !s.store.BuiltinEnabled("bioscan") {
		return
	}
	var v struct{ BodyName string }
	if json.Unmarshal(s.bioStatus, &v) != nil {
		return
	}
	state := s.commanders[s.active]
	if state.systemAddress == "" {
		return
	}
	survey := s.views.exploration.View(s.active, state.systemAddress)
	for _, b := range survey.Bodies {
		if b.Name == v.BodyName && b.BodyID != "" {
			if err := s.store.RecordBioPosition(s.active, state.systemAddress, b.BodyID, b.Name, s.bioStatus, time.Now(), s.activeSource); err != nil {
				s.bioStatus = nil
			}
			return
		}
	}
}

type FieldRouteState struct {
	Progress        history.ItineraryProgress
	Enabled         bool
	Total           int
	Current, Target string
	Evidence        history.NavigationEvidence
	Plans           []history.Itinerary
}

func (s *Service) FieldProgress(owner bool) (FieldRouteState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out FieldRouteState
	if s.store == nil {
		return out, nil
	}
	out.Enabled = s.store.BuiltinEnabled("itinerary")
	var e error
	out.Progress, e = s.store.ItineraryProgress(s.active)
	if e != nil {
		return out, e
	}
	out.Total, e = s.store.ItinerarySize(out.Progress.Plan.ID)
	if e != nil {
		return out, e
	}
	out.Current = s.summary.CurrentSystem
	out.Target = s.projectionLocked().Navigation.TargetSystem
	out.Evidence, e = s.store.FieldNavigationEvidence(s.active)
	if e != nil {
		return out, e
	}
	if owner {
		out.Plans, e = s.store.Itineraries(s.active)
	}
	return out, e
}

// Private normalized scans and commander identity never cross the viewer boundary.
func (s *Service) FieldBio(body string, owner bool) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]any{"version": fieldops.BioRuleVersion(), "candidates": []fieldops.Candidate{}, "samples": []history.BioSample{}}
	if s.store == nil {
		return out, nil
	}
	if !s.store.BuiltinEnabled("bioscan") {
		out["notice"] = "BioScan module disabled; stored observations retained"
		return out, nil
	}
	state := s.commanders[s.active]
	survey := s.views.exploration.View(s.active, state.systemAddress)
	if state.systemAddress == "" {
		return out, nil
	}
	for _, b := range survey.Bodies {
		if b.BodyID != body {
			continue
		}
		v, err := s.store.BioBody(context.Background(), s.active, state.systemAddress, body)
		if err != nil {
			return out, err
		}
		genera := b.Genuses
		if len(genera) == 0 {
			genera, _ = v["genera"].([]string)
		}
		if b.BioSignals != nil && *b.BioSignals > 0 {
			scan, _ := v["environment"].(map[string]any)
			v["candidates"] = fieldops.Infer(scan, genera)
		}
		delete(v, "scan")
		delete(v, "environment")
		delete(v, "genera")
		v["confirmed"] = b.Organisms
		v["signals"] = b.BioSignals
		v["bodyName"] = b.Name
		if !owner {
			samples := v["samples"].([]history.BioSample)
			for i := range samples {
				samples[i].Position = nil
			}
			v["samples"] = samples
		}
		return v, nil
	}
	return out, nil
}
