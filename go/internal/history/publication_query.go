package history

import (
	"context"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"time"
)

type PublicationRecord struct {
	ID, Name, Version string
	Event             journal.Event
}

func (s *Store) PublicationRecords(ctx context.Context, fid string, from, to time.Time, limit int) ([]PublicationRecord, error) {
	if fid == "" || len(fid) > 128 || from.IsZero() || !to.After(from) || limit < 1 || limit > 32 {
		return nil, errors.New("Choose commander, UTC time window and at most 32 records")
	}
	rows, err := s.reader.QueryContext(ctx, `SELECT cast(e.event_id AS TEXT),coalesce(c.latest_name,''),coalesce(g.game_version,''),e.source_id,e.byte_offset,e.record_sequence,e.raw_relevant_json FROM journal_events e JOIN commander c ON c.fid=e.commander_fid LEFT JOIN game_sessions g ON g.session_id=e.session_id WHERE e.commander_fid=? AND e.event_utc>=? AND e.event_utc<? AND e.type IN ('FSDJump','Loadout','Materials','Rank','Progress','Statistics','Scan','SAASignalsFound','ScanOrganic','CarrierStats','CarrierJumpRequest') ORDER BY e.event_utc,e.event_id LIMIT ?`, fid, from.UTC().Format("2006-01-02T15:04:05.000000000Z"), to.UTC().Format("2006-01-02T15:04:05.000000000Z"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicationRecord{}
	for rows.Next() {
		var r PublicationRecord
		var raw string
		var pos journal.SourcePosition
		if err = rows.Scan(&r.ID, &r.Name, &r.Version, &pos.FileID, &pos.Offset, &pos.Sequence, &raw); err != nil {
			return nil, err
		}
		if len(raw) > 256<<10 {
			continue
		}
		r.Event, err = journal.ParseLine([]byte(raw), pos)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
