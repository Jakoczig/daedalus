package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jakoczig/eridion/go/internal/history"
	"github.com/Jakoczig/eridion/go/internal/journal"
)

// No current Scan recipient has a verified delayed-upload contract. A proven
// sale must therefore remain held, explicitly disclosed as manual/unsupported,
// including after restart and a commander switch. Never fabricate a live date.
func TestAfterSaleDisclosesManualHoldAndNeverInventsDelayedUpload(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	dir, profile := t.TempDir(), t.TempDir()
	db, err := history.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	calls := 0
	opts := Options{RequireHistory: true, Manual: true, Vault: &testVault{}, Now: func() time.Time { return now }, Client: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"msgnum":100,"status":200}`))}, nil
	})}}
	m := New(context.Background(), profile, opts)
	defer func() { m.Close() }()
	if err = m.AttachHistory(db, history.ProviderSeed{}); err != nil {
		t.Fatal(err)
	}
	src := journal.Source{ID: "sale-release-fixture", Identity: "synthetic"}
	cp, err := db.Resume(src)
	if err != nil {
		t.Fatal(err)
	}
	var records []journal.Record
	appendRecord := func(raw string) journal.Event {
		t.Helper()
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: src.ID, Sequence: cp.Sequence + 1, Offset: cp.Offset, Length: 100})
		if err != nil {
			t.Fatal(err)
		}
		cp.Sequence++
		cp.Offset += 100
		if err = db.Commit(context.Background(), cp, []journal.Record{{Event: e, Phase: journal.Live}}, nil); err != nil {
			t.Fatal(err)
		}
		records = append(records, journal.Record{Event: e, Phase: journal.Live})
		m.Observe(e, journal.Live)
		return e
	}
	appendRecord(`{"event":"Fileheader","gameversion":"4.1"}`)
	appendRecord(`{"event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture"}`)
	if err = m.SetCredential("edsm", Credential{APIKey: "SYNTHETIC", Commander: "Fixture"}); err != nil {
		t.Fatal(err)
	}
	for id, cats := range map[string][]string{"edsm": {"exploration"}, "eddn": {"exploration"}, "edastro": {"biology"}} {
		if err = m.Configure(id, Config{Upload: true, Categories: cats}); err != nil {
			t.Fatal(err)
		}
	}
	if err = m.SetPublicationPolicy("SYNTHETIC-A", "After sale"); err != nil {
		t.Fatal(err)
	}
	appendRecord(`{"event":"Location","StarSystem":"Fixture sold","SystemAddress":9007199254740993,"StarPos":[1,2,3]}`)
	sold := appendRecord(`{"event":"Scan","timestamp":"2026-11-15T12:00:00Z","BodyID":1,"BodyName":"Fixture sold 1","SystemAddress":9007199254740993}`)
	unsold := appendRecord(`{"event":"Scan","timestamp":"2026-11-15T12:00:00Z","BodyID":1,"SystemAddress":9007199254740994}`)
	organic := appendRecord(`{"event":"ScanOrganic","timestamp":"2026-11-15T12:00:00Z","ScanType":"Analyse","Genus":"fixture-genus","Species":"fixture-species","Body":1,"SystemAddress":9007199254740993}`)
	assertNoOutbound := func() {
		t.Helper()
		for _, id := range []string{"edsm", "eddn", "edastro"} {
			m.dispatch(id)
			if len(m.states[id].queue) != 0 {
				t.Fatal("held data queued", id)
			}
		}
		if calls != 0 {
			t.Fatal("unexpected outbound", calls)
		}
	}
	assertNoOutbound()
	appendRecord(`{"event":"SellExplorationData","timestamp":"2026-11-15T12:00:01Z","Systems":["Fixture sold"]}`)
	appendRecord(`{"event":"SellOrganicData","timestamp":"2026-11-15T12:00:02Z","BioData":[{"Genus":"fixture-genus","Species":"fixture-species"}]}`)
	assertNoOutbound()
	for _, tc := range []struct {
		event                      journal.Event
		provider, category, status string
	}{
		{sold, "eddn", "exploration", "ineligible_delayed"},
		{sold, "edsm", "exploration", "ineligible_delayed"},
		{unsold, "edsm", "exploration", "held"},
		{organic, "edastro", "biology", "ambiguous"},
	} {
		c, err := db.Candidate("SYNTHETIC-A", tc.provider, tc.category, tc.event, journal.Replay, true, now)
		if err != nil || c.Status != tc.status {
			t.Fatalf("sale attribution %+v %v", c, err)
		}
		if tc.status == "ineligible_delayed" && !strings.Contains(c.Reason, "manual review") {
			t.Errorf("missing manual/unsupported release disclosure: %s", c.Reason)
		}
	}
	appendRecord(`{"event":"LoadGame","FID":"SYNTHETIC-B","Commander":"Other"}`)
	assertNoOutbound()
	m.Close()
	db.Close()
	db, err = history.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m = New(context.Background(), profile, opts)
	if err = m.AttachHistory(db, history.ProviderSeed{}); err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		m.Observe(r.Event, journal.Replay)
	}
	assertNoOutbound()
	policy, err := db.PublicationPolicy("SYNTHETIC-A")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(policy)
	if !strings.Contains(string(raw), "hold + manual review only") {
		t.Fatal("policy status implies automatic release", string(raw))
	}
}
