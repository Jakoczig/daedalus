package webapp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestG2JPairedViewerCannotReadPrivateDatabaseOrAuthorizeProviders(t *testing.T) {
	s, _, v, u, _ := g2eFixture(t)
	c := pairedSocket(t, v, u)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, name := range []string{"getGoConstruction", "getGoDataStatus", "queryGoHistory", "backupGoHistory", "exportGoHistory", "stageGoRestore", "rebuildGoHistory", "reimportGoSource", "getGoPublicationPolicy", "setGoPublicationPolicy", "setGoPublicationException", "getGoSourceFacts", "getGoSaleLinks", "confirmGoOrganicSale", "getGoNearest", "previewGoHistorical", "confirmGoHistorical"} {
		if len(callRPC(t, ctx, c, name)["error"]) == 0 {
			t.Fatalf("viewer obtained owner operation %s", name)
		}
	}
	raw, _ := json.Marshal(s.game.Snapshot())
	if strings.Contains(string(raw), "SYNTHETIC") || strings.Contains(string(raw), s.profile) || strings.Contains(string(raw), "canonical_path") {
		t.Fatal("private history/source metadata in public status")
	}
	owner, ownerCtx := dialOwner(t, s)
	defer owner.CloseNow()
	r := callRPC(t, ownerCtx, owner, "getGoDataStatus")
	if len(r["error"]) != 0 {
		t.Fatalf("owner status denied %s", r["error"])
	}
}
