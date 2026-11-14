package gamestate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestG2CSubscriberDetailsAndOrganismsAreIndependent(t *testing.T) {
	dir := t.TempDir()
	for _, item := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2c", item[0]))
		if err != nil {
			t.Fatal(err)
		}
		if os.WriteFile(filepath.Join(dir, item[1]), raw, 0600) != nil {
			t.Fatal("fixture")
		}
	}
	s := startService(t, dir)
	waitProjection(t, s, func() bool {
		for _, b := range s.Exploration().Bodies {
			if len(b.Organisms) > 0 && b.Organisms[0].Complete {
				return true
			}
		}
		return false
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a, stopA := s.SubscribeProjections(ctx)
	defer stopA()
	b, stopB := s.SubscribeProjections(ctx)
	defer stopB()
	<-a
	<-b
	appendEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:20Z","Name":"Fixture New Target","SystemAddress":42}`)
	next := func(ch <-chan Projections) Projections {
		for {
			select {
			case p := <-ch:
				if p.Navigation.TargetSystem == "Fixture New Target" {
					return p
				}
			case <-ctx.Done():
				t.Fatal("missing broadcast")
				return Projections{}
			}
		}
	}
	first, second := next(a), next(b)
	index := -1
	for i, body := range first.Exploration.Bodies {
		if len(body.Organisms) > 0 {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("missing biology")
	}
	*first.Exploration.Bodies[index].Details.RadiusKM = 42
	first.Exploration.Bodies[index].Details.Parents[0].BodyID = "tampered"
	*first.Exploration.Bodies[index].Organisms[0].Samples = 42
	got := second.Exploration.Bodies[index]
	if *got.Details.RadiusKM != 1000 || got.Details.Parents[0].BodyID != "0" || *got.Organisms[0].Samples != 3 {
		t.Fatal("subscriber alias")
	}
	baseline := s.Exploration().Bodies[index]
	if *baseline.Details.RadiusKM != 1000 || *baseline.Organisms[0].Samples != 3 {
		t.Fatal("shared cached baseline mutated")
	}
}
