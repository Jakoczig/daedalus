package paritystate

import "github.com/Jakoczig/eridion/go/internal/journal"

func (m *Model) RestoreStartup(key string, load journal.Event, batch journal.StartupBatch) {
	s := m.states[key]
	if s == nil || !s.verified || m.active != key || s.file != load.Position.FileID || s.offset != load.Position.Offset || s.sequence != load.Position.Sequence || !s.session.Equal(load.Timestamp) {
		return
	}
	for _, e := range batch.Events(load) {
		if e.Type != "Materials" {
			continue
		}
		v := parseMaterials(object(e.Raw))
		if v.State == "known" && !v.Truncated {
			s.v.Materials = v
		}
	}
}
