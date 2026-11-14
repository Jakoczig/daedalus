package providers

import "time"

// Idle providers park without timers/network. Queues wake their own worker;
// an unavailable provider cannot block another destination's worker.
func (m *Manager) worker(id string) {
	defer m.wg.Done()
	s := m.states[id]
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-s.wake:
		}
		for {
			m.dispatch(id)
			m.mu.Lock()
			pending := !m.closed && s.config.Upload && !s.config.Paused && len(s.queue) > 0
			wait := s.next.Sub(m.opts.Now())
			m.mu.Unlock()
			if !pending {
				break
			}
			if wait < 50*time.Millisecond {
				wait = 50 * time.Millisecond
			}
			timer := time.NewTimer(wait)
			select {
			case <-m.ctx.Done():
				timer.Stop()
				return
			case <-s.wake:
				timer.Stop()
			case <-timer.C:
			}
		}
	}
}
