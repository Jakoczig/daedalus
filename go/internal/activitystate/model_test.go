package activitystate

import (
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"strings"
	"testing"
)

func event(t *testing.T, seq int, body string) journal.Event {
	t.Helper()
	e, err := journal.ParseLine([]byte(`{"timestamp":"2026-01-01T00:00:01Z",`+body+`}`), journal.SourcePosition{FileID: "file", Offset: int64(seq * 100), Sequence: uint64(seq)})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func apply(t *testing.T, m *Model, seq int, body string) {
	t.Helper()
	m.ApplyEvent("key", event(t, seq, body))
}
func load(t *testing.T, m *Model) {
	apply(t, m, 1, `"event":"LoadGame","FID":"private-fid","Commander":"SECRET"`)
}
func TestActivityProjection(t *testing.T) {
	m := NewModel()
	if m.View("key").Evidence != "unknown" {
		t.Fatal("initial evidence")
	}
	load(t, m)
	apply(t, m, 2, `"event":"Rank","Combat":3,"Trade":4,"Unexpected":8`)
	apply(t, m, 3, `"event":"Progress","Combat":30`)
	apply(t, m, 4, `"event":"FSDJump","StarSystem":"Test","JumpDist":12.5`)
	apply(t, m, 5, `"event":"MarketBuy","Count":4,"TotalCost":999`)
	v := m.View("key")
	if len(v.Commander.Ranks) != 2 || v.Commander.Session.Jumps != 1 || v.Commander.Session.DistanceLY != 12.5 || v.Commander.Session.BoughtUnits != 4 {
		t.Fatalf("facts missing %+v", v)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "private-fid") || strings.Contains(string(raw), "999") {
		t.Fatal("private data leaked")
	}
	*v.Commander.Ranks[0].Rank = 999
	if *m.View("key").Commander.Ranks[0].Rank == 999 {
		t.Fatal("alias")
	}
	m.ResetSource()
	if m.View("key").Evidence != "unknown" {
		t.Fatal("reset")
	}
}
func TestMissionExactIDAndSnapshot(t *testing.T) {
	m := NewModel()
	load(t, m)
	apply(t, m, 2, `"event":"MissionAccepted","MissionID":9007199254740992,"Name":"A"`)
	apply(t, m, 3, `"event":"MissionAccepted","MissionID":9007199254740993,"Name":"B","Count":2`)
	apply(t, m, 4, `"event":"MissionCompleted","MissionID":9007199254740992,"Reward":120`)
	v := m.View("key")
	if len(v.Missions.Active) != 1 || v.Missions.Active[0].Name != "B" || len(v.Missions.Recent) != 1 {
		t.Fatalf("IDs merged %+v", v.Missions)
	}
	apply(t, m, 5, `"event":"Missions","Active":[{"MissionID":9007199254740993},{"MissionID":3}],"Failed":[],"Complete":[]`)
	v = m.View("key")
	if !v.Missions.SnapshotKnown || len(v.Missions.Active) != 2 {
		t.Fatal(v.Missions)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "MissionID") || strings.Contains(string(raw), "9007199254740993") {
		t.Fatal("identifier exposed")
	}
	apply(t, m, 6, `"event":"Missions","Active":[],"Failed":[],"Complete":[]`)
	if len(m.View("key").Missions.Active) != 0 {
		t.Fatal("snapshot not authoritative")
	}
}
func TestMiningAndBarrier(t *testing.T) {
	m := NewModel()
	load(t, m)
	apply(t, m, 2, `"event":"Location","StarSystem":"Test"`)
	apply(t, m, 3, `"event":"ProspectedAsteroid","Content":"Low","Remaining":85,"MotherlodeMaterial":"opal","Materials":[{"Name":"platinum","Proportion":24.5}]`)
	apply(t, m, 4, `"event":"MiningRefined","Type":"platinum"`)
	v := m.View("key")
	if len(v.Mining.Prospects) != 1 || *v.Mining.Prospects[0].Materials[0].Proportion != 24.5 || v.Mining.Refined[0].Units != 1 {
		t.Fatal(v.Mining)
	}
	*v.Mining.Prospects[0].Remaining = 0
	v.Mining.Refined[0].Units = 999
	if *m.View("key").Mining.Prospects[0].Remaining != 85 {
		t.Fatal("alias")
	}
	apply(t, m, 6, `"event":"MiningRefined","Type":"platinum"`)
	if m.View("key").Evidence != "unverified" {
		t.Fatal("gap accepted")
	}
	apply(t, m, 7, `"event":"MiningRefined","Type":"platinum"`)
	if len(m.View("key").Mining.Refined) != 0 {
		t.Fatal("stale resurrection")
	}
	apply(t, m, 8, `"event":"LoadGame","FID":"private-fid"`)
	apply(t, m, 9, `"event":"MiningRefined","Type":"opal"`)
	if m.View("key").Mining.Refined[0].Name != "opal" {
		t.Fatal("fresh load did not restore")
	}
}
func TestBoundsAndMalformed(t *testing.T) {
	m := NewModel()
	load(t, m)
	for i := 2; i < 90; i++ {
		apply(t, m, i, fmt.Sprintf(`"event":"MissionAccepted","MissionID":%d,"Name":"Mission"`, i))
	}
	v := m.View("key")
	if len(v.Missions.Active) != 64 || !v.Missions.Truncated {
		t.Fatal("mission cap")
	}
	apply(t, m, 90, `"event":"MarketSell","Count":"20"`)
	if m.View("key").Commander.Session.SoldUnits != 0 {
		t.Fatal("coerced string")
	}
	apply(t, m, 91, `"event":"FSDJump","JumpDist":1e100`)
	if m.View("key").Commander.Session.DistanceLY != 0 {
		t.Fatal("unbounded number")
	}
}
func TestSourceSessionAndChronology(t *testing.T) {
	m := NewModel()
	load(t, m)
	apply(t, m, 2, `"event":"MissionAccepted","MissionID":1,"Name":"First"`)
	missing := event(t, 3, `"event":"MissionAccepted","MissionID":2`)
	missing.Timestamp = missing.Timestamp.Add(-1000000000)
	m.ApplyEvent("key", missing)
	if m.View("key").Evidence != "unverified" {
		t.Fatal("time reversal accepted")
	}
	apply(t, m, 4, `"event":"LoadGame","FID":"private-fid"`)
	apply(t, m, 5, `"event":"MissionAccepted","MissionID":2`)
	other := event(t, 6, `"event":"LoadGame","FID":"other-private"`)
	m.ApplyEvent("other", other)
	apply(t, m, 7, `"event":"MissionAccepted","MissionID":3`)
	if m.View("key").Evidence != "unverified" || len(m.View("other").Missions.Active) != 0 {
		t.Fatal("commander crossed")
	}
	m.ApplyEvent("other", event(t, 7, `"event":"Fileheader"`))
	m.ApplyEvent("other", event(t, 8, `"event":"MissionAccepted","MissionID":9`))
	if len(m.View("other").Missions.Active) != 0 {
		t.Fatal("fileheader barrier")
	}
}
func TestMiningBoundsAndDetach(t *testing.T) {
	m := NewModel()
	load(t, m)
	for i := 2; i < 72; i++ {
		apply(t, m, i, `"event":"ProspectedAsteroid","Materials":[{"Name":"ore","Proportion":12}],"Remaining":50`)
	}
	v := m.View("key")
	if len(v.Mining.Prospects) != 16 || !v.Mining.Truncated {
		t.Fatal("prospect cap")
	}
	c := Clone(v)
	*c.Mining.Prospects[0].Materials[0].Proportion = 99
	if *v.Mining.Prospects[0].Materials[0].Proportion != 12 {
		t.Fatal("clone material alias")
	}
	apply(t, m, 72, `"event":"StartJump"`)
	if len(m.View("key").Mining.Prospects) != 0 {
		t.Fatal("old system prospects")
	}
	for i := 73; i < 143; i++ {
		apply(t, m, i, fmt.Sprintf(`"event":"MiningRefined","Type":"ore%d"`, i))
	}
	v = m.View("key")
	if len(v.Mining.Refined) != 64 || len(v.Mining.RecentRefined) > 64 || !v.Mining.Truncated {
		t.Fatal("refined cap")
	}
}
func TestUnknownSnapshotAndMalformed(t *testing.T) {
	m := NewModel()
	load(t, m)
	if m.View("key").Missions.SnapshotKnown {
		t.Fatal("unknown asserted empty")
	}
	apply(t, m, 2, `"event":"Missions","Active":[{"MissionID":"12"}]`)
	if m.View("key").Missions.SnapshotKnown {
		t.Fatal("string ID accepted")
	}
	apply(t, m, 3, `"event":"ProspectedAsteroid","Materials":[{"Name":"bad\u0001","Proportion":"24"}]`)
	if len(m.View("key").Mining.Prospects[0].Materials) != 0 {
		t.Fatal("control character accepted")
	}
	bad := event(t, 4, `"event":"Rank"`)
	bad.Raw = []byte(`{} {}`)
	m.ApplyEvent("key", bad)
	if m.View("key").Evidence != "partial" {
		t.Fatal("trailing JSON")
	}
	for i := 0; i < 20; i++ {
		m.ApplyEvent(fmt.Sprint(i), event(t, 1, `"event":"LoadGame","FID":"bounded"`))
	}
	if len(m.states) > 8 {
		t.Fatal("commander cache unbounded")
	}
}
func TestEngineerSnapshotDeltaResetAndCopies(t *testing.T) {
	m := NewModel()
	load(t, m)
	apply(t, m, 2, `"event":"EngineerProgress","Engineers":[{"Engineer":"Felicity Farseer","EngineerID":123,"Progress":"Unlocked","Rank":4,"RankProgress":0.5},{"Engineer":"Elvira Martuuk","Progress":"Known"}]`)
	v := m.View("key")
	if len(v.Commander.Engineers) != 2 || *v.Commander.Engineers[0].Rank != 4 || *v.Commander.Engineers[0].RankProgress != 0.5 {
		t.Fatal(v.Commander)
	}
	*v.Commander.Engineers[0].Rank = 0
	c := Clone(m.View("key"))
	*c.Commander.Engineers[0].RankProgress = 0
	if *m.View("key").Commander.Engineers[0].Rank != 4 || *m.View("key").Commander.Engineers[0].RankProgress != 0.5 {
		t.Fatal("engineer alias")
	}
	apply(t, m, 3, `"event":"EngineerProgress","Engineer":"Elvira Martuuk","Progress":"Invited","Rank":2`)
	if len(m.View("key").Commander.Engineers) != 2 || m.View("key").Commander.Engineers[1].Progress != "Invited" {
		t.Fatal("delta replacement")
	}
	apply(t, m, 4, `"event":"EngineerProgress","Engineers":[{"Engineer":"Elvira Martuuk","Progress":"Unlocked","Rank":5}]`)
	if len(m.View("key").Commander.Engineers) != 1 {
		t.Fatal("snapshot merge instead of replacement")
	}
	raw, _ := json.Marshal(m.View("key"))
	if strings.Contains(string(raw), "EngineerID") {
		t.Fatal("ID leaked")
	}
	apply(t, m, 5, `"event":"LoadGame","FID":"other"`)
	if len(m.View("key").Commander.Engineers) != 0 {
		t.Fatal("new commander inherited engineers")
	}
}
func TestEngineerMalformedAndBounds(t *testing.T) {
	m := NewModel()
	load(t, m)
	apply(t, m, 2, `"event":"EngineerProgress","Engineer":"Felicity Farseer","Progress":"Unknown future state","Rank":6,"RankProgress":"0.5"`)
	v := m.View("key")
	if len(v.Commander.Engineers) != 1 || v.Commander.Engineers[0].Rank != nil || v.Commander.Engineers[0].RankProgress != nil || v.Commander.Engineers[0].Progress != "" || v.Commander.Evidence != "partial" {
		t.Fatal(v.Commander)
	}
	apply(t, m, 3, `"event":"EngineerProgress","Engineer":"Felicity Farseer","Rank":3`)
	v = m.View("key")
	if *v.Commander.Engineers[0].Rank != 3 {
		t.Fatal("valid later delta")
	}
	noTime := event(t, 4, `"event":"EngineerProgress","Engineer":"Elvira Martuuk","Progress":"Known"`)
	noTime.Timestamp = journal.Event{}.Timestamp
	m.ApplyEvent("key", noTime)
	if len(m.View("key").Commander.Engineers) != 1 {
		t.Fatal("missing time accepted")
	}
	for i := 5; i < 75; i++ {
		apply(t, m, i, fmt.Sprintf(`"event":"EngineerProgress","Engineer":"NPC %d","Progress":"Known"`, i))
	}
	if len(m.View("key").Commander.Engineers) != 64 || m.View("key").Commander.Evidence != "partial" {
		t.Fatal("engineer cap")
	}
	apply(t, m, 75, `"event":"EngineerProgress","Engineers":[]`)
	if len(m.View("key").Commander.Engineers) != 0 {
		t.Fatal("explicit empty snapshot")
	}
	apply(t, m, 76, `"event":"EngineerProgress","Engineers":"bad"`)
	if m.View("key").Commander.Evidence != "partial" {
		t.Fatal("malformed snapshot")
	}
}
