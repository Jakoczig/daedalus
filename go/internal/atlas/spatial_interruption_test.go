package atlas

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSpatialInterruptedUpdateRetainsTrustedRelease(t *testing.T) {
	files, key := spatialFixture(t)
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		b := files[strings.TrimPrefix(r.URL.Path, "/atlas/spatial/")]
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(b)), Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})}
	m := newSpatial(context.Background(), t.TempDir(), client, key, time.Now)
	defer m.Close()
	m.Configure(SpatialPreferences{Downloads: true})
	if e := m.Update(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	old := m.Status().Installed
	p, _ := m.pointer(files["latest.json"], true)
	d, _ := m.distribution(files[p.Path], p)
	manifest := files[d.Spatial.Path]
	p.ID = strings.Repeat("c", 24)
	p.Path = "releases/" + p.ID + "/distribution.json"
	p.Issued = p.Issued.Add(time.Second)
	d.ID = p.ID
	d.Spatial.Path = "releases/" + p.ID + "/spatial.json"
	files[p.Path] = spatialSignTest(d)
	p.Hash = digest(files[p.Path])
	files["latest.json"] = spatialSignTest(p)
	files[d.Spatial.Path] = manifest[:len(manifest)-10]
	if e := m.Update(context.Background(), false); e == nil {
		t.Fatal("interrupted update accepted")
	}
	if m.Status().Installed != old {
		t.Fatal("active pointer changed")
	}
	v, e := m.View(SpatialQuery{Low: [3]float64{-2, -2, -2}, High: [3]float64{2, 2, 2}}, false)
	if e != nil || !v.Complete || len(v.Points) != 4 {
		t.Fatal("trusted offline Bubble lost", e)
	}
}
