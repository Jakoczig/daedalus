// Package catalog contains reference data selected from the repository's bundled
// EDCD FDevIDs CSVs and Coriolis modules. These are static reference facts, never
// measurements of a player's ship. No user files, runtime IO or network is used.
package catalog

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

//go:embed *.csv modules.json
var data embed.FS

type Module struct {
	Source, Name, Rating, Description string
	Class                             *int
	MassT, PowerMW                    *float64
}

var once sync.Once
var modules map[string]Module
var labels map[string]string

func initData() {
	modules = map[string]Module{}
	labels = map[string]string{}
	for _, filename := range []string{"shipyard.csv", "outfitting.csv", "material.csv", "microresources.csv", "commodity.csv"} {
		b, _ := data.ReadFile(filename)
		rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
		if err != nil || len(rows) == 0 {
			continue
		}
		cols := map[string]int{}
		for i, h := range rows[0] {
			cols[h] = i
		}
		nameCol, ok := cols["name"]
		if !ok {
			nameCol = cols["English name"]
		}
		symbolCol := cols["symbol"]
		for _, row := range rows[1:] {
			if len(row) <= nameCol || len(row) <= symbolCol {
				continue
			}
			key := moduleKey(row[symbolCol])
			labels[strings.ToLower(row[symbolCol])] = row[nameCol]
			if filename == "outfitting.csv" {
				modules[key] = Module{Source: "bundled_edcd_coriolis_reference", Name: row[nameCol], Rating: row[cols["rating"]]}
			}
		}
	}
	b, _ := data.ReadFile("modules.json")
	var rows []struct {
		Symbol, Name, Rating, Description string
		Class                             *int
		Mass, Power                       *float64
	}
	if json.Unmarshal(b, &rows) != nil {
		return
	}
	for _, r := range rows {
		key := moduleKey(r.Symbol)
		v := modules[key]
		v.Source = "bundled_edcd_coriolis_reference"
		if v.Name == "" && r.Name != "" {
			v.Name = r.Name
		}
		v.Class = r.Class
		v.Rating = r.Rating
		v.MassT = r.Mass
		v.PowerMW = r.Power
		v.Description = r.Description
		modules[key] = v
	}
}
func copyPtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func ModuleBySymbol(symbol string) (Module, bool) {
	once.Do(initData)
	m, ok := modules[moduleKey(symbol)]
	if !ok && moduleKey(symbol) == "intfighterbaymk2s" {
		m = Module{Name: "Fighter Hangar Mk II", Source: "symbol_display_name_only", Description: "Display name only; class, rating and performance not verified."}
		ok = true
	}
	m.Class = copyPtr(m.Class)
	m.MassT = copyPtr(m.MassT)
	m.PowerMW = copyPtr(m.PowerMW)
	return m, ok
}
func Label(symbol string) string {
	once.Do(initData)
	if label := labels[strings.ToLower(symbol)]; label != "" {
		return label
	}
	return symbol
}

// Formatting aliases never map unknown variants to another module's statistics.
func moduleKey(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

var wordBreak = regexp.MustCompile(`([a-z])([A-Z0-9])`)

func ModuleDisplayName(symbol string) string {
	if m, ok := ModuleBySymbol(symbol); ok && m.Name != "" {
		return m.Name
	}
	s := strings.Trim(strings.TrimSpace(symbol), "$;")
	s = wordBreak.ReplaceAllString(s, "$1 $2")
	s = strings.ReplaceAll(s, "_", " ")
	parts := strings.Fields(strings.ToLower(s))
	if len(parts) > 0 && (parts[0] == "int" || parts[0] == "hpt") {
		parts = parts[1:]
	}
	for i, w := range parts {
		r := []rune(w)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
			parts[i] = string(r)
		}
	}
	if len(parts) == 0 {
		return "Unidentified module"
	}
	return strings.Join(parts, " ")
}

// ShipDisplayName is a label only; it never substitutes another hull's stats.
func ShipDisplayName(symbol string) string {
	once.Do(initData)
	if v := labels[strings.ToLower(symbol)]; v != "" {
		return v
	}
	return ModuleDisplayName(symbol)
}
