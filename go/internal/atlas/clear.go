package atlas

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ClearAll owns only generated atlas releases inside the private atlas root.
// Persist disabling before deleting any bytes; retain signature high-water state.
func (m *Manager) ClearAll() error {
	m.mu.Lock()
	if m.clearing {
		m.mu.Unlock()
		return errors.New("Atlas cache is already being cleared")
	}
	next := m.state
	next.Disabled = true
	next.Automatic = false
	next.Details = false
	next.Installed = ""
	next.Previous = ""
	next.Available = ""
	next.ETag = ""
	if err := m.save(next); err != nil {
		m.mu.Unlock()
		return err
	}
	m.state = next
	m.clearing = true
	m.networkCancel()
	if m.viewCancel != nil {
		m.viewCancel()
	}
	m.current = nil
	m.mu.Unlock()
	m.wg.Wait()
	m.operation.Lock()
	defer m.operation.Unlock()
	defer func() {
		m.mu.Lock()
		m.clearing = false
		m.networkCtx, m.networkCancel = context.WithCancel(m.ctx)
		m.mu.Unlock()
	}()
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if idPattern.MatchString(name) || strings.HasPrefix(name, ".staging-") || name == "pointer.json" {
			if err = os.RemoveAll(filepath.Join(m.root, name)); err != nil {
				return errors.New("Some map files could not be removed; atlas remains disabled")
			}
		}
	}
	m.mu.Lock()
	m.downloaded = 0
	m.estimated = 0
	m.lastError = ""
	m.viewKey = ""
	m.viewPending = false
	m.cacheMu.Lock()
	m.visible = nil
	m.disk = nil
	m.cacheBytes = 0
	m.baseBytes = 0
	m.cacheMu.Unlock()
	m.mu.Unlock()
	return nil
}
