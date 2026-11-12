package webapp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"time"
	"unicode"

	"github.com/Jakoczig/daedalus/go/internal/privatefs"
)

type device struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Expires time.Time `json:"expires"`
	Hash    string    `json:"hash"`
}
type remoteState struct {
	Version int      `json:"version"`
	Enabled bool     `json:"enabled"`
	Active  bool     `json:"active"`
	Address string   `json:"address"`
	Port    int      `json:"port"`
	Devices []device `json:"devices"`
}
type pairingCode struct {
	Hash    string
	Expires time.Time
}

func randomToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func validName(name string) bool {
	if len(name) == 0 || len(name) > 80 {
		return false
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func (m *remoteManager) load() error {
	raw, err := privatefs.Read(m.dir, "remote.json")
	if errors.Is(err, fs.ErrNotExist) {
		if _, e := privatefs.Read(m.dir, "tls.initialized"); !errors.Is(e, fs.ErrNotExist) {
			return errors.New("remote settings are missing from an initialized profile")
		}
		m.state = remoteState{Version: 1}
		return m.saveLocked()
	}
	if err != nil {
		return err
	}
	if json.Unmarshal(raw, &m.state) != nil || m.state.Version != 1 || len(m.state.Devices) > 32 {
		return errors.New("invalid remote settings")
	}
	seen := map[string]bool{}
	for _, d := range m.state.Devices {
		if len(d.Hash) != 64 || len(d.ID) != 32 || !validName(d.Name) || seen[d.ID] {
			return errors.New("invalid device state")
		}
		seen[d.ID] = true
	}
	if m.state.Active {
		m.state.Enabled = false
		m.state.Active = false
		m.state.Devices = nil
		m.notice = "Previous remote session ended unexpectedly. Enable and pair devices again."
		return m.saveLocked()
	}
	return nil
}
func (m *remoteManager) saveLocked() error {
	raw, err := json.Marshal(m.state)
	if err != nil {
		return err
	}
	return privatefs.Write(m.dir, "remote.json", raw)
}
func (m *remoteManager) findLocked(token string) (device, bool) {
	if len(token) != 64 || m.fault || m.closed || !m.state.Enabled {
		return device{}, false
	}
	hash := tokenHash(token)
	for _, d := range m.state.Devices {
		if d.Hash == hash && m.now().Before(d.Expires) {
			return d, true
		}
	}
	return device{}, false
}
func (m *remoteManager) pruneLocked() {
	now := m.now()
	live := m.state.Devices[:0]
	for _, d := range m.state.Devices {
		if now.Before(d.Expires) {
			live = append(live, d)
		}
	}
	m.state.Devices = live
}
func (m *remoteManager) pairLocked(code, name string) (string, error) {
	now := m.now()
	if m.attemptStart.IsZero() || now.Sub(m.attemptStart) >= time.Minute {
		m.attemptStart = now
		m.attempts = 0
	}
	m.attempts++
	if m.attempts > 10 {
		return "", errors.New("pairing temporarily limited")
	}
	if !m.state.Enabled || m.fault || m.pairing == nil || !now.Before(m.pairing.Expires) || len(code) != 32 || tokenHash(code) != m.pairing.Hash || !validName(name) {
		return "", errors.New("invalid or expired pairing code")
	}
	m.pruneLocked()
	if len(m.state.Devices) >= 32 {
		return "", errors.New("device limit reached")
	}
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	id, err := randomToken(16)
	if err != nil {
		return "", err
	}
	m.pairing = nil
	m.state.Devices = append(m.state.Devices, device{ID: id, Name: name, Expires: now.Add(12 * time.Hour), Hash: tokenHash(token)})
	if err = m.saveLocked(); err != nil {
		m.failLocked()
		return "", errors.New("cannot persist device access")
	}
	return token, nil
}
