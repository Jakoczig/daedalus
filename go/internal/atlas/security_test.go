package atlas

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func signedTest(t *testing.T, p map[string]any) []byte {
	t.Helper()
	key := ed25519.NewKeyFromSeed([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31})
	b, e := canonical(p)
	if e != nil {
		t.Fatal(e)
	}
	out, _ := json.Marshal(map[string]any{"payload": p, "signature": map[string]any{"algorithm": "Ed25519", "key_id": digest(key.Public().(ed25519.PublicKey))[:16], "value": base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, b))}})
	return out
}
func fixtureManager(t *testing.T) (*Manager, fixture, *atomic.Int32) {
	t.Helper()
	f, key := testFixture(t)
	calls := new(atomic.Int32)
	c := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		p := r.URL.Path
		var b []byte
		switch {
		case strings.HasSuffix(p, "latest.json"):
			b = un64(f.Pointer)
		case strings.HasSuffix(p, "index.json"):
			b = un64(f.Index)
		case strings.HasSuffix(p, "overview.zip"):
			b = un64(f.Overview)
		case strings.Contains(p, "/catalogs/"):
			b = un64(f.Catalogs[p[strings.Index(p, "catalogs/"):]])
		case strings.HasSuffix(p, ".png"):
			b = un64(f.Tile)
		default:
			return nil, errors.New("unexpected fixture request")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(b)), ContentLength: int64(len(b))}, nil
	})}
	m := newManager(context.Background(), t.TempDir(), c, key, func() time.Time { return time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC) })
	t.Cleanup(m.Close)
	return m, f, calls
}
func installFixture(t testing.TB, m *Manager) {
	t.Helper()
	if e := m.Check(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	if e := m.Install(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestPointerAndIndexContracts(t *testing.T) {
	m, f, _ := fixtureManager(t)
	p, _ := verifyEnvelope(un64(f.Pointer), m.key)
	for _, tc := range []struct {
		field string
		value any
	}{{"protocol", "v1"}, {"atlas_id", "../not-safe"}, {"index", "https://attacker/"}, {"issued_at", "2026-12-20T00:00:00Z"}, {"expires_at", "2026-01-01T00:00:00Z"}, {"index_sha256", "wrong"}} {
		t.Run(tc.field, func(t *testing.T) {
			copy := map[string]any{}
			for k, v := range p {
				copy[k] = v
			}
			copy[tc.field] = tc.value
			if _, e := m.pointer(signedTest(t, copy)); e == nil {
				t.Fatal("invalid pointer accepted")
			}
		})
	}
	good, e := m.pointer(un64(f.Pointer))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.index(append(un64(f.Index), ' '), good); e == nil {
		t.Fatal("index hash ignored")
	}
	v, _ := verifyEnvelope(un64(f.Index), m.key)
	v["renderer_schema"] = json.Number("999")
	raw := signedTest(t, v)
	good.Hash = digest(raw)
	if _, e = m.index(raw, good); e == nil {
		t.Fatal("renderer mismatch")
	}
	for _, path := range []string{"../latest.json", "releases/0123456789abcdef01234567/tiles/xz/7/0-0.png?secret=x", "https://evil/", `releases/0123456789abcdef01234567/tiles/xz/7/..\a`} {
		if networkPath(path) {
			t.Fatal("unsafe network path", path)
		}
	}
}
func TestConditionalDailyAndDisabledChecks(t *testing.T) {
	m, f, calls := fixtureManager(t)
	if e := m.Check(context.Background(), true); e != nil || calls.Load() != 0 {
		t.Fatal("default network")
	}
	now := m.now()
	m.now = func() time.Time { return now }
	if e := m.Configure(Preferences{Automatic: true}); e != nil {
		t.Fatal(e)
	}
	if e := m.Check(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	before := calls.Load()
	m.Check(context.Background(), true)
	if calls.Load() != before {
		t.Fatal("daily cap ignored")
	}
	now = now.Add(25 * time.Hour)
	m.client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.Header.Get("If-None-Match") != "fixture-etag" {
			t.Fatal("conditional missing")
		}
		return &http.Response{StatusCode: 304, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	m.state.ETag = "fixture-etag"
	if e := m.Check(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != before+1 {
		t.Fatal("unexpected requests")
	}
	m.Configure(Preferences{Automatic: true, Metered: true})
	now = now.Add(25 * time.Hour)
	m.Check(context.Background(), true)
	if calls.Load() != before+1 {
		t.Fatal("metered network")
	}
	_ = f
}
func TestRedirectAndHTTPFailures(t *testing.T) {
	m, _, _ := fixtureManager(t)
	for _, target := range []string{"https://evil.invalid/atlas/latest.json", "http://eridion.fitoserver.org/atlas/latest.json", "https://eridion.fitoserver.org/auth/frontier/callback"} {
		req, _ := http.NewRequest("GET", target, nil)
		if m.client.CheckRedirect(req, nil) == nil {
			t.Fatal("unsafe redirect")
		}
	}
	m.client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if m.Check(context.Background(), false) == nil {
		t.Fatal("404 accepted")
	}
}
func TestUnsafeArchivesAndPNGs(t *testing.T) {
	m, f, _ := fixtureManager(t)
	p, _ := m.pointer(un64(f.Pointer))
	idx, _ := m.index(un64(f.Index), p)
	for _, name := range []string{"../escape", "/absolute", "xz/4/0-0.png", "atlas.json"} {
		var out bytes.Buffer
		z := zip.NewWriter(&out)
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		if name == "xz/4/0-0.png" {
			h.SetMode(os.ModeSymlink | 0777)
		}
		w, _ := z.CreateHeader(h)
		w.Write([]byte("bad"))
		z.Close()
		if _, e := openOverview(out.Bytes(), idx, nil); e == nil {
			t.Fatal("unsafe ZIP accepted", name)
		}
	}
	if _, e := openOverview(un64(f.Overview)[:100], idx, nil); e == nil {
		t.Fatal("truncated archive")
	}
	if validPNG(un64(f.Tile), 64, 256) == nil || validPNG([]byte("notPNG"), 256, 256) == nil {
		t.Fatal("invalid PNG accepted")
	}
}
func TestDiskFullPreservesWorkingRelease(t *testing.T) {
	m, _, _ := fixtureManager(t)
	installFixture(t, m)
	id := m.Status().Installed
	m.write = func(root, name string, b []byte) error {
		if name == "overview.zip" {
			return syscall.ENOSPC
		}
		return writeFile(root, name, b)
	}
	if e := m.Install(context.Background()); !errors.Is(e, syscall.ENOSPC) {
		t.Fatal("disk full injection not reached", e)
	}
	if m.Status().Installed != id {
		t.Fatal("lost active release")
	}
	if _, e := m.Asset(id, "xz/0/0-0.png"); e != nil {
		t.Fatal("lost usable old assets", e)
	}
	entries, _ := os.ReadDir(m.root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".staging-") {
			t.Fatal("incomplete staging leaked")
		}
	}
}
func TestVisibleSparseDetailCacheAndReadOnlyViewer(t *testing.T) {
	m, f, calls := fixtureManager(t)
	installFixture(t, m)
	q := ViewRequest{Frame: Frame{"x": {-65000, -64000}, "y": {-15000, 15000}, "z": {94000, 95000}}, Pixels: 1024}
	before := calls.Load()
	v, e := m.View(q, true)
	if e != nil {
		t.Fatal(e)
	}
	if v.Pending || calls.Load() != before {
		t.Fatal("unconsented detail request")
	}
	if e = m.Configure(Preferences{Details: true}); e != nil {
		t.Fatal(e)
	}
	m.View(q, false)
	if calls.Load() != before {
		t.Fatal("viewer initiated network")
	}
	v, e = m.View(q, true)
	if e != nil || !v.Pending {
		t.Fatal("detail not scheduled", e)
	}
	m.wg.Wait()
	v, e = m.View(q, true)
	if e != nil || len(v.Tiles) != 3 || v.Pending {
		t.Fatal("missing visible result", v, e)
	}
	for _, tile := range v.Tiles {
		if tile.SourceLevel != 7 {
			t.Fatal("detail never replaced fallback")
		}
	}
	if calls.Load()-before != 6 {
		t.Fatalf("downloaded more than three visible tiles/catalogs: %d", calls.Load()-before)
	}
	if m.Status().CacheBytes > maxDetailCache+maxOverview*3 {
		t.Fatal("cache unbounded")
	}
	oldCalls := calls.Load()
	root := m.root
	m.Close()
	other := newManager(context.Background(), root, m.client, m.key, m.now)
	defer other.Close()
	other.View(q, false)
	if calls.Load() != oldCalls {
		t.Fatal("restart network")
	}
	id := other.Status().Installed
	if _, e = other.Asset(id, "tiles/xz/7/0-0.png"); e != nil {
		t.Fatal("persistent detail lost", e)
	}
	if e = other.ClearDetails(); e != nil {
		t.Fatal(e)
	}
	if _, e = other.Asset(id, "tiles/xz/7/0-0.png"); e == nil {
		t.Fatal("clear did not remove detail")
	}
	if _, e = other.Asset(id, "xz/0/0-0.png"); e != nil {
		t.Fatal("clear damaged overview")
	}
	_ = f
}
func TestCatalogCorruptionAndCacheBudget(t *testing.T) {
	m, _, _ := fixtureManager(t)
	installFixture(t, m)
	r := m.current
	if e := writeFile(r.dir, "catalogs/xz-L7.json", []byte("{}")); e != nil {
		t.Fatal(e)
	}
	if _, e := m.catalog(r, "xz", 7, false, context.Background()); e == nil {
		t.Fatal("corrupt catalog trusted")
	}
	if _, e := m.Asset(r.index.ID, "../state.json"); e == nil {
		t.Fatal("arbitrary file")
	}
	m.cacheMu.Lock()
	for i := 0; i < 4; i++ {
		p := filepath.Join(m.root, fmt.Sprintf("fake-%d", i))
		os.WriteFile(p, []byte("test-only"), 0600)
		m.disk[p] = diskEntry{64 << 20, time.Now().Add(time.Duration(i) * time.Second)}
	}
	m.evictLocked()
	if m.cacheBytes > maxDetailCache || len(m.disk) > 2 {
		t.Fatal("eviction budget")
	}
	m.cacheMu.Unlock()
}
func TestCancellationAndConcurrency(t *testing.T) {
	m, f, _ := fixtureManager(t)
	installFixture(t, m)
	m.Configure(Preferences{Details: true})
	var active, maxActive atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	original := m.client.Transport
	m.client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, ".png") {
			return original.RoundTrip(r)
		}
		n := active.Add(1)
		defer active.Add(-1)
		for {
			old := maxActive.Load()
			if n <= old || maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-release:
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(un64(f.Tile))), ContentLength: int64(len(un64(f.Tile)))}, nil
		}
	})
	q := ViewRequest{Frame: Frame{"x": {-65000, -64000}, "y": {-15000, 15000}, "z": {94000, 95000}}, Pixels: 1024}
	m.View(q, true)
	<-started
	<-started
	q.Frame = Frame{"x": {64000, 65000}, "y": {-15000, 15000}, "z": {-35000, -34000}}
	m.View(q, true)
	close(release)
	m.wg.Wait()
	if maxActive.Load() > 2 || active.Load() != 0 {
		t.Fatal("unbounded/cancelled workers")
	}
}

