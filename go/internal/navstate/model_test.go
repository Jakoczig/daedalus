package navstate

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
)

const routeFixture = `{"timestamp":"2026-01-01T00:00:02Z","event":"NavRoute","Route":[{"StarSystem":"Fixture Alpha","SystemAddress":9007199254740993,"StarPos":[0,0,0],"StarClass":"G"},{"StarSystem":"Fixture Beta","SystemAddress":9007199254740994,"StarPos":[3,4,0],"StarClass":"M"}]}`

func event(t *testing.T, raw string) journal.Event {
	t.Helper()
	e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "fixture-1"})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func session(t *testing.T, m *Model, key string) {
	t.Helper()
	m.ApplyEvent(key, event(t, `{"timestamp":"2026-01-01T00:00:01Z","event":"LoadGame","FID":"SYNTHETIC-`+key+`"}`))
}
func marker(t *testing.T, m *Model, key string) {
	t.Helper()
	m.ApplyEvent(key, event(t, `{"timestamp":"2026-01-01T00:00:02Z","event":"NavRoute"}`))
}
func file(raw string) sidecar.Change {
	stamp, _ := time.Parse(time.RFC3339Nano, "2026-01-01T00:00:02Z")
	return sidecar.Change{Kind: "NavRoute", Valid: true, State: "valid", Timestamp: stamp, Raw: json.RawMessage(raw)}
}
func TestRouteSidecarMarkerBeforeAndAfterWrite(t *testing.T) {
	for _, first := range []bool{true, false} {
		t.Run(fmt.Sprint(first), func(t *testing.T) {
			m := NewModel()
			session(t, m, "A")
			if first {
				marker(t, m, "A")
			} else {
				m.ApplySidecar(file(routeFixture))
			}
			if v := m.View("A"); len(v.Hops) != 0 || v.DestinationSystem != "" {
				t.Fatal("uncorrelated route actionable", v)
			}
			if first {
				m.ApplySidecar(file(routeFixture))
			} else {
				marker(t, m, "A")
			}
			v := m.View("A")
			if len(v.Hops) != 2 || v.DestinationSystem != "Fixture Beta" || v.RouteState != "verified" {
				t.Fatalf("route not correlated: %+v", v)
			}
			// Views are immutable snapshots, never aliases into shared model memory.
			v.Hops[0].Name = "tampered"
			if m.View("A").Hops[0].Name != "Fixture Alpha" {
				t.Fatal("view aliases model")
			}
		})
	}
}
func TestRouteOldCommanderCannotReuseSidecar(t *testing.T) {
	m := NewModel()
	session(t, m, "A")
	marker(t, m, "A")
	m.ApplySidecar(file(routeFixture))
	m.ApplyEvent("B", event(t, `{"timestamp":"2026-01-01T00:00:03Z","event":"LoadGame","FID":"SYNTHETIC-B"}`))
	marker(t, m, "B")
	if v := m.View("B"); len(v.Hops) != 0 || v.DestinationSystem != "" {
		t.Fatal("old commander route reused", v)
	}
	if len(m.View("A").Hops) != 2 {
		t.Fatal("switch erased unrelated private route")
	}
	m.ApplyEvent("", event(t, `{"event":"LoadGame"}`))
	marker(t, m, "")
	if len(m.View("").Hops) != 0 {
		t.Fatal("unknown identity trusted")
	}
}
func TestRouteNavRouteClearStaysCleared(t *testing.T) {
	m := NewModel()
	session(t, m, "A")
	marker(t, m, "A")
	m.ApplySidecar(file(routeFixture))
	m.ApplyEvent("A", event(t, `{"timestamp":"2026-01-01T00:00:03Z","event":"NavRouteClear"}`))
	m.ApplySidecar(file(routeFixture))
	marker(t, m, "A")
	if v := m.View("A"); v.RouteState != "cleared" || len(v.Hops) != 0 || v.DestinationSystem != "" {
		t.Fatal("cleared route resurrected", v)
	}
}
func TestRouteRejectsOverflowNonfiniteOrWrongTimestamp(t *testing.T) {
	tooMany := strings.Replace(routeFixture, `[{"StarSystem"`, `[{"StarSystem"`, 1)
	hop := `{"StarSystem":"Fixture Overflow","SystemAddress":123,"StarPos":[0,0,0],"StarClass":"G"}`
	tooMany = `{"event":"NavRoute","timestamp":"2026-01-01T00:00:02Z","Route":[` + strings.TrimSuffix(strings.Repeat(hop+",", 2049), ",") + `]}`
	for _, raw := range []string{tooMany, strings.Replace(routeFixture, "[3,4,0]", "[1e400,0,0]", 1), strings.Replace(routeFixture, "00:00:02Z", "00:00:04Z", 1), strings.Replace(routeFixture, `"StarClass":"G"`, `"StarClass":"NOT_A_CLASS"`, 1), strings.Replace(routeFixture, "[3,4,0]", "[999999999,0,0]", 1)} {
		m := NewModel()
		session(t, m, "A")
		marker(t, m, "A")
		m.ApplySidecar(file(raw))
		if v := m.View("A"); len(v.Hops) != 0 || v.DestinationSystem != "" {
			t.Fatal("invalid route actionable", v)
		}
	}
	m := NewModel()
	session(t, m, "A")
	marker(t, m, "A")
	m.ApplySidecar(file(routeFixture))
	m.ApplySidecar(sidecar.Change{Kind: "NavRoute", State: "invalid"})
	if v := m.View("A"); v.RouteState != "stale" {
		t.Fatal("bad rewrite did not mark last-good stale", v)
	}
}
func TestRouteExactLargeSystemAddress(t *testing.T) {
	m := NewModel()
	session(t, m, "A")
	marker(t, m, "A")
	m.ApplySidecar(file(routeFixture))
	// Internal identity must keep the low bit that float64 would erase.
	if got := m.sessions["A"].route.hops[0].address; got != "9007199254740993" {
		t.Fatal("rounded route address", got)
	}
	raw, err := json.Marshal(m.View("A"))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"SystemAddress", "StarPos", "SYNTHETIC", "9007199254740993"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("private route projection", string(raw))
		}
	}
}

