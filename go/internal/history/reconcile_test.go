package history

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"testing"
	"time"
)

func TestRemoteFetchCannotReplaceNewerWitnessedJournalFact(t *testing.T) {
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	now := at.Add(6 * time.Minute)
	for _, tc := range []struct {
		name     string
		observed time.Time
		expired  bool
		want     string
	}{
		{"unknown observation", time.Time{}, false, "journal"},
		{"older observation", at.Add(-time.Hour), false, "journal"},
		{"same observation", at, false, "journal"},
		{"newer observation", at.Add(time.Minute), false, "frontier"},
		{"future observation", now.Add(time.Hour), false, "journal"},
		{"expired newer snapshot", at.Add(time.Minute), true, "journal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			src := journal.Source{ID: "facts-fixture", Identity: "synthetic"}
			cp, err := s.Resume(src)
			if err != nil {
				t.Fatal(err)
			}
			e, err := journal.ParseLine([]byte(`{"event":"LoadGame","timestamp":"2026-11-15T12:00:00Z","FID":"SYNTHETIC-A","Commander":"Fixture","Credits":500}`), journal.SourcePosition{FileID: src.ID, Sequence: 1, Length: 100})
			if err != nil {
				t.Fatal(err)
			}
			cp.Offset, cp.Sequence = 100, 1
			if err = s.Commit(context.Background(), cp, []journal.Record{{Event: e, Phase: journal.Live}}, nil); err != nil {
				t.Fatal(err)
			}
			fetched := now
			if tc.expired {
				fetched = at
			}
			if err = s.SaveExternal(context.Background(), ExternalFacts{Provider: "frontier", Entity: "commander", EntityID: "SYNTHETIC-A", FID: "SYNTHETIC-A", IdentityVerified: true, Observed: tc.observed, Fetched: fetched, Expires: fetched.Add(5 * time.Minute), Fields: map[string]any{"credits": "100"}}); err != nil {
				t.Fatal(err)
			}
			// A different commander never contributes evidence to this comparison.
			if err = s.SaveExternal(context.Background(), ExternalFacts{Provider: "frontier", Entity: "commander", EntityID: "SYNTHETIC-B", FID: "SYNTHETIC-B", IdentityVerified: true, Observed: now, Fetched: now, Expires: now.Add(time.Minute), Fields: map[string]any{"credits": "999"}}); err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			facts, err := s.EffectiveFacts(context.Background(), "SYNTHETIC-A", "commander", now)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range facts {
				if f.Field != "credits" {
					continue
				}
				if f.Source != tc.want || len(f.Alternatives) != 1 {
					t.Fatalf("effective source = %+v, want %s with retained alternative", f, tc.want)
				}
				if f.Source == "journal" && string(f.Value) != `"500"` {
					t.Fatal("local evidence changed", f)
				}
				return
			}
			t.Fatal("credits missing")
		})
	}
}

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
