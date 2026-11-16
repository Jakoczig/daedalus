package gamestate

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/history"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
	"path/filepath"
	"testing"
	"time"
)

func statisticsFixture(t testing.TB) (*Service, *sql.DB, func(int, bool)) {
	t.Helper()
	s, e := NewService(Options{EnableParity: true})
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	s.store, e = history.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.store.Close() })
	audit, e := sql.Open("sqlite3", filepath.Join(dir, "daedalus.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { audit.Close() })
	if _, e = audit.Exec("CREATE TABLE stats_write_audit(id INTEGER PRIMARY KEY); CREATE TRIGGER stats_insert AFTER INSERT ON derived_statistics BEGIN INSERT INTO stats_write_audit(id) VALUES(NULL); END;"); e != nil {
		t.Fatal(e)
	}
	s.summary.ImportInProgress = false
	s.summary.SourceState = "live"
	for i, raw := range []string{`{"event":"LoadGame","timestamp":"2026-11-16T12:00:00Z","FID":"SYNTHETIC"}`, `{"event":"Loadout","timestamp":"2026-11-16T12:00:01Z","ShipID":9007199254740993,"Ship":"asp","Modules":[{"Slot":"PowerPlant","Item":"fixture","On":true,"Engineering":{"Modifiers":[{"Label":"PowerCapacity","Value":20}]}},{"Slot":"MainEngines","Item":"fixture","On":true,"Priority":0,"Engineering":{"Modifiers":[{"Label":"PowerDraw","Value":6}]}},{"Slot":"LargeHardpoint1","Item":"fixture","On":true,"Priority":2,"Engineering":{"Modifiers":[{"Label":"PowerDraw","Value":3}]}}]}`} {
		v, e := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture", Sequence: uint64(i + 1), Offset: int64(i * 100), Length: 100})
		if e != nil {
			t.Fatal(e)
		}
		s.applyEventLocked(v, journal.Live)
	}
	update := func(i int, deployed bool) {
		stamp := time.Date(2026, 11, 16, 12, 1, 0, 0, time.UTC).Add(time.Duration(i) * time.Second)
		flags := 16777216
		if deployed {
			flags |= 64
		}
		raw := json.RawMessage(fmt.Sprintf(`{"event":"Status","timestamp":%q,"Flags":%d,"Fuel":{"FuelMain":%d,"FuelReservoir":0.5},"Cargo":%d,"Latitude":%d}`, stamp.Format(time.RFC3339), flags, i%30, i%5, i%80))
		s.mu.Lock()
		s.sidecarProjectionLocked(sidecar.Change{Kind: "Status", Valid: true, Timestamp: stamp, Raw: raw})
		s.mu.Unlock()
	}
	return s, audit, update
}
func statsWrites(t testing.TB, db *sql.DB) int {
	t.Helper()
	var n int
	if e := db.QueryRow("SELECT count(*) FROM stats_write_audit").Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func TestStatisticsStatusUpdatesDoNotWriteStaticCache(t *testing.T) {
	s, db, update := statisticsFixture(t)
	update(0, false)
	initial := s.EngineeredStatistics()
	if initial.StowedMW == nil || *initial.StowedMW != 6 {
		t.Fatal(initial)
	}
	writes := statsWrites(t, db)
	for i := 1; i <= 100; i++ {
		update(i, false)
		s.EngineeredStatistics()
	}
	if n := statsWrites(t, db); n != writes {
		t.Errorf("timestamp/fuel/cargo/latitude changes wrote cache: %d -> %d", writes, n)
	}
	update(101, true)
	v := s.EngineeredStatistics()
	if v.ObservedHardpointsDeployed == nil || !*v.ObservedHardpointsDeployed || v.ObservedDeploymentRatedMW == nil || *v.ObservedDeploymentRatedMW != 9 {
		t.Fatal("deployment did not update", v)
	}
	if n := statsWrites(t, db); n != writes {
		t.Errorf("deployment wrote static cache: %d -> %d", writes, n)
	}
	// Relevant loadout evidence changes with the same observed timestamp must invalidate.
	s.mu.Lock()
	s.views.parity.ApplyEvent(s.active, eventForStatistics(t, `{"event":"Loadout","timestamp":"2026-11-16T12:00:01Z","ShipID":9007199254740993,"Ship":"asp","Modules":[{"Slot":"MainEngines","Item":"fixture","On":true,"Priority":1,"Engineering":{"Modifiers":[{"Label":"PowerDraw","Value":7}]}}]}`, 3))
	s.mu.Unlock()
	v = s.EngineeredStatistics()
	if v.RatedPowerMW == nil || *v.RatedPowerMW != 7 || v.KnownPriorityMW[1] != 7 {
		t.Fatal("loadout did not invalidate", v)
	}
	if n := statsWrites(t, db); n != writes+1 {
		t.Fatal("new loadout did not persist exactly once", n)
	}
	// Priority-only and On-only changes invalidate, even at identical timestamps.
	for i, on := range []bool{true, false} {
		raw := fmt.Sprintf(`{"event":"Loadout","timestamp":"2026-11-16T12:00:01Z","ShipID":9007199254740993,"Ship":"asp","Modules":[{"Slot":"MainEngines","Item":"fixture","On":%t,"Priority":3,"Engineering":{"Modifiers":[{"Label":"PowerDraw","Value":7}]}}]}`, on)
		s.mu.Lock()
		s.views.parity.ApplyEvent(s.active, eventForStatistics(t, raw, uint64(4+i)))
		s.mu.Unlock()
		v = s.EngineeredStatistics()
		want := float64(7)
		if !on {
			want = 0
		}
		if v.KnownRatedMW != want || v.KnownPriorityMW[3] != want {
			t.Fatal("On/priority invalidation", v)
		}
	}
	if n := statsWrites(t, db); n != writes+3 {
		t.Fatal("On/priority cache writes", n)
	}
	// Simulate an in-memory cache loss: persistent static evidence is reused,
	// but the latest deployment observation is still applied.
	s.statisticsKey, s.statisticsJSON = "", nil
	update(102, false)
	v = s.EngineeredStatistics()
	if v.ObservedHardpointsDeployed == nil || *v.ObservedHardpointsDeployed || statsWrites(t, db) != writes+3 {
		t.Fatal("persistent cache reuse/deployment")
	}

}
func eventForStatistics(t testing.TB, raw string, sequence uint64) journal.Event {
	t.Helper()
	v, e := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture", Sequence: sequence, Offset: int64(sequence-1) * 100, Length: 100})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func BenchmarkStatisticsLiveStatus(b *testing.B) {
	s, db, update := statisticsFixture(b)
	update(0, false)
	s.EngineeredStatistics()
	before := statsWrites(b, db)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		update(i+1, i%2 == 1)
		s.EngineeredStatistics()
	}
	b.StopTimer()
	b.ReportMetric(float64(statsWrites(b, db)-before), "static-cache-writes")
	if statsWrites(b, db) != before {
		b.Fatal("live Status workload wrote static cache")
	}
}
