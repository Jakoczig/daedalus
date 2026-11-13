package gamestate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func g2bSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, item := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2b", item[0]))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, item[1]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func waitProjection(t *testing.T, s *Service, p func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for !p() {
		select {
		case <-ctx.Done():
			t.Fatalf("G2B view timed out: %+v %+v", s.Navigation(), s.Exploration())
		case <-ticker.C:
		}
	}
}
func TestG2BProfileSwitchClearsOldRouteAndBodies(t *testing.T) {
	dir := g2bSource(t)
	s := startService(t, dir)
	waitProjection(t, s, func() bool { return len(s.Navigation().Hops) == 2 && len(s.Exploration().Bodies) == 1 })
	appendEvent(t, dir, `{"event":"LoadGame","timestamp":"2026-01-01T00:00:09Z","FID":"SYNTHETIC-B"}`)
	waitState(t, s, func(v Summary) bool { return v.EventsRead == 10 })
	if n, e := s.Navigation(), s.Exploration(); len(n.Hops) != 0 || len(e.Bodies) != 0 || n.TargetSystem != "" || e.SystemName != "" {
		t.Fatal("previous commander projections leaked", n, e)
	}
	appendEvent(t, dir, `{"event":"Location","StarSystem":"Fixture Beta","SystemAddress":9007199254740994}`)
	waitProjection(t, s, func() bool { return s.Exploration().SystemName == "Fixture Beta" })
	if len(s.Exploration().Bodies) != 0 || len(s.Navigation().Hops) != 0 {
		t.Fatal("new commander reused old body/route")
	}
}
func TestG2BSourceClearAndShutdownCancelSubscriptions(t *testing.T) {
	s := startService(t, g2bSource(t))
	waitProjection(t, s, func() bool { return len(s.Navigation().Hops) == 2 })
	ctx, cancel := context.WithCancel(context.Background())
	ch, unsubscribe := s.SubscribeProjections(ctx)
	defer unsubscribe()
	<-ch
	if err := s.SetDirectory(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if n, e := s.Navigation(), s.Exploration(); len(n.Hops) != 0 || len(e.Bodies) != 0 {
		t.Fatal("source change retained projections", n, e)
	}
	cancel()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("cancelled subscription leaked")
		}
	}
}
func TestG2BReplayDoesNotFloodClients(t *testing.T) {
	dir := g2bSource(t)
	original, err := os.ReadFile(sourceFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	original = append(original, []byte(strings.Repeat(`{"event":"Scan","SystemAddress":9007199254740993,"BodyID":9007199254740997,"BodyName":"Fixture Alpha 1"}`+"\n", 3000))...)
	if err := os.WriteFile(sourceFile(dir), original, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(Options{Directory: dir, PollInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, stop := s.SubscribeProjections(ctx)
	defer stop()
	<-ch
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	defer func() { cancel(); <-done }()
	settled := false
	deadline := time.After(3 * time.Second)
	messages := 0
	for !settled {
		select {
		case p := <-ch:
			messages++
			if p.Source.ImportInProgress {
				if len(p.Exploration.Bodies) > 0 || len(p.Navigation.Hops) > 0 {
					t.Fatal("intermediate replay projected")
				}
			} else if len(p.Exploration.Bodies) == 1 {
				settled = true
			}
		case <-deadline:
			t.Fatal("no settled replay projection")
		}
	}
	if messages > 4 {
		t.Fatal("replay broadcast storm", messages)
	}
	if s.Details().ReaderStarts != 1 || s.Details().ActiveReaders != 1 {
		t.Fatal("projection started extra ingestion")
	}
}
func TestG2BProjectionSubscriptionsBoundedAndIsolated(t *testing.T) {
	s := startService(t, g2bSource(t))
	waitProjection(t, s, func() bool { return len(s.Navigation().Hops) == 2 })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for range 128 {
		_, unsubscribe := s.SubscribeProjections(ctx)
		defer unsubscribe()
	}
	ch, stop := s.SubscribeProjections(ctx)
	defer stop()
	if _, ok := <-ch; ok {
		t.Fatal("unbounded subscriptions")
	}
	v := s.Exploration()
	v.Bodies[0].Name = "changed"
	if s.Exploration().Bodies[0].Name != "Fixture Alpha 1" {
		t.Fatal("mutable API alias")
	}
}

func TestG2BBroadcastSnapshotsAreIndependent(t *testing.T) {
	dir := g2bSource(t)
	s := startService(t, dir)
	waitProjection(t, s, func() bool { return len(s.Navigation().Hops) == 2 && len(s.Exploration().Bodies) == 1 })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a, stopA := s.SubscribeProjections(ctx)
	defer stopA()
	b, stopB := s.SubscribeProjections(ctx)
	defer stopB()
	<-a
	<-b
	appendEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:09Z","Name":"Fixture Changed","SystemAddress":42}`)
	next := func(ch <-chan Projections) Projections {
		for {
			select {
			case p := <-ch:
				if p.Navigation.TargetSystem == "Fixture Changed" {
					return p
				}
			case <-ctx.Done():
				t.Fatal("broadcast missing")
				return Projections{}
			}
		}
	}
	first, second := next(a), next(b)
	first.Navigation.Hops[0].Name = "mutated"
	*first.Navigation.Hops[1].DistanceFromPreviousLY = 99
	first.Exploration.Bodies[0].Name = "mutated"
	*first.Exploration.Bodies[0].Landable = false
	first.Exploration.Bodies[0].Genuses[0] = "mutated"
	if second.Navigation.Hops[0].Name != "Fixture Alpha" || *second.Navigation.Hops[1].DistanceFromPreviousLY != 5 || second.Exploration.Bodies[0].Name != "Fixture Alpha 1" || !*second.Exploration.Bodies[0].Landable || second.Exploration.Bodies[0].Genuses[0] != "Bacterium" {
		t.Fatal("subscriber snapshots share memory")
	}
	s.mu.Lock()
	changed := s.views.last.Navigation.Hops[0].Name == "mutated" || s.views.last.Exploration.Bodies[0].Name == "mutated"
	s.mu.Unlock()
	if changed {
		t.Fatal("subscriber mutated dedup baseline")
	}
}
