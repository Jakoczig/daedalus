package paritystate

import (
	"github.com/Jakoczig/daedalus/go/internal/catalog"
	"strings"
)

func parseModules(d map[string]any) ([]Module, string) {
	out := []Module{}
	rows, ok := d["Modules"].([]any)
	if !ok {
		return out, "invalid"
	}
	state := "journal_last_known"
	if len(rows) > 128 {
		rows = rows[:128]
		state = "truncated"
	}
	for _, r := range rows {
		v, ok := r.(map[string]any)
		if !ok {
			state = "partial"
			continue
		}
		slot, item := text(v["Slot"]), text(v["Item"])
		if slot == "" || item == "" {
			state = "partial"
			continue
		}
		mod := Module{Slot: slot, Item: item, Name: text(v["Item_Localised"]), Health: number(v["Health"], 0, 1), On: boolean(v["On"]), Priority: integer(v["Priority"], 10), AmmoInClip: integer(v["AmmoInClip"], 1000000), AmmoInHopper: integer(v["AmmoInHopper"], 1000000)}
		if ref, ok := catalog.ModuleBySymbol(item); ok {
			if mod.Name == "" {
				mod.Name = ref.Name
			}
			mod.Reference = &ModuleReference{Source: ref.Source, Class: ref.Class, Rating: ref.Rating, MassT: ref.MassT, PowerMW: ref.PowerMW, Description: ref.Description}
		}
		if mod.Name == "" {
			mod.Name = item
		}
		if e, ok := v["Engineering"].(map[string]any); ok {
			en := &Engineering{Blueprint: text(e["BlueprintName"]), Grade: integer(e["Level"], 5), Quality: number(e["Quality"], 0, 1), Experimental: text(e["ExperimentalEffect_Localised"]), Modifiers: []Modifier{}}
			if en.Experimental == "" {
				en.Experimental = text(e["ExperimentalEffect"])
			}
			if rows, ok := e["Modifiers"].([]any); ok {
				if len(rows) > 32 {
					rows = rows[:32]
					en.Truncated = true
				}
				for _, r := range rows {
					a, ok := r.(map[string]any)
					if !ok {
						continue
					}
					label := text(a["Label"])
					val := number(a["Value"], -1e15, 1e15)
					if label == "" || val == nil {
						continue
					}
					en.Modifiers = append(en.Modifiers, Modifier{Label: label, Value: val, OriginalValue: number(a["OriginalValue"], -1e15, 1e15), LessIsGood: boolean(a["LessIsGood"])})
				}
			}
			mod.Engineering = en
		}
		out = append(out, mod)
	}
	return out, state
}
func parseCargo(d map[string]any) CargoView {
	out := CargoView{State: "invalid", Items: []CargoItem{}}
	if text(d["Vessel"]) != "Ship" {
		return out
	}
	out.Count = integer(d["Count"], 1000000)
	rows, ok := d["Inventory"].([]any)
	if !ok || out.Count == nil {
		return out
	}
	out.State = "known"
	if len(rows) > 512 {
		rows = rows[:512]
		out.Truncated = true
		out.State = "truncated"
	}
	total := 0
	for _, r := range rows {
		v, ok := r.(map[string]any)
		if !ok {
			out.State = "partial"
			continue
		}
		name, count := text(v["Name"]), integer(v["Count"], 1000000)
		if name == "" || count == nil {
			out.State = "partial"
			continue
		}
		label := text(v["Name_Localised"])
		if label == "" {
			label = catalog.Label(name)
		}
		stolen := integer(v["Stolen"], 1000000)
		if stolen != nil && *stolen > *count {
			out.State = "partial"
			stolen = nil
		}
		mission := ident(v["MissionID"]) != ""
		out.Items = append(out.Items, CargoItem{Name: name, Label: label, Count: *count, Stolen: stolen, Mission: mission})
		total += *count
	}
	if !out.Truncated && total != *out.Count {
		out.State = "partial"
	}
	return out
}
func parseMaterials(d map[string]any) MaterialView {
	out := MaterialView{State: "known", Items: []MaterialItem{}}
	seen := map[string]bool{}
	for _, cat := range []string{"Raw", "Encoded", "Manufactured"} {
		rows, ok := d[cat].([]any)
		if !ok {
			out.State = "partial"
			continue
		}
		for _, r := range rows {
			if len(out.Items) >= 512 {
				out.Truncated = true
				out.State = "truncated"
				break
			}
			v, ok := r.(map[string]any)
			if !ok {
				out.State = "partial"
				continue
			}
			name, count := text(v["Name"]), integer(v["Count"], 1000000)
			if name == "" || count == nil {
				out.State = "partial"
				continue
			}
			key := strings.ToLower(cat + ":" + name)
			if seen[key] {
				out.State = "partial"
				continue
			}
			seen[key] = true
			label := text(v["Name_Localised"])
			if label == "" {
				label = catalog.Label(name)
			}
			out.Items = append(out.Items, MaterialItem{Name: name, Label: label, Category: strings.ToLower(cat), Count: *count})
		}
	}
	return out
}
func applyMaterialDelta(v *MaterialView, d map[string]any, subtract bool) {
	if v.State != "known" {
		return
	}
	name, count, cat := text(d["Name"]), integer(d["Count"], 1000000), strings.ToLower(text(d["Category"]))
	if name == "" || count == nil || (cat != "raw" && cat != "encoded" && cat != "manufactured") {
		*v = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
		return
	}
	for i, item := range v.Items {
		if strings.EqualFold(item.Name, name) && item.Category == cat {
			n := item.Count + *count
			if subtract {
				n = item.Count - *count
			}
			if n < 0 || n > 1000000 {
				*v = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
				return
			}
			if n == 0 {
				v.Items = append(v.Items[:i], v.Items[i+1:]...)
			} else {
				v.Items[i].Count = n
			}
			return
		}
	}
	if subtract || len(v.Items) >= 512 {
		*v = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
		return
	}
	v.Items = append(v.Items, MaterialItem{Name: name, Label: catalog.Label(name), Category: cat, Count: *count})
}

