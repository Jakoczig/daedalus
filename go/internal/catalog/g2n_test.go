package catalog

import "testing"

func TestG2NOutfittingIdentityWithoutInventedPerformance(t *testing.T) {
	if ShipDisplayName("") != "" {
		t.Fatal("missing ship mislabeled as a module")
	}
	armour, ok := ModuleBySymbol("SideWinder_Armour_Grade1")
	if !ok || armour.Class == nil || *armour.Class != 1 || armour.MassT != nil || armour.PowerMW != nil {
		t.Fatal("CSV identity missing or performance invented", armour)
	}
	m, ok := ModuleBySymbol("Int_FighterBay_Size7_Class1")
	if !ok || m.Class == nil || *m.Class != 7 || m.Rating != "D" {
		t.Fatalf("verified CSV identity missing: %+v", m)
	}
	if _, ok := ModuleBySymbol("Int_FighterBay_UNKNOWN_VARIANT"); ok {
		t.Fatal("invented variant")
	}
}
