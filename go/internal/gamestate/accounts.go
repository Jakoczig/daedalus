package gamestate

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/catalog"
	"github.com/Jakoczig/eridion/go/internal/history"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"time"
)

type AccountSettings struct {
	Multiple bool   `json:"multiple"`
	Selected string `json:"selected"`
}

func (s *Service) displayFIDLocked() string {
	if s.accounts.Multiple {
		return s.accounts.Selected
	}
	return s.active
}
func (s *Service) selectedSummaryLocked() Summary {
	v := s.summary
	if fid := s.displayFIDLocked(); fid != s.active {
		v.CurrentSystem = s.commanders[fid].system
		v.LastEventType = ""
		v.LastEventAt = ""
	}
	return v
}
func (s *Service) Accounts() AccountSettings { s.mu.Lock(); defer s.mu.Unlock(); return s.accounts }

// OwnerShipIdentity recovers witnessed renames from durable history even when an
// older reducer cache predates rename support. Exact commander/ship ID binding
// is mandatory; provider current-ship guesses are not used.
func (s *Service) OwnerShipIdentity() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	fid := s.displayFIDLocked()
	v := s.views.ship.View(fid)
	out := map[string]string{"shipName": v.ShipName, "shipType": catalog.ShipDisplayName(v.ShipType)}
	if s.summary.ImportInProgress {
		return map[string]string{"shipName": "", "shipType": ""}
	}
	if s.store != nil && fid != "" {
		var name, at string
		if e := s.store.WitnessedShipName(context.Background(), fid, s.views.ship.LastKnownIdentity(fid), &name, &at); e == nil && name != "" {
			observed, e := time.Parse(time.RFC3339Nano, at)
			loaded, _ := time.Parse(time.RFC3339Nano, v.LoadoutAt)
			if e == nil && !observed.Before(loaded) {
				out["shipName"] = name
			}
		}
	}
	return out
}
func (s *Service) SetAccounts(multiple bool, fid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !multiple {
		fid = ""
	} else {
		if fid == "" {
			fid = s.active
		}
		if _, ok := s.commanders[fid]; !ok || fid == "" {
			return errors.New("Choose a commander witnessed in verified local journals")
		}
	}
	next := AccountSettings{multiple, fid}
	raw, _ := json.Marshal(next)
	if s.opts.StorageDir != "" {
		if e := privatefs.Write(s.opts.StorageDir, "accounts-v1.json", raw); e != nil {
			return errors.New("Cannot persist account selection; previous selection retained")
		}
	}
	s.accounts = next
	s.cachedProjection = nil
	s.views.dirty = true
	s.publishLocked()
	return nil
}

type AccountCard struct {
	FID      string `json:"fid"`
	Name     string `json:"name"`
	ShipName string `json:"shipName"`
	ShipType string `json:"shipType"`
	System   string `json:"system"`
	Active   bool   `json:"active"`
}

func (s *Service) AccountCards() []AccountCard {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []AccountCard{}
	names := map[string]string{}
	if s.store != nil {
		if people, e := s.store.Commanders(); e == nil {
			for _, p := range people {
				names[p.FID] = p.Name
			}
		}
	}
	for _, fid := range s.order {
		ship := s.views.ship.View(fid)
		out = append(out, AccountCard{fid, names[fid], ship.ShipName, catalog.ShipDisplayName(ship.ShipType), s.commanders[fid].system, fid == s.displayFIDLocked()})
	}
	return out
}
func (s *Service) CommanderOverview(ctx context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fid := s.displayFIDLocked()
	out := map[string]any{"fid": fid, "system": s.commanders[fid].system, "shipName": s.views.ship.View(fid).ShipName, "shipType": catalog.ShipDisplayName(s.views.ship.View(fid).ShipType), "journalMismatch": fid != s.active}
	if s.store == nil {
		return out, nil
	}
	people, e := s.store.Commanders()
	if e != nil {
		return nil, e
	}
	for _, p := range people {
		if p.FID == fid {
			out["name"] = p.Name
		}
	}
	facts, e := s.store.EffectiveFacts(ctx, fid, "commander", time.Now())
	if e != nil {
		return nil, e
	}
	out["facts"] = facts
	fleet, e := s.store.ReconciledFleetCount(ctx, fid, facts)
	if e != nil {
		return nil, e
	}
	out["fleetCount"] = fleet
	power, e := s.store.Operations(ctx, fid, history.OperationQuery{Kind: "powerplay", Limit: 10})
	if e == nil {
		out["powerplay"] = power.Current
	}
	squad, e := s.store.Operations(ctx, fid, history.OperationQuery{Kind: "squadron", Limit: 10})
	if e == nil {
		out["squadron"] = squad.Current
	}
	return out, nil
}
