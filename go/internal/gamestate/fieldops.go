package gamestate

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/fieldops"
	"github.com/Jakoczig/eridion/go/internal/history"
	"time"
)

func (s *Service) FieldCommander() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.displayFIDLocked()
}
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
	out.Progress, e = s.store.ItineraryProgress(s.displayFIDLocked())
	if e != nil {
		return out, e
	}
	out.Total, e = s.store.ItinerarySize(out.Progress.Plan.ID)
	if e != nil {
		return out, e
	}
	out.Current = s.selectedSummaryLocked().CurrentSystem
	out.Target = s.projectionLocked().Navigation.TargetSystem
	out.Evidence, e = s.store.FieldNavigationEvidence(s.displayFIDLocked())
	if e != nil {
		return out, e
	}
	if owner {
		out.Plans, e = s.store.Itineraries(s.displayFIDLocked())
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
	state := s.commanders[s.displayFIDLocked()]
	survey := s.views.exploration.View(s.displayFIDLocked(), state.systemAddress)
	if state.systemAddress == "" {
		return out, nil
	}
	for _, b := range survey.Bodies {
		if b.BodyID != body {
			continue
		}
		v, err := s.store.BioBody(context.Background(), s.displayFIDLocked(), state.systemAddress, body)
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
		samples := v["samples"].([]history.BioSample)
		if s.displayFIDLocked() == s.active && !s.stopped && s.summary.SourceState == "live" && len(samples) > 0 && (samples[0].Stage == "Log" || samples[0].Stage == "Sample") && samples[0].Session != "" && samples[0].Session == s.store.BioSession(s.displayFIDLocked(), s.activeSource) && samples[0].Event == s.store.ActiveOrganicEvent(s.displayFIDLocked(), s.activeSource) {
			if current, ok := fieldops.StatusPosition(s.bioStatus, b.Name, time.Now(), time.Now()); ok {
				points := []fieldops.Position{}
				complete := false
				expected := 0
				for _, sample := range samples {
					if sample.Session != samples[0].Session || sample.Genus != samples[0].Genus || sample.Species != samples[0].Species || sample.Variant != samples[0].Variant {
						continue
					}
					if sample.Stage == "Analyse" {
						break
					}
					expected++
					if sample.Position != nil {
						points = append(points, *sample.Position)
					}
					if sample.Stage == "Log" {
						complete = len(points) > 0 && len(points) == expected
						break
					}
				}
				v["liveSpacing"] = fieldops.LiveSpacing(current, points, fieldops.MinimumDistance(samples[0].Genus), complete)
			}
		}
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
