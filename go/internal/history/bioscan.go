package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/fieldops"
	"time"
)

type BioSample struct {
	Source     string             `json:"-"`
	Sequence   int64              `json:"-"`
	Offset     int64              `json:"-"`
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
	rows, e := s.reader.QueryContext(ctx, `SELECT e.raw_relevant_json,coalesce(e.event_utc,''),p.latitude,p.longitude,p.radius,p.status_at,coalesce(e.session_id,''),e.event_id,e.source_id,e.record_sequence,e.byte_offset FROM bio_samples b JOIN journal_events e ON e.event_id=b.source_event_id LEFT JOIN bio_positions p ON p.source_event=e.event_id WHERE b.commander_fid=? AND b.system_address=? AND b.entity_id=? ORDER BY b.source_event_id DESC LIMIT 24`, fid, system, body)
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
		if e = rows.Scan(&raw, &v.At, &lat, &lon, &radius, &at, &v.Session, &v.Event, &v.Source, &v.Sequence, &v.Offset); e != nil {
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
	rows.Close()
	for i := range samples {
		a := &samples[i]
		if a.Session == "" || a.Position == nil || (a.Stage != "Sample" && a.Stage != "Analyse") {
			continue
		}
		// A witnessed prefix Log->Sample (or completed Log->Sample->Analyse)
		// is required. Never borrow positions from an incomplete previous cycle.
		stages := []string{"Sample", "Log"}
		if a.Stage == "Analyse" {
			stages = []string{"Analyse", "Sample", "Log"}
		}
		if i+len(stages) > len(samples) {
			continue
		}
		cycle := samples[i : i+len(stages)]
		valid := true
		for k, b := range cycle {
			if b.Stage != stages[k] || b.Session != a.Session || b.Source != a.Source || b.Genus != a.Genus || b.Species != a.Species || b.Variant != a.Variant || b.Position == nil {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		log := cycle[len(cycle)-1]
		valid, err := s.bioCycleContinuous(ctx, fid, system, body, log, *a)
		if err != nil {
			return out, err
		}
		if !valid {
			continue
		}
		var nearest *float64
		for _, b := range cycle[1:] {
			d, ok := fieldops.SurfaceDistance(*a.Position, *b.Position)
			if !ok {
				valid = false
				break
			}
			if nearest == nil || d < *nearest {
				nearest = &d
			}
		}
		if !valid {
			continue
		}
		a.DistanceM = nearest
		if minimum := fieldops.MinimumDistance(a.Genus); minimum != nil && nearest != nil {
			remaining := *minimum - *nearest
			if remaining < 0 {
				remaining = 0
			}
			a.RemainingM = &remaining
		}
	}
	out["samples"] = samples
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

// Metadata for every journal record is retained, even when its payload is not.
// Bound the proof work and fail closed on gaps or context/cycle boundaries.
func (s *Store) bioCycleContinuous(ctx context.Context, fid, system, body string, log, target BioSample) (bool, error) {
	if target.Sequence <= log.Sequence || target.Sequence-log.Sequence > 4096 {
		return false, nil
	}
	rows, err := s.reader.QueryContext(ctx, `SELECT e.record_sequence,coalesce(e.session_id,''),coalesce(e.commander_fid,''),e.type,e.raw_relevant_json FROM journal_events e JOIN journal_sources j ON j.source_id=e.source_id JOIN game_sessions g ON g.session_id=e.session_id AND g.commander_fid=e.commander_fid AND g.file_id=e.source_id WHERE e.source_id=? AND j.state='verified' AND e.byte_offset>=? AND e.byte_offset<=? ORDER BY e.byte_offset LIMIT 4097`, log.Source, log.Offset, target.Offset)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	next := log.Sequence
	organic := 0
	expected := []string{"Log", "Sample", "Analyse"}
	for rows.Next() {
		var sequence int64
		var session, commander, kind, raw string
		if err = rows.Scan(&sequence, &session, &commander, &kind, &raw); err != nil {
			return false, err
		}
		if sequence != next || session != log.Session || commander != fid {
			return false, nil
		}
		next++
		switch kind {
		case "Fileheader", "LoadGame", "Shutdown", "FSDJump", "ApproachBody", "LeaveBody":
			return false, nil
		case "ScanOrganic":
			v := object([]byte(raw))
			if organic >= len(expected) || text(v["ScanType"]) != expected[organic] || identifier(v["Body"]) != body || identifier(v["SystemAddress"]) != system || text(v["Genus"]) != target.Genus || text(v["Species"]) != target.Species || text(v["Variant"]) != target.Variant {
				return false, nil
			}
			organic++
		}
	}
	if err = rows.Err(); err != nil {
		return false, err
	}
	want := 2
	if target.Stage == "Analyse" {
		want = 3
	}
	return next == target.Sequence+1 && organic == want, nil
}
