package atlas

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Cold entries exercise disk accounting/eviction without allocating 128 MiB of
// PNG payloads. The displayed fallback and incoming tile use signed fixtures and
// are checked through Asset(), including the actual catalog/size/hash checks.
func fillColdCache(t *testing.T, m *Manager, target int64) []string {
	t.Helper()
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	var total int64
	for _, e := range m.disk {
		total += e.bytes
	}
	var paths []string
	for i := 0; total < target; i++ {
		p := filepath.Join(m.current.dir, "details", "tiles", "yz", "7", fmt.Sprintf("%d-%d.png", 1+i%31, i/31))
		if _, exists := m.disk[p]; exists {
			continue
		}
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		size := min(int64(4<<20), target-total)
		f, e := os.Create(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = f.Truncate(size); e != nil {
			f.Close()
			t.Fatal(e)
		}
		f.Close()
		age := time.Unix(int64(i+1), 0)
		if e := os.Chtimes(p, age, age); e != nil {
			t.Fatal(e)
		}
		m.disk[p] = diskEntry{size, age}
		paths = append(paths, p)
		total += size
	}
	m.cacheBytes = total
	return paths
}
func fallbackViewport() ViewRequest {
	return ViewRequest{Frame: Frame{"x": {-65000, -64000}, "y": {-15000, 15000}, "z": {94000, 95000}}, Pixels: 1024}
}
func emptyViewport() ViewRequest {
	return ViewRequest{Frame: Frame{"x": {64000, 65000}, "y": {-15000, 15000}, "z": {-35000, -34000}}, Pixels: 1024}
}
func seedDisplayedFallback(t *testing.T, m *Manager) string {
	t.Helper()
	installFixture(t, m)
	if e := m.downloadTile(context.Background(), m.current, wanted{"xz", 5, 0, 0}); e != nil {
		t.Fatal(e)
	}
	v, e := m.View(fallbackViewport(), true)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, tile := range v.Tiles {
		if tile.Axes == "xz" && tile.SourceLevel == 5 {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture did not display L5 fallback")
	}
	p := filepath.Join(m.current.dir, "details", "tiles", "xz", "5", "0-0.png")
	m.cacheMu.Lock()
	entry := m.disk[p]
	entry.used = time.Unix(0, 0)
	if e := os.Chtimes(p, entry.used, entry.used); e != nil {
		t.Fatal(e)
	}
	m.disk[p] = entry
	m.cacheMu.Unlock()
	return p
}
func TestAutomaticEvictionPreservesDisplayedFallback(t *testing.T) {
	m, _, _ := fixtureManager(t)
	fallback := seedDisplayedFallback(t, m)
	cold := fillColdCache(t, m, maxDetailCache-1)
	if e := m.downloadTile(context.Background(), m.current, wanted{"xz", 7, 0, 0}); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Asset(m.current.index.ID, "tiles/xz/5/0-0.png"); e != nil {
		t.Fatal("automatic eviction removed displayed verified fallback", e)
	}
	if _, e := m.Asset(m.current.index.ID, "tiles/xz/7/0-0.png"); e != nil {
		t.Fatal("incoming visible tile unreadable", e)
	}
	if _, e := os.Stat(cold[0]); !os.IsNotExist(e) {
		t.Fatal("oldest nonvisible entry was not evicted", e)
	}
	if _, e := os.Stat(cold[len(cold)-1]); e != nil {
		t.Fatal("newer nonvisible entry unnecessarily evicted", e)
	}
	// A repeat of the same viewport now displays L7, not its historic L5 source.
	if _, e := m.View(fallbackViewport(), true); e != nil {
		t.Fatal(e)
	}
	m.cacheMu.Lock()
	historicPin := m.visible[fallback]
	m.cacheMu.Unlock()
	if historicPin {
		t.Fatal("replaced fallback pinned for entire historic viewport")
	}
	if _, e := m.View(emptyViewport(), true); e != nil {
		t.Fatal(e)
	}
	m.cacheMu.Lock()
	entry := m.disk[fallback]
	entry.used = time.Unix(0, 0)
	m.disk[fallback] = entry
	m.cacheMu.Unlock()
	fillColdCache(t, m, maxDetailCache-1)
	if e := m.downloadTile(context.Background(), m.current, wanted{"yz", 7, 0, 0}); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(fallback); !os.IsNotExist(e) {
		t.Fatal("old viewport pin retained after pan", e)
	}
	m.cacheMu.Lock()
	over := m.cacheBytes > maxDetailCache
	m.cacheMu.Unlock()
	if over {
		t.Fatal("normal budget did not resume")
	}
}

func TestEvictionVisibleWorkingSetBudget(t *testing.T) {
	m, _, _ := fixtureManager(t)
	installFixture(t, m)
	// Policy-only sparse files stand in for a large verified working set; the
	// integration test above checks genuine signed fallback/incoming asset reads.
	paths := fillColdCache(t, m, 144<<20)
	m.cacheMu.Lock()
	m.visible = map[string]bool{}
	for _, p := range paths[:35] {
		m.visible[p] = true
	}
	m.evictLocked()
	m.cacheMu.Unlock()
	s := m.Status()
	if s.DetailBytes != 140<<20 || s.VisibleDetailBytes != 140<<20 || s.DetailBudgetBytes != 140<<20 || s.WorkingSetLimitBytes != maxWorkingSetBytes {
		t.Fatalf("working-set budget not explicit: %+v", s)
	}
	for _, p := range paths[:35] {
		if _, e := os.Stat(p); e != nil {
			t.Fatal("visible working-set entry evicted", e)
		}
	}
	if _, e := os.Stat(paths[35]); !os.IsNotExist(e) {
		t.Fatal("over-budget nonvisible file retained", e)
	}
	if _, e := m.View(emptyViewport(), true); e != nil {
		t.Fatal(e)
	}
	s = m.Status()
	if s.DetailBytes > maxDetailCache || s.DetailBudgetBytes != maxDetailCache || s.VisibleDetailBytes != 0 {
		t.Fatalf("budget did not return on viewport change: %+v", s)
	}
	if _, e := os.Stat(paths[0]); !os.IsNotExist(e) {
		t.Fatal("old working-set pin not released", e)
	}
}

func TestEvictionDownloadCompletionAndViewportLockOrder(t *testing.T) {
	m, _, _ := fixtureManager(t)
	fallback := seedDisplayedFallback(t, m)
	fillColdCache(t, m, maxDetailCache-1)
	entered := make(chan struct{})
	finishWrite := make(chan struct{})
	defer closeIfOpen(finishWrite)
	original := m.write
	m.write = func(root, name string, b []byte) error {
		if strings.HasSuffix(name, "tiles/xz/7/0-0.png") {
			close(entered)
			<-finishWrite
		}
		return original(root, name, b)
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- m.downloadTile(context.Background(), m.current, wanted{"xz", 7, 0, 0}) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not reach insertion")
	}
	secondDone := make(chan error, 1)
	go func() { secondDone <- m.downloadTile(context.Background(), m.current, wanted{"xy", 7, 0, 0}) }()
	// The first download holds cacheMu at its write boundary. Holding m.mu here
	// proves its automatic eviction never attempts the forbidden reverse lock.
	m.mu.Lock()
	viewDone := make(chan error, 1)
	go func() { _, e := m.View(emptyViewport(), true); viewDone <- e }()
	close(finishWrite)
	select {
	case e := <-firstDone:
		if e != nil {
			m.mu.Unlock()
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		m.mu.Unlock()
		t.Fatal("cacheMu -> m.mu lock-order regression")
	}
	_, presentError := os.Stat(fallback)
	m.mu.Unlock()
	if presentError != nil {
		t.Fatal("completion evicted still-displayed fallback", presentError)
	}
	for _, done := range []chan error{viewDone, secondDone} {
		select {
		case e := <-done:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent viewport/download did not finish")
		}
	}
	m.cacheMu.Lock()
	oldPins := len(m.visible)
	m.cacheMu.Unlock()
	if oldPins != 0 || m.Status().DetailBytes > maxDetailCache {
		t.Fatal("late completion restored historic pins or exceeded normal budget")
	}
	if e := m.ClearDetails(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(fallback); !os.IsNotExist(e) {
		t.Fatal("cleanup after pan retained old source", e)
	}
}
func closeIfOpen(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}

func TestScanCachePreservesDisplayedFallback(t *testing.T) {
	m, _, _ := fixtureManager(t)
	seedDisplayedFallback(t, m)
	cold := fillColdCache(t, m, maxDetailCache+1)
	m.scanCache()
	if _, e := m.Asset(m.current.index.ID, "tiles/xz/5/0-0.png"); e != nil {
		t.Fatal("scan evicted displayed fallback", e)
	}
	if _, e := os.Stat(cold[0]); !os.IsNotExist(e) {
		t.Fatal("scan did not evict oldest nonvisible entry", e)
	}
	if m.Status().DetailBytes > maxDetailCache {
		t.Fatal("scan exceeded normal budget")
	}
}
