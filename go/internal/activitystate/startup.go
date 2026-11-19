package activitystate

import (
	"bytes"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/journal"
)

// RestoreStartup changes snapshot fields only after the normal LoadGame reset.
// It never changes the source cursor/session clock or counts activity twice.
func (m *Model) RestoreStartup(key string, load journal.Event, batch journal.StartupBatch) {
	s := m.states[key]
	if s == nil || !s.verified || s.fid != load.FID || s.file != load.Position.FileID || s.path != load.Position.Path || s.offset != load.Position.Offset || s.sequence != load.Position.Sequence {
		return
	}
	for _, e := range batch.Events(load) {
		switch e.Type {
		case "Rank", "Progress", "Reputation", "EngineerProgress":
		default:
			continue
		}
		var d map[string]any
		decoder := json.NewDecoder(bytes.NewReader(e.Raw))
		decoder.UseNumber()
		if decoder.Decode(&d) != nil {
			return
		}
		probe := state{v: empty("journal_last_known")}
		probe.commander(e, d)
		if probe.v.Commander.Evidence == "partial" {
			continue
		}
		if rows, ok := d["Engineers"].([]any); ok && len(rows) != len(probe.v.Commander.Engineers) {
			continue
		}
		s.commander(e, d)
	}
}
