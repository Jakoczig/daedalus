package providers

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/history"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"strings"
	"testing"
	"time"
)

func TestHistoricalWizardRequiresExactPreviewConsentAndNeverRetriesQueued(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	db, err := history.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := New(context.Background(), t.TempDir(), Options{RequireHistory: true, Manual: true, Vault: &testVault{}, InaraAppName: "synthetic-contract-test-only", Now: func() time.Time { return now }})
	defer m.Close()
	if err = m.AttachHistory(db, history.ProviderSeed{}); err != nil {
		t.Fatal(err)
	}
	src := journal.Source{ID: "wizard-fixture", Identity: "synthetic"}
	cp, _ := db.Resume(src)
	records := []journal.Record{}
	for i, raw := range []string{`{"event":"Fileheader","gameversion":"4.1"}`, `{"event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture"}`, `{"event":"FSDJump","timestamp":"2026-11-14T12:00:00Z","StarSystem":"Fixture","SystemAddress":9007199254740993,"StarPos":[1,2,3],"Commander":"PRIVATE"}`, `{"event":"Loadout","timestamp":"2026-11-14T12:00:01Z","ShipID":18446744073709551615,"Ship":"asp","Modules":[{"Slot":"FrameShiftDrive","Item":"int_hyperdrive_size5_class5"}]}`} {
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: src.ID, Sequence: uint64(i + 1), Offset: int64(i * 100), Length: 100})
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, journal.Record{Event: e, Phase: journal.Replay})
	}
	cp.Offset = 400
	cp.Sequence = 4
	if err = db.Commit(context.Background(), cp, records, nil); err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		m.Observe(r.Event, r.Phase)
	}
	if err = m.SetCredential("inara", Credential{APIKey: "SYNTHETIC_SECRET", Commander: "Fixture"}); err != nil {
		t.Fatal(err)
	}
	if err = m.Configure("inara", Config{Upload: true, Categories: []string{"travel", "loadout"}}); err != nil {
		t.Fatal(err)
	}
	selection := HistorySelection{FID: "SYNTHETIC-A", Provider: "inara", Category: "travel", From: "2026-11-13T00:00:00Z", To: "2026-11-15T12:00:00Z"}
	preview, err := m.Historical(context.Background(), selection, false)
	if err != nil || len(preview.Rows) != 1 || preview.Rows[0].Status != "held" {
		t.Fatalf("Never bypass: %+v %v", preview, err)
	}
	if err = m.SetPublicationPolicy("SYNTHETIC-A", "Live"); err != nil {
		t.Fatal(err)
	}
	preview, err = m.Historical(context.Background(), selection, false)
	if err != nil || len(preview.Rows) != 1 || preview.Rows[0].Status != "eligible" {
		t.Fatalf("explicit review blocked: %+v %v", preview, err)
	}
	b, _ := json.Marshal(preview)
	if strings.Contains(string(b), "SYNTHETIC_SECRET") || strings.Contains(string(b), "PRIVATE") {
		t.Fatal("review disclosed credential/private raw fields")
	}
	if len(m.states["inara"].queue) != 0 {
		t.Fatal("preview enqueued data")
	}
	loadoutSelection := selection
	loadoutSelection.Category = "loadout"
	loadoutPreview, err := m.Historical(context.Background(), loadoutSelection, false)
	if err != nil || len(loadoutPreview.Rows) != 1 || !strings.Contains(loadoutPreview.Rows[0].Fields, "18446744073709551615") {
		t.Fatalf("exact outgoing review text lost: %+v %v", loadoutPreview, err)
	}
	selection.Token = preview.Token
	if _, err = m.Historical(context.Background(), selection, true); err == nil {
		t.Fatal("missing disclosure acknowledgment")
	}
	selection.Acknowledge = true
	accepted, err := m.Historical(context.Background(), selection, true)
	if err != nil || accepted.Queued != 1 {
		t.Fatalf("confirmation %+v %v", accepted, err)
	}
	if _, err = m.Historical(context.Background(), selection, true); err == nil {
		t.Fatal("consumed preview token accepted again")
	}
	if len(m.states["inara"].queue) != 1 {
		t.Fatal("queued event duplicated")
	}
}
