package history

import (
	"database/sql"
)

func commanderCurrent(tx *sql.Tx, fid, category, entity, kind, at string, event int64, data map[string]any) error {
	field := "last_observed_event"
	switch category {
	case "goals":
		switch kind {
		case "CommunityGoal":
			field = "progress_snapshot"
		case "CommunityGoalJoin", "CommunityGoalDiscard":
			field = "participation_event"
		case "CommunityGoalReward":
			field = "reward_received"
		}
	case "powerplay":
		switch kind {
		case "Powerplay", "PowerplayMerits", "PowerplayRank":
			field = kind
		case "PowerplayJoin", "PowerplayLeave", "PowerplayDefect":
			field = "pledge_event"
		}
	case "squadron":
		field = "affiliation_event"
	case "crew":
		if entity == "" {
			field = "last_event_identity_not_witnessed"
		} else {
			field = "crew_assignment_event"
		}
	default:
		return nil
	}
	_, e := tx.Exec(`INSERT INTO commander_current VALUES(?,?,?,?,?,?,?) ON CONFLICT(commander_fid,category,entity_id,field) DO UPDATE SET content_json=excluded.content_json,observed_at=excluded.observed_at,event_id=excluded.event_id WHERE excluded.observed_at>commander_current.observed_at OR (excluded.observed_at=commander_current.observed_at AND excluded.event_id>commander_current.event_id)`, fid, category, entity, field, encoded(map[string]any{"event": kind, "fields": data}), at, event)
	return e
}
