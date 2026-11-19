package webapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestG2MCViewerSnapshotAllowlistAndPrivateLedger(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile("../../../tests/testdata/g2mc/startup.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "Journal.20261119.01.log"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, _, viewer, url, _ := startGameFixture(t, dir, true, "Fixture Alpha", true, true, true)
	deadline := time.Now().Add(3 * time.Second)
	for s.game.Workspace().Parity.Materials.State != "known" {
		if time.Now().After(deadline) {
			t.Fatal("startup not reconstructed")
		}
		time.Sleep(time.Millisecond)
	}
	c := pairedSocket(t, viewer, url)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reply := callRPC(t, ctx, c, "getGoWorkspace")
	body := string(reply["message"])
	if !strings.Contains(body, "iron") || strings.Contains(body, "SYNTHETIC-A") || strings.Contains(body, `"Fixture A"`) || strings.Contains(body, "Journal.") || strings.Contains(body, "FileID") {
		t.Fatal("startup workspace allowlist changed", body)
	}
	for _, name := range []string{"getGoBGSTally", "getGoCommanderOverview", "exportGoBGS"} {
		if len(callRPC(t, ctx, c, name)["error"]) == 0 {
			t.Fatal("viewer accessed private commander data", name)
		}
	}
}