func verified(t *testing.T) *Model {
	t.Helper()
	m := NewModel()
	session(t, m, "A")
	marker(t, m, "A")
	m.ApplySidecar(file(routeFixture))
	return m
}
func arrive(t *testing.T, m *Model, name, address string) {
	t.Helper()
	m.ApplyEvent("A", event(t, `{"event":"FSDJump","timestamp":"2026-01-01T00:00:04Z","StarSystem":"`+name+`","SystemAddress":`+address+`}`))
}
func TestRouteAdvancesOnlyOnConfirmedArrival(t *testing.T) {
	m := verified(t)
	arrive(t, m, "Fixture Alpha", "9007199254740993")
	v := m.View("A")
	if v.CurrentSystem != "Fixture Alpha" || v.RemainingJumps == nil || *v.RemainingJumps != 1 || !v.Hops[0].IsCurrent || !v.Hops[1].IsNext || v.Hops[1].DistanceFromPreviousLY == nil || *v.Hops[1].DistanceFromPreviousLY != 5 {
		t.Fatalf("bad initial progression: %+v", v)
	}
	m.ApplyEvent("A", event(t, `{"event":"StartJump","timestamp":"2026-01-01T00:00:05Z","StarSystem":"Fixture Beta","SystemAddress":9007199254740994,"JumpType":"Hyperspace"}`))
	if v = m.View("A"); v.CurrentSystem != "Fixture Alpha" || *v.RemainingJumps != 1 {
		t.Fatal("StartJump advanced route", v)
	}
	arrive(t, m, "Fixture Beta", "9007199254740994")
	v = m.View("A")
	if v.CurrentSystem != "Fixture Beta" || *v.RemainingJumps != 0 || !v.Hops[1].IsCurrent {
		t.Fatal("arrival failed", v)
	}
}
func TestFSDTargetBeforeArrivalDoesNotMoveCurrent(t *testing.T) {
	m := verified(t)
	arrive(t, m, "Fixture Alpha", "9007199254740993")
	m.ApplyEvent("A", event(t, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:05Z","Name":"Fixture Beta","SystemAddress":9007199254740994,"RemainingJumpsInRoute":7}`))
	v := m.View("A")
	if v.TargetSystem != "Fixture Beta" || v.TargetEvidence != "journal_target" || v.CurrentSystem != "Fixture Alpha" || v.ReportedRemainingJumps == nil || *v.ReportedRemainingJumps != 7 || *v.RemainingJumps != 1 {
		t.Fatalf("target confused with position/count: %+v", v)
	}
}
func TestRouteTargetConflictMarked(t *testing.T) {
	m := verified(t)
	arrive(t, m, "Fixture Alpha", "9007199254740993")
	m.ApplyEvent("A", event(t, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:05Z","Name":"Fixture Elsewhere","SystemAddress":99}`))
	v := m.View("A")
	if v.TargetSystem != "Fixture Elsewhere" || v.TargetEvidence != "route_conflict" || v.DestinationSystem != "Fixture Beta" || v.Hops[1].IsNext {
		t.Fatal("conflict hidden", v)
	}
}
func TestRouteUnmatchedCurrentHasUnknownRemaining(t *testing.T) {
	m := verified(t)
	arrive(t, m, "Fixture Alpha", "999")
	v := m.View("A")
	if v.RemainingJumps != nil || v.TargetSystem != "" || v.Hops[0].IsCurrent {
		t.Fatal("matched by name or invented next", v)
	}
}
func TestRouteDirectJumpDoesNotFakePlottedRoute(t *testing.T) {
	m := NewModel()
	session(t, m, "A")
	arrive(t, m, "Fixture Direct", "42")
	if v := m.View("A"); v.CurrentSystem != "Fixture Direct" || v.RemainingJumps != nil || v.DestinationSystem != "" || len(v.Hops) != 0 {
		t.Fatal("invented plotted route", v)
	}
}
func TestRouteClearRemovesActiveHopsAndCannotResurrect(t *testing.T) {
	TestRouteNavRouteClearStaysCleared(t)
}
func TestRouteReplayEvidenceAndSessionReset(t *testing.T) {
	m := NewModel()
	session(t, m, "A")
	m.ApplyEventPhase("A", event(t, `{"event":"NavRoute","timestamp":"2026-01-01T00:00:02Z"}`), journal.Replay)
	m.ApplySidecar(file(routeFixture))
	if v := m.View("A"); v.RouteState != "verified_last_known" {
		t.Fatal("replayed route claimed live", v)
	}
	m.ApplyEvent("A", event(t, `{"event":"LoadGame","FID":"SYNTHETIC-A","timestamp":"2026-01-01T00:00:03Z"}`))
	marker(t, m, "A")
	if v := m.View("A"); len(v.Hops) != 0 || v.TargetSystem != "" {
		t.Fatal("same commander stale session reused", v)
	}
}

func TestRouteRejectsNullCoordinatesAndCombinedStarClass(t *testing.T) {
	for i, raw := range []string{strings.Replace(routeFixture, "[3,4,0]", "[null,4,0]", 1), strings.Replace(routeFixture, `"StarClass":"G"`, `"StarClass":"O|B"`, 1)} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			m := verified(t)
			m.ApplySidecar(file(raw))
			v := m.View("A")
			if len(v.Hops) != 0 || v.DestinationSystem != "" || v.RouteState != "stale" {
				t.Fatalf("malformed rewrite fabricated valid route: %+v", v)
			}
		})
	}
}

func TestRoutePublicPayloadHasByteBudget(t *testing.T) {
	hop := `{"StarSystem":"` + strings.Repeat("🚀", 128) + `","SystemAddress":123,"StarPos":[0,0,0],"StarClass":"G"}`
	raw := `{"event":"NavRoute","timestamp":"2026-01-01T00:00:02Z","Route":[` + strings.TrimSuffix(strings.Repeat(hop+",", 2048), ",") + `]}`
	m := NewModel()
	session(t, m, "A")
	marker(t, m, "A")
	m.ApplySidecar(file(raw))
	v := m.View("A")
	data, _ := json.Marshal(v)
	if len(data) > 256<<10 || !v.Truncated || len(v.Hops) != 0 || v.DestinationSystem != "" {
		t.Fatal("unbounded/fake partial route", len(data), len(v.Hops), v.Truncated)
	}
}
