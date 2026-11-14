// Package catalog contains reference data selected from the repository's bundled
// EDCD FDevIDs CSVs and Coriolis modules. These are static reference facts, never
// measurements of a player's ship. No user files, runtime IO or network is used.
package catalog

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"strings"
	"sync"
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
	for _, filename := range []string{"outfitting.csv", "material.csv", "microresources.csv", "commodity.csv"} {
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
			key := strings.ToLower(row[symbolCol])
			labels[key] = row[nameCol]
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
		key := strings.ToLower(r.Symbol)
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
	m, ok := modules[strings.ToLower(symbol)]
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
