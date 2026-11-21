package atlas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// 64 MiB holds two worst-case query working sets: each <=4 MiB indexes +
// 128*200,000 encoded block bytes. Bubble and density have separate budgets.
const spatialCacheBudget int64 = 64 << 20

var spatialAssetPath = regexp.MustCompile(`^assets/(indexes/[0-9a-f]{64}\.json|blocks/[0-9a-f]{64}\.z|bubbles/[0-9a-f]{64}\.zip)$`)
var spatialReleasePath = regexp.MustCompile(`^releases/[0-9a-f]{24}/(distribution|spatial)\.json$`)

type SpatialPreferences struct {
	Downloads bool `json:"downloads"`
	Automatic bool `json:"automatic"`
	Offline   bool `json:"offline"`
	Metered   bool `json:"metered"`
}
type spatialState struct {
	SpatialPreferences
	Installed     string    `json:"installed"`
	Previous      string    `json:"previous"`
	BubbleRelease string    `json:"bubbleRelease"`
	LastCheck     time.Time `json:"lastCheck"`
	Issued        time.Time `json:"issued"`
	ETag          string    `json:"etag"`
}
type SpatialStatus struct {
	SpatialPreferences
	Installed       string    `json:"installed"`
	Previous        string    `json:"previous"`
	BubbleInstalled bool      `json:"bubbleInstalled"`
	BubbleBytes     int64     `json:"bubbleBytes"`
	BubbleHash      string    `json:"bubbleHash"`
	BubbleSystems   int64     `json:"bubbleSystems"`
	BubbleSource    string    `json:"bubbleSource"`
	BubbleUpdate    bool      `json:"bubbleUpdate"`
	Busy            bool      `json:"busy"`
	Pending         bool      `json:"pending"`
	CacheBytes      int64     `json:"cacheBytes"`
	CacheBudget     int64     `json:"cacheBudget"`
	Downloaded      int64     `json:"downloaded"`
	Error           string    `json:"error"`
	LastCheck       time.Time `json:"lastCheck"`
}
type SpatialManager struct {
	mu               sync.Mutex
	ioMu             sync.Mutex
	operation        sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	closed           bool
	root             string
	client           *http.Client
	key              []byte
	now              func() time.Time
	state            spatialState
	current          *spatialManifest
	distributionData spatialDistribution
	bubble           *spatialBubble
	busy, pending    bool
	lastError        string
	downloaded       int64
	queueCancel      context.CancelFunc
	queueKey         string
	network          context.Context
	networkCancel    context.CancelFunc
	backoff          map[string]time.Time
	failures         map[string]int
	pins             map[string]bool
	cachedKey        string
	cached           SpatialView
	slots            chan struct{}
	rateMu           sync.Mutex
	nextRequest      time.Time
	revision         atomic.Uint64
}

