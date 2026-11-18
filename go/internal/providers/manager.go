package providers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/history"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	accountFID, pinnedAccount                      string
	accountConfigs                                 map[string]map[string]storedConfig
	hostCooldown                                   map[string]time.Time
	station                                        stationSession
	history                                        *history.Store
	fid                                            string
	vaultMu                                        sync.Mutex
	sourceFile                                     string
	sourceSequence                                 uint64
	client                                         *http.Client
	mu                                             sync.Mutex
	ctx                                            context.Context
	cancel                                         context.CancelFunc
	wg                                             sync.WaitGroup
	dir                                            string
	opts                                           Options
	states                                         map[string]*providerState
	secrets                                        map[string]Credential
	ledger                                         map[string]int64
	uploader                                       string
	notice                                         string
	identity, name, gameVersion, gameBuild, system string
	position                                       []any
	address                                        any
	odyssey                                        bool
	layers                                         Layers
	oauth                                          *oauthFlow
	closed                                         bool
}

func New(parent context.Context, dir string, opts Options) *Manager {
	ctx, cancel := context.WithCancel(parent)
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Vault == nil {
		opts.Vault = newVault(dir)
	}
	opts.Vault = &scopedVault{backing: opts.Vault}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if opts.Client != nil {
		client.Transport = opts.Client.Transport
	} else {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.Proxy = nil
		tr.ForceAttemptHTTP2 = false
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		tr.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		client.Transport = tr
	}
	m := &Manager{client: client, ctx: ctx, cancel: cancel, dir: dir, opts: opts, states: map[string]*providerState{}, secrets: map[string]Credential{}, ledger: map[string]int64{}}
	for _, id := range ids {
		c, k := context.WithCancel(ctx)
		m.states[id] = &providerState{wake: make(chan struct{}, 1), ctx: c, cancel: k, counts: map[string]uint64{}}
	}
	var disk providerDisk
	if b, e := privatefs.Read(dir, "providers.json"); e == nil {
		if json.Unmarshal(b, &disk) != nil || len(disk.Ledger) > 4096 || len(disk.Accounts) > 64 {
			m.notice = "Invalid provider state: all external access disabled; preserve providers.json for diagnosis."
		} else {
			m.accountConfigs = disk.Accounts
			if m.accountConfigs == nil {
				m.accountConfigs = map[string]map[string]storedConfig{}
			}
			if disk.ActiveAccount != "" {
				m.accountConfigs[disk.ActiveAccount] = disk.Providers
			}
			m.ledger = disk.Ledger
			if m.ledger == nil {
				m.ledger = map[string]int64{}
			}
			m.uploader = disk.Uploader
			for id, c := range disk.Providers {
				if s := m.states[id]; s != nil && m.validConfig(id, c.Config) == nil {
					c.Config.CommanderHash = c.Identity
					s.config = c.Config
				}
			}
		}
	} else if !os.IsNotExist(e) {
		m.notice = "Provider storage unavailable; external access disabled."
	}
	if c, e := opts.Vault.Load(); e == nil && c != nil {
		m.secrets = c
	} else if e != nil {
		m.notice = "Credential vault unavailable. Windows uses DPAPI; Linux requires an unlocked Secret Service and secret-tool."
	}
	if len(m.uploader) != 32 {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err == nil {
			m.uploader = hex.EncodeToString(b)
		}
	}
	if m.notice != "" {
		for _, s := range m.states {
			s.config = Config{}
		}
	}
	if !opts.Manual {
		for _, id := range ids {
			m.wg.Add(1)
			go m.worker(id)
		}
	}
	return m
}
func (m *Manager) cap(id string) Capability {
	for _, c := range capabilities() {
		if c.ID == id {
			c.Evidence = "Contract-fixture tested; live authentication/publication acceptance not established"
			if id == "inara" && (m.opts.InaraAppName != "" || InaraApplicationName != "") {
				c.Gate = ""
			}
			if id == "frontier" && (m.opts.FrontierClientID != "" || FrontierClientID != "") {
				c.Gate = ""
			}
			return c
		}
	}
	return Capability{}
}
func (m *Manager) validConfig(id string, c Config) error {
	cap := m.cap(id)
	if cap.ID == "" || len(c.Categories) > 16 || c.Subscribe {
		return errors.New("unsupported provider or subscription")
	}
	for _, cat := range c.Categories {
		if !slices.Contains(cap.Upload, cat) {
			return errors.New("unsupported upload category")
		}
	}
	if (c.Read || c.Upload) && cap.Gate != "" {
		return errors.New(cap.Gate)
	}
	if c.Read && len(cap.Read) == 0 || c.Upload && (len(cap.Upload) == 0 || len(c.Categories) == 0) {
		return errors.New("explicit supported category required")
	}
	return nil
}
func (m *Manager) saveLocked() error {
	b, _ := m.saveDiskLocked()
	if m.history != nil {
		return m.history.SaveProviderConsent(b)
	}
	if m.opts.RequireHistory {
		return errors.New("Database consent is not ready")
	}
	return privatefs.Write(m.dir, "providers.json", b)
}
func (m *Manager) Configure(id string, c Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || (m.opts.RequireHistory && m.history == nil) {
		return errors.New("Provider service or database consent unavailable")
	}
	s := m.states[id]
	if s == nil {
		return errors.New("unknown provider")
	}
	if err := m.validConfig(id, c); err != nil {
		return err
	}
	if c.Upload {
		if m.accountFID != "" && m.accountFID != m.fid {
			return errors.New("Selected account differs from live journal; upload consent cannot be changed here")
		}
		if m.identity == "" {
			return errors.New("Start the game and witness a commander before consenting to uploads")
		}
		c.CommanderHash = m.identity
	}
	selectedFID := m.accountFID
	if selectedFID == "" {
		selectedFID = m.fid
	}
	if m.history != nil && selectedFID != "" {
		if err := m.history.CancelCandidates(selectedFID, id); err != nil {
			return errors.New("Could not cancel durable queue")
		}
	}
	s.cancel()
	s.ctx, s.cancel = context.WithCancel(m.ctx)
	s.generation++
	s.queue = nil
	s.config = c
	select {
	case s.wake <- struct{}{}:
	default:
	}
	s.snapshot = nil
	if id == "edastro" && !c.Read {
		m.layers = Layers{}
	}
	if err := m.saveLocked(); err != nil {
		s.cancel()
		s.config = Config{}
		s.lastError = "Cannot persist consent; provider disabled"
		return errors.New(s.lastError)
	}
	m.notice = ""
	return nil
}
func (m *Manager) SetCredential(id string, c Credential) error {
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || (m.opts.RequireHistory && m.history == nil) {
		return errors.New("Provider service or database consent unavailable")
	}
	if id != "edsm" && id != "inara" {
		return errors.New("provider does not accept personal API keys")
	}
	if len(c.APIKey) > 512 || len(c.Commander) > 80 || strings.ContainsAny(c.APIKey+c.Commander, "\r\n\x00") {
		return errors.New("invalid credential")
	}
	if c.AccessToken != "" || c.RefreshToken != "" {
		return errors.New("OAuth tokens are server-owned")
	}
	next := map[string]Credential{}
	for k, v := range m.secrets {
		next[k] = v
	}
	c.JournalFID = m.accountFID
	next[id] = c
	m.mu.Unlock()
	vaultErr := m.opts.Vault.Save(next)
	m.mu.Lock()
	if vaultErr != nil {
		return errors.New("Cannot store credential in OS vault; no key was accepted")
	}
	m.secrets = next
	s := m.states[id]
	s.cancel()
	s.ctx, s.cancel = context.WithCancel(m.ctx)
	s.generation++
	s.queue = nil
	return nil
}
func (m *Manager) Disconnect(id string, remove bool) error {
	if err := m.Configure(id, Config{Paused: true}); err != nil {
		return err
	}
	if remove {
		m.vaultMu.Lock()
		defer m.vaultMu.Unlock()
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, exists := m.secrets[id]; !exists {
			return nil
		}
		next := map[string]Credential{}
		for k, v := range m.secrets {
			if k != id {
				next[k] = v
			}
		}
		m.mu.Unlock()
		vaultErr := m.opts.Vault.Save(next)
		m.mu.Lock()
		if vaultErr != nil {
			return errors.New("vault deletion failed")
		}
		m.secrets = next
	}
	return nil
}
func (m *Manager) Status() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Status{}
	for _, id := range ids {
		s := m.states[id]
		state := "disabled"
		if m.cap(id).Gate != "" {
			state = "approval_needed"
		} else if s.config.Paused {
			state = "paused"
		} else if s.config.Read || s.config.Upload {
			state = "configured_live_unverified"
		}
		if s.lastSuccess != "" && (s.config.Read || s.config.Upload) && !s.config.Paused && m.cap(id).Gate == "" {
			state = "last_request_succeeded"
		}
		if s.noNewLast && s.config.Upload && !s.config.Paused && m.cap(id).Gate == "" {
			state = "no_new_observations"
		}
		if m.notice != "" {
			state = "storage_warning"
		}
		counts := map[string]uint64{}
		for k, v := range s.counts {
			counts[k] = v
		}
		cfg := s.config
		cfg.Categories = slices.Clone(cfg.Categories)
		var snapshot *Snapshot
		if s.snapshot != nil {
			copy := *s.snapshot
			copy.ActiveCommander = copy.commanderHash != "" && copy.commanderHash == m.identity
			copy.Facts = slices.Clone(copy.Facts)
			snapshot = &copy
		}
		retry := ""
		if s.next.After(m.opts.Now()) {
			retry = s.next.UTC().Format(time.RFC3339)
		}
		out = append(out, Status{Capability: m.cap(id), Config: cfg, State: state, CredentialStored: m.secrets[id].APIKey != "" || m.secrets[id].RefreshToken != "", Queue: len(s.queue), Sent: s.sent, NoNewObservations: s.noNewObservations, Dropped: s.dropped, LastSuccess: s.lastSuccess, LastError: strings.TrimSpace(m.notice + " " + s.lastError), RetryAt: retry, Counts: counts, Snapshot: snapshot})
	}
	return out
}
func (m *Manager) Close() {
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.cancel()
	for _, s := range m.states {
		s.cancel()
		s.queue = nil
	}
	m.mu.Unlock()
	m.wg.Wait()
	m.client.CloseIdleConnections()
}
func object(b []byte) map[string]any {
	if len(b) > 2<<20 {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v map[string]any
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		return nil
	}
	return v
}
func hash(s string) string { x := sha256.Sum256([]byte(s)); return hex.EncodeToString(x[:]) }
func text(v any) string {
	s, _ := v.(string)
	if len(s) > 128 || strings.ContainsAny(s, "\r\n\x00") {
		return ""
	}
	return s
}
func (m *Manager) Observe(e journal.Event, phase journal.Phase) {
	if len(e.Raw) > 256<<10 {
		return
	}
	v := object(e.Raw)
	if v == nil {
		return
	}
	if e.Type == "LoadGame" && e.FID != "" && phase == journal.Live {
		m.mu.Lock()
		pin := m.pinnedAccount
		m.mu.Unlock()
		if pin == "" || pin == e.FID {
			m.activateObservedAccount(e.FID)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	if e.Position.Sequence != 0 {
		if e.Type != "Fileheader" && m.sourceSequence != 0 && (e.Position.FileID != m.sourceFile || e.Position.Sequence != m.sourceSequence+1) {
			m.identity = ""
			m.station = stationSession{}
			m.fid = ""
			m.name = ""
			m.system = ""
			m.position = nil
			m.address = nil
		}
		m.sourceFile = e.Position.FileID
		m.sourceSequence = e.Position.Sequence
	}
	m.stationEventLocked(e, phase)
	if e.Type == "Fileheader" {
		m.identity = ""
		m.fid = ""
		m.name = ""
		m.system = ""
		m.position = nil
		m.address = nil
		m.gameVersion = e.GameVersion
		m.gameBuild = e.GameBuild
		return
	}
	if e.Type == "LoadGame" {
		m.identity = ""
		m.fid = ""
		m.name = ""
		if e.FID != "" {
			m.identity = hash(e.FID)
			m.fid = e.FID
			m.name = text(v["Commander"])
		}
		m.odyssey, _ = v["Odyssey"].(bool)
		return
	}
	if e.Type == "Shutdown" {
		m.identity = ""
		m.fid = ""
		return
	}
	if e.Type == "FSDJump" || e.Type == "Location" || e.Type == "CarrierJump" {
		m.system = text(v["StarSystem"])
		m.position = nil
		m.address = nil
		if validPosition(v["StarPos"]) {
			m.position = v["StarPos"].([]any)
			m.address = v["SystemAddress"]
		}
	}
	if (m.opts.RequireHistory && m.history == nil) || phase != journal.Live || (m.pinnedAccount != "" && m.pinnedAccount != m.fid) || (m.accountFID != "" && m.accountFID != m.fid) || m.identity == "" || !strings.HasPrefix(m.gameVersion, "4.") || strings.ContainsAny(strings.ToLower(m.gameVersion), "abcdefghijklmnopqrstuvwxyz") {
		return
	}
	age := m.opts.Now().Sub(e.Timestamp)
	if e.Timestamp.IsZero() || age < -5*time.Second || age > 2*time.Minute {
		return
	}
	for _, id := range ids {
		s := m.states[id]
		if !s.config.Upload || s.config.Paused || s.config.CommanderHash != m.identity || m.cap(id).Gate != "" {
			continue
		}
		cat := category(id, e.Type)
		if cat == "" || !slices.Contains(s.config.Categories, cat) {
			continue
		}
		candidateID := ""
		if m.history != nil {
			candidate, err := m.history.Candidate(m.fid, id, cat, e, phase, false, m.opts.Now())
			if err != nil {
				s.lastError = "Database publication authorization failed"
				continue
			}
			if candidate.Status != "eligible" {
				continue
			}
			candidateID = candidate.ID
		}
		cred := m.secrets[id]
		if (id == "edsm" || id == "inara") && (cred.APIKey == "" || cred.Commander != m.name || m.name == "") {
			s.lastError = "API key/account must match the active journal commander"
			continue
		}
		body, endpoint, ct, err := m.encode(id, e, v, cred)
		if err != nil {
			s.lastError = "Event lacks required public fields; not queued"
			continue
		}
		key := hash(id + "/" + m.identity + "/" + string(e.Raw))
		if _, seen := m.ledger[key]; seen {
			continue
		}
		if slices.ContainsFunc(s.queue, func(q queued) bool { return q.key == key }) {
			continue
		}
		queuedBytes := len(body)
		for _, q := range s.queue {
			queuedBytes += len(q.body)
		}
		if len(s.queue) >= 64 || queuedBytes > 2<<20 {
			s.dropped++
			s.lastError = "Queue full (64 events / 2 MiB); newest event dropped"
			continue
		}
		if m.history != nil && candidateID != "" {
			if err := m.history.CandidateOutcome(candidateID, "queued", "Fresh authorized event queued"); err != nil {
				s.lastError = "Cannot persist publication queue"
				continue
			}
		}
		s.queue = append(s.queue, queued{candidateID: candidateID, key: key, category: cat, body: body, endpoint: endpoint, contentType: ct, generation: s.generation})
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}
func (m *Manager) dispatch(id string) {
	m.mu.Lock()
	s := m.states[id]
	if m.closed || s == nil || s.busy || !s.config.Upload || s.config.Paused || len(s.queue) == 0 || m.opts.Now().Before(s.next) {
		m.mu.Unlock()
		return
	}
	q := s.queue[0]
	if q.generation != s.generation {
		s.queue = s.queue[1:]
		m.mu.Unlock()
		return
	}
	for k, at := range m.ledger {
		if m.opts.Now().Unix()-at > 600 {
			delete(m.ledger, k)
		}
	}
	if len(m.ledger) >= 4096 {
		s.lastError = "Dedupe budget exhausted; uploads stopped"
		m.mu.Unlock()
		return
	}
	batch, body := batchRequest(id, s.queue)
	if len(m.ledger)+len(batch) > 4096 {
		s.lastError = "Dedupe budget exhausted; uploads stopped"
		m.mu.Unlock()
		return
	}
	for _, item := range batch {
		m.ledger[item.key] = m.opts.Now().Unix()
	}
	if err := m.saveLocked(); err != nil {
		s.lastError = "Cannot persist dedupe; upload suppressed"
		s.queue = nil
		s.config.Upload = false
		m.mu.Unlock()
		return
	}
	s.busy = true
	generation := s.generation
	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	s.next = m.opts.Now().Add(time.Duration(m.cap(id).IntervalSeconds) * time.Second)
	m.mu.Unlock()
	raw, status, err := m.request(ctx, "POST", q.endpoint, q.contentType, body, "")
	requestCancelled := ctx.Err() != nil
	cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	s.busy = false
	if generation != s.generation {
		return
	}
	if len(s.queue) > 0 && s.queue[0].key == q.key {
		s.queue = s.queue[len(batch):]
	}
	// EDAstro's JSON status 401 means no new observations, not HTTP authentication failure.
	if err == nil && status >= 200 && status < 300 && id == "edastro" && fmt.Sprint(object(raw)["status"]) == "401" {
		m.outcomeLocked(batch, "no_new_observations", "Recipient reported no new observations")
		s.noNewObservations += uint64(len(batch))
		s.noNewLast = true
		s.lastError = ""
		return
	}
	s.noNewLast = false
	if err == nil && status >= 200 && status < 300 && accepted(id, raw) {
		m.outcomeLocked(batch, "confirmed", "Recipient acceptance contract satisfied")
		for _, item := range batch {
			s.sent++
			s.counts[item.category]++
		}
		s.lastSuccess = m.opts.Now().UTC().Format(time.RFC3339)
		s.lastError = ""
		return
	}
	if err != nil {
		m.outcomeLocked(batch, "unknown_delivery", "Transport outcome uncertain; no silent retry")
	} else if status != 429 && status != 503 {
		m.outcomeLocked(batch, "failed", "Recipient did not confirm acceptance")
	}
	s.lastError = "Submission not confirmed; no private server response retained"
	// Only explicit refusal responses are safe to retry. A timeout is ambiguous:
	// leave its durable marker and do not risk silently submitting twice.
	if (status == 429 || status == 503) && q.attempts < 3 && !requestCancelled {
		for i := range batch {
			batch[i].attempts++
		}
		q.attempts++
		jitter := time.Duration(q.key[0]%15) * time.Second
		s.next = m.opts.Now().Add(time.Minute*time.Duration(1<<q.attempts) + jitter)
		s.queue = append(batch, s.queue...)
	}
}
func (m *Manager) request(ctx context.Context, method, endpoint, ct string, body []byte, bearer string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, errors.New("invalid provider request")
	}
	if !allowedEndpoint(req.URL) {
		return nil, 0, errors.New("unapproved provider endpoint")
	}
	req.Header.Set("User-Agent", software+"/"+version)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	m.mu.Lock()
	until := m.hostCooldown[req.URL.Host]
	m.mu.Unlock()
	if m.opts.Now().Before(until) {
		return nil, 429, nil
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, 0, errors.New("Provider request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		wait := time.Minute
		if seconds, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && seconds > 0 {
			wait = time.Duration(min(seconds, 3600)) * time.Second
		} else if at, e := http.ParseTime(resp.Header.Get("Retry-After")); e == nil {
			wait = at.Sub(m.opts.Now())
			if wait < time.Minute {
				wait = time.Minute
			}
			if wait > time.Hour {
				wait = time.Hour
			}
		}
		m.mu.Lock()
		if m.hostCooldown == nil {
			m.hostCooldown = map[string]time.Time{}
		}
		m.hostCooldown[req.URL.Host] = m.opts.Now().Add(wait)
		m.mu.Unlock()
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return nil, resp.StatusCode, errors.New("Provider response exceeds limit")
	}
	return raw, resp.StatusCode, nil
}
func accepted(id string, b []byte) bool {
	v := object(b)
	if id == "eddn" {
		return true
	}
	if v == nil {
		return false
	}
	switch id {
	case "edsm":
		n, _ := v["msgnum"].(json.Number)
		code, err := n.Int64()
		return err == nil && code >= 100 && code < 200
	case "inara":
		h, _ := v["header"].(map[string]any)
		n, _ := h["eventStatus"].(json.Number)
		if n != "200" && n != "202" {
			return false
		}
		events, _ := v["events"].([]any)
		if len(events) == 0 {
			return false
		}
		for _, e := range events {
			o, _ := e.(map[string]any)
			n, _ := o["eventStatus"].(json.Number)
			if n != "200" && n != "202" {
				return false
			}
		}
		return true
	case "edastro":
		return fmt.Sprint(v["status"]) == "200"
	}
	return false
}
