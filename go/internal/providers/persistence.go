package providers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/history"
	"net/http"
)

func (m *Manager) AttachHistory(db *history.Store, seed history.ProviderSeed) error {
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, _ := m.saveDiskLocked()
	raw, err := db.LoadProviderConsent(raw)
	if err != nil {
		for _, s := range m.states {
			s.cancel()
			s.queue = nil
			s.config = Config{}
		}
		m.notice = "Database consent could not be verified; all external access disabled"
		return err
	}
	var state providerDisk
	if json.Unmarshal(raw, &state) != nil || len(state.Ledger) > 4096 || len(state.Accounts) > 64 {
		return errors.New("Invalid canonical provider state")
	}
	m.history = db
	m.ledger = state.Ledger
	if m.ledger == nil {
		m.ledger = map[string]int64{}
	}
	m.uploader = state.Uploader
	for id, s := range m.states {
		s.cancel()
		s.ctx, s.cancel = context.WithCancel(m.ctx)
		s.generation++
		s.queue = nil
		s.config = Config{}
		if c, ok := state.Providers[id]; ok && m.validConfig(id, c.Config) == nil {
			c.Config.CommanderHash = c.Identity
			s.config = c.Config
		}
	}
	m.accountConfigs = state.Accounts
	if m.accountConfigs == nil {
		m.accountConfigs = map[string]map[string]storedConfig{}
	}
	// Legacy canonical consent is adopted only to the verified cached FID. A
	// namespace already stored for a different FID never migrates implicitly.
	if seed.FID != "" {
		m.name = seed.Name
		if state.ActiveAccount != "" {
			m.accountConfigs[state.ActiveAccount] = state.Providers
			for _, s := range m.states {
				s.config = Config{}
			}
		}
		if err := m.activateAccountLocked(seed.FID, state.ActiveAccount == ""); err != nil {
			return err
		}
	}
	if seed.FID == "" {
		m.secrets = map[string]Credential{}
		for _, s := range m.states {
			s.config = Config{}
		}
	}

	m.fid = seed.FID
	m.name = seed.Name
	m.identity = ""
	if seed.FID != "" {
		m.identity = hash(seed.FID)
	}
	m.sourceFile = seed.SourceID
	m.sourceSequence = seed.Sequence
	m.gameVersion = seed.GameVersion
	m.gameBuild = seed.GameBuild
	m.odyssey = seed.Odyssey
	m.system = seed.System
	m.position = seed.Position
	m.address = nil
	if seed.Address != "" {
		m.address = json.Number(seed.Address)
	}
	// Durable references survive restarts, but unsent events require explicit review.
	if err := db.HoldPending("Restarted service: explicit publication review required"); err != nil {
		m.history = nil
		m.notice = "Publication references unavailable; external access disabled"
		return err
	}
	return nil
}
func (m *Manager) outcomeLocked(batch []queued, status, reason string) {
	if m.history == nil {
		return
	}
	for _, q := range batch {
		if q.candidateID != "" {
			if err := m.history.CandidateOutcome(q.candidateID, status, reason); err != nil {
				m.notice = "Delivery receipt could not be persisted. Do not retry uncertain submissions."
			}
		}
	}
}
func (m *Manager) cancelPublicationLocked() {
	for id, s := range m.states {
		s.cancel()
		s.ctx, s.cancel = context.WithCancel(m.ctx)
		s.generation++
		s.queue = nil
		if m.history != nil && m.fid != "" {
			_ = m.history.CancelCandidates(m.fid, id)
		}
	}
}
func (m *Manager) PublicationPolicy(fid string) (history.PublicationPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.history == nil {
		return history.PublicationPolicy{}, errors.New("Database not ready")
	}
	return m.history.PublicationPolicy(fid)
}
func (m *Manager) SetPublicationPolicy(fid, mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.history == nil {
		return errors.New("Database not ready")
	}
	m.cancelPublicationLocked()
	return m.history.SetPublicationPolicy(fid, mode)
}
func (m *Manager) SetLiveException(fid, provider, category string, enabled, acknowledged bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[provider]
	if m.history == nil || s == nil {
		return errors.New("Database/provider not ready")
	}
	found := false
	for _, c := range m.cap(provider).Upload {
		if c == category {
			found = true
		}
	}
	if !found {
		return errors.New("Unsupported publication category")
	}
	m.cancelPublicationLocked()
	return m.history.SetLiveException(fid, provider, category, enabled, acknowledged)
}

// PublicReadClient shares the existing fixed TLS transport/pool, never credentials.
func (m *Manager) PublicReadClient() *http.Client {
	return &http.Client{Transport: m.client.Transport, Timeout: m.client.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (m *Manager) CancelPublications() { m.mu.Lock(); defer m.mu.Unlock(); m.cancelPublicationLocked() }
