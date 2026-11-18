package history

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestFrontierFleetFillsMissingShipsWithoutOverwritingWitnessedState(t *testing.T) {
	s := operationFixture(t, `{"event":"Loadout","ShipID":1,"Ship":"asp","ShipName":"Witnessed name","HullValue":10,"ModulesValue":5,"Modules":[]}`, `{"event":"ShipyardTransfer","ShipID":1,"TransferPrice":100,"TransferTime":60}`)
	now := time.Now()
	x := ExternalFacts{Provider: "frontier", Entity: "commander", EntityID: "SYNTHETIC", FID: "SYNTHETIC", IdentityVerified: true, Fetched: now, Expires: now.Add(time.Minute), Fields: map[string]any{"ships": []any{map[string]any{"id": "1", "model": "cobra_mk_iii", "name": "Old CAPI name", "systemName": "Old system", "value": "1000"}, map[string]any{"id": "9007199254740993", "model": "asp", "name": "Supplemental ship", "systemName": "Last-known CAPI system", "value": "9007199254740995"}}}}
	if e := s.SaveExternal(context.Background(), x); e != nil {
		t.Fatal(e)
	}
	page, e := s.Operations(context.Background(), "SYNTHETIC", OperationQuery{Kind: "fleet"})
	if e != nil {
		t.Fatal(e)
	}
	count, e := s.ReconciledFleetCount(context.Background(), "SYNTHETIC", page.Reference)
	if e != nil || count != 2 {
		t.Fatal("overview count does not match reconciled fleet", count, e)
	}
	if len(page.Rows) != 2 {
		t.Fatalf("CAPI-only owned ship missing from Fleet: %d", len(page.Rows))
	}
	for _, r := range page.Rows {
		var d map[string]any
		json.Unmarshal(r.Details, &d)
		if r.Entity == "1" {
			if r.State != "in_transit" || d["shipName"] != "Witnessed name" || d["system"] == "Old system" {
				t.Fatal("CAPI overwrote witnessed ship/transfer")
			}
		} else {
			if r.Entity != "9007199254740993" || r.At != "" || r.State != "capi_last_known" || d["Value"] != "9007199254740995" {
				t.Fatal("supplemental provenance/precision", r)
			}
		}
	}
	other, e := s.Operations(context.Background(), "OTHER", OperationQuery{Kind: "fleet"})
	if e != nil || len(other.Rows) != 0 {
		t.Fatal("CAPI fleet leaked across commander")
	}
}
