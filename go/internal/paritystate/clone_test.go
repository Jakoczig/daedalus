package paritystate

import (
	"github.com/Jakoczig/eridion/go/internal/shipstate"
	"reflect"
	"testing"
)

func TestCloneDetachesEveryMutableField(t *testing.T) {
	fixture := func() View {
		f := func() *float64 { v := 1.0; return &v }
		i := func() *int { v := 1; return &v }
		b := func() *bool { v := true; return &v }
		return View{Evidence: "journal_last_known", Ship: shipstate.View{ShipType: "adder", Evidence: "journal_last_known", HullHealth: f(), FuelMainT: f(), FuelCapacityT: f(), CargoCapacityT: f(), MaxJumpRangeLY: f(), UnladenMassT: f(), Docked: b(), Landed: b()}, Modules: []Module{{Slot: "A", Item: "module", Name: "Module", Health: f(), On: b(), Priority: i(), AmmoInClip: i(), AmmoInHopper: i(), Engineering: &Engineering{Blueprint: "Example", Grade: i(), Quality: f(), Experimental: "Effect", Modifiers: []Modifier{{Label: "Mass", Value: f(), OriginalValue: f(), LessIsGood: b()}}}, Reference: &ModuleReference{Source: "reference", Class: i(), Rating: "A", MassT: f(), PowerMW: f(), Description: "Description"}}}, ModulesState: "journal_last_known", Cargo: CargoView{State: "known", Count: i(), Items: []CargoItem{{Name: "gold", Label: "Gold", Count: 1, Stolen: i(), Mission: true}}}, Inventory: InventoryView{State: "known", Items: []InventoryItem{{Name: "item", Label: "Item", Category: "items", Location: "backpack", Count: 1}}}, Materials: MaterialView{State: "known", Items: []MaterialItem{{Name: "iron", Label: "Iron", Category: "raw", Count: 1}}}, Status: StatusView{State: "known", ObservedAt: "2026-11-14T10:00:02Z", Flags: &StatusFlags{InMainShip: true}, Flags2: &StatusFlags2{GlideMode: true}, Pips: &Pips{Sys: 2, Eng: 2, Wep: 2}, FuelMainT: f(), FuelReservoirT: f(), CargoT: f(), Temperature: f(), GuiFocus: i()}}
	}
	original := fixture()
	cloned := Clone(original)
	if !reflect.DeepEqual(original, cloned) {
		t.Fatal("clone changed explicit values")
	}
	// Mutate every leaf through the clone, including pointers nested in slices.
	// Any shared pointer or backing array would now change the source fixture.
	var mutate func(reflect.Value)
	mutate = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				mutate(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				mutate(v.Field(i))
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				mutate(v.Index(i))
			}
		case reflect.String:
			v.SetString("changed")
		case reflect.Bool:
			v.SetBool(!v.Bool())
		case reflect.Int:
			v.SetInt(v.Int() + 1)
		case reflect.Float64:
			v.SetFloat(v.Float() + 1)
		}
	}
	mutate(reflect.ValueOf(&cloned).Elem())
	if !reflect.DeepEqual(original, fixture()) {
		t.Fatal("clone mutation reached original view")
	}
}

func TestClonePreservesNilAndEmptyCollections(t *testing.T) {
	for _, v := range []View{{}, blank(), {Modules: []Module{{Engineering: &Engineering{Modifiers: []Modifier{}}}}}} {
		if got := Clone(v); !reflect.DeepEqual(got, v) {
			t.Fatalf("clone changed collection presence: %#v", got)
		}
	}
}
