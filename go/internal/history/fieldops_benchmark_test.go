package history

import (
	"context"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/fieldops"
	"testing"
)

func BenchmarkG2KFieldQueries(b *testing.B) {
	s, e := Open(b.TempDir())
	if e != nil {
		b.Fatal(e)
	}
	defer s.Close()
	tx, e := s.writer.Begin()
	if e != nil {
		b.Fatal(e)
	}
	if _, e = tx.Exec("INSERT INTO commander VALUES('SYNTHETIC','Fixture','2026-01-01T00:00:00Z'); INSERT INTO journal_sources(source_id,canonical_path,file_identity,observed_size,first_seen_at,last_seen_at) VALUES('bench','','synthetic',10000000,'','')"); e != nil {
		b.Fatal(e)
	}
	ev, e := tx.Prepare("INSERT INTO journal_events(event_id,source_id,byte_offset,byte_length,record_sequence,line_hash,event_utc,type,commander_fid,system_address,raw_relevant_json,ingested_at) VALUES(?,'bench',?,100,?,'fixture','2026-01-01T00:00:00Z','MissionFailed','SYNTHETIC','9007199254740993','{}','')")
	if e != nil {
		b.Fatal(e)
	}
	row, e := tx.Prepare("INSERT INTO field_tally VALUES(?,0,'SYNTHETIC','9007199254740993','Fixture faction','mission_failed','2026-01-01T00:00:00Z','','event','Fixture','','journal-effects-v1')")
	if e != nil {
		b.Fatal(e)
	}
	for i := 1; i <= 100000; i++ {
		if _, e = ev.Exec(i, i*100, i); e != nil {
			b.Fatal(e)
		}
		if _, e = row.Exec(i); e != nil {
			b.Fatal(e)
		}
	}
	ev.Close()
	row.Close()
	if e = tx.Commit(); e != nil {
		b.Fatal(e)
	}
	csv := "System Name\n"
	for i := 0; i < 2048; i++ {
		csv += fmt.Sprintf("Fixture %d\n", i)
	}
	if _, e = s.ImportItinerary("SYNTHETIC", "Max route", []byte(csv)); e != nil {
		b.Fatal(e)
	}
	b.Run("RouteProgress2048", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, e = s.ItineraryProgress("SYNTHETIC"); e != nil {
				b.Fatal(e)
			}
		}
	})
	b.Run("Tally100Of100000", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			v, e := s.FieldTally(context.Background(), "SYNTHETIC", "", "", 0, 100, false)
			if e != nil || len(v) != 100 {
				b.Fatal(e)
			}
		}
	})
	b.Run("BioCriteria116", func(b *testing.B) {
		scan := map[string]any{"AtmosphereType": "CarbonDioxide", "PlanetClass": "Rocky body", "SurfaceTemperature": 200.0, "SurfaceGravity": 1.0}
		for i := 0; i < b.N; i++ {
			if len(fieldops.Infer(scan, nil)) == 0 {
				b.Fatal("empty pack")
			}
		}
	})
}
