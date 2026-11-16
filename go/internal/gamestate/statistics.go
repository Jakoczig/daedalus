package gamestate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/paritystate"
)

func (s *Service) EngineeredStatistics() paritystate.EngineeredStatistics {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.projectionLocked().Workspace.Parity
	// Only fitted evidence participates in the persistent cache. Status snapshots
	// (including deployment) are volatile observations, not formula inputs.
	modules := append([]paritystate.Module(nil), p.Modules...)
	for i := range modules {
		modules[i].Name, modules[i].Health, modules[i].AmmoInClip, modules[i].AmmoInHopper = "", nil, nil, nil
	}
	raw, _ := json.Marshal(struct {
		Format, FID, ShipID, Formula string
		Modules                      []paritystate.Module
	}{"loadout-static-v1", s.active, s.views.ship.Identity(s.active), paritystate.StatisticsFormula, modules})
	sum := sha256.Sum256(raw)
	key := hex.EncodeToString(sum[:])
	var v paritystate.EngineeredStatistics
	cached := s.statisticsJSON
	if s.statisticsKey != key {
		cached = nil
	}
	if len(cached) == 0 && s.store != nil {
		cached, _ = s.store.DerivedStatistics(key, paritystate.StatisticsFormula)
	}
	if len(cached) == 0 || json.Unmarshal(cached, &v) != nil {
		v = paritystate.Statistics(p.Modules)
		cached, _ = json.Marshal(v)
		if s.store != nil {
			_ = s.store.SaveStatistics(key, paritystate.StatisticsFormula, cached)
		}
	}
	// Keep a single bounded entry, encoded to prevent callers mutating shared DTOs.
	s.statisticsKey, s.statisticsJSON = key, cached
	if p.Status.State == "known" && p.Status.Flags != nil {
		deployed := p.Status.Flags.HardpointsDeployed
		v.ObservedHardpointsDeployed = &deployed
		if deployed {
			v.ObservedDeploymentRatedMW = v.DeployedMW
		} else {
			v.ObservedDeploymentRatedMW = v.StowedMW
		}
	}
	return v
}
