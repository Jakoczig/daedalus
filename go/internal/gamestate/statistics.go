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
	raw, _ := json.Marshal(struct {
		FID  string
		View paritystate.View
	}{s.active, p})
	sum := sha256.Sum256(raw)
	key := hex.EncodeToString(sum[:])
	var v paritystate.EngineeredStatistics
	if s.store != nil {
		if b, e := s.store.DerivedStatistics(key, paritystate.StatisticsFormula); e == nil && len(b) > 0 && json.Unmarshal(b, &v) == nil {
			return v
		}
	}
	v = paritystate.Statistics(p.Modules)
	if s.store != nil {
		raw, _ = json.Marshal(v)
		_ = s.store.SaveStatistics(key, paritystate.StatisticsFormula, raw)
	}
	return v
}
