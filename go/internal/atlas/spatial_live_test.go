package atlas

import (
	"context"
	"os"
	"testing"
	"time"
)

// Opt-in public static GET acceptance only. Never touches a real profile.
func TestSpatialLiveSignedRelease(t *testing.T) {
	if os.Getenv("ERIDION_SPATIAL_LIVE") != "1" {
		t.Skip("explicit public static HTTPS acceptance only")
	}
	m := NewSpatial(context.Background(), t.TempDir())
	defer m.Close()
	if e := m.Configure(SpatialPreferences{Downloads: true}); e != nil {
		t.Fatal(e)
	}
	t.Log("starting signed manifest + Bubble")
	start := time.Now()
	if e := m.Update(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	t.Log("install", time.Since(start), m.Status())
	q := SpatialQuery{Low: [3]float64{-100, -50, -100}, High: [3]float64{100, 50, 100}}
	start = time.Now()
	v, e := m.View(q, false)
	if e != nil || !v.Complete || len(v.Points) == 0 {
		t.Fatal(v.Notice, e)
	}
	t.Log("Sol", len(v.Points), v.Blocks, v.Decoded, time.Since(start))
}
