package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestG2CExplorationSmokeIsExplicitIsolatedAndVerifiesOrganicData(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}, "nav/route.html": {Data: []byte("route")}}
	var out bytes.Buffer
	launch := func(context.Context, string, func()) error { t.Fatal("headless opened a window"); return nil }
	for _, mode := range []string{"--exploration-smoke-test", "--desktop-exploration-smoke-test"} {
		if run(context.Background(), []string{mode}, assets, launch, &out) == nil {
			t.Fatal("personal discovery allowed")
		}
	}
	dir := t.TempDir()
	for _, item := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2c", item[0]))
		if err != nil {
			t.Fatal(err)
		}
		if os.WriteFile(filepath.Join(dir, item[1]), raw, 0600) != nil {
			t.Fatal("fixture")
		}
	}
	profile := t.TempDir()
	if err := run(context.Background(), []string{"--exploration-smoke-test", "--journal-dir", dir, "--data-dir", profile}, assets, launch, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "G2C synthetic map, biology and navigation smoke passed") {
		t.Fatal("new evidence not verified")
	}
	entries, err := os.ReadDir(profile)
	if err != nil || len(entries) > 0 {
		t.Fatal("smoke modified owner profile")
	}
}
