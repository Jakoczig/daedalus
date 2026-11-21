package atlas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSpatialSelectiveQueueReuseAndCorruption(t *testing.T) {
	files, key := spatialFixture(t)
	var mu sync.Mutex
	var paths []string
	var active, peak atomic.Int32
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		p := strings.TrimPrefix(r.URL.Path, "/atlas/spatial/")
		mu.Lock()
		paths = append(paths, p)
		mu.Unlock()
		b := files[p]
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), ContentLength: int64(len(b)), Header: http.Header{}}, nil
	})}
	dir := t.TempDir()
	m := newSpatial(context.Background(), dir, client, key, time.Now)
	defer m.Close()
	m.Configure(SpatialPreferences{Downloads: true})
	if e := m.Update(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	q := SpatialQuery{Low: [3]float64{-2, -2, -2}, High: [3]float64{2, 2, 2}, Route: [][3]float64{{800, 0, 0}}}
	mu.Lock()
	before := len(paths)
	mu.Unlock()
	v, e := m.View(q, false)
	if e != nil || v.Complete || v.Missing == 0 || len(v.Points) != 0 {
		t.Fatal(v, e)
	}
	mu.Lock()
	if len(paths) != before {
		t.Fatal("viewer downloaded")
	}
	mu.Unlock()
	m.View(q, true)
	m.wg.Wait()
	v, e = m.View(q, false)
	if e != nil || !v.Complete || len(v.Points) != 4 {
		t.Fatal(v, e)
	}
	m.wg.Wait()
	if peak.Load() > 2 {
		t.Fatal("unbounded downloads")
	}
	mu.Lock()
	count := len(paths)
	first := paths[before]
	mu.Unlock()
	if !strings.Contains(first, "indexes/") {
		t.Fatal("blocks before index")
	}
	m.View(q, true)
	m.wg.Wait()
	if e = m.Update(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	if len(paths) != count+2 {
		t.Fatal("unchanged content redownloaded", paths[count:])
	}
	mu.Unlock()
	// New release metadata retains content-addressed bytes; corrupted child must
	// never become a successful zero-position response after restart.
	p, _ := m.pointer(files["latest.json"], true)
	d, _ := m.distribution(files[p.Path], p)
	manifest := files[d.Spatial.Path]
	p.ID = strings.Repeat("b", 24)
	p.Path = "releases/" + p.ID + "/distribution.json"
	p.Issued = p.Issued.Add(time.Second)
	d.ID = p.ID
	d.Spatial.Path = "releases/" + p.ID + "/spatial.json"
	files[p.Path] = spatialSignTest(d)
	p.Hash = digest(files[p.Path])
	files[d.Spatial.Path] = manifest
	files["latest.json"] = spatialSignTest(p)
	if e = m.Update(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	afterUpdate := len(paths)
	mu.Unlock()
	v, e = m.View(q, false)
	if e != nil || !v.Complete {
		t.Fatal("cross-release child reuse", v, e)
	}
	mu.Lock()
	if len(paths) != afterUpdate {
		t.Fatal("cross-release redownload")
	}
	mu.Unlock()
	if e = m.Rollback(); e != nil {
		t.Fatal("offline rollback", e)
	}
	if v, e = m.View(q, false); e != nil || !v.Complete {
		t.Fatal("rollback lost coordinates", e)
	}
	m.Close()
	es, _ := os.ReadDir(filepath.Join(dir, "assets", "blocks"))
	if len(es) == 0 {
		t.Fatal("no blocks")
	}
	for _, f := range es {
		os.WriteFile(filepath.Join(dir, "assets", "blocks", f.Name()), []byte("bad"), 0600)
	}
	m = newSpatial(context.Background(), dir, client, key, time.Now)
	defer m.Close()
	v, e = m.View(q, false)
	if e != nil || v.Complete || v.Missing == 0 || len(v.Points) != 0 {
		t.Fatal("corruption looked empty", v, e)
	}
}
func TestSpatialQueueCancellationAndOwnerConsent(t *testing.T) {
	files, key := spatialFixture(t)
	started := make(chan struct{})
	canceled := make(chan struct{})
	var cancelOnce sync.Once
	var once sync.Once
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		p := strings.TrimPrefix(r.URL.Path, "/atlas/spatial/")
		if strings.HasPrefix(p, "assets/") {
			once.Do(func() { close(started) })
			<-r.Context().Done()
			cancelOnce.Do(func() { close(canceled) })
			return nil, r.Context().Err()
		}
		b := files[p]
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), ContentLength: int64(len(b)), Header: http.Header{}}, nil
	})}
	m := newSpatial(context.Background(), t.TempDir(), client, key, time.Now)
	defer m.Close()
	m.Configure(SpatialPreferences{Downloads: true})
	if e := m.Update(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	q := SpatialQuery{Low: [3]float64{-2, -2, -2}, High: [3]float64{2, 2, 2}}
	m.View(q, true)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("no download")
	}
	q.Route = [][3]float64{{800, 0, 0}}
	m.View(q, true)
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("route change did not cancel obsolete request")
	}
	m.Configure(SpatialPreferences{Downloads: true, Offline: true})
	m.wg.Wait()
	if m.Status().Pending {
		t.Fatal("queue not canceled")
	}
	v, _ := m.View(q, false)
	if v.Complete {
		t.Fatal("canceled incomplete result accepted")
	}
}
func TestSpatialBudgetsAndEvictionPreserveBubble(t *testing.T) {
	m := newSpatial(context.Background(), t.TempDir(), http.DefaultClient, make([]byte, 32), time.Now)
	defer m.Close()
	m.current = &spatialManifest{RootLY: 100, Roots: make([]spatialRoot, 257)}
	for i := range m.current.Roots {
		m.current.Roots[i] = spatialRoot{Cell: [3]int64{int64(i), 0, 0}, SHA256: strings.Repeat("a", 64), Bytes: 100, Count: 1}
	}
	v, _ := m.View(SpatialQuery{Low: [3]float64{0, 0, 0}, High: [3]float64{30000, 1, 1}}, false)
	if v.Complete || !strings.Contains(v.Notice, "budget") || len(v.Points) != 0 {
		t.Fatal(v)
	}
	// Sparse files exercise disk policy without allocating large buffers.
	writeFile(m.root, "assets/blocks/keep.z", []byte("pin"))
	m.pins = map[string]bool{"assets/blocks/keep.z": true}
	writeFile(m.root, "bubbles/keep.zip", []byte("permanent"))
	p := filepath.Join(m.root, "assets/blocks/old.z")
	f, _ := os.Create(p)
	f.Truncate(spatialCacheBudget + 1)
	f.Close()
	m.ioMu.Lock()
	m.evict()
	m.ioMu.Unlock()
	if _, e := os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("cache not evicted")
	}
	for _, p := range []string{"assets/blocks/keep.z", "bubbles/keep.zip"} {
		if _, e := os.Stat(filepath.Join(m.root, p)); e != nil {
			t.Fatal("lost protected cache", p)
		}
	}
}
func TestSpatialUnsafePathsAndVectorDimensions(t *testing.T) {
	for _, p := range []string{"../latest.json", "https://evil.test/latest.json", "assets/blocks/../../secret.z", "assets/blocks/" + strings.Repeat("A", 64) + ".z", "latest.json?q=secret"} {
		if spatialNetworkPath(p) {
			t.Fatal(p)
		}
	}
	b, _ := os.ReadFile("testdata/spatial-v1/spatial.json")
	var v map[string]any
	json.Unmarshal(b, &v)
	roots := v["roots"].([]any)
	roots[0].(map[string]any)["cell"] = []int{0, 0}
	b, _ = json.Marshal(v)
	if _, e := parseSpatial(b); e == nil {
		t.Fatal("short vector")
	}
}
