package webapp

import (
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"os"
	"slices"
	"sync"
)

type displayConfig struct {
	Charge     bool     `json:"charge"`
	Jump       bool     `json:"jump"`
	Restore    bool     `json:"restore"`
	Owner      bool     `json:"owner"`
	AllViewers bool     `json:"allViewers"`
	Devices    []string `json:"devices"`
	CloseMode  string   `json:"closeMode"`
}
type companionState struct {
	mu       sync.Mutex
	config   displayConfig
	notice   string
	action   func(string) error
	setClose func(string)
}

func (s *Server) initCompanion(opts Options) {
	s.copyWaypoint = opts.CopyWaypoint
	s.saveFieldCSV = opts.SaveFieldCSV
	s.companion = &companionState{config: displayConfig{Restore: true, CloseMode: "tray"}, action: opts.DesktopAction, setClose: opts.DesktopCloseMode}
	if raw, err := privatefs.Read(opts.StorageDir, "companion.json"); err == nil {
		var c displayConfig
		if json.Unmarshal(raw, &c) == nil && validDisplayConfig(c) {
			s.companion.config = c
		} else {
			s.companion.notice = "Invalid companion settings; automatic transitions disabled. Save settings to recover."
		}
	} else if !os.IsNotExist(err) {
		s.companion.notice = "Companion settings unavailable; automatic transitions disabled."
	}
	if opts.DesktopCloseMode != nil {
		opts.DesktopCloseMode(s.companion.config.CloseMode)
	}
}
func validDisplayConfig(c displayConfig) bool {
	if c.CloseMode != "tray" && c.CloseMode != "quit" || len(c.Devices) > 64 {
		return false
	}
	for _, id := range c.Devices {
		if len(id) > 128 || id == "" {
			return false
		}
	}
	return true
}
func (s *Server) displayPolicy(p socketPrincipal) map[string]any {
	if s.companion == nil {
		return map[string]any{"selected": false}
	}
	s.companion.mu.Lock()
	c := s.companion.config
	s.companion.mu.Unlock()
	selected := c.Owner
	if p.role != "owner" {
		selected = c.AllViewers || slices.Contains(c.Devices, p.deviceID)
	}
	return map[string]any{"selected": selected, "charge": c.Charge, "jump": c.Jump, "restore": c.Restore}
}
func (s *Server) saveDisplayConfig(raw []byte) error {
	var c displayConfig
	if strictDecode(raw, &c) != nil || !validDisplayConfig(c) {
		return errors.New("invalid companion settings")
	}
	s.companion.mu.Lock()
	defer s.companion.mu.Unlock()
	b, _ := json.Marshal(c)
	if privatefs.Write(s.profile, "companion.json", b) != nil {
		return errors.New("companion settings could not be saved")
	}
	s.companion.config = c
	s.companion.notice = ""
	if s.companion.setClose != nil {
		s.companion.setClose(c.CloseMode)
	}
	return nil
}
func (s *Server) launcherView() any {
	s.companion.mu.Lock()
	c, native, notice := s.companion.config, s.companion.action != nil, s.companion.notice
	s.companion.mu.Unlock()
	devices := []map[string]string{}
	connected := 0
	if s.remote != nil {
		s.remote.mu.Lock()
		for _, d := range s.remote.state.Devices {
			devices = append(devices, map[string]string{"id": d.ID, "name": d.Name})
		}
		seen := map[string]bool{}
		for _, id := range s.remote.connections {
			seen[id] = true
		}
		connected = len(seen)
		s.remote.mu.Unlock()
	}
	provider := any(nil)
	if s.community != nil {
		a, b := s.game.SystemIdentity()
		v := s.community.View(a, b)
		provider = map[string]any{"enabled": v.Enabled, "status": v.Status, "fetchedAt": v.FetchedAt, "expiresAt": v.ExpiresAt, "notice": v.Notice}
	}
	return map[string]any{"config": c, "notice": notice, "native": native, "devices": devices, "connectedDevices": connected, "journal": s.game.Snapshot(), "provider": provider}
}
