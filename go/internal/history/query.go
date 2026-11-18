package history

import (
	"context"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"strconv"
	"strings"
	"time"
)

type Commander struct {
	FID       string `json:"fid"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updatedAt"`
}
type HistoryRow struct {
	Truncated     bool   `json:"truncated,omitempty"`
	RawBytes      int    `json:"rawBytes"`
	ID            string `json:"id"`
	Type          string `json:"type"`
	ObservedAt    string `json:"observedAt"`
	SystemAddress string `json:"systemAddress"`
	SystemName    string `json:"systemName"`
	JSON          string `json:"json"`
}
type HistoryPage struct {
	Rows []HistoryRow `json:"rows"`
	Next string       `json:"next"`
}

var pageTypes = map[string][]string{"history": {}, "ships": {"Loadout", "ShipyardBuy", "ShipyardSell", "ShipyardSwap", "StoredShips", "StoredModules"}, "scans": {"Scan", "FSSDiscoveryScan", "SAAScanComplete", "SAASignalsFound"}, "biology": {"ScanOrganic"}, "sales": {"SellExplorationData", "MultiSellExplorationData", "SellOrganicData"}, "bgs": {"MissionCompleted", "RedeemVoucher", "MarketSell", "FSDJump", "Location"}, "colonisation": {"ColonisationConstructionDepot", "ColonisationContribution"}, "squadron": {"SquadronStartup", "SquadronPromotion", "SquadronDemotion", "JoinedSquadron", "LeftSquadron"}, "goals": {"CommunityGoal", "CommunityGoalJoin", "CommunityGoalReward"}}

func (s *Store) Commanders() ([]Commander, error) {
	rows, err := s.reader.Query("SELECT fid,coalesce(latest_name,''),coalesce(updated_at,'') FROM commander ORDER BY updated_at DESC LIMIT 65")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Commander{}
	for rows.Next() {
		var v Commander
		if err = rows.Scan(&v.FID, &v.Name, &v.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *Store) Page(ctx context.Context, fid, kind, before string, limit int) (HistoryPage, error) {
	result := HistoryPage{Rows: []HistoryRow{}}
	types, ok := pageTypes[kind]
	if !ok || len(fid) == 0 || len(fid) > 128 || limit < 1 || limit > 100 {
		return result, errors.New("Invalid indexed history query")
	}
	cursor := int64(9223372036854775807)
	if before != "" {
		n, e := strconv.ParseInt(before, 10, 64)
		if e != nil || n < 1 || strconv.FormatInt(n, 10) != before {
			return result, errors.New("Invalid cursor")
		}
		cursor = n
	}
	query := "SELECT cast(e.event_id AS TEXT),e.type,coalesce(e.event_utc,''),coalesce(e.system_address,''),coalesce(s.name,''),e.raw_relevant_json FROM journal_events e LEFT JOIN systems s ON s.system_address=e.system_address WHERE e.commander_fid=? AND e.event_id<? AND e.raw_relevant_json!='{}'"
	args := []any{fid, cursor}
	if len(types) > 0 {
		query += " AND e.type IN (" + strings.TrimRight(strings.Repeat("?,", len(types)), ",") + ")"
		for _, v := range types {
			args = append(args, v)
		}
	}
	query += " ORDER BY e.event_id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.reader.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var r HistoryRow
		if err = rows.Scan(&r.ID, &r.Type, &r.ObservedAt, &r.SystemAddress, &r.SystemName, &r.JSON); err != nil {
			return result, err
		}
		r.RawBytes = len(r.JSON)
		if len(r.JSON) > 16384 {
			r.JSON = r.JSON[:16384]
			r.Truncated = true
		}
		result.Rows = append(result.Rows, r)
	}
	if len(result.Rows) > limit {
		result.Rows = result.Rows[:limit]
		result.Next = result.Rows[len(result.Rows)-1].ID
	}
	return result, rows.Err()
}

// Walk streams original source order, including payload-free continuity markers.
// Caller controls cancellation; no records are accumulated in memory.
func (s *Store) Walk(ctx context.Context, consume func(journal.Event) error) error {
	rows, err := s.reader.QueryContext(ctx, "SELECT e.type,coalesce(e.event_utc,''),e.raw_relevant_json,s.canonical_path,e.source_id,e.byte_offset,e.byte_length,e.record_sequence FROM journal_events e JOIN journal_sources s USING(source_id) JOIN (SELECT source_id,min(event_utc) first_utc FROM journal_events GROUP BY source_id) o USING(source_id) ORDER BY coalesce(o.first_utc,s.first_seen_at),s.canonical_path,e.byte_offset")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e journal.Event
		var stamp, raw string
		if err = rows.Scan(&e.Type, &stamp, &raw, &e.Position.Path, &e.Position.FileID, &e.Position.Offset, &e.Position.Length, &e.Position.Sequence); err != nil {
			return err
		}
		if raw != "{}" {
			parsed, parseErr := journal.ParseLine([]byte(raw), e.Position)
			if parseErr != nil {
				return parseErr
			}
			e = parsed
		} else {
			e.Raw = []byte("{}")
			if stamp != "" {
				e.Timestamp, err = time.Parse(time.RFC3339Nano, stamp)
				if err != nil {
					return err
				}
			}
		}
		if err = consume(e); err != nil {
			return err
		}
	}
	return rows.Err()
}
func (s *Store) SaveCache(ctx context.Context, raw []byte) error {
	if len(raw) > maxCache {
		return errors.New("Cache budget exceeded")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.writer.ExecContext(ctx, "INSERT INTO app_state VALUES('reducers-v1',?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,content_hash=excluded.content_hash,updated_at=excluded.updated_at", raw, digest(raw), utc())
	return err
}

func (s *Store) OwnedFleetCount(ctx context.Context, fid string) (int, error) {
	var n int
	e := s.reader.QueryRowContext(ctx, "SELECT count(*) FROM fleet_ships WHERE commander_fid=? AND state<>'sold'", fid).Scan(&n)
	return n, e
}
