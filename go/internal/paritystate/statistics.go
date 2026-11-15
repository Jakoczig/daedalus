package paritystate

import (
	"math"
	"strings"
)

const StatisticsFormula = "effective-fitted-power-sum-v2"

type EffectiveAttribute struct {
	Label    string   `json:"label"`
	Value    float64  `json:"value"`
	Original *float64 `json:"original"`
	Source   string   `json:"source"`
}
type EffectiveModule struct {
	Slot       string               `json:"slot"`
	Item       string               `json:"item"`
	Attributes []EffectiveAttribute `json:"attributes"`
	Notice     string               `json:"notice"`
}
type EngineeredStatistics struct {
	ObservedHardpointsDeployed *bool             `json:"observedHardpointsDeployed"`
	ObservedDeploymentRatedMW  *float64          `json:"observedDeploymentRatedMW"`
	KnownRatedMW               float64           `json:"knownRatedMW"`
	StowedMW                   *float64          `json:"stowedMW"`
	DeployedMW                 *float64          `json:"deployedMW"`
	DeployedPercent            *float64          `json:"deployedPercent"`
	Formula                    string            `json:"formula"`
	CapacityMW                 *float64          `json:"capacityMW"`
	RatedPowerMW               *float64          `json:"ratedPowerMW"`
	KnownPriorityMW            [5]float64        `json:"knownPriorityMW"`
	UnknownPower               int               `json:"unknownPower"`
	Modules                    []EffectiveModule `json:"modules"`
	Unavailable                []string          `json:"unavailable"`
}

func Statistics(modules []Module) EngineeredStatistics {
	out := EngineeredStatistics{Formula: StatisticsFormula, Modules: []EffectiveModule{}, Unavailable: []string{"Laden jump range: missing permission-cleared reference curves and gold fixtures", "Shield/hull effective strengths and resistance stacking: validated complete model unavailable", "Thruster speed, boost, acceleration and turn curves: validated model unavailable", "Weapon DPS/energy/ammunition endurance: complete effective attributes and validated firing model unavailable"}}
	total := 0.0
	stowed := 0.0
	if len(modules) == 0 {
		out.UnknownPower = 1
	}
	for _, m := range modules {
		row := EffectiveModule{Slot: m.Slot, Item: m.Item, Attributes: []EffectiveAttribute{}}
		var power *float64
		if m.Engineering != nil && !m.Engineering.Truncated {
			for _, mod := range m.Engineering.Modifiers {
				if mod.Value == nil || math.IsNaN(*mod.Value) || math.IsInf(*mod.Value, 0) {
					continue
				}
				row.Attributes = append(row.Attributes, EffectiveAttribute{Label: mod.Label, Value: *mod.Value, Original: mod.OriginalValue, Source: "Game-reported effective final Loadout modifier; no blueprint multiplier applied"})
				if mod.Label == "PowerCapacity" && m.Slot == "PowerPlant" && *mod.Value >= 0 {
					v := *mod.Value
					out.CapacityMW = &v
				}
				if mod.Label == "PowerDraw" && *mod.Value >= 0 {
					v := *mod.Value
					power = &v
				}
			}
		} else {
			row.Notice = "No complete witnessed engineering modifier set; values unavailable"
			if m.Engineering == nil && m.Reference != nil && m.Reference.PowerMW != nil && !math.IsNaN(*m.Reference.PowerMW) && !math.IsInf(*m.Reference.PowerMW, 0) && *m.Reference.PowerMW >= 0 {
				v := *m.Reference.PowerMW
				power = &v
				row.Notice = "Unengineered module rated draw: bundled reference, not game-reported"
			}
		}
		out.Modules = append(out.Modules, row)
		if m.Slot == "PowerPlant" || (m.On != nil && !*m.On) {
			continue
		}
		if m.On == nil || power == nil {
			out.UnknownPower++
			continue
		}
		total += *power
		weapon := false
		for _, prefix := range []string{"HugeHardpoint", "LargeHardpoint", "MediumHardpoint", "SmallHardpoint"} {
			if strings.HasPrefix(m.Slot, prefix) {
				weapon = true
			}
		}
		if !weapon {
			stowed += *power
		}
		if m.Priority != nil && *m.Priority >= 0 && *m.Priority < 5 {
			out.KnownPriorityMW[*m.Priority] += *power
		}
	}
	out.KnownRatedMW = total
	if len(modules) > 0 && out.UnknownPower == 0 {
		out.RatedPowerMW = &total
		out.StowedMW = &stowed
		out.DeployedMW = &total
		if out.CapacityMW != nil && *out.CapacityMW > 0 {
			p := total / *out.CapacityMW * 100
			out.DeployedPercent = &p
		}
	}
	return out
}
