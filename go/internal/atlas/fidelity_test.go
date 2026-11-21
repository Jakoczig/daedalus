package atlas

import (
	"strings"
	"testing"
)

func TestFidelityDiagnosticsRefineVerifiedFallbackWithoutStationaryDownloads(t *testing.T) {
	m, _, calls := fixtureManager(t)
	installFixture(t, m)
	q := ViewRequest{Frame: Frame{"x": {-65000, -64000}, "y": {-15000, 15000}, "z": {94000, 95000}}, Pixels: 1024, Compact: true}
	v, err := m.View(q, true)
	if err != nil {
		t.Fatal(err)
	}
	if v.RequestedLevels["xz"] != 7 || v.Missing == 0 || v.Pending || len(v.Tiles) == 0 || v.Tiles[0].SourceLevel > 4 {
		t.Fatal("fallback diagnostics", v)
	}
	before := calls.Load()
	if _, err = m.View(q, false); err != nil || calls.Load() != before {
		t.Fatal("viewer requested detail", err)
	}
	if err = m.Configure(Preferences{Details: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.View(q, true); err != nil {
		t.Fatal(err)
	}
	m.wg.Wait()
	v, err = m.View(q, true)
	if err != nil {
		t.Fatal(err)
	}
	if v.Missing != 0 || v.Pending || len(v.Tiles) == 0 || v.Tiles[0].SourceLevel != 7 {
		t.Fatal("refinement diagnostics", v)
	}
	for _, tile := range v.Tiles {
		if _, err = m.Asset(v.Installed, strings.TrimPrefix(tile.URL, "/atlas/assets/"+v.Installed+"/")); err != nil {
			t.Fatal("verified source not readable", err)
		}
	}
	before = calls.Load()
	for i := 0; i < 3; i++ {
		if _, err = m.View(q, true); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != before {
		t.Fatal("stationary viewport refetched detail")
	}
}

func TestProjectionPixelDimensionsDriveLODAndRemainBounded(t *testing.T) {
	m, _, _ := fixtureManager(t)
	installFixture(t, m)
	q := ViewRequest{Frame: Frame{"x": {-65000, 65000}, "y": {-15000, 15000}, "z": {-35000, 95000}}, Pixels: 256, Compact: true}
	a, err := selectTiles(m.current.meta, q)
	if err != nil {
		t.Fatal(err)
	}
	q.ProjectionPixels = map[string][2]int{"xz": {512, 512}}
	b, err := selectTiles(m.current.meta, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) == 0 || len(b) == 0 || b[0].level != a[0].level+1 {
		t.Fatal("device pixels did not refine LOD", a, b)
	}
	q.ProjectionPixels["xz"] = [2]int{4096, 4096}
	if _, err = selectTiles(m.current.meta, q); err == nil {
		t.Fatal("unbounded physical resolution")
	}
}
