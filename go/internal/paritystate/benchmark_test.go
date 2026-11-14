package paritystate

import (
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
	"strings"
	"testing"
	"time"
)

// Includes detached public snapshot construction, the service's hot read path.
func BenchmarkStatusApplyAndView(b *testing.B) {
	m := NewModel()
	stamp := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	m.ApplyEvent("a", journal.Event{Type: "LoadGame", FID: "fixture", Timestamp: stamp, Raw: []byte(`{"Ship":"anaconda","ShipID":1}`)})
	rows := make([]string, 32)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"Slot":"S%d","Item":"int_powerplant_size2_class1","Health":1,"On":true,"Engineering":{"BlueprintName":"Example","Level":5,"Quality":1,"Modifiers":[{"Label":"Mass","Value":1,"OriginalValue":2,"LessIsGood":true}]}}`, i)
	}
	m.ApplyEvent("a", journal.Event{Type: "Loadout", Timestamp: stamp.Add(time.Second), Raw: []byte(`{"Ship":"anaconda","ShipID":1,"Modules":[` + strings.Join(rows, ",") + `]}`)})
	c := sidecar.Change{Kind: "Status", Valid: true, Timestamp: stamp.Add(2 * time.Second), Raw: []byte(`{"timestamp":"2026-11-14T10:00:02Z","event":"Status","Flags":16777224,"Pips":[4,4,4],"Fuel":{"FuelMain":32,"FuelReservoir":0.5},"Cargo":16}`)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.ApplySidecar("a", c)
		v := m.View("a")
		if len(v.Modules) != 32 || v.Status.State != "known" {
			b.Fatal("fixture state lost")
		}
	}
}

var snapshotCloneSink View

// Same fixture and process compare the replaced JSON method with typed copies.
func BenchmarkSnapshotClone(b *testing.B) {
	v := blank()
	health := 0.9
	level := 5
	quality := 0.8
	for i := 0; i < 32; i++ {
		v.Modules = append(v.Modules, Module{Slot: fmt.Sprintf("Slot%d", i), Item: "int_hyperdrive_size5_class5", Name: "Frame Shift Drive", Health: &health, Engineering: &Engineering{Blueprint: "FSD_LongRange", Grade: &level, Quality: &quality, Modifiers: []Modifier{{Label: "FSDOptimalMass", Value: &health, OriginalValue: &quality}}}})
	}
	b.Run("JSON_baseline", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			raw, err := json.Marshal(v)
			if err != nil {
				b.Fatal(err)
			}
			var out View
			if err = json.Unmarshal(raw, &out); err != nil {
				b.Fatal(err)
			}
			snapshotCloneSink = out
		}
	})
	b.Run("typed", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			snapshotCloneSink = Clone(v)
		}
	})
}
