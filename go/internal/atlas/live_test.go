package atlas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in read-only acceptance against the pinned public publisher.
// No CI/default test contacts any external endpoint; profile is disposable.
func TestLivePublisherAcceptance(t *testing.T) {
	if os.Getenv("ERIDION_ATLAS_LIVE") != "1" {
		t.Skip("explicit live read-only acceptance only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	m := New(ctx, root)
	start := time.Now()
	if e := m.Check(ctx, false); e != nil {
		t.Fatal(e)
	}
	if e := m.Install(ctx); e != nil {
		t.Fatal(e)
	}
	installMS := time.Since(start).Milliseconds()
	before := m.Status().Downloaded
	m.Configure(Preferences{Details: true})
	q := ViewRequest{Frame: Frame{"x": {-500, 500}, "y": {-500, 500}, "z": {-500, 500}}, Pixels: 1024}
	v, e := m.View(q, true)
	if e != nil {
		t.Fatal(e)
	}
	m.wg.Wait()
	v, e = m.View(q, true)
	if e != nil {
		t.Fatal(e)
	}
	details := 0
	for _, tile := range v.Tiles {
		if tile.SourceLevel == 7 {
			details++
		}
		name := tile.URL[len("/atlas/assets/")+len(v.Installed)+1:]
		if _, e := m.Asset(v.Installed, name); e != nil {
			t.Fatal(e)
		}
	}
	if details < 1 {
		t.Fatal("no real L7 tiles")
	}
	status := m.Status()
	detailBytes := status.Downloaded - before
	// Interrupt a real overview response locally. No server mutation or upload.
	original := m.client.Transport
	m.client.Transport = transport(func(r *http.Request) (*http.Response, error) {
		resp, e := original.RoundTrip(r)
		if e == nil && strings.HasSuffix(r.URL.Path, "overview.zip") {
			resp.Body = &cutBody{Reader: io.LimitReader(resp.Body, 8192), Closer: resp.Body}
		}
		return resp, e
	})
	if e = m.Install(ctx); e == nil {
		t.Fatal("interrupted overview accepted")
	}
	if m.Status().Installed != v.Installed {
		t.Fatal("interruption lost current release")
	}
	for _, tile := range v.Tiles {
		name := strings.TrimPrefix(tile.URL, "/atlas/assets/"+v.Installed+"/")
		if _, e = m.Asset(v.Installed, name); e != nil {
			t.Fatal("interruption lost cached tile", e)
		}
	}
	m.client.Transport = original
	m.Configure(Preferences{})
	m.Close()
	start = time.Now()
	m = New(ctx, root)
	defer m.Close()
	warmMS := time.Since(start).Milliseconds()
	if m.Status().Installed != v.Installed {
		t.Fatal("offline restart lost atlas")
	}
	m.client.Transport = transport(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("offline startup/view contacted network")
		return nil, nil
	})
	if _, e = m.View(q, false); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(map[string]any{"release": v.Installed, "overviewInstallMS": installMS, "verifiedWarmLoadMS": warmMS, "visibleL7Tiles": details, "detailAndCatalogBytes": detailBytes, "fullOptionalDetailBytes": 254133263, "cacheBytes": status.CacheBytes, "interruptedOverviewPreservedCache": true})
	t.Log(string(b))
}

type cutBody struct {
	io.Reader
	io.Closer
}
