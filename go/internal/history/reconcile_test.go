package history

import (
	"context"
	"testing"
	"time"
)

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
