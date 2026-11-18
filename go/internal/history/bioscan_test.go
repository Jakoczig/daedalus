package history

import (
	"context"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"math"
	"testing"
	"time"
)

func TestHistoricalBioDistancesRequireVerifiedCycleAndSession(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps []string
		want  bool
	}{
		{"verified Log Sample", []string{"Log", "Sample"}, true},
		{"verified completed cycle", []string{"Log", "Sample", "Analyse"}, true},
		{"same body new session missing Log", []string{"Log", "Sample", "session", "Sample"}, false},
		{"same body new session verified cycle", []string{"Log", "Sample", "session", "Log", "Sample", "Analyse"}, true},
		{"missing Log", []string{"Sample", "Analyse"}, false},
		{"missing Sample", []string{"Log", "Analyse"}, false},
		{"missing Analyse then new Log", []string{"Log", "Sample", "Log", "Sample"}, true},
		{"repeated Sample without new Log", []string{"Log", "Sample", "Sample"}, false},
		{"missing session", []string{"Log", "Sample", "erase-session"}, false},
		{"intervening body departure", []string{"Log", "leave", "Sample"}, false},
		{"retained metadata gap", []string{"Log", "metadata-gap", "Sample"}, false},
		{"journal gap", []string{"Log", "gap", "Sample"}, false},
		{"unknown sample position", []string{"Log", "no-position", "Sample", "Analyse"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := Open(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			src := journal.Source{ID: "cycle", Identity: "synthetic"}
			cp, _ := s.Resume(src)
			session := 0
			lastID := int64(0)
			position := true
			organic := 0
			add := func(raw string) {
				t.Helper()
				v, e := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: src.ID, Sequence: cp.Sequence + 1, Offset: cp.Offset, Length: 100})
				if e != nil {
					t.Fatal(e)
				}
				cp.Offset += 100
				cp.Sequence++
				if e = s.Commit(context.Background(), cp, []journal.Record{{Event: v, Phase: journal.Replay}}, nil); e != nil {
					t.Fatal(e)
				}
			}
			add(`{"event":"LoadGame","FID":"SYNTHETIC"}`)
			for _, step := range tc.steps {
				switch step {
				case "session":
					session++
					add(`{"event":"LoadGame","FID":"SYNTHETIC"}`)
					continue
				case "erase-session":
					if _, e = s.writer.Exec("UPDATE journal_events SET session_id=NULL WHERE event_id=?", lastID); e != nil {
						t.Fatal(e)
					}
					continue
				case "leave":
					add(`{"event":"LeaveBody"}`)
					continue
				case "metadata-gap":
					add(`{"event":"Music","MusicTrack":"synthetic"}`)
					if _, e = s.writer.Exec("DELETE FROM journal_events WHERE type='Music'"); e != nil {
						t.Fatal(e)
					}
					continue
				case "gap":
					cp.Sequence++
					continue
				case "no-position":
					position = false
					continue
				}
				organic++
				add(fmt.Sprintf(`{"event":"ScanOrganic","timestamp":"2026-11-16T12:00:%02dZ","SystemAddress":9007199254740993,"Body":1,"ScanType":%q,"Genus":"$Codex_Ent_Bacterial_Genus_Name;","Species":"fixture"}`, organic, step))
				if e = s.reader.QueryRow("SELECT max(event_id) FROM journal_events").Scan(&lastID); e != nil {
					t.Fatal(e)
				}
				if position {
					if _, e = s.writer.Exec("INSERT INTO bio_positions VALUES(?,?,?,?,?,?,?,?,?)", lastID, "SYNTHETIC", "9007199254740993", "1", 0, float64(organic)*0.01+float64(session)*30, 1000000, "2026-11-16T12:00:00Z", "synthetic correlated snapshot"); e != nil {
						t.Fatal(e)
					}
				}
				position = true
			}
			v, e := s.BioBody(context.Background(), "SYNTHETIC", "9007199254740993", "1")
			if e != nil {
				t.Fatal(e)
			}
			samples := v["samples"].([]BioSample)
			found := false
			// A gap may cause the latest unauthenticated event to have no commander;
			// retained older evidence must never be mistaken for that target record.
			for _, sample := range samples {
				if sample.Event != lastID {
					continue
				}
				found = true
				if tc.want {
					if sample.DistanceM == nil || math.Abs(*sample.DistanceM-174.532925) > 0.01 {
						t.Fatalf("valid cycle lost distance: %+v", sample)
					}
				} else if sample.DistanceM != nil || sample.RemainingM != nil {
					t.Fatalf("unverified cycle produced spacing: %+v", sample)
				}
			}
			if !found && tc.name != "journal gap" {
				t.Fatal("target sample missing")
			}
			if found && tc.name == "journal gap" {
				t.Fatal("gapped journal must lose attribution")
			}
		})
	}
}

func TestBioSnapshotsRequireCurrentCorrelatedBodyAndDoNotInventReplayLocations(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	src := journal.Source{ID: "bio-position", Identity: "synthetic"}
	cp, _ := s.Resume(src)
	now := time.Now().UTC().Truncate(time.Second)
	records := []journal.Record{}
	for i, raw := range []string{`{"event":"LoadGame","FID":"SYNTHETIC"}`, fmt.Sprintf(`{"event":"ScanOrganic","timestamp":%q,"SystemAddress":9007199254740993,"Body":18446744073709551615,"ScanType":"Log","Genus":"$Codex_Ent_Bacterial_Genus_Name;","Species":"fixture"}`, now.Format(time.RFC3339))} {
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
	status := []byte(fmt.Sprintf(`{"event":"Status","timestamp":%q,"Flags":2097152,"BodyName":"Fixture 1","Latitude":0,"Longitude":179.9,"PlanetRadius":1000000}`, now.Format(time.RFC3339)))
	for _, x := range []struct {
		fid, system, body, name string
		at                      time.Time
	}{{"OTHER", "9007199254740993", "18446744073709551615", "Fixture 1", now}, {"SYNTHETIC", "9007199254740994", "18446744073709551615", "Fixture 1", now}, {"SYNTHETIC", "9007199254740993", "2", "Fixture 1", now}, {"SYNTHETIC", "9007199254740993", "18446744073709551615", "Other body", now}, {"SYNTHETIC", "9007199254740993", "18446744073709551615", "Fixture 1", now.Add(10 * time.Second)}} {
		if e = s.RecordBioPosition(x.fid, x.system, x.body, x.name, status, x.at, src.ID); e != nil {
			t.Fatal(e)
		}
	}
	var n int
	s.reader.QueryRow("SELECT count(*) FROM bio_positions").Scan(&n)
	if n != 0 {
		t.Fatal("uncorrelated snapshot stored")
	}
	if e = s.RecordBioPosition("SYNTHETIC", "9007199254740993", "18446744073709551615", "Fixture 1", status, now, src.ID); e != nil {
		t.Fatal(e)
	}
	v, e := s.BioBody(context.Background(), "SYNTHETIC", "9007199254740993", "18446744073709551615")
	if e != nil {
		t.Fatal(e)
	}
	samples := v["samples"].([]BioSample)
	if len(samples) != 1 || samples[0].Position == nil || samples[0].DistanceM != nil {
		t.Fatal("position/range invented", v)
	}
	v, e = s.BioBody(context.Background(), "SYNTHETIC", "9007199254740993", "2")
	if e != nil || len(v["samples"].([]BioSample)) != 0 {
		t.Fatal("body separation", v, e)
	}
}
