package atlas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSpatialIndependentQueryBudgets(t *testing.T) {
	for _, kind := range []string{"index-bytes", "block-count", "decoded-bytes"} {
		t.Run(kind, func(t *testing.T) {
			m := newSpatial(context.Background(), t.TempDir(), http.DefaultClient, make([]byte, 32), time.Now)
			defer m.Close()
			m.current = &spatialManifest{RootLY: 800}
			q := SpatialQuery{Low: [3]float64{0, 0, 0}, High: [3]float64{3000, 10, 1}}
			if kind == "index-bytes" {
				for i := range 3 {
					m.current.Roots = append(m.current.Roots, spatialRoot{Cell: [3]int64{int64(i), 0, 0}, SHA256: strings.Repeat("a", 64), Bytes: 2 << 20, Count: 1})
				}
			} else {
				var ds []spatialBlock
				count, blocks := 1, 129
				if kind == "decoded-bytes" {
					count = 16384
					blocks = 86
				}
				for i := range blocks {
					ds = append(ds, spatialBlock{Lo: [3]int32{0, int32(i), 0}, Hi: [3]int32{int32(count), int32(i + 1), 1}, Count: count, Bytes: 1, SHA256: fmt.Sprintf("%064x", i+1)})
				}
				b, _ := json.Marshal(ds)
				r := spatialRoot{SHA256: digest(b), Bytes: int64(len(b)), Count: int64(count * blocks)}
				m.current.Roots = []spatialRoot{r}
				writeFile(m.root, spatialPath("indexes", r.SHA256), b)
			}
			v, e := m.View(q, false)
			if e != nil || v.Complete || len(v.Points) != 0 || !strings.Contains(v.Notice, "budget") {
				t.Fatal("budget bypass", kind, v, e)
			}
		})
	}
}
