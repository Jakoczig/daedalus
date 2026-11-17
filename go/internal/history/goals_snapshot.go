package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const goalSnapshotField = "subscription_snapshot"

type GoalMembership struct {
	Known      bool     `json:"known"`
	IDs        []string `json:"ids"`
	ObservedAt string   `json:"observedAt"`
	Notice     string   `json:"notice"`
}

// Only an explicit, bounded, well-formed complete array can change membership.
// Missing/null/invalid arrays are not evidence that the player left all goals.
func validGoalSnapshot(v map[string]any) ([]any, []string, bool) {
	goals, ok := v["CurrentGoals"].([]any)
	if !ok || len(goals) > 2048 {
		return nil, nil, false
	}
	ids := make([]string, 0, len(goals))
	seen := map[string]bool{}
	for _, item := range goals {
		g, ok := item.(map[string]any)
		if !ok {
			return nil, nil, false
		}
		id := identifier(g["CGID"])
		if id == "" || seen[id] {
			return nil, nil, false
		}
		seen[id] = true
		for _, key := range []string{"Title", "SystemName", "MarketName", "TierReached"} {
			if x, present := g[key]; present {
				str, ok := x.(string)
				if !ok || (str != "" && text(str) == "") {
					return nil, nil, false
				}
			}
		}
		for _, key := range []string{"IsComplete", "PlayerInTopRank"} {
			if x, present := g[key]; present {
				if _, ok := x.(bool); !ok {
					return nil, nil, false
				}
			}
		}
		for _, key := range []string{"CurrentTotal", "PlayerContribution", "NumContributors", "PlayerPercentileBand", "TopRankSize", "Bonus"} {
			if x, present := g[key]; present && identifier(x) == "" {
				return nil, nil, false
			}
		}
		if x, present := g["Expiry"]; present {
			str, ok := x.(string)
			if !ok {
				return nil, nil, false
			}
			if _, e := time.Parse(time.RFC3339Nano, str); e != nil {
				return nil, nil, false
			}
		}
		ids = append(ids, id)
	}
	return goals, ids, true
}

func recordGoalSnapshot(tx *sql.Tx, fid, at string, event int64, v map[string]any) (bool, error) {
	_, ids, valid := validGoalSnapshot(v)
	if !valid || fid == "" {
		return false, nil
	}
	if _, e := time.Parse(time.RFC3339Nano, at); e != nil {
		return false, nil
	}
	// Strictly newer observation time wins. Ingestion order is not journal time;
	// equal-time conflicting snapshots cannot silently replace known membership.
	_, e := tx.Exec(`INSERT INTO commander_current VALUES(?, 'goals', '', ?, ?, ?, ?) ON CONFLICT(commander_fid,category,entity_id,field) DO UPDATE SET content_json=excluded.content_json,observed_at=excluded.observed_at,event_id=excluded.event_id WHERE excluded.observed_at>commander_current.observed_at`, fid, goalSnapshotField, encoded(map[string]any{"event": "CommunityGoal", "ids": ids, "complete": true}), at, event)
	return true, e
}

func (s *Store) goalMembership(ctx context.Context, fid string) (GoalMembership, error) {
	out := GoalMembership{IDs: []string{}, Notice: "Unknown: no valid complete subscribed-goal snapshot witnessed. Expiry, completion and rewards are not inferred."}
	var raw string
	e := s.reader.QueryRowContext(ctx, "SELECT content_json,observed_at FROM commander_current WHERE commander_fid=? AND category='goals' AND entity_id='' AND field=?", fid, goalSnapshotField).Scan(&raw, &out.ObservedAt)
	if e == sql.ErrNoRows {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	var value struct {
		IDs      []string `json:"ids"`
		Complete bool     `json:"complete"`
	}
	if json.Unmarshal([]byte(raw), &value) != nil || !value.Complete || value.IDs == nil {
		return out, nil
	}
	out.Known = true
	out.IDs = value.IDs
	out.Notice = "Subscribed as last witnessed at the complete snapshot time, not a live status. Historical contributions/rewards are retained; expiry/completion do not imply membership or payment."
	return out, nil
}

// Repair the reviewed v6 profile once from retained CommunityGoal payloads,
// including empty snapshots which previously had no per-goal projection row.
// No schema/checksum changes, source reread, checkpoint reset or publication.
func repairGoalMembership(db *sql.DB) error {
	var done string
	e := db.QueryRow("SELECT value FROM operations_state WHERE key='cg-membership-v1'").Scan(&done)
	if e == nil {
		return nil
	}
	if e != sql.ErrNoRows {
		return e
	}
	if !moduleEnabled(db, "commander-operations") {
		return nil
	}
	tx, e := db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var cursor int64
	for {
		rows, e := tx.Query(`SELECT event_id,commander_fid,coalesce(event_utc,''),raw_relevant_json FROM journal_events WHERE type='CommunityGoal' AND commander_fid IS NOT NULL AND event_id>? ORDER BY event_id LIMIT 128`, cursor)
		if e != nil {
			return e
		}
		type entry struct {
			id           int64
			fid, at, raw string
		}
		batch := []entry{}
		for rows.Next() {
			var x entry
			if e = rows.Scan(&x.id, &x.fid, &x.at, &x.raw); e != nil {
				rows.Close()
				return e
			}
			batch = append(batch, x)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(batch) == 0 {
			break
		}
		for _, x := range batch {
			cursor = x.id
			v := object([]byte(x.raw))
			if _, e = recordGoalSnapshot(tx, x.fid, x.at, x.id, v); e != nil {
				return e
			}
		}
	}
	if _, e = tx.Exec("INSERT OR IGNORE INTO operations_state VALUES('cg-membership-v1','Complete snapshot membership repaired; source history unchanged')"); e != nil {
		return e
	}
	return tx.Commit()
}
