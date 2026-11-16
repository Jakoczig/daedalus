package paritystate

import "testing"

func TestNativeStowedDeployedMatchesEffectiveInputs(t *testing.T) {
	on := true
	p := 2
	cap, draw, base, orig := 20.0, 6.0, 3.0, 5.0
	modules := []Module{{Slot: "PowerPlant", On: &on, Engineering: &Engineering{Modifiers: []Modifier{{Label: "PowerCapacity", Value: &cap}}}}, {Slot: "MainEngines", On: &on, Engineering: &Engineering{Modifiers: []Modifier{{Label: "PowerDraw", Value: &draw, OriginalValue: &orig}}}}, {Slot: "LargeHardpoint1", On: &on, Priority: &p, Reference: &ModuleReference{PowerMW: &base}}}
	v := Statistics(modules)
	if v.StowedMW == nil || *v.StowedMW != 6 || v.DeployedMW == nil || *v.DeployedMW != 9 || v.DeployedPercent == nil || *v.DeployedPercent != 45 || v.KnownPriorityMW[2] != 3 {
		t.Fatal(v)
	}
	modules[1].Engineering.Modifiers = nil
	v = Statistics(modules)
	if v.StowedMW != nil || v.DeployedMW != nil || v.UnknownPower != 1 {
		t.Fatal("engineering used base", v)
	}
	modules[1].Engineering.Truncated = true
	v = Statistics(modules)
	if len(v.Modules[1].Attributes) != 0 {
		t.Fatal("truncated accepted")
	}
}

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