// Resource lists carry explicit counts but omit categories. Match a unique
// observed entry; do not infer stock or a material's category from its spelling.
func materialCategory(v MaterialView, name string) string {
	cat := ""
	for _, item := range v.Items {
		if strings.EqualFold(item.Name, name) {
			if cat != "" {
				return ""
			}
			cat = item.Category
		}
	}
	return cat
}
func applyConsumption(v *MaterialView, raw any) {
	if v.State != "known" {
		return
	}
	rows, ok := raw.([]any)
	if !ok || len(rows) > 512 {
		*v = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
		return
	}
	next := *v
	next.Items = append([]MaterialItem{}, v.Items...)
	for _, r := range rows {
		d, ok := r.(map[string]any)
		if !ok {
			next.State = "pending_snapshot"
			break
		}
		name := text(d["Name"])
		cat := materialCategory(next, name)
		if cat == "" {
			next.State = "pending_snapshot"
			break
		}
		applyMaterialDelta(&next, map[string]any{"Name": d["Name"], "Count": d["Count"], "Category": cat}, true)
		if next.State != "known" {
			break
		}
	}
	if next.State != "known" {
		*v = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
	} else {
		*v = next
	}
}
func applyTrade(v *MaterialView, d map[string]any) {
	if v.State != "known" {
		return
	}
	next := *v
	next.Items = append([]MaterialItem{}, v.Items...)
	for i, part := range []string{"Paid", "Received"} {
		a, ok := d[part].(map[string]any)
		if !ok {
			next.State = "pending_snapshot"
			break
		}
		name := text(a["Material"])
		cat := materialCategory(next, name)
		if cat == "" {
			cat = strings.ToLower(text(a["Category"]))
		}
		applyMaterialDelta(&next, map[string]any{"Name": name, "Count": a["Quantity"], "Category": cat}, i == 0)
		if next.State != "known" {
			break
		}
	}
	if next.State != "known" {
		*v = MaterialView{State: "pending_snapshot", Items: []MaterialItem{}}
	} else {
		*v = next
	}
}
func applyInventory(s *state, d map[string]any, location string) {
	items := []InventoryItem{}
	state := "known"
	truncated := false
	for _, cat := range []string{"Items", "Components", "Consumables", "Data"} {
		rows, ok := d[cat].([]any)
		if !ok {
			state = "partial"
			continue
		}
		for _, r := range rows {
			if len(items) >= 512 {
				state = "truncated"
				truncated = true
				break
			}
			v, ok := r.(map[string]any)
			if !ok {
				state = "partial"
				continue
			}
			name, count := text(v["Name"]), integer(v["Count"], 1000000)
			if name == "" || count == nil {
				state = "partial"
				continue
			}
			label := text(v["Name_Localised"])
			if label == "" {
				label = catalog.Label(name)
			}
			items = append(items, InventoryItem{Name: name, Label: label, Category: strings.ToLower(cat), Location: strings.ToLower(location), Count: *count})
		}
	}
	for _, old := range s.v.Inventory.Items {
		if old.Location != strings.ToLower(location) {
			if len(items) >= 512 {
				state = "truncated"
				truncated = true
				break
			}
			items = append(items, old)
		}
	}
	if location == "ShipLocker" {
		s.locker = state == "known"
	} else {
		s.backpack = state == "known"
	}
	if state == "known" && (!s.locker || !s.backpack) {
		state = "partial"
	}
	lockerState, backpackState := "unknown", "unknown"
	if s.locker {
		lockerState = "known"
	}
	if s.backpack {
		backpackState = "known"
	}
	if truncated {
		lockerState = "unknown"
		backpackState = "unknown"
	}
	s.v.Inventory = InventoryView{State: state, ShipLockerState: lockerState, BackpackState: backpackState, Items: items, Truncated: truncated}
}
func parseStatus(d map[string]any) StatusView {
	out := StatusView{State: "invalid"}
	n := ident(d["Flags"])
	if n == "" {
		return out
	}
	flags := uint64(0)
	for _, r := range n {
		flags = flags*10 + uint64(r-'0')
	}
	if flags > 0xffffffff {
		return out
	}
	bit := func(i uint) bool { return flags&(1<<i) != 0 }
	out.Flags = &StatusFlags{Docked: bit(0), Landed: bit(1), LandingGearDown: bit(2), ShieldsUp: bit(3), Supercruise: bit(4), FlightAssistOff: bit(5), HardpointsDeployed: bit(6), LightsOn: bit(8), CargoScoopDeployed: bit(9), SilentRunning: bit(10), ScoopingFuel: bit(11), FSDMassLocked: bit(16), FSDCharging: bit(17), FSDCooldown: bit(18), LowFuel: bit(19), Overheating: bit(20), BeingInterdicted: bit(23), InMainShip: bit(24), InFighter: bit(25), InSRV: bit(26), AnalysisMode: bit(27), NightVision: bit(28), InDanger: bit(22), FSDJump: bit(30)}
	if raw, exists := d["Flags2"]; exists {
		f2 := integer(raw, 0x7fffffff)
		if f2 == nil {
			return StatusView{State: "invalid"}
		}
		b := func(i uint) bool { return uint64(*f2)&(1<<i) != 0 }
		out.Flags2 = &StatusFlags2{OnFoot: b(0), InTaxi: b(1), InMulticrew: b(2), LowOxygen: b(6), LowHealth: b(7), Cold: b(8), Hot: b(9), VeryCold: b(10), VeryHot: b(11), GlideMode: b(12), BreathableAtmosphere: b(16), TelepresenceMulticrew: b(17), PhysicalMulticrew: b(18), FSDHyperdriveCharging: b(19)}
		out.Flags.GlideMode = b(12)
		if b(0) || b(1) || b(2) || b(17) || b(18) {
			out.Flags.InMainShip = false
		}
	}
	out.State = "known"
	out.GuiFocus = integer(d["GuiFocus"], 32)
	// Status fuel/cargo belong to the controlled vehicle. Only a main-ship
	// attestation permits projecting these as ship facts.
	if !out.Flags.InMainShip || out.Flags.InSRV || out.Flags.InFighter {
		return out
	}
	if rows, ok := d["Pips"].([]any); ok && len(rows) == 3 {
		a, b, c := integer(rows[0], 8), integer(rows[1], 8), integer(rows[2], 8)
		if a != nil && b != nil && c != nil && *a+*b+*c == 12 {
			out.Pips = &Pips{float64(*a) / 2, float64(*b) / 2, float64(*c) / 2}
		}
	}
	if fuel, ok := d["Fuel"].(map[string]any); ok {
		out.FuelMainT = number(fuel["FuelMain"], 0, 1000000)
		out.FuelReservoirT = number(fuel["FuelReservoir"], 0, 1000000)
	}
	out.CargoT = number(d["Cargo"], 0, 1000000)
	out.GuiFocus = integer(d["GuiFocus"], 32)
	out.Temperature = number(d["Temperature"], 0, 1e9)
	return out
}
