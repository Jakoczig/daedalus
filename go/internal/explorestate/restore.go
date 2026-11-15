package explorestate

// RestoreLinks rebinds the active sampler after value serialization. Pointer
// aliases are not a persistent identity; key and witnessed timestamp are.
func (m *Model) RestoreLinks() {
	for fid, c := range m.sessions {
		if c.activeOrganic == nil {
			continue
		}
		old := c.activeOrganic
		var match *organismState
		count := 0
		if sys := m.systems[systemKey{fid, c.address}]; sys != nil {
			for _, b := range sys.bodies {
				for _, o := range b.organisms {
					if o.key == old.key && o.view.LastObservedAt == old.view.LastObservedAt {
						match = o
						count++
					}
				}
			}
		}
		if count == 1 {
			c.activeOrganic = match
		} else {
			c.activeOrganic = nil
		}
	}
}
