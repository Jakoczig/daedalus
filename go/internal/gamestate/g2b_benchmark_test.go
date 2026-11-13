package gamestate

import (
	"bytes"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
	"os"
	"testing"
	"time"
)

// Projection-only replay benchmark. Includes reducer locks and DTO construction,
// excludes source IO/parser, Wails, network and UI. Not retained RSS or idle CPU.
func BenchmarkProjectionReplay(b *testing.B) {
	const records = 50000
	raw, err := os.ReadFile("../../../tests/testdata/g2b/replay.jsonl")
	if err != nil {
		b.Fatal(err)
	}
	events := []journal.Event{}
	for i, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		e, err := journal.ParseLine(line, journal.SourcePosition{FileID: "fixture-bench", Offset: int64(i)})
		if err != nil {
			b.Fatal(err)
		}
		events = append(events, e)
	}
	routeRaw, err := os.ReadFile("../../../tests/testdata/g2b/NavRoute.json")
	if err != nil {
		b.Fatal(err)
	}
	stamp, _ := time.Parse(time.RFC3339Nano, "2026-01-01T00:00:02Z")
	routeChange := sidecar.Change{Kind: "NavRoute", Valid: true, State: "valid", Timestamp: stamp, Raw: routeRaw}
	update := events[6]
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		s, _ := NewService(Options{})
		s.views.navigation.ApplySidecar(routeChange)
		for _, e := range events {
			s.apply(e, journal.Replay)
		}
		for i := 0; i < records; i++ {
			e := update
			e.Position.Offset = int64(len(events) + i)
			s.apply(e, journal.Replay)
		}
		s.summary.ImportInProgress = false
		s.summary.SourceState = "live"
		if len(s.Exploration().Bodies) != 1 || len(s.Navigation().Hops) != 2 {
			b.Fatal("projection replay lost scan")
		}
	}
	b.ReportMetric(float64(records*b.N)/b.Elapsed().Seconds(), "events/s")
}
