package navcontext

import (
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/paritystate"
	"testing"
	"time"
)

func feed(t *testing.T, m *Model, key, raw string, phase journal.Phase) {
	t.Helper()
	e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{})
	if err != nil {
		t.Fatal(err)
	}
	m.Apply(key, e, phase)
}
func TestLiveJumpReplayIsolationAndExpiry(t *testing.T) {
	m := New()
	feed(t, m, "a", `{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"PRIVATE"}`, journal.Replay)
	feed(t, m, "a", `{"event":"Location","timestamp":"2026-01-01T00:00:02Z","StarSystem":"Fixture","SystemAddress":9007199254740993,"StarPos":[3,4,5],"SystemSecurity_Localised":"High Security","Powers":["Fixture power"],"Latitude":42}`, journal.Replay)
	jump := `{"event":"StartJump","timestamp":"2026-01-01T00:00:03Z","JumpType":"Hyperspace"}`
	feed(t, m, "a", jump, journal.Replay)
	at, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:04Z")
	if m.View("a", at).Mode != "idle" {
		t.Fatal("replay auto transition")
	}
	feed(t, m, "a", jump, journal.Live)
	v := m.View("a", at)
	if v.Mode != "jump" || v.Current.Coordinates[1] != 4 || v.Current.Address != "9007199254740993" {
		t.Fatal(v)
	}
	v.Current.Coordinates[0] = 9
	if m.View("a", at).Current.Coordinates[0] != 3 {
		t.Fatal("alias")
	}
	if m.View("b", at).Current != nil || m.View("a", at.Add(time.Minute)).Mode != "idle" {
		t.Fatal("isolation/expiry")
	}
	feed(t, m, "a", `{"event":"FSDJump","timestamp":"2026-01-01T00:00:05Z","StarSystem":"Other","SystemAddress":2}`, journal.Live)
	if m.View("a", at).Mode != "idle" {
		t.Fatal("arrival did not restore")
	}
}
func TestChargeRequiresExplicitHyperdriveAndCancel(t *testing.T) {
	m := New()
	feed(t, m, "a", `{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"a"}`, journal.Live)
	feed(t, m, "a", `{"event":"Location","timestamp":"2026-01-01T00:00:02Z","StarSystem":"Fixture","SystemAddress":1}`, journal.Live)
	v := paritystate.StatusView{State: "known", ObservedAt: "2026-01-01T00:00:03Z", Flags: &paritystate.StatusFlags{InMainShip: true, FSDCharging: true}}
	m.Status("a", v)
	at, _ := time.Parse(time.RFC3339, v.ObservedAt)
	if m.View("a", at).Mode != "idle" {
		t.Fatal("generic supercruise charge accepted")
	}
	v.Flags2 = &paritystate.StatusFlags2{FSDHyperdriveCharging: true}
	m.Status("a", v)
	if m.View("a", at).Mode != "charge" {
		t.Fatal("hyperdrive ignored")
	}
	v.Flags2.FSDHyperdriveCharging = false
	m.Status("a", v)
	if m.View("a", at).Mode != "idle" {
		t.Fatal("cancel ignored")
	}
}

func TestOnlyPublicNumericStarPositions(t *testing.T) {
	if Coordinates([]byte(`{"StarPos":["1",2,3]}`)) != nil || Coordinates([]byte(`{"Latitude":1,"Longitude":2,"Altitude":3}`)) != nil {
		t.Fatal("unverified/surface coordinates")
	}
	m := New()
	feed(t, m, "a", `{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"a"}`, journal.Live)
	feed(t, m, "a", `{"event":"Location","timestamp":"2026-01-01T00:00:02Z","StarSystem":"One","SystemAddress":1,"StarPos":[1,2,3]}`, journal.Live)
	feed(t, m, "b", `{"event":"LoadGame","timestamp":"2026-01-01T00:00:03Z","FID":"b"}`, journal.Live)
	if len(m.View("b", time.Now()).Points) != 0 {
		t.Fatal("foreign commander positions")
	}
}

func TestCancelledChargeRejectsOlderDelayedJump(t *testing.T) {
	m := New()
	feed(t, m, "a", `{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"a"}`, journal.Live)
	feed(t, m, "a", `{"event":"Location","timestamp":"2026-01-01T00:00:02Z","StarSystem":"One","SystemAddress":1}`, journal.Live)
	v := paritystate.StatusView{State: "known", ObservedAt: "2026-01-01T00:00:04Z", Flags: &paritystate.StatusFlags{InMainShip: true}, Flags2: &paritystate.StatusFlags2{FSDHyperdriveCharging: true}}
	m.Status("a", v)
	v.ObservedAt = "2026-01-01T00:00:05Z"
	v.Flags2.FSDHyperdriveCharging = false
	m.Status("a", v)
	feed(t, m, "a", `{"event":"StartJump","JumpType":"Hyperspace","timestamp":"2026-01-01T00:00:03Z"}`, journal.Live)
	at, _ := time.Parse(time.RFC3339, v.ObservedAt)
	if m.View("a", at).Mode != "idle" {
		t.Fatal("cancelled charge resurrected by delayed jump")
	}
}
