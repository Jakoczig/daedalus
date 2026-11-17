package webapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestG2LViewerCannotReadFleetFinanceTransactionsCrewOrExport(t *testing.T) {
	_, _, viewer, url, _ := g2eFixture(t)
	c := pairedSocket(t, viewer, url)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, name := range []string{"getGoOperations", "exportGoOperations", "backfillGoOperationsSource"} {
		r := callRPC(t, ctx, c, name)
		if string(r["error"]) != `"Local owner required"` {
			t.Fatalf("must be rejected before decoding private query: %s %s", name, r["error"])
		}
	}
}

func TestG2LOwnerExportNeedsPreviewAcknowledgment(t *testing.T) {
	s, _, _, _, _ := g2eFixture(t)
	if _, e := s.operationsRPC(context.Background(), "exportGoOperations", json.RawMessage(`{"kind":"fleet"}`)); e == nil {
		t.Fatal("private export without acknowledgment")
	}
	preview, e := s.operationsRPC(context.Background(), "getGoOperations", json.RawMessage(`{"kind":"fleet"}`))
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(preview)
	var hash struct{ PreviewHash string }
	json.Unmarshal(b, &hash)
	value, e := s.operationsRPC(context.Background(), "exportGoOperations", json.RawMessage(fmt.Sprintf(`{"kind":"fleet","acknowledge":true,"previewHash":%q}`, hash.PreviewHash)))
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(value.(map[string]string)["csv"], "observed_utc") {
		t.Fatal(value)
	}
}
