package atlas

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func spatialFixture(t testing.TB) (map[string][]byte, []byte) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(make([]byte, 32))
	pub := key.Public().(ed25519.PublicKey)
	sign := func(x any) []byte {
		b, _ := json.Marshal(x)
		v, _ := parseJSON(b, 16<<20)
		c, _ := canonical(v)
		b, _ = json.Marshal(map[string]any{"payload": x, "signature": map[string]any{"algorithm": "Ed25519", "key_id": digest(pub)[:16], "value": base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, c))}})
		return b
	}
	manifest, _ := os.ReadFile("testdata/spatial-v1/spatial.json")
	bubble, _ := os.ReadFile("testdata/spatial-v1/bubble-350.zip")
	m, _ := parseSpatial(manifest)
	id := strings.Repeat("a", 24)
	d := spatialDistribution{Protocol: spatialProtocol, ID: id, Format: spatialFormat, Frozen: frozenSpatial, RootLY: 800, Systems: 8, Roots: len(m.Roots), Indexes: len(m.Roots), Blocks: len(m.Roots), Prefix: "assets/", Total: 10000}
	d.Source.URL = sourceURL
	d.Source.SHA256 = m.Source.SHA256
	d.Source.Bytes = 100
	d.Source.Fresh = true
	d.Spatial = Asset{Path: "releases/" + id + "/spatial.json", Bytes: int64(len(manifest)), SHA256: digest(manifest)}
	d.Bubble = spatialBubbleAsset{Path: "assets/bubbles/" + digest(bubble) + ".zip", Bytes: int64(len(bubble)), SHA256: digest(bubble), Radius: 350, Systems: 5}
	descriptor := sign(d)
	now := time.Now().UTC()
	p := spatialPointer{Protocol: spatialProtocol, ID: id, Path: "releases/" + id + "/distribution.json", Hash: digest(descriptor), Issued: now.Add(-time.Hour), Expires: now.Add(24 * time.Hour)}
	files := map[string][]byte{"latest.json": sign(p), p.Path: descriptor, d.Spatial.Path: manifest, d.Bubble.Path: bubble}
	for _, r := range m.Roots {
		b, _ := os.ReadFile("testdata/spatial-v1/indexes/" + r.SHA256 + ".json")
		files["assets/indexes/"+r.SHA256+".json"] = b
		ds, _ := parseSpatialIndex(b, r, m.RootLY)
		for _, d := range ds {
			b, _ = os.ReadFile("testdata/spatial-v1/blocks/" + d.SHA256 + ".z")
			files["assets/blocks/"+d.SHA256+".z"] = b
		}
	}
	return files, pub
}
func TestSpatialSignedInstallOfflineRestartAndViewerNoNetwork(t *testing.T) {
	files, key := spatialFixture(t)
	var calls atomic.Int32
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.URL.Host != "eridion.fitoserver.org" {
			t.Error("private traffic")
		}
		b, ok := files[strings.TrimPrefix(r.URL.Path, "/atlas/spatial/")]
		code := 200
		if !ok {
			code = 404
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(string(b))), ContentLength: int64(len(b)), Header: http.Header{}}, nil
	})}
	dir := t.TempDir()
	m := newSpatial(context.Background(), dir, client, key, time.Now)
	defer m.Close()
	if calls.Load() != 0 {
		t.Fatal("startup network")
	}
	if e := m.Update(context.Background(), true); e == nil {
		t.Fatal("missing consent")
	}
	if e := m.Configure(SpatialPreferences{Downloads: true}); e != nil {
		t.Fatal(e)
	}
	if e := m.Update(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	q := SpatialQuery{Low: [3]float64{-2, -2, -2}, High: [3]float64{2, 2, 2}}
	before := calls.Load()
	v, e := m.View(q, false)
	if e != nil || !v.Complete || len(v.Points) != 4 {
		t.Fatalf("%+v %v", v, e)
	}
	if calls.Load() != before {
		t.Fatal("viewer network")
	}
	m.Close()
	m = newSpatial(context.Background(), dir, client, key, time.Now)
	defer m.Close()
	v, e = m.View(q, false)
	if e != nil || !v.Complete || len(v.Points) != 4 || calls.Load() != before {
		t.Fatal("offline restart", v, e)
	}
	if v.Points[0] != [3]float64{-0.03125, -0.0625, -0.125} {
		t.Fatal("wrong ticks", v.Points)
	}
	m.distributionData.Bubble.Systems = 7
	if m.Status().BubbleSystems != 5 {
		t.Fatal("new release count mislabeled as installed Bubble")
	}
	files["latest.json"] = []byte(`{"payload":{},"signature":{}}`)
	if e = m.Update(context.Background(), false); e == nil {
		t.Fatal("corrupt signature")
	}
	v, e = m.View(q, false)
	if e != nil || !v.Complete {
		t.Fatal("lost working state")
	}
	if _, e = m.View(SpatialQuery{Low: [3]float64{0, 0, 0}, High: [3]float64{0, 0, 0}}, true); e == nil {
		t.Fatal("bad volume")
	}
	if m.RemoveBubble(false) == nil {
		t.Fatal("unconfirmed removal")
	}
	if e = m.RemoveBubble(true); e != nil {
		t.Fatal(e)
	}
	if m.Status().BubbleInstalled {
		t.Fatal("Bubble retained after removal")
	}
}
