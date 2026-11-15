package navstate

import "reflect"

// Gob encodes pointers as values. Rebind only a route which was already owned by
// this exact commander/session epoch and remains byte/content-identical to the
// persisted candidate. This never calls correlate to create new verification.
func (m *Model) RestoreLinks() {
	c := m.candidate
	if c == nil || c.ambiguous || !m.sourceValid {
		return
	}
	for key, s := range m.sessions {
		if s == nil || s.route == nil || !s.verified || s.clearUnknown || c.owner != key || c.epoch != s.epoch || !c.stamp.Equal(s.marker) || !c.stamp.After(s.reset) || !c.stamp.After(s.barrier) {
			continue
		}
		if reflect.DeepEqual(*s.route, *c) {
			s.route = c
		}
	}
}
