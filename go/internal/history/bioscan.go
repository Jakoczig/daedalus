package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/fieldops"
	"time"
)

type BioSample struct {
	Event      int64              `json:"-"`
	Session    string             `json:"-"`
	Stage      string             `json:"stage"`
	Genus      string             `json:"genus"`
	Species    string             `json:"species"`
	Variant    string             `json:"variant"`
	At         string             `json:"at"`
	Position   *fieldops.Position `json:"position,omitempty"`
	Notice     string             `json:"notice"`
	DistanceM  *float64           `json:"distanceM"`
	RemainingM *float64           `json:"remainingM"`
}

// The caller supplies current verified live context, never browser-provided IDs.
func (s *Store) RecordBioPosition(fid, system, body, name string, status []byte, now time.Time, source string) error {
	if fid == "" || system == "" || body == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var id int64
	var raw, at string
	e := s.writer.QueryRow(`SELECT e.event_id,e.raw_relevant_json,coalesce(e.event_utc,'') FROM journal_events e WHERE e.commander_fid=? AND e.system_address=? AND e.type='ScanOrganic' AND e.source_id=? AND e.session_id=(SELECT context_session FROM journal_sources WHERE source_id=e.source_id AND context_fid=e.commander_fid AND state='verified') ORDER BY e.event_id DESC LIMIT 1`, fid, system, source).Scan(&id, &raw, &at)
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	v := object([]byte(raw))
	if identifier(v["Body"]) != body {
		return nil
	}
	stage := text(v["ScanType"])
	if stage != "Log" && stage != "Sample" && stage != "Analyse" {
		return nil
	}
	sample, e := time.Parse(time.RFC3339Nano, at)
	if e != nil {
		return nil
	}
	p, ok := fieldops.StatusPosition(status, name, sample, now)
	if !ok {
		return nil
	}
	_, e = s.writer.Exec("INSERT OR IGNORE INTO bio_positions VALUES(?,?,?,?,?,?,?,?,?)", id, fid, system, body, p.Latitude, p.Longitude, p.Radius, p.At, "Time-adjacent Status snapshot (up to 2s), not an exact sample pin")
	return e
}
func (s *Store) BioBody(ctx context.Context, fid, system, body string) (map[string]any, error) {
	out := map[string]any{"version": fieldops.BioRuleVersion(), "candidates": []fieldops.Candidate{}, "samples": []BioSample{}, "notice": "Pinned community criteria may be outdated. Possible species are not confirmed. Sample location unknown unless a correlated Status snapshot exists."}
	var raw string
	e := s.reader.QueryRowContext(ctx, "SELECT raw_json FROM body_observations WHERE commander_fid=? AND system_address=? AND body_id=? AND json_extract(raw_json,'$.event')='Scan' ORDER BY source_event_id DESC LIMIT 1", fid, system, body).Scan(&raw)
	if e != nil && e != sql.ErrNoRows {
		return out, e
	}
	scan := object([]byte(raw))
	rows, e := s.reader.QueryContext(ctx, `SELECT e.raw_relevant_json,coalesce(e.event_utc,''),p.latitude,p.longitude,p.radius,p.status_at,coalesce(e.session_id,''),e.event_id FROM bio_samples b JOIN journal_events e ON e.event_id=b.source_event_id LEFT JOIN bio_positions p ON p.source_event=e.event_id WHERE b.commander_fid=? AND b.system_address=? AND b.entity_id=? ORDER BY b.source_event_id DESC LIMIT 24`, fid, system, body)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	samples := []BioSample{}
	genera := []string{}
	for rows.Next() {
		var v BioSample
		var raw string
		var lat, lon, radius sql.NullFloat64
		var at sql.NullString
		if e = rows.Scan(&raw, &v.At, &lat, &lon, &radius, &at, &v.Session, &v.Event); e != nil {
			return out, e
		}
		x := object([]byte(raw))
		v.Stage = text(x["ScanType"])
		v.Genus = text(x["Genus"])
		v.Species = text(x["Species"])
		v.Variant = text(x["Variant"])
		genera = append(genera, v.Genus)
		v.Notice = "sample location unknown"
		if lat.Valid && lon.Valid && radius.Valid && at.Valid {
			v.Position = &fieldops.Position{Latitude: lat.Float64, Longitude: lon.Float64, Radius: radius.Float64, At: at.String}
			v.Notice = "Derived from time-adjacent Status snapshots; not game-confirmed eligible or an exact pin"
		}
		samples = append(samples, v)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	out["samples"] = samples
	for i := range samples {
		if samples[i].Position == nil || samples[i].Stage == "Log" {
			continue
		}
		for j := i + 1; j < len(samples); j++ {
			a, b := samples[i], samples[j]
			if a.Genus != b.Genus || a.Species != b.Species || a.Variant != b.Variant {
				continue
			}
			if b.Stage == "Analyse" {
				break
			}
			if b.Position == nil {
				if b.Stage == "Log" {
					break
				}
				continue
			}
			if d, ok := fieldops.SurfaceDistance(*a.Position, *b.Position); ok {
				if samples[i].DistanceM != nil && *samples[i].DistanceM < d {
					d = *samples[i].DistanceM
				}
				samples[i].DistanceM = &d
				if minimum := fieldops.MinimumDistance(a.Genus); minimum != nil {
					remaining := *minimum - d
					if remaining < 0 {
						remaining = 0
					}
					samples[i].RemainingM = &remaining
				}
			}
			if b.Stage == "Log" {
				break
			}
		}
	}
	// A missing genus stays unknown. Observed signal evidence is needed before
	// environmental suggestions; the caller supplies it from the survey reducer.
	out["scan"] = json.RawMessage(raw)
	out["genera"] = genera
	out["environment"] = scan
	return out, nil
}
func (s *Store) ActiveOrganicEvent(fid, source string) int64 {
	var event int64
	var kind string
	if e := s.reader.QueryRow("SELECT event_id,type FROM journal_events WHERE commander_fid=? AND source_id=? AND type IN ('ScanOrganic','ApproachBody','LeaveBody','FSDJump','LoadGame','Shutdown') ORDER BY event_id DESC LIMIT 1", fid, source).Scan(&event, &kind); e != nil || kind != "ScanOrganic" {
		return 0
	}
	return event
}
func (s *Store) BioSession(fid, source string) string {
	var session string
	_ = s.reader.QueryRow("SELECT coalesce(context_session,'') FROM journal_sources WHERE source_id=? AND context_fid=? AND state='verified'", source, fid).Scan(&session)
	return session
}
