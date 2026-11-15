package providers

import (
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func allowedEndpoint(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Fragment != "" {
		return false
	}
	switch u.Host {
	case "elitebgs.app", "api.canonn.tech", ravenHost:
		return communityPath(u)
	case "spansh.co.uk":
		return u.RawQuery == "" && spanshPath(u.Path)
	case "www.edsm.net":
		return u.Path == "/api-journal-v1" || u.Path == "/api-logs-v1/get-logs"
	case "inara.cz":
		return u.Path == "/inapi/v1/"
	case "edastro.com":
		return u.Path == "/api/journal" || u.Path == "/api/starsystem" || u.Path == "/gec/json/rare"
	case "eddn.edcd.io:4430":
		return u.Path == "/upload/"
	case "auth.frontierstore.net":
		return u.Path == "/token"
	case "companion.orerve.net":
		return u.Path == "/profile" || u.Path == "/market" || u.Path == "/shipyard" || u.Path == "/fleetcarrier" || u.Path == "/communitygoals"
	}
	return false
}
func category(id, kind string) string {
	switch kind {
	case "FSDJump", "Location", "Docked", "CarrierJump":
		return "travel"
	case "Scan", "SAASignalsFound", "FSSDiscoveryScan":
		return "exploration"
	case "Loadout":
		return "loadout"
	case "Materials":
		return "materials"
	case "Rank", "Progress":
		return "ranks"
	case "Statistics":
		return "credits"
	case "ScanOrganic":
		return "biology"
	case "CarrierStats":
		return "carrier"
	case "CarrierJumpRequest":
		return "carrier_destination"
	}
	return ""
}
func validPosition(v any) bool {
	a, ok := v.([]any)
	if !ok || len(a) != 3 {
		return false
	}
	for _, x := range a {
		n, ok := x.(json.Number)
		if !ok {
			return false
		}
		f, err := n.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || math.Abs(f) > 1e6 {
			return false
		}
	}
	return true
}
func validID(v any) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	_, e := strconv.ParseUint(string(n), 10, 64)
	return e == nil
}
func pick(v map[string]any, fields string) map[string]any {
	o := map[string]any{}
	for _, k := range strings.Fields(fields) {
		if x, ok := v[k]; ok {
			o[k] = x
		}
	}
	return o
}
func cleanedJournal(v map[string]any) map[string]any {
	// Build from a public-field allowlist, never remove just a few private keys.
	o := pick(v, "timestamp event StarSystem SystemName SystemAddress StarPos BodyID BodyName BodyType StarType StellarMass Radius SurfaceTemperature AbsoluteMagnitude Age_MY Luminosity Subclass PlanetClass EarthMasses Gravity SurfaceGravity Atmosphere AtmosphereType SurfacePressure Volcanism Landable TerraformState OrbitalPeriod RotationPeriod SemiMajorAxis Eccentricity OrbitalInclination Periapsis AxialTilt TidalLock DistanceFromArrivalLS ScanType WasDiscovered WasMapped BodyCount NonBodyCount Progress StationName StationType StationGovernment StationAllegiance StationEconomy StationEconomy_Localised")
	for _, k := range []string{"StationEconomy_Localised"} {
		delete(o, k)
	}
	for _, k := range []string{"Rings", "Materials", "Signals", "Genuses"} {
		if a, ok := v[k].([]any); ok && len(a) <= 256 {
			out := []any{}
			for _, x := range a {
				if obj, ok := x.(map[string]any); ok {
					out = append(out, pick(obj, "Name NameID Count Percent RingClass MassMT InnerRad OuterRad Genus"))
				}
			}
			o[k] = out
		}
	}
	if a, ok := v["Parents"].([]any); ok && len(a) <= 16 {
		parents := []any{}
		for _, x := range a {
			if obj, ok := x.(map[string]any); ok {
				parents = append(parents, pick(obj, "Star Planet Null"))
			}
		}
		o["Parents"] = parents
	}
	return o
}
func (m *Manager) encode(id string, e journal.Event, v map[string]any, cred Credential) ([]byte, string, string, error) {
	fail := func() ([]byte, string, string, error) {
		return nil, "", "", errors.New("unsupported or incomplete event")
	}
	endpoint := m.cap(id).Endpoint
	ct := "application/json"
	var payload any
	switch id {
	case "eddn":
		if e.Type != "FSDJump" && e.Type != "Location" && e.Type != "CarrierJump" && e.Type != "Scan" && e.Type != "SAASignalsFound" {
			return fail()
		}
		msg := cleanedJournal(v)
		if text(msg["StarSystem"]) == "" {
			msg["StarSystem"] = m.system
		}
		if msg["StarPos"] == nil {
			msg["StarPos"] = m.position
		}
		if msg["SystemAddress"] == nil {
			msg["SystemAddress"] = m.address
		}
		if text(msg["StarSystem"]) == "" || !validPosition(msg["StarPos"]) || !validID(msg["SystemAddress"]) || (m.address != nil && msg["SystemAddress"] != m.address) {
			return fail()
		}
		payload = map[string]any{"$schemaRef": "https://eddn.edcd.io/schemas/journal/1", "header": map[string]any{"uploaderID": m.uploader, "softwareName": software, "softwareVersion": version, "gameversion": m.gameVersion, "gamebuild": m.gameBuild}, "message": msg}
	case "edastro":
		fields := ""
		switch e.Type {
		case "ScanOrganic":
			fields = "timestamp event ScanType Genus Species Variant Body SystemAddress Genus_Localised Species_Localised Variant_Localised"
		case "CarrierStats":
			fields = "timestamp event Callsign Name CarrierID AllowNotorious PendingDecommission DockingAccess FuelLevel"
		case "CarrierJumpRequest":
			fields = "timestamp event SystemName SystemAddress CarrierID Body BodyID DepartureTime"
		default:
			return fail()
		}
		msg := pick(v, fields)
		if e.Type == "ScanOrganic" && (!validID(msg["SystemAddress"]) || text(msg["Species"]) == "") {
			return fail()
		}
		if e.Type != "ScanOrganic" && !validID(msg["CarrierID"]) {
			return fail()
		}
		payload = []any{map[string]any{"appName": software, "odyssey": m.odyssey, "system": m.system}, msg}
	case "edsm":
		msg := cleanedJournal(v)
		switch e.Type {
		case "Loadout":
			msg = pick(v, "timestamp event Ship ShipID HullHealth UnladenMass MaxJumpRange CargoCapacity FuelCapacity")
			msg["Modules"] = publicModules(v)
		case "Materials":
			msg = materialJournal(v)
		case "Rank", "Progress":
			msg = pick(v, "timestamp event Combat Trade Explore Empire Federation CQC Soldier Exobiologist")
		case "FSDJump", "Location", "CarrierJump", "Docked", "Scan", "SAASignalsFound", "FSSDiscoveryScan":
		default:
			return fail()
		}
		raw, _ := json.Marshal([]any{msg})
		form := url.Values{"commanderName": {cred.Commander}, "apiKey": {cred.APIKey}, "fromSoftware": {software}, "fromSoftwareVersion": {version}, "fromGameVersion": {m.gameVersion}, "fromGameBuild": {m.gameBuild}, "message": {string(raw)}}
		encoded := []byte(form.Encode())
		if len(encoded) > 256<<10 {
			return fail()
		}
		return encoded, endpoint, "application/x-www-form-urlencoded", nil
	case "inara":
		eventName := ""
		var data any
		switch e.Type {
		case "FSDJump":
			eventName = "addCommanderTravelFSDJump"
			if text(v["StarSystem"]) == "" || !validPosition(v["StarPos"]) {
				return fail()
			}
			d := map[string]any{"starsystemName": v["StarSystem"], "starsystemCoords": v["StarPos"]}
			if n, ok := v["JumpDist"].(json.Number); ok {
				d["jumpDistance"] = n
			}
			data = d
		case "Materials":
			eventName = "setCommanderInventoryMaterials"
			a := []any{}
			for _, k := range []string{"Raw", "Manufactured", "Encoded"} {
				items, ok := v[k].([]any)
				if !ok || len(items) > 512 {
					return fail()
				}
				for _, x := range items {
					if o, ok := x.(map[string]any); ok && text(o["Name"]) != "" && validID(o["Count"]) {
						a = append(a, map[string]any{"itemName": o["Name"], "itemCount": o["Count"]})
					} else {
						return fail()
					}
				}
			}
			data = a
		case "Rank", "Progress":
			eventName = "setCommanderRankPilot"
			a := []any{}
			for key, name := range map[string]string{"Combat": "combat", "Trade": "trade", "Explore": "explore", "Empire": "empire", "Federation": "federation", "CQC": "cqc", "Soldier": "soldier", "Exobiologist": "exobiologist"} {
				if n, ok := v[key].(json.Number); ok {
					f, err := n.Float64()
					if err != nil || f < 0 {
						return fail()
					}
					d := map[string]any{"rankName": name}
					if e.Type == "Progress" {
						if f > 100 {
							return fail()
						}
						d["rankProgress"] = f / 100
					} else {
						if f > 14 || f != math.Trunc(f) {
							return fail()
						}
						d["rankValue"] = n
					}
					a = append(a, d)
				}
			}
			if len(a) == 0 {
				return fail()
			}
			data = a
		case "Loadout":
			eventName = "setCommanderShipLoadout"
			if !validID(v["ShipID"]) || text(v["Ship"]) == "" {
				return fail()
			}
			items, ok := v["Modules"].([]any)
			if !ok || len(items) > 128 {
				return fail()
			}
			modules := []any{}
			for _, x := range items {
				o, ok := x.(map[string]any)
				if !ok || text(o["Slot"]) == "" || text(o["Item"]) == "" {
					return fail()
				}
				d := map[string]any{"slotName": o["Slot"], "itemName": o["Item"]}
				if v, ok := o["On"].(bool); ok {
					d["isOn"] = v
				}
				if n, ok := o["Priority"].(json.Number); ok {
					d["itemPriority"] = n
				}
				for source, target := range map[string]string{"Health": "itemHealth", "Value": "itemValue", "AmmoInClip": "itemAmmoClip", "AmmoInHopper": "itemAmmoHopper"} {
					if v, ok := o[source].(json.Number); ok {
						d[target] = v
					}
				}
				if eng, ok := o["Engineering"].(map[string]any); ok {
					mapped := map[string]any{}
					for source, target := range map[string]string{"BlueprintName": "blueprintName", "Level": "blueprintLevel", "Quality": "blueprintQuality", "ExperimentalEffect": "experimentalEffect"} {
						if value, ok := eng[source]; ok {
							mapped[target] = value
						}
					}
					mods, ok := eng["Modifiers"].([]any)
					if !ok || len(mods) > 64 {
						return fail()
					}
					out := []any{}
					for _, x := range mods {
						mod, ok := x.(map[string]any)
						if !ok || text(mod["Label"]) == "" {
							return fail()
						}
						r := map[string]any{"name": mod["Label"]}
						for source, target := range map[string]string{"Value": "value", "OriginalValue": "originalValue", "LessIsGood": "lessIsGood"} {
							if value, ok := mod[source]; ok {
								r[target] = value
							}
						}
						out = append(out, r)
					}
					mapped["modifiers"] = out
					d["engineering"] = mapped
				}
				modules = append(modules, d)
			}
			data = map[string]any{"shipType": v["Ship"], "shipGameID": v["ShipID"], "shipLoadout": modules}
		case "Statistics":
			eventName = "setCommanderCredits"
			bank, ok := v["Bank_Account"].(map[string]any)
			if !ok || !validID(bank["Current_Wealth"]) {
				return fail()
			}
			data = map[string]any{"commanderCredits": bank["Current_Wealth"]}
		default:
			return fail()
		}
		app := m.opts.InaraAppName
		if app == "" {
			app = InaraApplicationName
		}
		payload = map[string]any{"header": map[string]any{"appName": app, "appVersion": version, "isBeingDeveloped": true, "APIkey": cred.APIKey, "commanderName": m.name}, "events": []any{map[string]any{"eventName": eventName, "eventTimestamp": v["timestamp"], "eventData": data}}}
	default:
		return fail()
	}
	if id == "eddn" {
		if err := validateEDDN(payload); err != nil {
			return fail()
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) > 256<<10 {
		return fail()
	}
	return raw, endpoint, ct, nil
}
func publicModules(v map[string]any) []any {
	out := []any{}
	if a, ok := v["Modules"].([]any); ok && len(a) <= 128 {
		for _, x := range a {
			if o, ok := x.(map[string]any); ok {
				mod := pick(o, "Slot Item On Priority Health Value AmmoInClip AmmoInHopper")
				if eng, ok := o["Engineering"].(map[string]any); ok {
					r := pick(eng, "BlueprintName Level Quality ExperimentalEffect")
					if a, ok := eng["Modifiers"].([]any); ok && len(a) <= 64 {
						mods := []any{}
						for _, x := range a {
							if v, ok := x.(map[string]any); ok {
								mods = append(mods, pick(v, "Label Value OriginalValue LessIsGood"))
							}
						}
						r["Modifiers"] = mods
					}
					mod["Engineering"] = r
				}
				out = append(out, mod)
			}
		}
	}
	return out
}
func materialJournal(v map[string]any) map[string]any {
	out := pick(v, "timestamp event")
	for _, key := range []string{"Raw", "Manufactured", "Encoded"} {
		if a, ok := v[key].([]any); ok && len(a) <= 512 {
			b := []any{}
			for _, x := range a {
				if o, ok := x.(map[string]any); ok {
					b = append(b, pick(o, "Name Count"))
				}
			}
			out[key] = b
		}
	}
	return out
}
