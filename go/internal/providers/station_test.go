package providers

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/history"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/sidecar"
	"strings"
	"testing"
	"time"
)

func TestEDDNStationSchemasSanitizeAndPreserveExactMarketID(t *testing.T) {
	m := &Manager{uploader: "fixture", gameVersion: "4.1", gameBuild: "fixture"}
	for kind, items := range map[string]string{"Market": `"Items":[{"Name":"gold","MeanPrice":1,"BuyPrice":2,"Stock":3,"StockBracket":1,"SellPrice":4,"Demand":5,"DemandBracket":1,"Producer":true,"Rare":false,"id":123}]`, "Shipyard": `"PriceList":[{"ShipType":"SideWinder"},{"ShipType":"CobraMkIV"}]`, "Outfitting": `"Items":[{"Name":"int_hyperdrive_size5_class5"},{"Name":"Hpt_PulseLaserBurst_Special"},{"Name":"PRIVATE_DECAL"}]`} {
		raw := `{"timestamp":"2026-11-15T12:00:00Z","StarSystem":"Fixture","StationName":"Fixture Port","MarketID":18446744073709551615,"Commander":"PRIVATE",` + items + `}`
		out, err := m.stationPayload(kind, object([]byte(raw)))
		if err != nil {
			t.Fatal(kind, err)
		}
		if !strings.Contains(string(out), "18446744073709551615") || strings.Contains(string(out), "PRIVATE") || strings.Contains(string(out), "Producer") || strings.Contains(string(out), "CobraMkIV") || strings.Contains(string(out), "Special") {
			t.Fatal("station fields escaped", string(out))
		}
	}
}
func TestEDDNStationRequiresCommittedMarkerDockAndExplicitLiveException(t *testing.T) {
	db, err := history.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	m := New(context.Background(), t.TempDir(), Options{RequireHistory: true, Manual: true, Vault: &testVault{}, Now: func() time.Time { return now }})
	defer m.Close()
	if err = m.AttachHistory(db, history.ProviderSeed{}); err != nil {
		t.Fatal(err)
	}
	src := journal.Source{ID: "station-fixture", Identity: "synthetic"}
	cp, _ := db.Resume(src)
	raws := []string{`{"event":"Fileheader","gameversion":"4.1","build":"fixture"}`, `{"event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture"}`, `{"event":"Location","StarSystem":"Fixture","SystemAddress":9007199254740993,"StarPos":[1,2,3]}`, `{"event":"Docked","StarSystem":"Fixture","StationName":"Fixture Port","MarketID":18446744073709551615}`, `{"event":"Market","timestamp":"2026-11-15T12:00:00Z","MarketID":18446744073709551615}`}
	events := []journal.Record{}
	for i, raw := range raws {
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: src.ID, Sequence: uint64(i + 1), Offset: int64(i * 100), Length: 100})
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, journal.Record{Event: e, Phase: journal.Live})
	}
	cp.Offset = 500
	cp.Sequence = 5
	if err = db.Commit(context.Background(), cp, events, nil); err != nil {
		t.Fatal(err)
	}
	for _, e := range events[:4] {
		m.Observe(e.Event, e.Phase)
	}
	if err = m.Configure("eddn", Config{Upload: true, Categories: []string{"shipyard"}}); err != nil {
		t.Fatal(err)
	}
	// This sidecar is deliberately wrong kind: no Shipyard journal marker.
	c := sidecar.Change{Kind: "Shipyard", Valid: true, Timestamp: now, Raw: json.RawMessage(`{"event":"Shipyard","timestamp":"2026-11-15T12:00:00Z","StarSystem":"Fixture","StationName":"Fixture Port","MarketID":18446744073709551615,"PriceList":[{"ShipType":"SideWinder"}]}`)}
	if err = m.SetPublicationPolicy("SYNTHETIC-A", "After sale"); err != nil {
		t.Fatal(err)
	}
	m.ObserveSidecar(c)
	if len(m.states["eddn"].queue) != 0 {
		t.Fatal("sidecar without matching journal marker queued")
	}
	// Committed Market marker still cannot authorize Shipyard data.
	m.Observe(events[4].Event, journal.Live)
	m.ObserveSidecar(c)
	if len(m.states["eddn"].queue) != 0 {
		t.Fatal("wrong marker kind queued")
	}
	// Publish a matching committed Shipyard marker, then prove the privacy hold.
	e, _ := journal.ParseLine([]byte(`{"event":"Shipyard","timestamp":"2026-11-15T12:00:00Z","MarketID":18446744073709551615}`), journal.SourcePosition{FileID: src.ID, Sequence: 6, Offset: 500, Length: 100})
	cp.Offset = 600
	cp.Sequence = 6
	if err = db.Commit(context.Background(), cp, []journal.Record{{Event: e, Phase: journal.Live}}, nil); err != nil {
		t.Fatal(err)
	}
	m.Observe(e, journal.Live)
	if len(m.states["eddn"].queue) != 0 {
		t.Fatal("After sale station disclosure leaked")
	}
	if err = m.SetLiveException("SYNTHETIC-A", "eddn", "shipyard", true, true); err != nil {
		t.Fatal(err)
	}
	m.ObserveSidecar(c)
	if len(m.states["eddn"].queue) != 1 {
		t.Fatalf("exact live exception did not authorize station payload: %s", m.states["eddn"].lastError)
	}
	if err = m.SetLiveException("SYNTHETIC-A", "eddn", "shipyard", false, false); err != nil {
		t.Fatal(err)
	}
	if len(m.states["eddn"].queue) != 0 {
		t.Fatal("revoked exception left unsent disclosure")
	}
}
