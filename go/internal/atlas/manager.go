package atlas

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxOverview = 64 << 20
const maxDetailCache = 128 << 20

type Asset struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
	Count  int    `json:"count,omitempty"`
}
type Pointer struct {
	Protocol string    `json:"protocol"`
	ID       string    `json:"atlas_id"`
	Index    string    `json:"index"`
	Hash     string    `json:"index_sha256"`
	Issued   time.Time `json:"issued_at"`
	Expires  time.Time `json:"expires_at"`
}
type Index struct {
	Protocol    string          `json:"protocol"`
	ID          string          `json:"atlas_id"`
	Format      int             `json:"format_version"`
	Schema      int             `json:"renderer_schema"`
	MaxLevel    int             `json:"max_level"`
	TileSize    int             `json:"tile_size"`
	Views       []string        `json:"views"`
	Fingerprint string          `json:"build_fingerprint"`
	Source      json.RawMessage `json:"source"`
	Attribution string          `json:"attribution"`
	Overview    Asset           `json:"overview"`
	Catalogs    []Asset         `json:"catalogs"`
	DetailCount int             `json:"detail_tile_count"`
	DetailBytes int64           `json:"detail_tile_bytes"`
	TileCount   int             `json:"tile_count"`
	AtlasHash   string          `json:"atlas_json_sha256"`
}
type Preferences struct {
	Automatic bool `json:"automatic"`
	Details   bool `json:"details"`
	Metered   bool `json:"metered"`
}
type state struct {
	Preferences
	Installed   string    `json:"installed"`
	Previous    string    `json:"previous"`
	Available   string    `json:"available"`
	ETag        string    `json:"etag"`
	LastAttempt time.Time `json:"lastAttempt"`
	LastSuccess time.Time `json:"lastSuccess"`
	Issued      time.Time `json:"issued"`
}
type Status struct {
	Preferences
	Installed  string    `json:"installed"`
	Available  string    `json:"available"`
	Previous   string    `json:"previous"`
	LastCheck  time.Time `json:"lastCheck"`
	Source     string    `json:"source"`
	Generated  string    `json:"generated"`
	SourceDate string    `json:"sourceDate"`
	Busy       bool      `json:"busy"`
	Downloaded int64     `json:"downloaded"`
	Estimated  int64     `json:"estimated"`
	CacheBytes int64     `json:"cacheBytes"`
	Error      string    `json:"error"`
}
type Manager struct {
	baseBytes             int64
	visible               map[string]bool
	write                 func(string, string, []byte) error
	mu                    sync.Mutex
	operation             sync.Mutex
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	root                  string
	client                *http.Client
	key                   []byte
	now                   func() time.Time
	state                 state
	current               *release
	lastError             string
	busy                  bool
	downloaded, estimated int64
	slots                 chan struct{}
	tileMu                sync.Mutex
	viewCancel            context.CancelFunc
	viewKey               string
	viewPending           bool
	cacheMu               sync.Mutex
	disk                  map[string]diskEntry
	cacheBytes            int64
}

