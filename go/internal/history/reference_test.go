package history

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestReferenceCacheTTLThrottleAndExactNearestDistance(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	if err = s.AuthorizeRead("spansh", "station", now); err != nil {
		t.Fatal(err)
	}
	if s.AuthorizeRead("spansh", "system", now.Add(time.Second)) == nil {
		t.Fatal("cross-kind throttle bypass")
	}
	if err = s.SaveReference(ctx, "spansh", "system/9007199254740993", json.RawMessage(`{"facts":[]}`), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if raw, err := s.CachedReference("spansh", "system/9007199254740993", now.Add(2*time.Hour)); err != nil || raw != nil {
		t.Fatal("expired reference served")
	}
	s.writer.Exec("INSERT INTO systems VALUES('9007199254740993','Fixture',0,0,0)")
	station := StationReference{Provider: "spansh", MarketID: "18446744073709551615", SystemAddress: "9007199254740994", SystemName: "Fixture other", Name: "Fixture port", Coordinates: [3]float64{3, 4, 0}, Services: map[string]bool{"repair": true}, Fetched: now.Format(time.RFC3339Nano), Expires: now.Add(time.Hour).Format(time.RFC3339Nano)}
	if err = s.SaveStation(station); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Nearest(ctx, "9007199254740993", "repair", 10, now)
	if err != nil || len(rows) != 1 || rows[0].DistanceLY != 5 || rows[0].MarketID != "18446744073709551615" || rows[0].ArrivalLS != nil {
		t.Fatalf("nearest: %+v %v", rows, err)
	}
	rows, err = s.Nearest(ctx, "9007199254740993", "shipyard", 10, now)
	if err != nil || len(rows) != 0 {
		t.Fatal("unknown service fabricated")
	}
}
