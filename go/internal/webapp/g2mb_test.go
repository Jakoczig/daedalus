package webapp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestG2MBActivityContextUsesSelectedAccountAndStrictEnum(t *testing.T) {
	s, _, _, _, _ := g2eFixture(t)
	for _, scope := range []string{"commander", "missions", "powerplay", "colonisation"} {
		raw, _ := json.Marshal(map[string]any{"context": scope})
		if _, err := s.fieldRPC(context.Background(), "getGoBGSTally", raw); err != nil {
			t.Fatal(scope, err)
		}
	}
	for _, raw := range []string{`{"context":"unknown"}`, `{"context":"missions' OR 1=1"}`, `{"context":"missions","fid":"OTHER_ACCOUNT"}`, `{"context":"missions","role":"owner"}`} {
		if _, err := s.fieldRPC(context.Background(), "getGoBGSTally", json.RawMessage(raw)); err == nil {
			t.Fatal("Invalid/foreign-account context authorized", raw)
		}
	}
	if _, err := s.fieldRPC(context.Background(), "exportGoBGSTally", json.RawMessage(`{"context":"missions"}`)); err == nil {
		t.Fatal("Context export skipped acknowledgment")
	}
}

func TestG2MBRouteCapacityRecoveryRequiresConfirmedInactivePlan(t *testing.T) {
	s, _, _, _, _ := g2eFixture(t)
	call := func(name string, v any) (any, error) {
		raw, _ := json.Marshal(v)
		return s.fieldRPC(context.Background(), name, raw)
	}
	db, _ := s.game.History()
	fid := s.game.FieldCommander()
	var firstID, firstHash string
	for i := 0; i < 16; i++ {
		if _, e := call("importGoItinerary", map[string]any{"name": fmt.Sprintf("Route %02d", i), "csv": "system\nAlpha\nBeta\n"}); e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			p, _ := db.Itinerary(fid, "")
			firstID = p.ID
			firstHash = p.Hash
		}
	}
	active, _ := db.Itinerary(fid, "")
	if _, e := call("importGoItinerary", map[string]any{"name": "Over limit", "csv": "system\nGamma\nDelta\n"}); e == nil {
		t.Fatal("unbounded import")
	}
	for _, req := range []map[string]any{{"id": firstID, "hash": firstHash}, {"id": active.ID, "hash": active.Hash, "acknowledge": true}, {"id": firstID, "hash": "wrong", "acknowledge": true}, {"id": firstID, "hash": firstHash, "fid": "OTHER", "acknowledge": true}} {
		if _, e := call("deleteGoItinerary", req); e == nil {
			t.Fatal("Unsafe delete", req)
		}
	}
	if _, e := call("deleteGoItinerary", map[string]any{"id": firstID, "hash": firstHash, "acknowledge": true}); e != nil {
		t.Fatal("confirmed inactive delete must free capacity", e)
	}
	p, e := db.Itinerary(fid, "")
	if e != nil || p.ID != active.ID || p.Next != active.Next {
		t.Fatal("active plan changed", p, e)
	}
	if _, e = call("importGoItinerary", map[string]any{"name": "Route 17", "csv": "system\nGamma\nDelta\n"}); e != nil {
		t.Fatal("capacity not recovered", e)
	}
	saved := false
	s.saveFieldCSV = func(name, body string) error {
		saved = true
		if name != "eridion-itinerary.csv" {
			t.Fatal("native export name", name)
		}
		return nil
	}
	if _, e = call("exportGoItinerary", map[string]any{"acknowledge": true}); e != nil || !saved {
		t.Fatal("native export", e, saved)
	}

}
