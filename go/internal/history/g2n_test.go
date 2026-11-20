package history

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestG2NFrontierOnlyFleetHasBoundedCompletePaging(t *testing.T) {
	s := operationFixture(t, `{"event":"Loadout","ShipID":1,"Ship":"asp","ShipName":"Fixture","Modules":[]}`, `{"event":"ShipyardSell","SellShipID":2}`)
	ships := []any{}
	for i := 1; i <= 75; i++ {
		ships = append(ships, map[string]any{"id": fmt.Sprint(i), "model": "asp", "name": fmt.Sprintf("Fixture %d", i)})
	}
	now := time.Now()
	if e := s.SaveExternal(context.Background(), ExternalFacts{Provider: "frontier", Entity: "commander", FID: "SYNTHETIC", IdentityVerified: true, Fetched: now, Expires: now.Add(time.Minute), Fields: map[string]any{"ships": ships}}); e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	q := OperationQuery{Kind: "fleet", FleetState: "owned", Limit: 10}
	for n := 0; n < 10; n++ {
		p, e := s.Operations(context.Background(), "SYNTHETIC", q)
		if e != nil {
			t.Fatal(e)
		}
		if len(p.Rows) > 10 {
			t.Fatal("unbounded page")
		}
		for _, r := range p.Rows {
			if seen[r.Entity] || r.Entity == "2" {
				t.Fatal("duplicate or sold ship", r.Entity)
			}
			seen[r.Entity] = true
		}
		if p.Next != "" {
			q.Before = p.Next
		} else if p.FleetNext > 0 {
			q.Before = ""
			q.FleetAfter = p.FleetNext
		} else {
			break
		}
	}
	if len(seen) != 74 {
		t.Fatalf("fleet dead end: %d/74", len(seen))
	}
	other, e := s.Operations(context.Background(), "OTHER", q)
	if e != nil || len(other.Rows) > 0 {
		t.Fatal("foreign fleet exposed", e)
	}
}

func TestG2NEarnedMeritsNeverUsesBalanceAndRepairsRetainedRows(t *testing.T) {
	s := operationFixture(t, `{"event":"Powerplay","Power":"Fixture Power","Merits":9000}`, `{"event":"PowerplayMerits","Power":"Fixture Power","MeritsGained":7}`)
	if _, e := s.writer.Exec("UPDATE field_tally SET amount='9000',unit='merits if reported' WHERE category='PowerplayMerits'"); e != nil {
		t.Fatal(e)
	}
	rows, e := s.FieldTallyCursor(context.Background(), "SYNTHETIC", "", "", "", 100, false, "powerplay")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.Category == "PowerplayMerits" && (r.Amount != "7" || r.Unit != "earned merits") {
			t.Fatalf("earned merits confused with balance: %+v", r)
		}
		if r.Category == "Powerplay" && r.Unit != "reported merit balance" {
			t.Fatal("balance labeled activity", r)
		}
	}
}
