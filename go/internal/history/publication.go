package history

import (
	"database/sql"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"strconv"
	"time"
)

type Candidate struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type LiveException struct {
	Provider string `json:"provider"`
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
}
type PublicationPolicy struct {
	Mode       string           `json:"mode"`
	Notice     string           `json:"notice"`
	Exceptions []LiveException  `json:"exceptions"`
	Counts     map[string]int64 `json:"counts"`
}

func (s *Store) PublicationPolicy(fid string) (PublicationPolicy, error) {
	v := PublicationPolicy{Mode: "Never", Exceptions: []LiveException{}, Counts: map[string]int64{}}
	if fid == "" {
		return v, nil
	}
	err := s.reader.QueryRow("SELECT mode FROM publication_policy WHERE commander_fid=?", fid).Scan(&v.Mode)
	if err != nil && err != sql.ErrNoRows {
		return v, err
	}
	if v.Mode == "After sale" {
		v.Notice = "After sale: hold + manual review only. No automatic sale-triggered uploads. Manual review cannot authorize unsupported delayed recipients; EDDN is live-only. Separately consented fresh live exceptions remain independent."
	}
	rows, err := s.reader.Query("SELECT provider,category,enabled FROM publication_exceptions WHERE commander_fid=? LIMIT 100", fid)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var x LiveException
		if err = rows.Scan(&x.Provider, &x.Category, &x.Enabled); err != nil {
			rows.Close()
			return v, err
		}
		v.Exceptions = append(v.Exceptions, x)
	}
	rows.Close()
	rows, err = s.reader.Query("SELECT status,count(*) FROM publication_candidates WHERE commander_fid=? GROUP BY status", fid)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int64
		if err = rows.Scan(&key, &count); err != nil {
			return v, err
		}
		v.Counts[key] = count
	}
	return v, rows.Err()
}
func (s *Store) SetPublicationPolicy(fid, mode string) error {
	if len(fid) == 0 || len(fid) > 128 || (mode != "Never" && mode != "Live" && mode != "After sale") {
		return errors.New("Invalid commander publication policy")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO publication_policy VALUES(?,?,?) ON CONFLICT(commander_fid) DO UPDATE SET mode=excluded.mode,updated_at=excluded.updated_at", fid, mode, utc()); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE publication_candidates SET status='held',reason='Policy changed; review and authorize again' WHERE commander_fid=? AND status IN ('queued','eligible')", fid); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) SetLiveException(fid, provider, category string, enabled, acknowledged bool) error {
	if len(fid) == 0 || len(fid) > 128 || len(provider) == 0 || len(provider) > 32 || len(category) == 0 || len(category) > 32 || (enabled && !acknowledged) {
		return errors.New("Explicit category, recipient and location-leak acknowledgment required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.writer.Exec("INSERT INTO publication_exceptions VALUES(?,?,?,?) ON CONFLICT(commander_fid,provider,category) DO UPDATE SET enabled=excluded.enabled", fid, provider, category, enabled)
	return err
}

// Candidate only evaluates already committed, exactly attributed source records.
// Historical eligibility does not enqueue anything; the owner wizard is separate.
func (s *Store) Candidate(fid, provider, category string, e journal.Event, phase journal.Phase, historical bool, now time.Time) (Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Candidate{Status: "held", Reason: "Never: publication is private by default"}
	if fid == "" || provider == "" || category == "" || len(provider) > 32 || len(category) > 32 {
		return v, errors.New("Invalid publication recipient")
	}
	var eventID int64
	var owner sql.NullString
	if err := s.writer.QueryRow("SELECT event_id,commander_fid FROM journal_events WHERE source_id=? AND byte_offset=? AND line_hash=?", e.Position.FileID, e.Position.Offset, digest(e.Raw)).Scan(&eventID, &owner); err != nil || !owner.Valid || owner.String != fid {
		return v, errors.New("Publication event is not attributed to this commander")
	}
	mode := "Never"
	err := s.writer.QueryRow("SELECT mode FROM publication_policy WHERE commander_fid=?", fid).Scan(&mode)
	if err != nil && err != sql.ErrNoRows {
		return v, err
	}
	age := now.Sub(e.Timestamp)
	fresh := phase == journal.Live && !e.Timestamp.IsZero() && age >= -5*time.Second && age <= 2*time.Minute
	if mode == "Live" && fresh {
		v.Status = "eligible"
		v.Reason = "Fresh live event; independent provider/category consent still required"
	}
	if mode == "Live" && !fresh {
		v.Reason = "Historical replay and catch-up never publish automatically"
	}
	if mode == "After sale" {
		var exception bool
		s.writer.QueryRow("SELECT enabled FROM publication_exceptions WHERE commander_fid=? AND provider=? AND category=?", fid, provider, category).Scan(&exception)
		if exception && fresh {
			v.Status = "eligible"
			v.Reason = "Explicit recipient/category live location disclosure exception"
		} else {
			v.Reason = "Unsold or unmatched observation; location-sensitive categories held across providers"
			var confidence string
			err = s.writer.QueryRow("SELECT confidence FROM sale_item_links WHERE observation_event_id=? ORDER BY CASE confidence WHEN 'proven' THEN 0 WHEN 'owner_confirmed' THEN 0 ELSE 1 END LIMIT 1", eventID).Scan(&confidence)
			if err == nil {
				if confidence == "proven" || confidence == "owner_confirmed" {
					if historical && provider == "inara" && !e.Timestamp.IsZero() && age >= 0 && age <= 30*24*time.Hour {
						v.Status = "eligible"
						v.Reason = "Explicit historical Inara selection within documented 30-day window"
					} else {
						v.Status = "ineligible_delayed"
						v.Reason = "After sale: hold + manual review only; delayed contract not verified for this recipient. Manual review cannot authorize unsupported delayed recipients; live-only relay requires a separate fresh exception"
					}
				} else {
					v.Status = "ambiguous"
					v.Reason = "Sale association is not unique or complete; manual confirmation required"
				}
			}
		}
	}
	if historical && mode == "Live" {
		if provider == "inara" && !e.Timestamp.IsZero() && age >= 0 && age <= 30*24*time.Hour {
			v.Status = "eligible"
			v.Reason = "Explicit historical Inara selection within 30-day limit"
		} else {
			v.Status = "ineligible_delayed"
			v.Reason = "Recipient historical contract unavailable or event too old"
		}
	}
	_, err = s.writer.Exec("INSERT INTO publication_candidates(source_event_id,commander_fid,provider,category,status,reason,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(source_event_id,provider,category) DO UPDATE SET status=CASE WHEN status IN ('queued','confirmed','no_new_observations','unknown_delivery') THEN status ELSE excluded.status END,reason=CASE WHEN status IN ('queued','confirmed','no_new_observations','unknown_delivery') THEN reason ELSE excluded.reason END", eventID, fid, provider, category, v.Status, v.Reason, utc())
	if err != nil {
		return v, err
	}
	err = s.writer.QueryRow("SELECT cast(id AS TEXT),status,reason FROM publication_candidates WHERE source_event_id=? AND provider=? AND category=?", eventID, provider, category).Scan(&v.ID, &v.Status, &v.Reason)
	return v, err
}
func (s *Store) CandidateOutcome(id, status, reason string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n < 1 || strconv.FormatInt(n, 10) != id {
		return errors.New("Invalid candidate")
	}
	switch status {
	case "queued", "confirmed", "no_new_observations", "failed", "unknown_delivery", "held":
	default:
		return errors.New("Invalid publication outcome")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.writer.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE publication_candidates SET status=?,reason=? WHERE id=?", status, reason, n); err != nil {
		return err
	}
	if status != "queued" && status != "held" {
		if _, err = tx.Exec("INSERT INTO publication_attempts(candidate_id,attempted_at,receipt_hash,outcome) VALUES(?,?,?,?)", n, utc(), digest([]byte(reason)), status); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) CancelCandidates(fid, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.writer.Exec("UPDATE publication_candidates SET status='held',reason='Consent cancelled; no retry without review' WHERE commander_fid=? AND provider=? AND status IN ('queued','eligible')", fid, provider)
	return err
}

func (s *Store) HoldPending(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.writer.Exec("UPDATE publication_candidates SET status='held',reason=? WHERE status IN ('queued','eligible')", reason)
	return err
}