func New(ctx context.Context, profile string) *Manager {
	key, _ := hex.DecodeString(publicKeyHex)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.MaxConnsPerHost = 2
	tr.MaxIdleConnsPerHost = 2
	tr.MaxResponseHeaderBytes = 16 << 10
	tr.ResponseHeaderTimeout = 8 * time.Second
	tr.TLSHandshakeTimeout = 5 * time.Second
	return newManager(ctx, filepath.Join(profile, "galaxy-atlas-v2"), &http.Client{Transport: tr}, key, time.Now)
}
func newManager(ctx context.Context, root string, client *http.Client, key []byte, now func() time.Time) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	c := *client
	c.Timeout = 30 * time.Second
	c.Jar = nil
	c.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) > 2 || r.URL.Scheme != "https" || r.URL.Host != "eridion.fitoserver.org" || r.URL.User != nil || r.URL.RawQuery != "" || !strings.HasPrefix(r.URL.Path, "/atlas/") || !networkPath(strings.TrimPrefix(r.URL.Path, "/atlas/")) || r.URL.RawPath != "" {
			return errors.New("atlas redirect rejected")
		}
		return nil
	}
	m := &Manager{ctx: ctx, cancel: cancel, root: root, client: &c, key: append([]byte(nil), key...), now: now, slots: make(chan struct{}, 2), write: writeFile}
	if e := privatefs.Prepare(root); e != nil {
		m.lastError = "Atlas cache unavailable"
		return m
	}
	if b, e := privatefs.Read(root, "state.json"); e == nil {
		if decodeStrict(b, &m.state, 1<<20) != nil {
			m.state = state{}
			m.lastError = "Atlas settings damaged; preserved for diagnosis. Automatic access disabled."
		}
	} else if !os.IsNotExist(e) {
		m.lastError = "Atlas settings unavailable; automatic access disabled."
	}
	if m.state.Installed != "" {
		r, e := m.load(m.state.Installed)
		if e != nil {
			m.lastError = "Installed atlas failed verification; select a verified rollback or reinstall."
		} else {
			m.current = r
		}
	}
	if m.current != nil {
		m.pruneReleases()
	}
	m.scanCache()
	// A process crash can leave only unactivated staging directories. The locked,
	// private atlas root owns these generated names; never follow links here.
	if entries, e := os.ReadDir(root); e == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".staging-") && entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
				_ = os.RemoveAll(filepath.Join(root, entry.Name()))
			}
		}
	}
	if m.state.Automatic && !m.state.Metered {
		m.wg.Add(1)
		go func() { defer m.wg.Done(); _ = m.Check(ctx, true) }()
	}
	return m
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.cancel()
	if m.viewCancel != nil {
		m.viewCancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}
