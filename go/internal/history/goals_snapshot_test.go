package history

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"strings"
	"testing"
)

func goalView(t *testing.T, s *Store, fid string) (bool, []string, map[string]string) {
	t.Helper()
	p, e := s.Operations(context.Background(), fid, OperationQuery{Kind: "goals"})
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(p)
	var v struct {
		GoalMembership struct {
			Known bool     `json:"known"`
			IDs   []string `json:"ids"`
		} `json:"goalMembership"`
	}
	if e = json.Unmarshal(raw, &v); e != nil {
		t.Fatal(e)
	}
	states := map[string]string{}
	for _, r := range p.Current {
		if r.Title == "progress_snapshot" {
			states[r.Entity] = r.State
		}
	}
	return v.GoalMembership.Known, v.GoalMembership.IDs, states
}
func TestGoalCompleteSnapshotMembership(t *testing.T) {
	a := `{"event":"CommunityGoal","timestamp":"2026-01-01T00:01:01Z","CurrentGoals":[{"CGID":9007199254740993,"Title":"A","PlayerContribution":12}]}`
	ab := `{"event":"CommunityGoal","timestamp":"2026-01-01T00:01:01Z","CurrentGoals":[{"CGID":9007199254740993,"Title":"A"},{"CGID":18446744073709551615,"Title":"B"}]}`
	empty := `{"event":"CommunityGoal","timestamp":"2026-01-01T00:01:03Z","CurrentGoals":[]}`
	b := `{"event":"CommunityGoal","timestamp":"2026-01-01T00:01:02Z","CurrentGoals":[{"CGID":18446744073709551615,"Title":"B"}]}`
	for _, tc := range []struct {
		name       string
		events     []string
		ids        []string
		historical bool
	}{
		{"A then empty", []string{a, empty}, []string{}, true},
		{"AB then B", []string{ab, b}, []string{"18446744073709551615"}, true},
		{"older replay cannot resurrect", []string{ab, b, a}, []string{"18446744073709551615"}, true},
		{"older replay cannot erase", []string{empty, a}, []string{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := operationFixture(t, tc.events...)
			known, ids, states := goalView(t, s, "SYNTHETIC")
			if !known || fmt.Sprint(ids) != fmt.Sprint(tc.ids) {
				t.Fatalf("full membership snapshot lost: known=%v ids=%v want=%v", known, ids, tc.ids)
			}
			if tc.historical && states["9007199254740993"] != "historical_goal" {
				t.Fatal("removed A still looks current", states)
			}
			var count int
			if e := s.reader.QueryRow("SELECT count(*) FROM commander_operations WHERE category='goals' AND entity_id='9007199254740993'").Scan(&count); e != nil || count == 0 {
				t.Fatal("contribution history lost", count, e)
			}
		})
	}
}
func TestGoalMalformedSnapshotNeverClearsMembership(t *testing.T) {
	cases := []string{`{"Ignored":"absent-list"}`, `{"CurrentGoals":null}`, `{"CurrentGoals":{}}`, `{"CurrentGoals":"not-an-array"}`, `{"CurrentGoals":[null]}`, `{"CurrentGoals":[{}]}`, `{"CurrentGoals":[{"CGID":1},{"CGID":1}]}`, `{"CurrentGoals":[{"CGID":1,"IsComplete":"yes"}]}`}
	for i, extra := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			s := operationFixture(t, `{"event":"CommunityGoal","CurrentGoals":[{"CGID":9007199254740993}]}`, `{"event":"CommunityGoal",`+extra[1:])
			known, ids, _ := goalView(t, s, "SYNTHETIC")
			if !known || fmt.Sprint(ids) != "[9007199254740993]" {
				t.Fatal("invalid array changed membership", known, ids)
			}
		})
	}
	s := operationFixture(t, `{"event":"CommunityGoal","Title":"missing CurrentGoals"}`)
	known, _, _ := goalView(t, s, "SYNTHETIC")
	if known {
		t.Fatal("missing list became complete empty snapshot")
	}
}
func TestGoalSnapshotIsolationRestartAndExistingProfileRepair(t *testing.T) {
	s := operationFixture(t, `{"event":"CommunityGoal","CurrentGoals":[{"CGID":9007199254740993}]}`, `{"event":"CommunityGoalReward","CGID":9007199254740993,"Reward":100}`, `{"event":"CommunityGoalDiscard","CGID":9007199254740993}`, `{"event":"CommunityGoal","CurrentGoals":[]}`, `{"event":"LoadGame","FID":"OTHER"}`, `{"event":"CommunityGoal","CurrentGoals":[{"CGID":18446744073709551615}]}`)
	known, ids, _ := goalView(t, s, "SYNTHETIC")
	if !known || len(ids) != 0 {
		t.Fatal("other FID contaminated empty snapshot", known, ids)
	}
	known, ids, _ = goalView(t, s, "OTHER")
	if !known || fmt.Sprint(ids) != "[18446744073709551615]" {
		t.Fatal("other FID lost its snapshot", known, ids)
	}
	// Simulate the reviewed v6 profile: historical records exist, no complete marker.
	if _, e := s.writer.Exec("DELETE FROM commander_current WHERE category='goals' AND field='subscription_snapshot'; DELETE FROM operations_state WHERE key='cg-membership-v1'"); e != nil {
		t.Fatal(e)
	}
	dir := s.dir
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	known, ids, _ = goalView(t, reopened, "SYNTHETIC")
	if !known || len(ids) != 0 {
		t.Fatal("restart failed to recover explicitly empty snapshot", known, ids)
	}
	var count int
	reopened.reader.QueryRow("SELECT count(*) FROM commander_operations WHERE event_type IN ('CommunityGoalReward','CommunityGoalDiscard')").Scan(&count)
	if count != 2 {
		t.Fatal("reward/discard history lost", count)
	}
	if e = reopened.SetBuiltinModule("commander-operations", true); e != nil {
		t.Fatal(e)
	}
	known, ids, _ = goalView(t, reopened, "SYNTHETIC")
	if !known || len(ids) != 0 {
		t.Fatal("backfill resurrected removed goal", ids)
	}
}
func TestGoalExpiredSnapshotIsOnlyLastWitnessedMembership(t *testing.T) {
	s := operationFixture(t, `{"event":"CommunityGoal","CurrentGoals":[{"CGID":1,"Expiry":"2020-01-01T00:00:00Z","IsComplete":true,"Bonus":123}]}`)
	known, ids, states := goalView(t, s, "SYNTHETIC")
	if !known || len(ids) != 1 || states["1"] != "subscribed_as_last_witnessed" {
		t.Fatal("expiry inferred participation", known, ids, states)
	}
	var count int
	s.reader.QueryRow("SELECT count(*) FROM commander_current WHERE field='reward_received'").Scan(&count)
	if count != 0 {
		t.Fatal("completion/bonus invented a received reward")
	}
}

