package history

import (
	"database/sql"
	"errors"
)

type BuiltinModule struct {
	BuiltIn      bool   `json:"builtIn"`
	Upstream     string `json:"upstream"`
	LicenseURL   string `json:"licenseURL"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	Version      string `json:"version"`
	Enabled      bool   `json:"enabled"`
	Credit       string `json:"credit"`
	License      string `json:"license"`
	Dependencies string `json:"dependencies"`
	Network      string `json:"network"`
	Platforms    string `json:"platforms"`
	Schema       int    `json:"schema"`
}

func moduleEnabled(q interface{ QueryRow(string, ...any) *sql.Row }, id string) bool {
	var enabled bool
	e := q.QueryRow("SELECT enabled FROM builtin_modules WHERE id=?", id).Scan(&enabled)
	return e == sql.ErrNoRows || (e == nil && enabled)
}
func (s *Store) BuiltinEnabled(id string) bool { return moduleEnabled(s.reader, id) }
func (s *Store) BuiltinModules() []BuiltinModule {
	out := []BuiltinModule{{ID: "itinerary", Title: "Imported neutron itinerary", Version: "arrival-v1", Credit: "Original Go; Neutron Dancer inspiration (dwomble)", License: "GPL-3.0; MIT reference", Dependencies: "Location/FSDJump; private route tables"}, {ID: "bgs", Title: "Local BGS Tally", Version: "journal-effects-v1", Credit: "Original Go; BGS-Tally inspiration (tezw21/aussig)", License: "GPL-3.0; MIT reference", Dependencies: "MissionCompleted.FactionEffects; trade/combat/colonisation/Powerplay"}, {ID: "bioscan", Title: "BioScan assistance", Version: "bioscan-fc6b85c-explodata-fd4e188-env-v1", Credit: "Jeremy Rimpo / Silarn; DAEDALUS Go implementation", License: "GPL-2.0-or-later reference, GPL-3.0 selection", Dependencies: "Scan/organic signals; correlated Status snapshots"}}
	for _, id := range []string{"fleet", "carrier", "economy", "commander-operations"} {
		out = append(out, BuiltinModule{ID: id, Title: "Native " + id, Version: "journal-operations-v1", Credit: "DAEDALUS contributors; Frontier journal contracts", License: "GPL-3.0-or-later", Dependencies: "Single journal pipeline; private SQLite v6"})
	}
	for i := range out {
		out[i].BuiltIn = true
		switch out[i].ID {
		case "itinerary":
			out[i].Upstream = "https://github.com/dwomble/EDMC-NeutronDancer/tree/3b0bfc762b565397b71dc03fa9db1518c2c2c4d8"
			out[i].LicenseURL = out[i].Upstream + "/LICENSE"
		case "bgs":
			out[i].Upstream = "https://github.com/aussig/BGS-Tally/tree/3a15d9738fee2cddcc2bb0ddc0823cf10615ae66"
			out[i].LicenseURL = out[i].Upstream + "/LICENSE"
		case "bioscan":
			out[i].Upstream = "https://github.com/Silarn/EDMC-BioScan/tree/fc6b85c3de48c8fb79fa9e4f79ad780e48f56629"
			out[i].LicenseURL = out[i].Upstream + "/LICENSE.txt"
		}
		out[i].Enabled = s.BuiltinEnabled(out[i].ID)
		out[i].Network = "None; optional tick reference uses existing separate consent"
		out[i].Platforms = "Windows/Linux"
		out[i].Schema = schemaVersion
	}
	return out
}
func (s *Store) SetBuiltinModule(id string, enabled bool) error {
	if id != "itinerary" && id != "bgs" && id != "bioscan" && id != "fleet" && id != "carrier" && id != "economy" && id != "commander-operations" {
		return errors.New("Only compiled-in field modules may be controlled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.writer.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO builtin_modules VALUES(?,?) ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled", id, enabled); e != nil {
		return e
	}
	if id == "bgs" && enabled {
		if e = backfillFieldTally(tx); e != nil {
			return e
		}
	}
	if enabled && (id == "fleet" || id == "carrier" || id == "economy" || id == "commander-operations") {
		if e = backfillOperations(tx); e != nil {
			return e
		}
	}
	return tx.Commit()
}
