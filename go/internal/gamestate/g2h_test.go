package gamestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestG2HDestinationSurvivesBoundedGalaxyContext(t *testing.T) {
	dir := g2bSource(t)
	var route struct {
		Timestamp string           `json:"timestamp"`
		Event     string           `json:"event"`
		Route     []map[string]any `json:"Route"`
	}
	route.Timestamp = "2026-01-01T00:00:02Z"
	route.Event = "NavRoute"
	for i := 0; i < 220; i++ {
		name := fmt.Sprintf("Fixture Route %d", i)
		if i == 0 {
			name = "Fixture Alpha"
		}
		if i == 1 {
			name = "Fixture Beta"
		}
		route.Route = append(route.Route, map[string]any{"StarSystem": name, "SystemAddress": json.Number(fmt.Sprint(uint64(9007199254740993) + uint64(i))), "StarPos": []int{i, 0, 0}, "StarClass": "G"})
	}
	raw, _ := json.Marshal(route)
	if err := os.WriteFile(filepath.Join(dir, "NavRoute.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Journal.20260101.01.log")
	raw, _ = os.ReadFile(path)
	raw = []byte(strings.ReplaceAll(string(raw), `"RemainingJumpsInRoute":1`, `"RemainingJumpsInRoute":219`))
	os.WriteFile(path, raw, 0600)
	s := startService(t, dir)
	waitProjection(t, s, func() bool { return len(s.Navigation().Hops) == 220 })
	v := s.NavigationContext()
	if len(v.Points) > 192 {
		t.Fatal("unbounded context")
	}
	for _, p := range v.Points {
		if p.Name == "Fixture Route 219" && p.Coordinates != nil {
			return
		}
	}
	t.Fatal("known plotted destination was lost to the context budget")
}