func TestGoalSnapshotRejectsOversizeAndPreservesEqualTimeEvidence(t *testing.T) {
	ids := make([]string, 2049)
	for i := range ids {
		ids[i] = fmt.Sprintf(`{"CGID":%d}`, i)
	}
	oversized := `{"event":"CommunityGoal","CurrentGoals":[` + strings.Join(ids, ",") + `]}`
	s := operationFixture(t, `{"event":"CommunityGoal","timestamp":"2026-01-01T00:01:01Z","CurrentGoals":[{"CGID":1}]}`, `{"event":"CommunityGoal","timestamp":"2026-01-01T00:01:01Z","CurrentGoals":[{"CGID":2}]}`, oversized)
	known, got, _ := goalView(t, s, "SYNTHETIC")
	if !known || fmt.Sprint(got) != "[1]" {
		t.Fatal("ambiguous/oversized snapshot replaced known membership", got)
	}
}
func TestGoalSnapshotRollbackMatchesCheckpoint(t *testing.T) {
	s := operationFixture(t, `{"event":"CommunityGoal","CurrentGoals":[{"CGID":1}]}`)
	source := journal.Source{ID: "ops", Identity: "synthetic"}
	cp, e := s.Resume(source)
	if e != nil {
		t.Fatal(e)
	}
	offset := cp.Offset
	raw := []byte(`{"event":"CommunityGoal","timestamp":"2026-01-01T00:01:05Z","CurrentGoals":[]}`)
	ev, e := journal.ParseLine(raw, journal.SourcePosition{FileID: source.ID, Sequence: cp.Sequence + 1, Offset: cp.Offset, Length: int64(len(raw) + 1)})
	if e != nil {
		t.Fatal(e)
	}
	cp.Sequence++
	cp.Offset += int64(len(raw) + 1)
	if _, e = s.writer.Exec(`CREATE TEMP TRIGGER fail_goal_commit BEFORE UPDATE ON journal_sources BEGIN SELECT RAISE(ABORT,'synthetic commit failure'); END`); e != nil {
		t.Fatal(e)
	}
	if e = s.Commit(context.Background(), cp, []journal.Record{{Event: ev, Phase: journal.Replay}}, nil); e == nil {
		t.Fatal("injected commit failure ignored")
	}
	known, ids, _ := goalView(t, s, "SYNTHETIC")
	if !known || fmt.Sprint(ids) != "[1]" {
		t.Fatal("failed commit changed membership", ids)
	}
	current, e := s.Resume(source)
	if e != nil || current.Offset != offset {
		t.Fatal("checkpoint advanced after rollback", current, e)
	}
}
