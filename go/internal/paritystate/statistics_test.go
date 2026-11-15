package paritystate

import "testing"

func TestEffectiveEngineeringValuesAreFinalAndUnknownsStayUnknown(t *testing.T) {
	on := true
	priority := 1
	plantCapacity, draw := 30.0, 2.5
	original := 1.0
	modules := []Module{{Slot: "PowerPlant", On: &on, Engineering: &Engineering{Modifiers: []Modifier{{Label: "PowerCapacity", Value: &plantCapacity}}}}, {Slot: "FrameShiftDrive", On: &on, Priority: &priority, Engineering: &Engineering{Modifiers: []Modifier{{Label: "PowerDraw", Value: &draw, OriginalValue: &original}}}}}
	v := Statistics(modules)
	if v.RatedPowerMW == nil || *v.RatedPowerMW != 2.5 || v.CapacityMW == nil || *v.CapacityMW != 30 {
		t.Fatalf("effective values multiplied or omitted: %+v", v)
	}
	if v.KnownPriorityMW[1] != 2.5 {
		t.Fatal("power priority changed")
	}
	modules = append(modules, Module{Slot: "Thrusters", On: &on})
	v = Statistics(modules)
	if v.RatedPowerMW != nil || v.UnknownPower != 1 {
		t.Fatal("missing modifier treated as zero")
	}
	modules[1].Engineering.Truncated = true
	v = Statistics(modules)
	if len(v.Modules[1].Attributes) != 0 {
		t.Fatal("truncated engineering presented as complete")
	}
}
