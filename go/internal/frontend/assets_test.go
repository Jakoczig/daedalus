package frontend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"testing"
)

func TestRealEmbeddedExport(t *testing.T) {
	assets, err := FS()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "exploration/system.html", "nav/map.html"} {
		if _, err := fs.Stat(assets, name); err != nil {
			t.Fatal(err)
		}
	}
	data, err := fs.ReadFile(assets, "g1a-assets.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) < 62 {
		t.Fatal("incomplete existing frontend")
	}
	for name, want := range manifest.Files {
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			t.Errorf("embedded bytes changed: %s", name)
		}
	}
}
