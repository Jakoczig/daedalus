package history

import (
	"context"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"testing"
	"time"
)

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
