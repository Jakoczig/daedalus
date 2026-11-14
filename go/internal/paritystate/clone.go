package paritystate

import "github.com/Jakoczig/daedalus/go/internal/shipstate"

func copyPtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func copySlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	out := make([]T, len(s))
	copy(out, s)
	return out
}

// Clone returns a detached snapshot without serializing or reparsing its facts.
// Strings and scalar values are immutable; every pointer and slice owns storage.
func Clone(v View) View {
	v.Ship = shipstate.Clone(v.Ship)
	v.Modules = copySlice(v.Modules)
	for i := range v.Modules {
		m := &v.Modules[i]
		m.Health = copyPtr(m.Health)
		m.On = copyPtr(m.On)
		m.Priority = copyPtr(m.Priority)
		m.AmmoInClip = copyPtr(m.AmmoInClip)
		m.AmmoInHopper = copyPtr(m.AmmoInHopper)
		if m.Engineering != nil {
			e := copyPtr(m.Engineering)
			e.Grade = copyPtr(e.Grade)
			e.Quality = copyPtr(e.Quality)
			e.Modifiers = copySlice(e.Modifiers)
			for j := range e.Modifiers {
				mod := &e.Modifiers[j]
				mod.Value = copyPtr(mod.Value)
				mod.OriginalValue = copyPtr(mod.OriginalValue)
				mod.LessIsGood = copyPtr(mod.LessIsGood)
			}
			m.Engineering = e
		}
		if m.Reference != nil {
			r := copyPtr(m.Reference)
			r.Class = copyPtr(r.Class)
			r.MassT = copyPtr(r.MassT)
			r.PowerMW = copyPtr(r.PowerMW)
			m.Reference = r
		}
	}
	v.Cargo.Count = copyPtr(v.Cargo.Count)
	v.Cargo.Items = copySlice(v.Cargo.Items)
	for i := range v.Cargo.Items {
		v.Cargo.Items[i].Stolen = copyPtr(v.Cargo.Items[i].Stolen)
	}
	v.Inventory.Items = copySlice(v.Inventory.Items)
	v.Materials.Items = copySlice(v.Materials.Items)
	v.Status.Flags = copyPtr(v.Status.Flags)
	v.Status.Flags2 = copyPtr(v.Status.Flags2)
	v.Status.Pips = copyPtr(v.Status.Pips)
	v.Status.FuelMainT = copyPtr(v.Status.FuelMainT)
	v.Status.FuelReservoirT = copyPtr(v.Status.FuelReservoirT)
	v.Status.CargoT = copyPtr(v.Status.CargoT)
	v.Status.Temperature = copyPtr(v.Status.Temperature)
	v.Status.GuiFocus = copyPtr(v.Status.GuiFocus)
	return v
}
