package history

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"testing"
	"time"
)

func TestProviderConsentHasOneCanonicalSource(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	initial := []byte(`{"providers":{"edsm":{"read":true,"upload":false}},"ledger":{},"uploader":"fixture"}`)
	raw, e := s.LoadProviderConsent(initial)
	if e != nil {
		t.Fatal(e)
	}
	var got ProviderDisk
	json.Unmarshal(raw, &got)
	if len(got.Providers) != 1 {
		t.Fatal("canonical consent lost")
	}
	raw, e = s.LoadProviderConsent([]byte(`{"providers":{"edsm":{"upload":true}}}`))
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &got)
	var c map[string]any
	json.Unmarshal(got.Providers["edsm"], &c)
	if c["upload"] != false {
		t.Fatal("legacy file expanded canonical permission")
	}
}
func TestPublicationDefaultNeverAndLiveIsCommanderScoped(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	src := journal.Source{ID: "policy-source", Identity: "fixture"}
	cp, _ := s.Resume(src)
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	var records []journal.Record
	for i, raw := range []string{`{"event":"LoadGame","FID":"SYNTHETIC-A"}`, `{"event":"FSDJump","timestamp":"2026-11-15T12:00:00Z","StarSystem":"Fixture Alpha","SystemAddress":9007199254740993}`} {
		v, e := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: src.ID, Sequence: uint64(i + 1), Offset: int64(i * 100), Length: 100})
		if e != nil {
			t.Fatal(e)
		}
		records = append(records, journal.Record{Event: v, Phase: journal.Live})
	}
	cp.Offset = 200
	cp.Sequence = 2
	if e = s.Commit(context.Background(), cp, records, nil); e != nil {
		t.Fatal(e)
	}
	c, e := s.Candidate("SYNTHETIC-A", "edsm", "travel", records[1].Event, journal.Live, false, now)
	if e != nil || c.Status != "held" {
		t.Fatalf("default publication: %+v %v", c, e)
	}
	if e = s.SetPublicationPolicy("SYNTHETIC-A", "Live"); e != nil {
		t.Fatal(e)
	}
	c, e = s.Candidate("SYNTHETIC-A", "edsm", "travel", records[1].Event, journal.Live, false, now)
	if e != nil || c.Status != "eligible" {
		t.Fatalf("explicit live blocked: %+v %v", c, e)
	}
	if _, e = s.Candidate("SYNTHETIC-B", "edsm", "travel", records[1].Event, journal.Live, false, now); e == nil {
		t.Fatal("other commander authorized event")
	}
	c, e = s.Candidate("SYNTHETIC-A", "edsm", "travel", records[1].Event, journal.Replay, false, now)
	if e != nil || c.Status == "eligible" {
		t.Fatal("historical replay became publication")
	}
}
