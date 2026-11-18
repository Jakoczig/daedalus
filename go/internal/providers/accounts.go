package providers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// scopedVault retains unassociated legacy secrets without making them available
// to another FID. All writes still use the original atomic OS-protected vault.
type scopedVault struct {
	backing Vault
	all     map[string]Credential
	scope   string
}

func (v *scopedVault) Load() (map[string]Credential, error) {
	all, e := v.backing.Load()
	if e != nil {
		return nil, e
	}
	v.all = all
	if v.all == nil {
		v.all = map[string]Credential{}
	}
	out := map[string]Credential{}
	for k, c := range v.all {
		if strings.HasPrefix(k, v.scope) {
			name := strings.TrimPrefix(k, v.scope)
			if !strings.Contains(name, "/") {
				out[name] = c
			}
		}
	}
	return out, nil
}
func (v *scopedVault) Save(values map[string]Credential) error {
	next := map[string]Credential{}
	for k, c := range v.all {
		if !strings.HasPrefix(k, v.scope) || strings.Contains(strings.TrimPrefix(k, v.scope), "/") {
			next[k] = c
		}
	}
	for k, c := range values {
		next[v.scope+k] = c
	}
	if e := v.backing.Save(next); e != nil {
		return e
	}
	v.all = next
	return nil
}

type providerDisk struct {
	Providers     map[string]storedConfig            `json:"providers"`
	Ledger        map[string]int64                   `json:"ledger"`
	Uploader      string                             `json:"uploader"`
	Accounts      map[string]map[string]storedConfig `json:"accounts,omitempty"`
	ActiveAccount string                             `json:"activeAccount,omitempty"`
}

func (m *Manager) configsLocked() map[string]storedConfig {
	out := map[string]storedConfig{}
	for id, s := range m.states {
		out[id] = storedConfig{Config: s.config, Identity: s.config.CommanderHash}
	}
	return out
}
func (m *Manager) diskLocked() providerDisk {
	if m.accountConfigs == nil {
		m.accountConfigs = map[string]map[string]storedConfig{}
	}
	if m.accountFID != "" {
		m.accountConfigs[hash(m.accountFID)] = m.configsLocked()
	}
	return providerDisk{m.configsLocked(), m.ledger, m.uploader, m.accountConfigs, hashAccount(m.accountFID)}
}
func hashAccount(fid string) string {
	if fid == "" {
		return ""
	}
	return hash(fid)
}

// ActivateAccount cancels in-flight generations before changing the credential
// namespace. The caller has verified FID against local journal history.
func (m *Manager) ActivateAccount(fid string, adoptLegacy bool) error {
	if fid == "" || len(fid) > 128 {
		return errors.New("Verified journal account required")
	}
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pinnedAccount != "" && m.pinnedAccount != fid {
		return errors.New("Commander selection changed; retry in the selected profile")
	}
	if m.accountFID == fid {
		return nil
	}
	return m.activateAccountLocked(fid, adoptLegacy)
}

// Stage the destination namespace and consent before publishing either. Cancelled
// generations and upload candidates are intentionally never rolled back.
func (m *Manager) activateAccountLocked(fid string, adoptLegacy bool) error {
	accounts := map[string]map[string]storedConfig{}
	for k, v := range m.accountConfigs {
		accounts[k] = v
	}
	if m.accountFID != "" {
		accounts[hash(m.accountFID)] = m.configsLocked()
	}
	configs := accounts[hash(fid)]
	if configs == nil && adoptLegacy && m.accountFID == "" {
		configs = m.configsLocked()
	}
	if len(accounts) >= 64 && configs == nil {
		return errors.New("Account profile limit reached")
	}
	staged := map[string]storedConfig{}
	for id := range m.states {
		staged[id] = storedConfig{}
	}
	for id, c := range configs {
		if m.states[id] != nil && m.validConfig(id, c.Config) == nil {
			staged[id] = c
		}
	}
	accounts[hash(fid)] = staged
	scope := hash(fid) + "/"
	secrets := map[string]Credential{}
	// Cancel before any persistence can block, including direct activation callers.
	m.oauth = nil
	m.cancelPublicationLocked()
	if v, ok := m.opts.Vault.(*scopedVault); ok {
		if adoptLegacy && v.scope == "" {
			next := map[string]Credential{}
			changed := false
			for k, c := range v.all {
				if strings.Contains(k, "/") {
					next[k] = c
				} else if c.JournalFID == fid || ((k == "edsm" || k == "inara") && m.name != "" && c.Commander == m.name) {
					c.JournalFID = fid
					changed = true
					next[scope+k] = c
				} else {
					next[k] = c
					m.notice = "Unassociated legacy credentials retained in the encrypted vault. Reconnect Frontier or enter the selected commander's own API key; credentials were not assigned by guesswork."
				}
			}
			if changed {
				if e := v.backing.Save(next); e != nil {
					return errors.New("Cannot migrate account credentials; original vault retained")
				}
			}
			// A completed encrypted legacy migration is durable even if consent fails;
			// the active scope/secrets stay unchanged and a retry finds these same keys.
			v.all = next
		}
		for k, c := range v.all {
			if strings.HasPrefix(k, scope) {
				secrets[strings.TrimPrefix(k, scope)] = c
			}
		}
	}
	raw, e := json.Marshal(providerDisk{staged, m.ledger, m.uploader, accounts, hash(fid)})
	if e != nil {
		return e
	}
	if e = m.persistConsentLocked(raw); e != nil {
		return e
	}
	// Nothing below can fail: publish one consistent namespace under both locks.
	if v, ok := m.opts.Vault.(*scopedVault); ok {
		v.scope = scope
	}
	m.secrets = secrets
	m.accountConfigs = accounts
	m.accountFID = fid
	m.layers = Layers{}
	m.station = stationSession{}
	for id, s := range m.states {
		s.snapshot = nil
		s.config = Config{}
		s.lastSuccess = ""
		s.lastError = ""
		s.sent = 0
		s.dropped = 0
		s.counts = map[string]uint64{}
		if c, ok := staged[id]; ok {
			c.Config.CommanderHash = c.Identity
			s.config = c.Config
		}
	}
	return nil
}

// PinAccount prevents a different live journal account from changing provider
// context after the owner chose a profile. Its events cannot enter this queue.
func (m *Manager) PinAccount(fid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pinnedAccount != fid {
		m.oauth = nil
		m.cancelPublicationLocked()
	}
	m.pinnedAccount = fid
}
func (m *Manager) AccountMatches() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.accountFID == "" || m.accountFID == m.fid
}

func (m *Manager) saveDiskLocked() ([]byte, error) { return json.Marshal(m.diskLocked()) }

// A user unlocking the OS vault must not stall the single journal reader. Until
// a pending namespace can activate, mismatched-account reads/uploads fail closed.
func (m *Manager) activateObservedAccount(fid string) {
	if !m.vaultMu.TryLock() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.oauth = nil
		for _, s := range m.states {
			s.cancel()
			s.ctx, s.cancel = context.WithCancel(m.ctx)
			s.generation++
			s.queue = nil
			s.snapshot = nil
		}
		m.notice = "Provider account activation awaits the credential vault. Reopen provider settings after unlocking it; journal ingestion continues."
		return
	}
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if (m.pinnedAccount != "" && m.pinnedAccount != fid) || m.accountFID == fid {
		return
	}
	if e := m.activateAccountLocked(fid, false); e != nil {
		m.notice = "Provider account activation failed; previous account retained. Retry selecting the account after repairing consent storage. Mismatched-account external access remains disabled; journal history is retained."
	}
}
