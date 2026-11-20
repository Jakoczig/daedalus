package atlas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxProjectionTiles = 96

// At most 3 projections, each with 96 desired files and 96 displayed fallback
// sources. Each verified PNG is <=4 MiB: a finite 2.25 GiB worst-case working
// set, not a reserved cache allocation. Normal retention remains 128 MiB.
const maxWorkingSetBytes = int64(3*maxProjectionTiles*2) * (4 << 20)

type Frame map[string][2]float64
type ViewRequest struct {
	Frame   Frame `json:"frame"`
	Pixels  int   `json:"pixels"`
	Compact bool  `json:"compact"`
}
type Tile struct {
	Axes        string `json:"axes"`
	Level       int    `json:"level"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	SourceLevel int    `json:"sourceLevel"`
	SourceX     int    `json:"sourceX"`
	SourceY     int    `json:"sourceY"`
	URL         string `json:"url"`
}
type View struct {
	Installed   string                `json:"installed"`
	Extents     map[string][2]float64 `json:"extents,omitempty"`
	Tiles       []Tile                `json:"tiles"`
	Pending     bool                  `json:"pending"`
	Source      string                `json:"source"`
	Attribution string                `json:"attribution"`
	MaxLevel    int                   `json:"maxLevel"`
}
type catalog struct {
	Protocol string  `json:"protocol"`
	ID       string  `json:"atlas_id"`
	Axes     string  `json:"axes"`
	Level    int     `json:"level"`
	Count    int     `json:"count"`
	Bytes    int64   `json:"total_bytes"`
	Tiles    []Asset `json:"tiles"`
}
type wanted struct {
	axes        string
	level, x, y int
}

func selectTiles(meta Metadata, q ViewRequest) ([]wanted, error) {
	if len(q.Frame) != 3 || q.Pixels < 64 || q.Pixels > 2048 {
		return nil, errors.New("invalid atlas viewport")
	}
	for _, a := range []string{"x", "y", "z"} {
		v, ok := q.Frame[a]
		if !ok || !finite(v[0]) || !finite(v[1]) || v[1]-v[0] < 5 || v[1]-v[0] > 4e6 || math.Abs(v[0]) > 2e6 || math.Abs(v[1]) > 2e6 {
			return nil, errors.New("invalid atlas frame")
		}
	}
	byAxes := [][]wanted{}
	axesList := []string{"xz", "xy", "yz"}
	if q.Compact {
		axesList = axesList[:1]
	}
	for _, axes := range axesList {
		a, b := string(axes[0]), string(axes[1])
		ex, ey := meta.Extents[a], meta.Extents[b]
		fx, fy := q.Frame[a], q.Frame[b]
		w, h := dimensions(axes, 0)
		pw, ph := float64(q.Pixels), float64(q.Pixels)
		if axes == "xy" {
			ph /= 8
		}
		if axes == "yz" {
			pw /= 8
		}
		need := math.Max(pw*(ex[1]-ex[0])/(fx[1]-fx[0])/float64(w), ph*(ey[1]-ey[0])/(fy[1]-fy[0])/float64(h))
		n := max(0, min(7, int(math.Ceil(math.Log2(math.Max(1, need))))))
		l := meta.Views[axes].Levels[strconv.Itoa(n)]
		x0 := max(0, int(math.Floor((fx[0]-ex[0])/(ex[1]-ex[0])*float64(l.Width)/256)))
		x1 := min(l.Columns-1, int(math.Ceil((fx[1]-ex[0])/(ex[1]-ex[0])*float64(l.Width)/256))-1)
		y0 := max(0, int(math.Floor((ey[1]-fy[1])/(ey[1]-ey[0])*float64(l.Height)/256)))
		y1 := min(l.Rows-1, int(math.Ceil((ey[1]-fy[0])/(ey[1]-ey[0])*float64(l.Height)/256))-1)
		items := []wanted{}
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if l.occupied(x, y) {
					items = append(items, wanted{axes, n, x, y})
					if len(items) > maxProjectionTiles {
						return nil, errors.New("viewport tile bound")
					}
				}
			}
		}
		byAxes = append(byAxes, items)
	}
	out := []wanted{}
	for i := 0; ; i++ {
		added := false
		for _, list := range byAxes {
			if i < len(list) {
				out = append(out, list[i])
				added = true
			}
		}
		if !added {
			break
		}
	}
	return out, nil
}
func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }
func (m *Manager) catalog(r *release, axes string, level int, online bool, ctx context.Context) (map[string]Asset, error) {
	name := fmt.Sprintf("catalogs/%s-L%d.json", axes, level)
	r.catalogsMu.Lock()
	defer r.catalogsMu.Unlock()
	if r.catalogs == nil {
		r.catalogs = map[string]map[string]Asset{}
	}
	if v, ok := r.catalogs[name]; ok {
		return v, nil
	}
	var desc Asset
	for _, d := range r.index.Catalogs {
		if d.Path == name {
			desc = d
			break
		}
	}
	if desc.Path == "" {
		return nil, errors.New("missing signed catalog")
	}
	b, e := readFile(r.dir, name, 12<<20)
	oldSize := int64(len(b))
	if e == nil && (int64(len(b)) != desc.Bytes || digest(b) != desc.SHA256) {
		e = errors.New("catalog hash mismatch")
	}
	if e != nil && online {
		b, _, _, e = m.fetch(ctx, "releases/"+r.index.ID+"/"+name, 12<<20, &desc, "")
		if e == nil {
			e = m.write(r.dir, name, b)
			if e == nil {
				m.cacheMu.Lock()
				m.baseBytes += int64(len(b)) - oldSize
				m.cacheMu.Unlock()
			}
		}
	}
	if e != nil {
		return nil, e
	}
	if int64(len(b)) != desc.Bytes || digest(b) != desc.SHA256 {
		return nil, errors.New("catalog hash mismatch")
	}
	var cat catalog
	if e = decodeStrict(b, &cat, 12<<20); e != nil {
		return nil, e
	}
	l := r.meta.Views[axes].Levels[strconv.Itoa(level)]
	if cat.Protocol != "eridion-atlas-tile-catalog-v1" || cat.ID != r.index.ID || cat.Axes != axes || cat.Level != level || cat.Count != len(cat.Tiles) || cat.Count != desc.Count || cat.Count != l.Count {
		return nil, errors.New("catalog identity/count mismatch")
	}
	assets := map[string]Asset{}
	var total int64
	for _, a := range cat.Tiles {
		p := detailPattern.FindStringSubmatch(a.Path)
		if p == nil || p[1] != axes || p[2] != strconv.Itoa(level) || !validAsset(a, 4<<20) {
			return nil, errors.New("invalid catalog tile")
		}
		x, _ := strconv.Atoi(p[3])
		y, _ := strconv.Atoi(p[4])
		if !l.occupied(x, y) || assets[a.Path].Path != "" {
			return nil, errors.New("catalog tile occupancy/duplicate")
		}
		assets[a.Path] = a
		total += a.Bytes
	}
	if total != cat.Bytes {
		return nil, errors.New("catalog size mismatch")
	}
	r.catalogs[name] = assets
	return assets, nil
}
func (m *Manager) asset(r *release, name string) ([]byte, error) {
	if a, ok := r.overview[name]; ok {
		b, e := readFile(r.dir, name, 4<<20)
		if e == nil && (int64(len(b)) != a.Bytes || digest(b) != a.SHA256) {
			e = errors.New("overview tile damaged")
		}
		return b, e
	}
	p := detailPattern.FindStringSubmatch(name)
	if p == nil {
		return nil, errors.New("not a verified atlas tile")
	}
	n, _ := strconv.Atoi(p[2])
	assets, e := m.catalog(r, p[1], n, false, m.ctx)
	if e != nil {
		return nil, e
	}
	a, ok := assets[name]
	if !ok {
		return nil, errors.New("tile absent from signed catalog")
	}
	b, e := readFile(r.dir, "details/"+name, 4<<20)
	if e == nil && (int64(len(b)) != a.Bytes || digest(b) != a.SHA256) {
		e = errors.New("cached tile hash mismatch")
	}
	if e == nil {
		m.cacheMu.Lock()
		p := filepath.Join(r.dir, "details", filepath.FromSlash(name))
		if v, ok := m.disk[p]; ok {
			v.used = m.now()
			m.disk[p] = v
		}
		m.cacheMu.Unlock()
	}
	return b, e
}
func (m *Manager) Asset(id, name string) ([]byte, error) {
	m.mu.Lock()
	r := m.current
	if r == nil || id != r.index.ID || m.state.Disabled || m.clearing {
		m.mu.Unlock()
		return nil, errors.New("atlas release not active")
	}
	m.mu.Unlock()
	return m.asset(r, name)
}
func (m *Manager) View(q ViewRequest, owner bool) (View, error) {
	if m.ctx.Err() != nil {
		return View{}, errors.New("Atlas service stopped")
	}
	m.mu.Lock()
	r := m.current
	if m.state.Disabled || m.clearing {
		r = nil
	}
	m.mu.Unlock()
	v := View{Tiles: []Tile{}, Source: sourceURL}
	if r == nil {
		return v, nil
	}
	desired, e := selectTiles(r.meta, q)
	if e != nil {
		return v, e
	}
	v.Installed = r.index.ID
	v.Extents = r.meta.Extents
	v.MaxLevel = 7
	v.Attribution = r.index.Attribution
	missing := []wanted{}
	cats := map[string]map[string]Asset{}
	// Load catalog metadata before either manager/cache lock. A catalog download
	// may hold catalogsMu and need m.mu for progress; never wait for it while pinned.
	for _, w := range desired {
		for n := w.level; n > 4; n-- {
			key := fmt.Sprintf("%s%d", w.axes, n)
			if _, ok := cats[key]; !ok {
				assets, _ := m.catalog(r, w.axes, n, false, m.ctx)
				cats[key] = assets
			}
		}
	}
	// One lock order: m.mu -> cacheMu. File selection plus publication is atomic
	// with eviction, so an asset cannot disappear between verification and pinning.
	m.mu.Lock()
	if m.current != r || m.ctx.Err() != nil || m.state.Disabled || m.clearing {
		m.mu.Unlock()
		return v, errors.New("Atlas view changed; retry")
	}
	online := owner && m.state.Details && !m.state.Metered
	m.cacheMu.Lock()
	for _, w := range desired {
		found := false
		for n := w.level; n >= 0; n-- {
			shift := w.level - n
			x, y := w.x>>shift, w.y>>shift
			name := fmt.Sprintf("%s/%d/%d-%d.png", w.axes, n, x, y)
			if n > 4 {
				name = "tiles/" + name
				catKey := fmt.Sprintf("%s%d", w.axes, n)
				a := cats[catKey]
				desc, ok := a[name]
				if !ok {
					continue
				}
				b, e := readFile(r.dir, "details/"+name, 4<<20)
				if e != nil || int64(len(b)) != desc.Bytes || digest(b) != desc.SHA256 {
					continue
				}
			} else {
				if _, ok := r.overview[name]; !ok {
					continue
				}
			}
			v.Tiles = append(v.Tiles, Tile{Axes: w.axes, Level: w.level, X: w.x, Y: w.y, SourceLevel: n, SourceX: x, SourceY: y, URL: "/atlas/assets/" + r.index.ID + "/" + name})
			found = n == w.level
			break
		}
		if !found && w.level > 4 {
			missing = append(missing, w)
		}
	}
	if owner {
		m.visible = map[string]bool{}
		// A visible high-LOD cell may currently use a cached lower-LOD tile.
		// Keep that actual source as well as its pending replacement.
		for _, tile := range v.Tiles {
			if tile.SourceLevel > 4 {
				name := strings.TrimPrefix(tile.URL, "/atlas/assets/"+r.index.ID+"/")
				m.visible[filepath.Join(r.dir, "details", filepath.FromSlash(name))] = true
			}
		}
		for _, w := range desired {
			if w.level > 4 {
				m.visible[filepath.Join(r.dir, "details", filepath.FromSlash(fmt.Sprintf("tiles/%s/%d/%d-%d.png", w.axes, w.level, w.x, w.y)))] = true
			}
		}

		m.evictLocked()
	}
	m.cacheMu.Unlock()
	if owner {
		keyBytes, _ := json.Marshal(q)
		key := r.index.ID + string(keyBytes)
		if key != m.viewKey {
			if m.viewCancel != nil {
				m.viewCancel()
			}
			m.viewKey = key
			m.viewPending = false
			if online && len(missing) > 0 && m.ctx.Err() == nil {
				ctx, cancel := context.WithCancel(m.ctx)
				m.viewCancel = cancel
				m.viewPending = true
				m.wg.Add(1)
				go m.downloadVisible(ctx, key, r, missing)
			}
		}
		v.Pending = m.viewPending
	}
	m.mu.Unlock()
	return v, nil
}
func (m *Manager) downloadVisible(ctx context.Context, key string, r *release, items []wanted) {
	defer m.wg.Done()
	defer func() {
		m.mu.Lock()
		if m.viewKey == key {
			m.viewPending = false
		}
		m.mu.Unlock()
	}()
	jobs := make(chan wanted)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range jobs {
				if ctx.Err() != nil {
					return
				}
				if e := m.downloadTile(ctx, r, w); e != nil && ctx.Err() == nil {
					m.fail(e)
				}
			}
		}()
	}
	for _, w := range items {
		select {
		case jobs <- w:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}
func (m *Manager) downloadTile(ctx context.Context, r *release, w wanted) error {
	select {
	case m.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-m.slots }()
	m.tileMu.Lock()
	assets, e := m.catalog(r, w.axes, w.level, true, ctx)
	m.tileMu.Unlock()
	if e != nil {
		return e
	}
	name := fmt.Sprintf("tiles/%s/%d/%d-%d.png", w.axes, w.level, w.x, w.y)
	a, ok := assets[name]
	if !ok {
		return errors.New("occupied tile absent from catalog")
	}
	if b, e := readFile(r.dir, "details/"+name, 4<<20); e == nil && int64(len(b)) == a.Bytes && digest(b) == a.SHA256 {
		return nil
	}
	b, _, _, e := m.fetch(ctx, "releases/"+r.index.ID+"/"+name, 4<<20, &a, "")
	if e != nil {
		return e
	}
	if e = validPNG(b, 256, 256); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if e = m.write(r.dir, "details/"+name, b); e != nil {
		return e
	}
	path := filepath.Join(r.dir, "details", filepath.FromSlash(name))
	m.disk[path] = diskEntry{int64(len(b)), m.now()}
	m.evictLocked()
	return nil
}

type diskEntry struct {
	bytes int64
	used  time.Time
}

func (m *Manager) scanCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.disk = map[string]diskEntry{}
	m.baseBytes = 0
	_ = filepath.WalkDir(m.root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(m.root, p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 1 && idPattern.MatchString(parts[0]) && parts[1] != "details" && !d.IsDir() {
			if i, e := d.Info(); e == nil && i.Mode().IsRegular() {
				m.baseBytes += i.Size()
			}
		}
		if len(parts) == 6 && idPattern.MatchString(parts[0]) && parts[1] == "details" && detailPattern.MatchString(strings.Join(parts[2:], "/")) {
			if i, e := d.Info(); e == nil && i.Mode().IsRegular() {
				m.disk[p] = diskEntry{i.Size(), i.ModTime()}
			}
		}
		return nil
	})
	m.evictLocked()
}

// cacheMu guards both the pin snapshot and LRU entries. Never acquire m.mu here.
func (m *Manager) evictLocked() {
	keep := map[string]bool{}
	for p, v := range m.visible {
		keep[p] = v
	}
	var total, pinned int64
	paths := make([]string, 0, len(m.disk))
	for p, e := range m.disk {
		total += e.bytes
		if keep[p] {
			pinned += e.bytes
		} else {
			paths = append(paths, p)
		}
	}
	// A genuine visible working set may temporarily exceed 128 MiB. In that
	// case retain exactly the pinned files, not extra historic/cache entries.
	budget := max(int64(maxDetailCache), pinned)
	if total <= budget {
		m.cacheBytes = total
		return
	}
	sort.Slice(paths, func(i, j int) bool {
		a, b := m.disk[paths[i]], m.disk[paths[j]]
		if a.used.Equal(b.used) {
			return paths[i] < paths[j]
		}
		return a.used.Before(b.used)
	})
	for _, p := range paths {
		if total <= budget {
			break
		}
		if e := os.Remove(p); e == nil || os.IsNotExist(e) {
			total -= m.disk[p].bytes
			delete(m.disk, p)
		}
	}
	m.cacheBytes = total
}
func (m *Manager) cacheStats() (total, detail, pinned, budget int64) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	for p := range m.visible {
		pinned += m.disk[p].bytes
	}
	return m.cacheBytes + m.baseBytes, m.cacheBytes, pinned, max(int64(maxDetailCache), pinned)
}
func (m *Manager) ClearDetails() error {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	for p := range m.disk {
		if m.visible[p] {
			continue
		}
		if e := os.Remove(p); e != nil && !os.IsNotExist(e) {
			return e
		}
		delete(m.disk, p)
	}
	m.evictLocked()
	return nil
}

// Caller holds m.mu; keep the sole nested lock order when changing releases.
func (m *Manager) resetVisibleLocked() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.visible = nil
	m.evictLocked()
}
func (m *Manager) pruneReleases() {
	m.mu.Lock()
	keep := map[string]bool{m.state.Installed: true, m.state.Previous: true}
	m.mu.Unlock()
	entries, _ := os.ReadDir(m.root)
	for _, e := range entries {
		if idPattern.MatchString(e.Name()) && e.IsDir() && !keep[e.Name()] && e.Type()&os.ModeSymlink == 0 {
			_ = os.RemoveAll(filepath.Join(m.root, e.Name()))
		}
	}
}
