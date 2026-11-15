package history

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestProvenanceInspectionPreservesNestedUint64Text(t *testing.T) {
	raw := object([]byte(`{"CurrentGoals":[{"CGID":18446744073709551615,"MarketID":9007199254740993}]}`))
	encoded, err := json.Marshal(exactFactValue(raw))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	goal := decoded["CurrentGoals"].([]any)[0].(map[string]any)
	if goal["CGID"] != "18446744073709551615" || goal["MarketID"] != "9007199254740993" {
		t.Fatal("unsafe JSON number in provenance inspection", string(encoded))
	}
}

func TestReconciliationPreservesFactsAndFrontierPrecedence(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for _, x := range []ExternalFacts{
		{Provider: "inara", Entity: "commander", EntityID: "SYNTHETIC", FID: "SYNTHETIC", IdentityVerified: true, Observed: now.Add(-time.Hour), Fetched: now, Expires: now.Add(time.Hour), Fields: map[string]any{"name": "Fixture", "credits": "100"}},
		{Provider: "frontier", Entity: "commander", EntityID: "SYNTHETIC", FID: "SYNTHETIC", IdentityVerified: true, Observed: now.Add(-2 * time.Hour), Fetched: now, Expires: now.Add(time.Hour), Fields: map[string]any{"name": "Fixture", "credits": "200"}},
	} {
		if err = s.SaveExternal(context.Background(), x); err != nil {
			t.Fatal(err)
		}
	}
	facts, err := s.EffectiveFacts(context.Background(), "SYNTHETIC", "commander", now)
	if err != nil || len(facts) != 2 {
		t.Fatalf("facts: %+v %v", facts, err)
	}
	for _, f := range facts {
		if f.Source != "frontier" || len(f.Alternatives) != 1 {
			t.Fatalf("precedence/conflict: %+v", f)
		}
	}
	if s.NeedsInara(context.Background(), "SYNTHETIC", "commander", []string{"credits"}, now) {
		t.Fatal("redundant Inara request")
	}
	if !s.NeedsInara(context.Background(), "SYNTHETIC", "commander", []string{"rank"}, now) {
		t.Fatal("missing field hidden")
	}
	x := ExternalFacts{Provider: "inara", Entity: "commander", EntityID: "SYNTHETIC", FID: "SYNTHETIC", Fetched: now, Expires: now.Add(time.Hour), Fields: map[string]any{"name": "Fuzzy other player"}}
	if s.SaveExternal(context.Background(), x) == nil {
		t.Fatal("unverified identity stored")
	}
	x.IdentityVerified = true
	x.Fields = map[string]any{"accessToken": "secret"}
	if s.SaveExternal(context.Background(), x) == nil {
		t.Fatal("credential field stored")
	}
}
