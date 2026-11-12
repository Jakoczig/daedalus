package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSnapshot(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func findChange(changes []Change, kind string) *Change {
	for i := range changes {
		if changes[i].Kind == kind {
			return &changes[i]
		}
	}
	return nil
}
func TestSidecarStatusCargoMaterialsNavRoute(t *testing.T) {
	dir := t.TempDir()
	for _, kind := range []string{"Status", "Cargo", "Materials", "NavRoute"} {
		writeSnapshot(t, dir, kind, `{"timestamp":"2026-01-01T00:00:00Z","event":"`+kind+`","SystemAddress":9007199254740993}`)
	}
	r, err := NewReader(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	changes, err := r.Poll(context.Background())
	if err != nil || len(changes) != 4 {
		t.Fatal("four snapshots unavailable", err)
	}
	for _, change := range changes {
		if !change.Valid || change.Timestamp.IsZero() || !strings.Contains(string(change.Raw), "9007199254740993") {
			t.Fatal("invalid/exact-ID snapshot lost")
		}
	}
}
func TestSidecarAtomicReplacementAndPartialWrite(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, "Status", `{"event":"Status","Flags":1}`)
	r, _ := NewReader(dir, Options{})
	r.Poll(context.Background())
	writeSnapshot(t, dir, "Status", `{"event":"Status","Flags":`)
	changes, _ := r.Poll(context.Background())
	if c := findChange(changes, "Status"); c == nil || c.Valid || c.State != "invalid" {
		t.Fatal("partial snapshot published")
	}
	if got := r.LastGood("Status"); !strings.Contains(string(got.Raw), `"Flags":1`) {
		t.Fatal("partial write erased valid cache")
	}
	path := filepath.Join(dir, "new.tmp")
	os.WriteFile(path, []byte(`{"event":"Status","Flags":2}`), 0600)
	os.Remove(filepath.Join(dir, "Status.json"))
	os.Rename(path, filepath.Join(dir, "Status.json"))
	changes, _ = r.Poll(context.Background())
	if c := findChange(changes, "Status"); c == nil || !c.Valid || !strings.Contains(string(c.Raw), `"Flags":2`) {
		t.Fatal("replacement not read")
	}
}
func TestSidecarNoDuplicateWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, "Cargo", `{"event":"Cargo","Inventory":[]}`)
	r, _ := NewReader(dir, Options{})
	r.Poll(context.Background())
	changes, _ := r.Poll(context.Background())
	if len(changes) != 0 {
		t.Fatal("stable files re-emitted", changes)
	}
	copy := r.LastGood("Cargo")
	copy.Raw[0] = '!'
	if r.LastGood("Cargo").Raw[0] != '{' {
		t.Fatal("caller mutated retained snapshot")
	}
}
func TestSidecarRecoverAfterMalformedSnapshot(t *testing.T) {
	dir := t.TempDir()
	r, _ := NewReader(dir, Options{})
	r.Poll(context.Background())
	writeSnapshot(t, dir, "Materials", `{"event":"Materials","timestamp":"2026-01-01T00:00:02Z"}`)
	changes, _ := r.Poll(context.Background())
	if !findChange(changes, "Materials").Valid {
		t.Fatal("source did not appear")
	}
	writeSnapshot(t, dir, "Materials", `{"event":"Materials","timestamp":"2026-01-01T00:00:01Z"}`)
	changes, _ = r.Poll(context.Background())
	if c := findChange(changes, "Materials"); c == nil || c.Valid || c.State != "stale" {
		t.Fatal("older sidecar replaced newer state")
	}
	writeSnapshot(t, dir, "Materials", `bad`)
	r.Poll(context.Background())
	writeSnapshot(t, dir, "Materials", `{"event":"Materials","timestamp":"2026-01-01T00:00:03Z"}`)
	changes, _ = r.Poll(context.Background())
	if c := findChange(changes, "Materials"); c == nil || !c.Valid {
		t.Fatal("valid source did not recover")
	}
	os.Remove(filepath.Join(dir, "Materials.json"))
	changes, _ = r.Poll(context.Background())
	if c := findChange(changes, "Materials"); c == nil || c.Valid || c.State != "missing" {
		t.Fatal("missing source not reported")
	}
	if !r.LastGood("Materials").Valid {
		t.Fatal("missing file destroyed last-good snapshot")
	}
}
func TestSidecarRejectHugeUnknownFile(t *testing.T) {
	dir := t.TempDir()
	writeSnapshot(t, dir, "UnknownPrivate", strings.Repeat("x", 5<<20))
	writeSnapshot(t, dir, "NavRoute", strings.Repeat("x", (4<<20)+1))
	r, _ := NewReader(dir, Options{})
	changes, _ := r.Poll(context.Background())
	if len(changes) != 4 || findChange(changes, "UnknownPrivate") != nil {
		t.Fatal("unknown file incorporated")
	}
	if c := findChange(changes, "NavRoute"); c == nil || c.Valid || c.State != "too_large" {
		t.Fatal("unbounded snapshot read")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Poll(ctx); err == nil {
		t.Fatal("canceled sidecar poll continued")
	}
}
