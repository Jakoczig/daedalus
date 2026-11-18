package gamestate

import (
	"context"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"github.com/Jakoczig/eridion/go/internal/shipstate"
	"testing"
	"time"
)

func TestSelectedCommanderDoesNotFollowOtherJournalSession(t *testing.T) {
	dir := t.TempDir()
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-A"}`)
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture A","SystemAddress":1}`)
	s := startService(t, dir)
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 2 })
	if e := s.SetAccounts(true, "SYNTHETIC-A"); e != nil {
		t.Fatal(e)
	}
	appendEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-B"}`)
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture B","SystemAddress":2}`)
	// The reader keeps ingesting B, while the selected projection remains A.
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 4 })
	if s.FieldCommander() != "SYNTHETIC-A" || s.Snapshot().CurrentSystem != "Fixture A" {
		t.Fatal("silent account switch", s.Snapshot())
	}
	if e := s.SetAccounts(true, "NOT-VERIFIED"); e == nil {
		t.Fatal("browser invented FID accepted")
	}
	if e := s.SetAccounts(true, "SYNTHETIC-B"); e != nil {
		t.Fatal(e)
	}
	if s.FieldCommander() != "SYNTHETIC-B" {
		t.Fatal("explicit switch failed")
	}
}

func TestCachedProjectionCannotInheritAnotherReplayCommanderAuthorization(t *testing.T) {
	s, e := NewService(Options{EnableParity: true})
	if e != nil {
		t.Fatal(e)
	}
	s.active = "SYNTHETIC-A"
	s.commanders[s.active] = commanderState{system: "Account A system"}
	s.holdProjectionLocked(Projections{Ship: shipstate.View{ShipType: "ACCOUNT-A-SHIP"}})
	s.summary.ImportInProgress = true
	if p := s.projectionLocked(); p.Ship.ShipType != "ACCOUNT-A-SHIP" {
		t.Fatal("same-account cached startup lost")
	}
	s.active = "SYNTHETIC-B"
	s.commanders[s.active] = commanderState{system: "Account B system"}
	if p := s.projectionLocked(); p.Ship.ShipType == "ACCOUNT-A-SHIP" {
		t.Fatal("B replay authorization exposed A's cached projection")
	}
	s.accounts = AccountSettings{Multiple: true, Selected: "SYNTHETIC-A"}
	if p := s.projectionLocked(); p.Ship.ShipType != "ACCOUNT-A-SHIP" {
		t.Fatal("explicit selected A cannot recover its own cached view while B replays")
	}
}

func TestLegacyPairingProofUsesCompletePersistedCommanderSet(t *testing.T) {
	for _, tc := range []struct {
		name                string
		two, multiple, safe bool
	}{{"single-account", false, false, true}, {"two-recorded-first-active-B", true, false, false}, {"explicit-multiple", false, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			dir, profile := t.TempDir(), t.TempDir()
			appendEvent(t, dir, `{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"SYNTHETIC-A"}`)
			n := uint64(1)
			if tc.two {
				appendEvent(t, dir, `{"event":"LoadGame","timestamp":"2026-01-01T00:00:02Z","FID":"SYNTHETIC-B"}`)
				n++
			}
			first, stop := durableService(t, profile, dir)
			waitState(t, first, func(v Summary) bool { return v.EventsRead == n && !v.ImportInProgress })
			fid := first.FieldCommander()
			if tc.multiple {
				if e := first.SetAccounts(true, fid); e != nil {
					t.Fatal(e)
				}
			}
			stop()
			seen := make(chan bool, 20)
			next, e := NewService(Options{StorageDir: profile, Directory: dir, EnableParity: true, Discovery: journal.DiscoveryOptions{Roots: []string{}}, PollInterval: 5 * time.Millisecond, DisplayAccount: func(got string, safe bool) {
				if got == fid {
					select {
					case seen <- safe:
					default:
					}
				}
			}})
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- next.Run(ctx) }()
			defer func() {
				cancel()
				if e := <-done; e != nil {
					t.Error(e)
				}
			}()
			select {
			case safe := <-seen:
				if safe != tc.safe {
					t.Fatalf("legacy association proof=%v want %v", safe, tc.safe)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("no restored account publication")
			}
		})
	}
}
