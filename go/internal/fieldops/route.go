// Package fieldops implements bounded offline field-operation transformations.
package fieldops

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxCSV = 512 << 10

type Waypoint struct {
	Name        string      `json:"name"`
	Address     string      `json:"address,omitempty"`
	Neutron     bool        `json:"neutron"`
	Refuel      bool        `json:"refuel"`
	Coordinates *[3]float64 `json:"coordinates,omitempty"`
}

func SafeName(s string) bool {
	if s == "" || len(s) > 256 || !utf8.ValidString(s) || strings.Contains(s, "://") || strings.ContainsAny(s, "<>`") || strings.ContainsAny(s[:1], "=+@") {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func ParseCSV(raw []byte) ([]Waypoint, error) {
	fail := errors.New("Invalid CSV: choose UTF-8 System Name/system, unique systems, finite x/y/z and at most 2048 waypoints / 512 KiB")
	if len(raw) == 0 || len(raw) > MaxCSV || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return nil, fail
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\ufeff")))
	r.FieldsPerRecord = -1
	hdr, e := r.Read()
	if e != nil || len(hdr) > 32 {
		return nil, fail
	}
	cols := map[string]int{}
	for i, k := range hdr {
		k = strings.ToLower(strings.TrimSpace(k))
		switch k {
		case "system":
			k = "system name"
		case "neutron star":
			k = "neutron"
		case "restock":
			k = "refuel"
		}
		if _, ok := cols[k]; ok {
			return nil, fail
		}
		cols[k] = i
	}
	name, ok := cols["system name"]
	if !ok {
		name, ok = cols["system"]
	}
	if !ok {
		return nil, fail
	}
	out := []Waypoint{}
	seen := map[string]bool{}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil || len(row) != len(hdr) || len(out) >= 2048 {
			return nil, fail
		}
		for _, v := range row {
			if len(v) > 256 {
				return nil, fail
			}
		}
		w := Waypoint{Name: strings.TrimSpace(row[name])}
		if !SafeName(w.Name) || seen[strings.ToLower(w.Name)] {
			return nil, fail
		}
		seen[strings.ToLower(w.Name)] = true
		if i, ok := cols["systemaddress"]; ok && row[i] != "" {
			n, e := strconv.ParseUint(row[i], 10, 64)
			if e != nil || strconv.FormatUint(n, 10) != row[i] {
				return nil, fail
			}
			w.Address = row[i]
		}
		for k, p := range map[string]*bool{"neutron": &w.Neutron, "neutron star": &w.Neutron, "refuel": &w.Refuel} {
			if i, ok := cols[k]; ok && row[i] != "" {
				value := strings.ToLower(strings.TrimSpace(row[i]))
				if value == "yes" {
					value = "true"
				}
				if value == "no" {
					value = "false"
				}
				v, e := strconv.ParseBool(value)
				if e != nil {
					return nil, fail
				}
				*p = v
			}
		}
		var pos [3]float64
		count := 0
		headers := 0
		empty := 0
		for j, k := range []string{"x", "y", "z"} {
			if i, ok := cols[k]; ok {
				headers++
				if row[i] == "" {
					empty++
					continue
				}
				v, e := strconv.ParseFloat(row[i], 64)
				if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e6 {
					return nil, fail
				}
				pos[j] = v
				count++
			}
		}
		if (headers != 0 && headers != 3) || (empty != 0 && empty != 3) || (count != 0 && count != 3) {
			return nil, fail
		}
		if count == 3 {
			w.Coordinates = &pos
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		return nil, fail
	}
	return out, nil
}

// next is a cursor, never inferred from a target or a later matching waypoint.
func Advance(plan []Waypoint, next int, kind, name, address string) (int, string) {
	if kind != "FSDJump" && !(kind == "Location" && next == 0) {
		return next, "unchanged"
	}
	if next < 0 || next >= len(plan) {
		return next, "unchanged"
	}
	p := plan[next]
	if p.Address != "" {
		if address == p.Address {
			return next + 1, "address_verified"
		}
	} else if strings.EqualFold(strings.TrimSpace(name), p.Name) {
		return next + 1, "name_match_weaker"
	}
	if next > 0 {
		prev := plan[next-1]
		if (prev.Address != "" && prev.Address == address) || (prev.Address == "" && strings.EqualFold(prev.Name, name)) {
			return next, "unchanged"
		}
	}
	return next, "off_route"
}
