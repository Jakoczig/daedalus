package atlas

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func spatialSignTest(v any) []byte {
	key := ed25519.NewKeyFromSeed(make([]byte, 32))
	pub := key.Public().(ed25519.PublicKey)
	b, _ := json.Marshal(v)
	obj, _ := parseJSON(b, 16<<20)
	raw, _ := canonical(obj)
	b, _ = json.Marshal(map[string]any{"payload": v, "signature": map[string]any{"algorithm": "Ed25519", "key_id": digest(pub)[:16], "value": base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, raw))}})
	return b
}
func TestSpatialSignedRejectionAndExpiredOffline(t *testing.T) {
	files, key := spatialFixture(t)
	m := newSpatial(context.Background(), t.TempDir(), http.DefaultClient, key, time.Now)
	defer m.Close()
	pb := files["latest.json"]
	p, e := m.pointer(pb, true)
	if e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*spatialPointer){func(p *spatialPointer) { p.Path = "../distribution.json" }, func(p *spatialPointer) { p.Protocol = "eridion-atlas-signed-v2" }, func(p *spatialPointer) { p.Expires = p.Issued.Add(100 * 24 * time.Hour) }, func(p *spatialPointer) { p.Issued = time.Now().Add(time.Hour) }, func(p *spatialPointer) {
		p.Issued = time.Now().Add(-10 * 24 * time.Hour)
		p.Expires = time.Now().Add(-time.Hour)
	}} {
		bad := p
		mutate(&bad)
		if _, e := m.pointer(spatialSignTest(bad), true); e == nil {
			t.Fatal("invalid pointer accepted", bad)
		}
	}
	expired := p
	expired.Issued = time.Now().Add(-10 * 24 * time.Hour)
	expired.Expires = time.Now().Add(-time.Hour)
	if _, e = m.pointer(spatialSignTest(expired), false); e != nil {
		t.Fatal("offline expiry rejected trusted data", e)
	}
	if _, e = m.pointer(bytes.Replace(pb, []byte(`"payload":`), []byte(`"payload":{},"payload":`), 1), true); e == nil {
		t.Fatal("duplicate signed key")
	}
	saved := m.key
	m.key = make([]byte, 32)
	if _, e = m.pointer(pb, true); e == nil {
		t.Fatal("untrusted key")
	}
	m.key = saved
	var env map[string]any
	json.Unmarshal(files[p.Path], &env)
	payload := env["payload"].(map[string]any)
	payload["unknown"] = true
	db := spatialSignTest(payload)
	p.Hash = digest(db)
	if _, e = m.distribution(db, p); e == nil {
		t.Fatal("unknown signed field")
	}
}
func TestSpatialBinaryRejectsMalformedFramesAndCoordinates(t *testing.T) {
	raw := make([]byte, 32)
	copy(raw, "ESP1")
	binary.LittleEndian.PutUint32(raw[4:], 2)
	binary.LittleEndian.PutUint32(raw[20:], 1)
	encode := func(raw []byte) []byte {
		var b bytes.Buffer
		z := zlib.NewWriter(&b)
		z.Write(raw)
		z.Close()
		return b.Bytes()
	}
	valid := encode(raw)
	d := spatialBlock{Lo: [3]int32{0, 0, 0}, Hi: [3]int32{2, 1, 1}, Count: 2, Bytes: int64(len(valid)), SHA256: digest(valid)}
	if _, e := decodeESP(valid, d); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"header", "count", "duplicate", "unsorted", "overrun", "trailing", "second-stream", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			r := append([]byte(nil), raw...)
			b := []byte(nil)
			switch kind {
			case "header":
				copy(r, "BAD!")
			case "count":
				binary.LittleEndian.PutUint32(r[4:], 3)
			case "duplicate":
				binary.LittleEndian.PutUint32(r[20:], 0)
			case "unsorted":
				binary.LittleEndian.PutUint32(r[8:], 1)
				binary.LittleEndian.PutUint32(r[20:], 0)
			case "overrun":
				r = append(r, make([]byte, 1<<20)...)
			}
			b = encode(r)
			switch kind {
			case "trailing":
				b = append(b, 1)
			case "second-stream":
				b = append(b, valid...)
			case "truncated":
				b = b[:len(b)-1]
			}
			bad := d
			bad.Bytes = int64(len(b))
			bad.SHA256 = digest(b)
			if _, e := decodeESP(b, bad); e == nil {
				t.Fatal("accepted", kind)
			}
		})
	}
}
func TestSpatialClassicZipRejectsMaliciousMembers(t *testing.T) {
	for _, kind := range []string{"path", "compressed", "extra", "duplicate", "comment"} {
		t.Run(kind, func(t *testing.T) {
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			name := "spatial.json"
			if kind == "path" {
				name = "../spatial.json"
			}
			h := &zip.FileHeader{Name: name, Method: zip.Store}
			if kind == "compressed" {
				h.Method = zip.Deflate
			}
			if kind == "extra" {
				h.Extra = []byte{1, 2, 0, 0}
			}
			if kind == "comment" {
				z.SetComment("untrusted")
			}
			w, _ := z.CreateHeader(h)
			w.Write([]byte("{}"))
			if kind == "duplicate" {
				w, _ = z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
				w.Write([]byte("{}"))
			}
			z.Close()
			path := filepath.Join(t.TempDir(), "bad.zip")
			os.WriteFile(path, b.Bytes(), 0600)
			if v, e := openSpatialBubble(path, digest(b.Bytes())); e == nil {
				v.Close()
				t.Fatal("accepted", kind)
			}
		})
	}
}
func TestSpatialPointBudgetNeverPublishesTruncatedResults(t *testing.T) {
	m := newSpatial(context.Background(), t.TempDir(), http.DefaultClient, make([]byte, 32), time.Now)
	defer m.Close()
	var ds []spatialBlock
	for group := range 5 {
		count := 16384
		raw := make([]byte, 8+12*count)
		copy(raw, "ESP1")
		binary.LittleEndian.PutUint32(raw[4:], uint32(count))
		for i := range count {
			binary.LittleEndian.PutUint32(raw[8+i*12:], uint32(i))
			binary.LittleEndian.PutUint32(raw[12+i*12:], uint32(group*32))
		}
		var b bytes.Buffer
		z := zlib.NewWriter(&b)
		z.Write(raw)
		z.Close()
		d := spatialBlock{Lo: [3]int32{0, int32(group * 32), 0}, Hi: [3]int32{16384, int32(group*32) + 1, 1}, Count: count, Bytes: int64(b.Len()), SHA256: digest(b.Bytes())}
		writeFile(m.root, spatialPath("blocks", d.SHA256), b.Bytes())
		ds = append(ds, d)
	}
	index, _ := json.Marshal(ds)
	r := spatialRoot{Cell: [3]int64{0, 0, 0}, SHA256: digest(index), Bytes: int64(len(index)), Count: 5 * 16384}
	writeFile(m.root, spatialPath("indexes", r.SHA256), index)
	m.current = &spatialManifest{RootLY: 800, Systems: r.Count, Roots: []spatialRoot{r}}
	v, e := m.View(SpatialQuery{Low: [3]float64{0, 0, 0}, High: [3]float64{600, 10, 2}}, false)
	if e != nil || v.Complete || len(v.Points) != 0 || !strings.Contains(v.Notice, "budget") {
		t.Fatal("truncated volume appeared complete", v, e)
	}
}
