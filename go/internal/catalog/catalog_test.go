package catalog

import "testing"

func TestBundledReference(t *testing.T) {
	m, ok := ModuleBySymbol("int_powerplant_size2_class1")
	if !ok || m.Name != "Power Plant" || m.Class == nil || *m.Class != 2 || m.Source != "bundled_edcd_coriolis_reference" {
		t.Fatalf("missing reference: %+v", m)
	}
	if Label("iron") != "Iron" {
		t.Fatal("material name missing")
	}
	*m.Class = 99
	if n, _ := ModuleBySymbol("int_powerplant_size2_class1"); *n.Class != 2 {
		t.Fatal("catalog aliases mutable reference")
	}
}

func TestFDevDisplayNameTakesPriorityOverReferenceAbbreviation(t *testing.T) {
	m, ok := ModuleBySymbol("int_hyperdrive_size5_class5")
	if !ok || m.Name != "Frame Shift Drive" {
		t.Fatalf("display name lost: %q", m.Name)
	}
}
