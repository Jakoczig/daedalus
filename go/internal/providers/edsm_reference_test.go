package providers

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/history"
	"testing"
	"time"
)

func TestEDSMNearbyUsesLockedCoordinatesAndVerifiedStationMarket(t *testing.T) {
	now := time.Now().UTC()
	db, err := history.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	origin := history.SystemReference{Address: "9007199254740993", Name: "Fixture", Coordinates: [3]float64{0, 0, 0}}
	raw, _ := json.Marshal(origin)
	if err = db.SaveReference(ctx, "edsm-reference", "system/Fixture", raw, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	m := New(ctx, t.TempDir(), Options{Manual: true, Vault: &testVault{}, Now: func() time.Time { return now }})
	defer m.Close()
	m.history = db
	endpoint, err := m.edsmReferenceEndpoint(ctx, "nearby", "Fixture | 25")
	if err != nil || endpoint == "" {
		t.Fatal(err)
	}
	if _, err = m.edsmReferenceEndpoint(ctx, "nearby", "Fixture | 101"); err == nil {
		t.Fatal("unbounded sphere")
	}
	snapshot, err := m.normalizeEDSMReference(ctx, "nearby", "Fixture | 25", []byte(`[{"name":"Fixture other","id":7,"id64":9007199254740994,"coordsLocked":true,"coords":{"x":3,"y":4,"z":0}},{"name":"Unverified","id64":7,"coordsLocked":false,"coords":{"x":0,"y":0,"z":0}}]`), now)
	if err != nil || len(snapshot.Facts) != 2 {
		t.Fatalf("locked nearby %+v %v", snapshot, err)
	}
	_, err = m.normalizeEDSMReference(ctx, "stations", "Fixture other", []byte(`{"name":"Fixture other","id64":9007199254740994,"stations":[{"marketId":18446744073709551615,"name":"Fixture Port","type":"Outpost","haveShipyard":false,"haveOutfitting":true,"otherServices":["Repair"],"distanceToArrival":null,"updateTime":{"information":"2026-11-15 00:00:00"}}]}`), now)
	if err != nil {
		t.Fatal(err)
	}
	station, err := db.StationByMarket(ctx, "18446744073709551615")
	if err != nil || station.ArrivalLS != nil || !station.Services["repair"] || station.Services["large_pad"] {
		t.Fatal("invented station flags", station, err)
	}
	if _, err = m.normalizeEDSMReference(ctx, "market", station.MarketID, []byte(`{"name":"Fixture other","id64":9007199254740994,"marketId":1,"sName":"Fixture Port","commodities":[]}`), now); err == nil {
		t.Fatal("wrong market attached")
	}
}
