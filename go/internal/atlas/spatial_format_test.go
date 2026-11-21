package atlas

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSpatialFrozenFixturePrecisionAndBounds(t *testing.T) {
	base := "testdata/spatial-v1"
	b, _ := os.ReadFile(filepath.Join(base, "spatial.json"))
	m, e := parseSpatial(b)
	if e != nil {
		t.Fatal(e)
	}
	var all [][3]int32
	for _, r := range m.Roots {
		b, _ = os.ReadFile(filepath.Join(base, "indexes", r.SHA256+".json"))
		blocks, e := parseSpatialIndex(b, r, m.RootLY)
		if e != nil {
			t.Fatal(e)
		}
		for _, d := range blocks {
			b, _ = os.ReadFile(filepath.Join(base, "blocks", d.SHA256+".z"))
			p, e := decodeESP(b, d)
			if e != nil {
				t.Fatal(e)
			}
			all = append(all, p...)
			bad := append(append([]byte(nil), b...), 0)
			tampered := d
			tampered.Bytes = int64(len(bad))
			tampered.SHA256 = digest(bad)
			if _, e = decodeESP(bad, tampered); e == nil {
				t.Fatal("trailing compressed data accepted")
			}
		}
	}
	if len(all) != 8 {
		t.Fatal(len(all))
	}
	seen := false
	for _, p := range all {
		if p == [3]int32{-1, -2, -4} {
			seen = true
		}
	}
	if !seen {
		t.Fatal("negative 1/32 LY precision lost")
	}
	raw, _ := os.ReadFile(filepath.Join(base, "spatial.json"))
	if _, e = parseSpatial(bytes.Replace(raw, []byte(`"format":`), []byte(`"systems":8,"format":`), 1)); e == nil {
		t.Fatal("duplicate key accepted")
	}
	if _, e = parseSpatial(bytes.Replace(raw, []byte(`"format":`), []byte(`"unknown":0,"format":`), 1)); e == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestSpatialBubbleLazyAndTrustedDigest(t *testing.T) {
	p := "testdata/spatial-v1/bubble-350.zip"
	z, e := openSpatialBubble(p, "0ec1cfb361adeb4b862cd1327be9634546308c0e157f32f5fc61aa1b021443ba")
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	if z.manifest.RadiusLY != 350 || z.manifest.Systems != 5 {
		t.Fatal("bubble metadata")
	}
	if _, e = openSpatialBubble(p, string(bytes.Repeat([]byte{'0'}, 64))); e == nil {
		t.Fatal("untrusted zip")
	}
}