func TestNewReleaseRollbackAndImmutableIdentity(t *testing.T) {
	m, f, _ := fixtureManager(t)
	installFixture(t, m)
	first := m.Status().Installed
	p, _ := verifyEnvelope(un64(f.Pointer), m.key)
	idx, _ := verifyEnvelope(un64(f.Index), m.key)
	second := "1123456789abcdef01234567"
	idx["atlas_id"] = second
	raw := signedTest(t, idx)
	p["atlas_id"] = second
	p["index"] = "releases/" + second + "/index.json"
	p["index_sha256"] = digest(raw)
	pointer := signedTest(t, p)
	m.client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		b := pointer
		if strings.HasSuffix(r.URL.Path, "index.json") {
			b = raw
		}
		if strings.HasSuffix(r.URL.Path, "overview.zip") {
			b = un64(f.Overview)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b)), Header: http.Header{}, ContentLength: int64(len(b))}, nil
	})
	installFixture(t, m)
	if m.Status().Installed != second || m.Status().Previous != first {
		t.Fatal("previous release not retained")
	}
	if e := m.Rollback(); e != nil {
		t.Fatal(e)
	}
	if m.Status().Installed != first {
		t.Fatal("rollback not active")
	}
	idx["attribution"] = "Changed immutable content"
	raw = signedTest(t, idx)
	p["index_sha256"] = digest(raw)
	pointer = signedTest(t, p)
	if e := m.Check(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	if e := m.Install(context.Background()); e == nil || !strings.Contains(e.Error(), "collision") {
		t.Fatal("immutable ID changed", e)
	}
	if m.Status().Installed != first {
		t.Fatal("bad update changed active version")
	}
}
func TestDuplicateBombAndWrongCatalogIdentity(t *testing.T) {
	m, f, _ := fixtureManager(t)
	p, _ := m.pointer(un64(f.Pointer))
	idx, _ := m.index(un64(f.Index), p)
	z, _ := zip.NewReader(bytes.NewReader(un64(f.Overview)), int64(len(un64(f.Overview))))
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for _, f := range z.File {
		rd, _ := f.Open()
		b, _ := io.ReadAll(rd)
		rd.Close()
		file, _ := w.Create(f.Name)
		file.Write(b)
	}
	dup, _ := w.Create("xz/0/0-0.png")
	dup.Write(un64(f.Tile))
	w.Close()
	if _, e := openOverview(out.Bytes(), idx, nil); e == nil || !strings.Contains(e.Error(), "duplicate") {
		t.Fatal("duplicate ZIP accepted", e)
	}
	out.Reset()
	w = zip.NewWriter(&out)
	bomb, _ := w.Create("xz/0/0-0.png")
	bomb.Write(make([]byte, 5<<20))
	w.Close()
	if _, e := openOverview(out.Bytes(), idx, nil); e == nil || !strings.Contains(e.Error(), "limit") {
		t.Fatal("extraction bomb not bounded", e)
	}
	installFixture(t, m)
	r := m.current
	cat := un64(f.Catalogs["catalogs/xz-L7.json"])
	cat = bytes.Replace(cat, []byte(`"atlas_id":"0123456789abcdef01234567"`), []byte(`"atlas_id":"1123456789abcdef01234567"`), 1)
	for i := range r.index.Catalogs {
		if r.index.Catalogs[i].Path == "catalogs/xz-L7.json" {
			r.index.Catalogs[i].SHA256 = digest(cat)
		}
	}
	writeFile(r.dir, "catalogs/xz-L7.json", cat)
	if _, e := m.catalog(r, "xz", 7, false, context.Background()); e == nil {
		t.Fatal("foreign release catalog accepted")
	}
}

func BenchmarkAtlasCachedStartup(b *testing.B) {
	f, key := testFixture(b)
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		var data []byte
		switch {
		case strings.HasSuffix(r.URL.Path, "latest.json"):
			data = un64(f.Pointer)
		case strings.HasSuffix(r.URL.Path, "index.json"):
			data = un64(f.Index)
		default:
			data = un64(f.Overview)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data))}, nil
	})}
	root := b.TempDir()
	now := func() time.Time { return time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC) }
	m := newManager(context.Background(), root, client, key, now)
	installFixture(b, m)
	m.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m = newManager(context.Background(), root, client, key, now)
		if m.Status().Installed == "" {
			b.Fatal("cached atlas missing")
		}
		m.Close()
	}
}