func NewSpatial(ctx context.Context, profile string) *SpatialManager {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.MaxConnsPerHost = 2
	tr.MaxIdleConnsPerHost = 2
	tr.MaxResponseHeaderBytes = 16 << 10
	tr.ResponseHeaderTimeout = 8 * time.Second
	tr.TLSHandshakeTimeout = 5 * time.Second
	key, _ := hex.DecodeString(publicKeyHex)
	return newSpatial(ctx, filepath.Join(profile, "galaxy-spatial-v1"), &http.Client{Transport: tr}, key, time.Now)
}
func newSpatial(ctx context.Context, root string, client *http.Client, key []byte, now func() time.Time) *SpatialManager {
	ctx, cancel := context.WithCancel(ctx)
	c := *client
	c.Timeout = 45 * time.Second
	c.Jar = nil
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("Spatial redirects rejected") }
	m := &SpatialManager{ctx: ctx, cancel: cancel, root: root, client: &c, key: append([]byte(nil), key...), now: now, backoff: map[string]time.Time{}, failures: map[string]int{}, pins: map[string]bool{}, slots: make(chan struct{}, 2)}
	m.network, m.networkCancel = context.WithCancel(ctx)
	if privatefs.Prepare(root) != nil {
		m.lastError = "Spatial cache unavailable"
		return m
	}
	if b, e := privatefs.Read(root, "state.json"); e == nil {
		if decodeStrict(b, &m.state, 1<<20) != nil {
			m.state = spatialState{}
			m.lastError = "Spatial settings corrupt; downloads disabled"
		}
	}
	if m.state.Installed != "" {
		if e := m.loadRelease(m.state.Installed); e != nil {
			m.lastError = "Installed spatial release invalid; use rollback or reinstall"
		}
	}
	if idPattern.MatchString(m.state.BubbleRelease) {
		if _, d, _, e := m.readRelease(m.state.BubbleRelease); e == nil {
			path := filepath.Join(root, "bubbles", d.Bubble.SHA256+".zip")
			if privatefs.CheckPrivate(path) == nil {
				z, e := openSpatialBubble(path, d.Bubble.SHA256)
				if e == nil && bubbleMatches(z, d) {
					m.bubble = z
				} else if z != nil {
					z.Close()
				}
			}
		}
	}
	if m.state.Automatic && m.allowed() {
		_ = m.StartUpdate(false, true)
	}
	return m
}
func (m *SpatialManager) allowed() bool {
	return m.state.Downloads && !m.state.Offline && !m.state.Metered && !m.closed
}
func (m *SpatialManager) save(s spatialState) error {
	b, _ := json.Marshal(s)
	return privatefs.Write(m.root, "state.json", b)
}
func (m *SpatialManager) Configure(p SpatialPreferences) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return context.Canceled
	}
	s := m.state
	s.SpatialPreferences = p
	if e := m.save(s); e != nil {
		return e
	}
	m.state = s
	m.networkCancel()
	m.revision.Add(1)
	m.network, m.networkCancel = context.WithCancel(m.ctx)
	if m.queueCancel != nil {
		m.queueCancel()
	}
	m.queueKey = ""
	m.pending = false
	return nil
}
func (m *SpatialManager) Close() {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	if m.queueCancel != nil {
		m.queueCancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
	m.ioMu.Lock()
	if m.bubble != nil {
		m.bubble.Close()
		m.bubble = nil
	}
	m.ioMu.Unlock()
}
func (m *SpatialManager) Status() SpatialStatus {
	m.ioMu.Lock()
	defer m.ioMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := SpatialStatus{SpatialPreferences: m.state.SpatialPreferences, Installed: m.state.Installed, Previous: m.state.Previous, Busy: m.busy, Pending: m.pending, CacheBudget: spatialCacheBudget, Downloaded: m.downloaded, Error: m.lastError, LastCheck: m.state.LastCheck}
	if m.current == nil {
		s.Installed = ""
	}
	if m.distributionData.Bubble.Bytes > 0 {
		s.BubbleBytes = m.distributionData.Bubble.Bytes
		s.BubbleHash = m.distributionData.Bubble.SHA256
		s.BubbleSystems = m.distributionData.Bubble.Systems
	}
	s.BubbleInstalled = m.bubble != nil
	if m.bubble != nil {
		s.BubbleSystems = m.bubble.manifest.Systems
		s.BubbleSource = m.bubble.manifest.Source.Label
		s.BubbleUpdate = filepath.Base(m.bubble.file.Name()) != m.distributionData.Bubble.SHA256+".zip"
	}
	s.CacheBytes = m.diskBytes()
	return s
}
func (m *SpatialManager) fetch(ctx context.Context, path string, limit int64, tag string) ([]byte, string, bool, error) {
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		return nil, "", false, ctx.Err()
	}
	if !spatialNetworkPath(path) {
		return nil, "", false, errSpatial
	}
	m.mu.Lock()
	allowed := m.allowed()
	network := m.network
	m.mu.Unlock()
	if !allowed {
		return nil, "", false, errors.New("Spatial downloads require owner consent and unmetered online mode")
	}
	if e := m.throttle(ctx); e != nil {
		return nil, "", false, e
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	stop := context.AfterFunc(network, cancel)
	defer stop()
	r, _ := http.NewRequestWithContext(ctx, "GET", Origin+"/atlas/spatial/"+path, nil)
	r.Header.Set("Accept-Encoding", "identity")
	r.Header.Set("User-Agent", "ERIDION-Spatial/1")
	if tag != "" {
		r.Header.Set("If-None-Match", tag)
	}
	resp, e := m.client.Do(r)
	if e != nil {
		return nil, "", false, errors.New("Spatial HTTPS unavailable; verified offline data retained")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 304 && tag != "" {
		return nil, tag, true, nil
	}
	if resp.StatusCode != 200 || resp.ContentLength > limit || resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "identity" {
		return nil, "", false, errors.New("Invalid spatial HTTPS response")
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, "", false, errSpatial
	}
	m.mu.Lock()
	m.downloaded += int64(len(b))
	m.mu.Unlock()
	return b, resp.Header.Get("ETag"), false, nil
}
func (m *SpatialManager) readRelease(id string) (spatialPointer, spatialDistribution, *spatialManifest, error) {
	var p spatialPointer
	var d spatialDistribution
	if !idPattern.MatchString(id) {
		return p, d, nil, errSpatial
	}
	base := "releases/" + id + "/"
	pb, e := readFile(m.root, base+"pointer.json", 64<<10)
	if e != nil {
		return p, d, nil, e
	}
	p, e = m.pointer(pb, false)
	if e != nil || p.ID != id {
		return p, d, nil, errSpatial
	}
	db, e := readFile(m.root, base+"distribution.json", 1<<20)
	if e != nil {
		return p, d, nil, e
	}
	d, e = m.distribution(db, p)
	if e != nil {
		return p, d, nil, e
	}
	b, e := readFile(m.root, base+"spatial.json", 16<<20)
	if e != nil || int64(len(b)) != d.Spatial.Bytes || digest(b) != d.Spatial.SHA256 {
		return p, d, nil, errSpatial
	}
	v, e := parseSpatial(b)
	if e != nil || !manifestMatches(v, d) {
		return p, d, nil, errSpatial
	}
	return p, d, v, nil
}
func manifestMatches(v *spatialManifest, d spatialDistribution) bool {
	return v != nil && v.RadiusLY == 0 && v.RootLY == d.RootLY && v.Systems == d.Systems && len(v.Roots) == d.Roots && v.Source.SHA256 == d.Source.SHA256
}
func bubbleMatches(z *spatialBubble, d spatialDistribution) bool {
	return z.manifest.RadiusLY == 350 && z.manifest.RootLY == d.RootLY && z.manifest.Systems == d.Bubble.Systems && z.manifest.Source.SHA256 == d.Source.SHA256
}
func (m *SpatialManager) loadRelease(id string) error {
	_, d, v, e := m.readRelease(id)
	if e == nil {
		m.current = v
		m.distributionData = d
	}
	return e
}
func (m *SpatialManager) StartUpdate(bubble, automatic bool, expected ...spatialBubbleAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.allowed() {
		return errors.New("Spatial downloads disabled")
	}
	if m.busy {
		return errors.New("Spatial installation already running")
	}
	if automatic && m.now().Sub(m.state.LastCheck) < 24*time.Hour {
		return nil
	}
	m.busy = true
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		var expectation *spatialBubbleAsset
		if len(expected) > 0 {
			expectation = &expected[0]
		}
		e := m.update(m.ctx, bubble, expectation)
		m.mu.Lock()
		m.busy = false
		if e != nil {
			m.lastError = e.Error()
		}
		m.mu.Unlock()
	}()
	return nil
}
func (m *SpatialManager) Update(ctx context.Context, bubble bool) error {
	return m.update(ctx, bubble, nil)
}
func (m *SpatialManager) StartBubble(hash string, size int64) error {
	if !hashPattern.MatchString(hash) || size < 22 || size > 1<<30 {
		return errors.New("Confirm exact signed Bubble size before installation")
	}
	return m.StartUpdate(true, false, spatialBubbleAsset{SHA256: hash, Bytes: size})
}
func (m *SpatialManager) update(ctx context.Context, bubble bool, expected *spatialBubbleAsset) error {
	m.operation.Lock()
	defer m.operation.Unlock()
	m.mu.Lock()
	if !m.allowed() {
		m.mu.Unlock()
		return errors.New("Spatial downloads disabled")
	}
	s := m.state
	s.LastCheck = m.now()
	if e := m.save(s); e != nil {
		m.mu.Unlock()
		return e
	}
	m.state = s
	m.mu.Unlock()
	pb, tag, notModified, e := m.fetch(ctx, "latest.json", 64<<10, s.ETag)
	if e != nil {
		return e
	}
	if notModified {
		pb, e = readFile(m.root, "latest.json", 64<<10)
		if e != nil {
			return e
		}
	}
	p, e := m.pointer(pb, true)
	if e != nil {
		return e
	}
	if p.Issued.Before(s.Issued) {
		return errors.New("Older spatial pointer rejected; explicit offline rollback remains available")
	}
	db, _, _, e := m.fetch(ctx, p.Path, 1<<20, "")
	if e != nil {
		return e
	}
	d, e := m.distribution(db, p)
	if e != nil {
		return e
	}
	if bubble && expected != nil && (d.Bubble.SHA256 != expected.SHA256 || d.Bubble.Bytes != expected.Bytes) {
		return errors.New("Bubble changed; refresh signed index and confirm the new download size")
	}
	var b []byte
	// Reuse an already verified immutable manifest; releases do not key child caches.
	if _, old, v, err := m.readRelease(p.ID); err == nil && old.Spatial.SHA256 == d.Spatial.SHA256 && v != nil {
		b, e = readFile(m.root, d.Spatial.Path, 16<<20)
	} else {
		b, _, _, e = m.fetch(ctx, d.Spatial.Path, d.Spatial.Bytes, "")
	}
	if e != nil || int64(len(b)) != d.Spatial.Bytes || digest(b) != d.Spatial.SHA256 {
		return errSpatial
	}
	v, e := parseSpatial(b)
	if e != nil || !manifestMatches(v, d) {
		return errSpatial
	}
	base := "releases/" + p.ID + "/"
	for name, data := range map[string][]byte{"pointer.json": pb, "distribution.json": db, "spatial.json": b} {
		if e = writeFile(m.root, base+name, data); e != nil {
			return e
		}
	}
	if bubble {
		if e = m.installBubble(ctx, d); e != nil {
			return e
		}
	}
	m.ioMu.Lock()
	defer m.ioMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.allowed() {
		return context.Canceled
	}
	next := m.state
	if next.Installed != p.ID {
		next.Previous = next.Installed
	}
	next.Installed = p.ID
	next.Issued = p.Issued
	next.ETag = tag
	if bubble {
		next.BubbleRelease = p.ID
	}
	if e = m.save(next); e != nil {
		return e
	}
	_ = writeFile(m.root, "latest.json", pb)
	m.state = next
	m.current = v
	m.distributionData = d
	m.cachedKey = ""
	m.lastError = ""
	if m.queueCancel != nil {
		m.queueCancel()
	}
	m.queueKey = ""
	m.pending = false
	if bubble {
		if m.bubble != nil {
			m.bubble.Close()
		}
		m.bubble, _ = openSpatialBubble(filepath.Join(m.root, "bubbles", d.Bubble.SHA256+".zip"), d.Bubble.SHA256)
	}
	m.pruneMetadata()
	m.revision.Add(1)
	return nil
}
func (m *SpatialManager) installBubble(ctx context.Context, d spatialDistribution) error {
	dir := filepath.Join(m.root, "bubbles")
	if e := privatefs.Prepare(dir); e != nil {
		return e
	}
	target := filepath.Join(dir, d.Bubble.SHA256+".zip")
	if privatefs.CheckPrivate(target) == nil {
		if z, e := openSpatialBubble(target, d.Bubble.SHA256); e == nil {
			defer z.Close()
			if bubbleMatches(z, d) {
				return nil
			}
		}
	}
	m.mu.Lock()
	ok := m.allowed()
	network := m.network
	m.mu.Unlock()
	if !ok {
		return context.Canceled
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	stop := context.AfterFunc(network, cancel)
	defer stop()
	if e := m.throttle(ctx); e != nil {
		return e
	}
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	r, _ := http.NewRequestWithContext(ctx, "GET", Origin+"/atlas/spatial/"+d.Bubble.Path, nil)
	r.Header.Set("Accept-Encoding", "identity")
	r.Header.Set("User-Agent", "ERIDION-Spatial/1")
	resp, e := m.client.Do(r)
	if e != nil {
		return errors.New("Bubble HTTPS unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.ContentLength >= 0 && resp.ContentLength != d.Bubble.Bytes || resp.Header.Get("Content-Encoding") != "" && resp.Header.Get("Content-Encoding") != "identity" {
		return errSpatial
	}
	temp := d.Bubble.SHA256 + ".partial"
	f, e := privatefs.PrivateFile(filepath.Join(dir, temp), true)
	if e != nil {
		_ = os.Remove(filepath.Join(dir, temp))
		return errors.New("Interrupted Bubble staging retained; retry installation")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, d.Bubble.Bytes+1))
	if e != nil || n != d.Bubble.Bytes || hex.EncodeToString(h.Sum(nil)) != d.Bubble.SHA256 {
		return errSpatial
	}
	if e = f.Sync(); e != nil {
		return e
	}
	f.Close()
	z, e := openSpatialBubble(filepath.Join(dir, temp), d.Bubble.SHA256)
	if e != nil {
		return e
	}
	matches := bubbleMatches(z, d)
	z.Close()
	if !matches {
		return errSpatial
	}
	if e = os.Rename(filepath.Join(dir, temp), target); e != nil {
		return e
	}
	m.mu.Lock()
	m.downloaded += n
	m.mu.Unlock()
	return nil
}
func (m *SpatialManager) Rollback() error {
	m.operation.Lock()
	defer m.operation.Unlock()
	m.ioMu.Lock()
	defer m.ioMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	_, d, v, e := m.readRelease(m.state.Previous)
	if e != nil {
		return e
	}
	s := m.state
	s.Installed, s.Previous = s.Previous, s.Installed
	if e = m.save(s); e != nil {
		return e
	}
	m.state = s
	m.current = v
	m.distributionData = d
	m.cachedKey = ""
	m.revision.Add(1)
	if m.queueCancel != nil {
		m.queueCancel()
	}
	m.queueKey = ""
	m.pending = false
	return nil
}
func (m *SpatialManager) RemoveBubble(confirm bool) error {
	if !confirm {
		return errors.New("Confirm removal of offline Bubble")
	}
	m.operation.Lock()
	defer m.operation.Unlock()
	m.ioMu.Lock()
	defer m.ioMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state
	s.BubbleRelease = ""
	if e := m.save(s); e != nil {
		return e
	}
	m.state = s
	m.cachedKey = ""
	m.revision.Add(1)
	if m.bubble != nil {
		m.bubble.Close()
		m.bubble = nil
	}
	dir := filepath.Join(m.root, "bubbles")
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if len(entry.Name()) != 68 || filepath.Ext(entry.Name()) != ".zip" || !hashPattern.MatchString(entry.Name()[:64]) {
			continue
		}
		p := filepath.Join(dir, entry.Name())
		if e := privatefs.CheckPrivate(p); e != nil {
			return e
		}
		if e := os.Remove(p); e != nil {
			return e
		}
	}
	return nil
}
func (m *SpatialManager) diskBytes() int64 {
	var total int64
	for _, group := range []string{"indexes", "blocks"} {
		entries, _ := os.ReadDir(filepath.Join(m.root, "assets", group))
		for _, e := range entries {
			if st, err := e.Info(); err == nil && st.Mode().IsRegular() {
				total += st.Size()
			}
		}
	}
	return total
}
func spatialPath(group, sha string) string {
	suffix := ".z"
	if group == "indexes" {
		suffix = ".json"
	}
	return "assets/" + group + "/" + sha + suffix
}
func (m *SpatialManager) Retry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.queueCancel != nil {
		m.queueCancel()
	}
	m.queueKey = ""
	m.pending = false
}
func (m *SpatialManager) Revision() uint64 { return m.revision.Load() }
func (m *SpatialManager) throttle(ctx context.Context) error {
	m.rateMu.Lock()
	now := time.Now()
	next := m.nextRequest
	if next.Before(now) {
		next = now
	}
	m.nextRequest = next.Add(50 * time.Millisecond)
	m.rateMu.Unlock()
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Keep current, previous and the manifest authenticating the permanent Bubble.
// Delete only known metadata filenames; never recurse through an untrusted path.
func (m *SpatialManager) pruneMetadata() {
	keep := map[string]bool{m.state.Installed: true, m.state.Previous: true, m.state.BubbleRelease: true}
	dir := filepath.Join(m.root, "releases")
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		if !idPattern.MatchString(e.Name()) || keep[e.Name()] || !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if privatefs.CheckPrivate(p) != nil {
			continue
		}
		for _, name := range []string{"pointer.json", "distribution.json", "spatial.json"} {
			_ = os.Remove(filepath.Join(p, name))
		}
		_ = os.Remove(p)
	}
	// At most the active ZIP plus one prior verified package, separate from LRU.
	dir = filepath.Join(m.root, "bubbles")
	es, _ = os.ReadDir(dir)
	type old struct {
		p string
		t time.Time
	}
	var previous []old
	active := ""
	if m.bubble != nil {
		active = m.bubble.file.Name()
	}
	for _, e := range es {
		p := filepath.Join(dir, e.Name())
		if e.Name() == filepath.Base(active) || len(e.Name()) != 68 || !hashPattern.MatchString(e.Name()[:64]) || filepath.Ext(e.Name()) != ".zip" {
			continue
		}
		if st, err := e.Info(); err == nil && st.Mode().IsRegular() {
			previous = append(previous, old{p, st.ModTime()})
		}
	}
	sort.Slice(previous, func(i, j int) bool { return previous[i].t.After(previous[j].t) })
	for i := 1; i < len(previous); i++ {
		_ = os.Remove(previous[i].p)
	}
}
