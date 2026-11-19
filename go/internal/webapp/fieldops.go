package webapp

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/fieldops"
	"strconv"
	"strings"
)

func (s *Server) fieldView(owner bool) map[string]any {
	out := map[string]any{"authority": "IMPORTED ITINERARY — user plan, never the game NavRoute", "state": "no_plan", "next": nil, "previous": nil, "completed": 0, "total": 0, "notice": "No auto-targeting or auto-copy. Manual copy requires local owner and native clipboard. Fuel/range safety remains unknown."}
	if s.game == nil {
		return out
	}
	v, e := s.game.FieldProgress(owner)
	p := v.Progress.Plan
	if e != nil || p.ID == "" {
		return out
	}
	if !v.Enabled {
		out["state"] = "module_disabled"
		return out
	}
	out["state"] = p.State
	out["completed"] = p.Next
	out["total"] = v.Total
	out["next"] = v.Progress.Next
	out["previous"] = v.Progress.Previous
	out["gameTarget"] = v.Target
	out["currentSystem"] = v.Current
	out["evidence"] = v.Evidence
	if owner {
		out["plan"] = p
		out["plans"] = v.Plans
	}
	return out
}
func safeExportCell(v string) string {
	if len(v) > 0 && strings.ContainsAny(v[:1], "=+-@\t\r") {
		return "'" + v
	}
	return v
}
func (s *Server) fieldRPC(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.game == nil {
		return nil, errors.New("Journal service unavailable")
	}
	db, _ := s.game.History()
	if db == nil {
		return nil, errors.New("History database not ready")
	}
	var r struct {
		Name        string `json:"name"`
		CSVBase64   string `json:"csvBase64"`
		CSV         string `json:"csv"`
		ID          string `json:"id"`
		Hash        string `json:"hash"`
		Next        int    `json:"next"`
		Acknowledge bool   `json:"acknowledge"`
		FID         string `json:"fid"`
		System      string `json:"system"`
		Faction     string `json:"faction"`
		Before      string `json:"before"`
		Enabled     bool   `json:"enabled"`
		Ticks       bool   `json:"ticks"`
		Context     string `json:"context"`
	}
	if strictDecode(raw, &r) != nil {
		return nil, errors.New("Invalid field-operation request")
	}
	fid := s.game.FieldCommander()
	if r.FID != "" {
		if r.FID != fid {
			return nil, errors.New("Switch the global commander profile before querying another account")
		}
	}
	if len(fid) > 128 || len(r.System) > 20 || len(r.Faction) > 256 {
		return nil, errors.New("Invalid field filters")
	}
	switch name {
	case "getGoBuiltinModules":
		return db.BuiltinModules(), nil
	case "setGoBuiltinModule":
		if !r.Acknowledge {
			return nil, errors.New("Acknowledge module change; stored history is retained")
		}
		if e := db.SetBuiltinModule(r.ID, r.Enabled); e != nil {
			return nil, e
		}
		return db.BuiltinModules(), nil
	case "importGoItinerary":
		if r.CSVBase64 != "" {
			if r.CSV != "" || len(r.CSVBase64) > 699052 {
				return nil, errors.New("Invalid bounded CSV bytes")
			}
			b, e := base64.StdEncoding.Strict().DecodeString(r.CSVBase64)
			if e != nil {
				return nil, errors.New("Invalid CSV encoding")
			}
			r.CSV = string(b)
		}
		if len(r.CSV) > fieldops.MaxCSV {
			return nil, errors.New("CSV exceeds 512 KiB")
		}
		if _, e := db.ImportItinerary(fid, r.Name, []byte(r.CSV)); e != nil {
			return nil, e
		}
		return s.fieldView(true), nil
	case "deleteGoItinerary":
		if e := db.DeleteItinerary(fid, r.ID, r.Hash, r.Acknowledge); e != nil {
			return nil, e
		}
		return s.fieldView(true), nil
	case "getGoItinerary":
		return db.Itinerary(fid, r.ID)
	case "editGoItinerary":
		if _, e := db.EditItinerary(fid, r.ID, r.Next, r.Acknowledge); e != nil {
			return nil, e
		}
		return s.fieldView(true), nil
	case "copyGoItineraryNext":
		p, e := db.Itinerary(fid, "")
		if e != nil || p.Next >= len(p.Waypoints) || s.copyWaypoint == nil {
			return nil, errors.New("No next waypoint or native clipboard unavailable")
		}
		if e = s.copyWaypoint(p.Waypoints[p.Next].Name); e != nil {
			return nil, e
		}
		return map[string]string{"notice": "Copied next waypoint to the owner PC clipboard."}, nil
	case "exportGoItinerary":
		if !r.Acknowledge {
			return nil, errors.New("Export contains planned locations; acknowledge before saving")
		}
		p, e := db.Itinerary(fid, r.ID)
		if e != nil {
			return nil, e
		}
		var b strings.Builder
		w := csv.NewWriter(&b)
		w.Write([]string{"System Name", "SystemAddress", "Neutron", "Refuel", "x", "y", "z"})
		for _, p := range p.Waypoints {
			cells := []string{safeExportCell(p.Name), p.Address, strconv.FormatBool(p.Neutron), strconv.FormatBool(p.Refuel), "", "", ""}
			if p.Coordinates != nil {
				for i, v := range p.Coordinates {
					cells[4+i] = strconv.FormatFloat(v, 'g', -1, 64)
				}
			}
			w.Write(cells)
		}
		w.Flush()
		if e := w.Error(); e != nil {
			return nil, e
		}
		filename := "eridion-itinerary.csv"
		if name == "exportGoBGSTally" {
			filename = "eridion-private-activity.csv"
		}
		if s.saveFieldCSV != nil {
			if e := s.saveFieldCSV(filename, b.String()); e != nil {
				return nil, e
			}
			return map[string]string{"notice": "CSV saved to the selected local destination."}, nil
		}
		return map[string]string{"csv": b.String()}, nil
	case "getGoBGSTally", "exportGoBGSTally":
		if !db.BuiltinEnabled("bgs") {
			return nil, errors.New("BGS module disabled; stored history retained")
		}
		rows, e := db.FieldTallyCursor(ctx, fid, r.System, r.Faction, r.Before, 100, r.Ticks, r.Context)
		if e != nil {
			return nil, e
		}
		if name == "getGoBGSTally" {
			return rows, nil
		}
		if !r.Acknowledge {
			return nil, errors.New("Export contains commander, faction, location and activity. Explicit owner acknowledgment required")
		}
		var b strings.Builder
		w := csv.NewWriter(&b)
		w.Write([]string{"Commander", "Event", "SystemAddress", "Faction", "Category", "ObservedAt", "Amount", "Unit", "Qualifier", "Period"})
		for _, x := range rows {
			cells := []string{fid, x.Event, x.System, x.Faction, x.Category, x.At, x.Amount, x.Unit, x.Qualifier, x.Period}
			for i := range cells {
				cells[i] = safeExportCell(cells[i])
			}
			w.Write(cells)
		}
		w.Flush()
		if e := w.Error(); e != nil {
			return nil, e
		}
		filename := "eridion-itinerary.csv"
		if name == "exportGoBGSTally" {
			filename = "eridion-private-activity.csv"
		}
		if s.saveFieldCSV != nil {
			if e := s.saveFieldCSV(filename, b.String()); e != nil {
				return nil, e
			}
			return map[string]string{"notice": "CSV saved to the selected local destination."}, nil
		}
		return map[string]string{"csv": b.String()}, nil
	}
	return nil, errors.New("Unknown field operation")
}