func (m *Manager) save(s state) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	return privatefs.Write(m.root, "state.json", b)
}
func (m *Manager) Configure(p Preferences) error {
	m.operation.Lock()
	defer m.operation.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state
	s.Preferences = p
	if e := m.save(s); e != nil {
		return e
	}
	m.state = s
	m.viewPending = false
	m.viewKey = ""
	if m.viewCancel != nil {
		m.viewCancel()
	}
	if !p.Details || p.Metered {
		if m.viewCancel != nil {
			m.viewCancel()
		}
		m.viewKey = ""
	}
	return nil
}
func (m *Manager) Status() Status {
	m.mu.Lock()
	v := Status{Preferences: m.state.Preferences, Available: m.state.Available, Previous: m.state.Previous, LastCheck: m.state.LastSuccess, Busy: m.busy || m.viewPending, Downloaded: m.downloaded, Estimated: m.estimated, Error: m.lastError}
	if m.current != nil {
		v.Installed = m.current.index.ID
		v.Source = sourceURL
		v.Generated = m.current.meta.Generated()
		if v.Estimated == 0 {
			v.Estimated = m.current.index.Overview.Bytes
		}
		var source struct {
			Downloaded string `json:"downloaded_at"`
		}
		_ = json.Unmarshal(m.current.index.Source, &source)
		v.SourceDate = source.Downloaded
	}
	m.mu.Unlock()
	v.CacheBytes = m.cacheSize()
	return v
}
func (m *Manager) fail(e error) error { m.mu.Lock(); m.lastError = e.Error(); m.mu.Unlock(); return e }
func (m *Manager) fetch(ctx context.Context, path string, limit int64, expected *Asset, etag string) ([]byte, string, bool, error) {
	// Every caller builds one of these validated paths; no user-controlled URLs.
	if !networkPath(path) {
		return nil, "", false, errors.New("invalid atlas path")
	}
	timeout := 30 * time.Second
	if path == "latest.json" {
		timeout = 8 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	req, e := http.NewRequestWithContext(requestCtx, "GET", Origin+"/atlas/"+path, nil)
	if e != nil {
		return nil, "", false, e
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "ERIDION-Atlas/2")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, e := m.client.Do(req)
	if e != nil {
		return nil, "", false, errors.New("Atlas HTTPS request failed; verified cache remains available")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 304 && etag != "" {
		return nil, etag, true, nil
	}
	if resp.StatusCode != 200 {
		return nil, "", false, fmt.Errorf("Atlas HTTPS status %d", resp.StatusCode)
	}
	if resp.ContentLength > limit || (expected != nil && resp.ContentLength >= 0 && resp.ContentLength != expected.Bytes) {
		return nil, "", false, errors.New("atlas response length mismatch")
	}
	if expected != nil {
		limit = expected.Bytes
	}
	b, e := io.ReadAll(io.LimitReader(progressReader{resp.Body, m}, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, "", false, errors.New("interrupted or oversized atlas response")
	}
	if expected != nil && (int64(len(b)) != expected.Bytes || digest(b) != expected.SHA256) {
		return nil, "", false, errors.New("atlas asset hash/size mismatch")
	}
	return b, resp.Header.Get("ETag"), false, nil
}
func (m *Manager) pointer(b []byte) (Pointer, error) {
	var p Pointer
	v, e := verifyEnvelope(b, m.key)
	if e == nil {
		e = decodePayload(v, &p)
	}
	if e != nil {
		return p, e
	}
	n := m.now()
	if p.Protocol != protocol || !idPattern.MatchString(p.ID) || p.Index != "releases/"+p.ID+"/index.json" || !hashPattern.MatchString(p.Hash) || p.Issued.After(n.Add(5*time.Minute)) || !p.Expires.After(n) || !p.Expires.After(p.Issued) || p.Expires.Sub(p.Issued) > 63*24*time.Hour {
		return p, errors.New("invalid or expired signed atlas pointer")
	}
	return p, nil
}
func (m *Manager) Check(ctx context.Context, automatic bool) error {
	m.operation.Lock()
	defer m.operation.Unlock()
	m.mu.Lock()
	s := m.state
	if automatic && (!s.Automatic || s.Metered || m.now().Sub(s.LastAttempt) < 24*time.Hour) {
		m.mu.Unlock()
		return nil
	}
	s.LastAttempt = m.now()
	if e := m.save(s); e != nil {
		m.mu.Unlock()
		return e
	}
	m.state = s
	m.mu.Unlock()
	b, tag, notModified, e := m.fetch(ctx, "latest.json", 64<<10, nil, s.ETag)
	if e != nil {
		return m.fail(e)
	}
	if notModified {
		b, e = readFile(m.root, "pointer.json", 64<<10)
		if e != nil {
			return m.fail(e)
		}
	}
	p, e := m.pointer(b)
	if e != nil {
		return m.fail(e)
	}
	if p.Issued.Before(s.Issued) {
		return m.fail(errors.New("older signed pointer rejected; use explicit local rollback"))
	}
	if !notModified {
		if e = privatefs.Write(m.root, "pointer.json", b); e != nil {
			return m.fail(e)
		}
	}
	s.Available = p.ID
	s.Issued = p.Issued
	s.ETag = tag
	s.LastSuccess = m.now()
	if e = m.save(s); e != nil {
		return m.fail(e)
	}
	m.mu.Lock()
	m.state = s
	m.lastError = ""
	m.mu.Unlock()
	return nil
}
func (m *Manager) index(b []byte, p Pointer) (Index, error) {
	var v Index
	if digest(b) != p.Hash {
		return v, errors.New("signed index hash mismatch")
	}
	payload, e := verifyEnvelope(b, m.key)
	if e == nil {
		e = decodePayload(payload, &v)
	}
	if e == nil {
		e = validateIndex(v)
	}
	if e == nil && v.ID != p.ID {
		e = errors.New("release identity mismatch")
	}
	return v, e
}
func validateIndex(v Index) error {
	if v.Protocol != protocol || !idPattern.MatchString(v.ID) || v.Format != 2 || v.Schema != 2 || v.MaxLevel != 7 || v.TileSize != 256 || strings.Join(v.Views, ",") != "xz,xy,yz" || !hashPattern.MatchString(v.Fingerprint) || !hashPattern.MatchString(v.AtlasHash) || v.Overview.Path != "overview.zip" || !validAsset(v.Overview, maxOverview) || len(v.Catalogs) != 9 || v.DetailCount < 0 || v.DetailCount > 30000 || v.DetailBytes < 0 || v.DetailBytes > 512<<20 || v.TileCount < 1 || v.TileCount > 32000 {
		return errors.New("unsupported atlas format or size")
	}
	var source struct {
		URL   string `json:"url"`
		Hash  string `json:"sha256"`
		Bytes int64  `json:"bytes"`
		Fresh bool   `json:"freshness_verified"`
	}
	if json.Unmarshal(v.Source, &source) != nil || source.URL != sourceURL || !hashPattern.MatchString(source.Hash) || source.Bytes < 0 || !source.Fresh {
		return errors.New("unverified atlas source")
	}
	seen := map[string]bool{}
	count := 0
	for _, a := range v.Catalogs {
		if !catalogPattern.MatchString(a.Path) || seen[a.Path] || !validAsset(a, 12<<20) || a.Count < 0 || a.Count > 16384 {
			return errors.New("invalid signed catalog")
		}
		seen[a.Path] = true
		count += a.Count
	}
	if count != v.DetailCount {
		return errors.New("catalog summary mismatch")
	}
	return nil
}
func validAsset(a Asset, limit int64) bool {
	return a.Bytes > 0 && a.Bytes <= limit && hashPattern.MatchString(a.SHA256)
}
func (m *Manager) StartInstall() error {
	m.mu.Lock()
	if m.ctx.Err() != nil {
		m.mu.Unlock()
		return errors.New("Atlas service stopped")
	}
	if m.busy {
		m.mu.Unlock()
		return errors.New("Atlas installation already running")
	}
	m.busy = true
	m.wg.Add(1)
	m.mu.Unlock()
	go func() { defer m.wg.Done(); _ = m.Install(m.ctx); m.mu.Lock(); m.busy = false; m.mu.Unlock() }()
	return nil
}
func (m *Manager) Install(ctx context.Context) error {
	m.operation.Lock()
	defer m.operation.Unlock()
	m.mu.Lock()
	m.downloaded = 0
	m.mu.Unlock()
	b, e := readFile(m.root, "pointer.json", 64<<10)
	if e != nil {
		return m.fail(errors.New("Check for a signed atlas update first"))
	}
	p, e := m.pointer(b)
	if e != nil {
		return m.fail(e)
	}
	raw, _, _, e := m.fetch(ctx, p.Index, 4<<20, nil, "")
	if e != nil {
		return m.fail(e)
	}
	idx, e := m.index(raw, p)
	if e != nil {
		return m.fail(e)
	}
	m.mu.Lock()
	m.estimated = idx.Overview.Bytes
	m.mu.Unlock()
	zip, _, _, e := m.fetch(ctx, "releases/"+p.ID+"/overview.zip", maxOverview, &idx.Overview, "")
	if e != nil {
		return m.fail(e)
	}
	stage, e := os.MkdirTemp(m.root, ".staging-")
	if e != nil {
		return m.fail(e)
	}
	defer os.RemoveAll(stage)
	r, e := openOverview(zip, idx, func(name string, data []byte) error { return m.write(stage, name, data) })
	if e != nil {
		return m.fail(e)
	}
	if e = m.write(stage, "index.json", raw); e != nil {
		return m.fail(e)
	}
	if e = m.write(stage, "overview.zip", zip); e != nil {
		return m.fail(e)
	}
	dest := filepath.Join(m.root, p.ID)
	created := false
	if old, e := readFile(dest, "index.json", 4<<20); e == nil {
		if digest(old) != digest(raw) {
			return m.fail(errors.New("immutable release ID collision"))
		}
		if _, e = m.load(p.ID); e != nil {
			return m.fail(e)
		}
	} else if !os.IsNotExist(e) {
		return m.fail(e)
	} else {
		if e = os.Rename(stage, dest); e != nil {
			return m.fail(e)
		}
		created = true
	}
	m.mu.Lock()
	s := m.state
	if s.Installed != p.ID {
		s.Previous = s.Installed
	}
	s.Installed = p.ID
	if e = m.save(s); e != nil {
		m.mu.Unlock()
		if created {
			_ = os.RemoveAll(dest)
		}
		return m.fail(e)
	}
	m.state = s
	m.current = r
	m.current.dir = dest
	m.lastError = ""
	if m.viewCancel != nil {
		m.viewCancel()
	}
	m.viewKey = ""
	m.viewPending = false
	m.mu.Unlock()
	m.pruneReleases()
	m.scanCache()
	return nil
}
func (m *Manager) load(id string) (*release, error) {
	if !idPattern.MatchString(id) {
		return nil, errors.New("invalid installed ID")
	}
	dir := filepath.Join(m.root, id)
	raw, e := readFile(dir, "index.json", 4<<20)
	if e != nil {
		return nil, e
	}
	p, e := verifyEnvelope(raw, m.key)
	var idx Index
	if e == nil {
		e = decodePayload(p, &idx)
	}
	if e == nil {
		e = validateIndex(idx)
	}
	if e != nil || idx.ID != id {
		return nil, errors.New("invalid installed manifest")
	}
	b, e := readFile(dir, "overview.zip", maxOverview)
	if e != nil {
		return nil, e
	}
	if int64(len(b)) != idx.Overview.Bytes || digest(b) != idx.Overview.SHA256 {
		return nil, errors.New("cached overview damaged")
	}
	r, e := openOverview(b, idx, nil)
	if e != nil {
		return nil, e
	}
	r.dir = dir
	for name, a := range r.overview {
		b, e := readFile(dir, name, 4<<20)
		if e != nil || int64(len(b)) != a.Bytes || digest(b) != a.SHA256 {
			return nil, errors.New("cached overview tile damaged")
		}
	}
	return r, nil
}
func (m *Manager) Rollback() error {
	m.operation.Lock()
	defer m.operation.Unlock()
	m.mu.Lock()
	s := m.state
	m.mu.Unlock()
	r, e := m.load(s.Previous)
	if e != nil {
		return m.fail(e)
	}
	s.Installed, s.Previous = s.Previous, s.Installed
	if e = m.save(s); e != nil {
		return m.fail(e)
	}
	m.mu.Lock()
	m.current = r
	m.state = s
	m.lastError = ""
	if m.viewCancel != nil {
		m.viewCancel()
	}
	m.viewKey = ""
	m.viewPending = false
	m.mu.Unlock()
	return nil
}
func writeFile(root, name string, b []byte) error {
	if !localPath(name) {
		return errors.New("invalid cache path")
	}
	p := filepath.Join(root, filepath.FromSlash(name))
	if e := privatefs.Prepare(filepath.Dir(p)); e != nil {
		return e
	}
	return privatefs.Write(filepath.Dir(p), filepath.Base(p), b)
}
func readFile(root, name string, limit int64) ([]byte, error) {
	if !localPath(name) {
		return nil, errors.New("invalid cache path")
	}
	p := filepath.Join(root, filepath.FromSlash(name))
	for cur := p; ; cur = filepath.Dir(cur) {
		f, e := os.Lstat(cur)
		if e != nil {
			return nil, e
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("linked cache file")
		}
		if cur == filepath.Clean(root) {
			break
		}
		if filepath.Dir(cur) == cur {
			return nil, errors.New("cache path escape")
		}
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() > limit {
		return nil, errors.New("invalid cached file")
	}
	return io.ReadAll(io.LimitReader(f, limit+1))
}

type progressReader struct {
	io.Reader
	m *Manager
}

func (p progressReader) Read(b []byte) (int, error) {
	n, e := p.Reader.Read(b)
	p.m.mu.Lock()
	p.m.downloaded += int64(n)
	p.m.mu.Unlock()
	return n, e
}
