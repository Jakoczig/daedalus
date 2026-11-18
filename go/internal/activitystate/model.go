// Package activitystate projects bounded, explicit journal activity facts. The
// caller serializes access. This package has no IO, clock or game commands.
package activitystate

import (
	"bytes"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type View struct {
	Evidence  string        `json:"evidence"`
	Commander CommanderView `json:"commander"`
	Missions  MissionView   `json:"missions"`
	Mining    MiningView    `json:"mining"`
}
type RankView struct {
	Category string   `json:"category"`
	Rank     *float64 `json:"rank"`
	Progress *float64 `json:"progress"`
}
type ReputationView struct {
	Faction string  `json:"faction"`
	Value   float64 `json:"value"`
}
type EngineerProgressView struct {
	Name         string   `json:"name"`
	Progress     string   `json:"progress"`
	Rank         *float64 `json:"rank"`
	RankProgress *float64 `json:"rankProgress"`
}
type SessionView struct {
	StartedAt   string  `json:"startedAt"`
	LastEventAt string  `json:"lastEventAt"`
	GameState   string  `json:"gameState"`
	Jumps       uint64  `json:"jumps"`
	DistanceLY  float64 `json:"distanceLY"`
	BoughtUnits uint64  `json:"boughtUnits"`
	SoldUnits   uint64  `json:"soldUnits"`
}
type CommanderView struct {
	Evidence   string                 `json:"evidence"`
	Ranks      []RankView             `json:"ranks"`
	Reputation []ReputationView       `json:"reputation"`
	Engineers  []EngineerProgressView `json:"engineers"`
	Session    SessionView            `json:"session"`
}
type MissionCard struct {
	Name              string   `json:"name"`
	LocalizedName     string   `json:"localizedName"`
	Faction           string   `json:"faction"`
	DestinationSystem string   `json:"destinationSystem"`
	Commodity         string   `json:"commodity"`
	Count             *float64 `json:"count"`
	Progress          *float64 `json:"progress"`
	Reward            *float64 `json:"reward"`
	Expiry            string   `json:"expiry"`
	State             string   `json:"state"`
}
type MissionView struct {
	Evidence      string        `json:"evidence"`
	SnapshotKnown bool          `json:"snapshotKnown"`
	Truncated     bool          `json:"truncated"`
	Active        []MissionCard `json:"active"`
	Recent        []MissionCard `json:"recent"`
}
type MaterialView struct {
	Name          string   `json:"name"`
	LocalizedName string   `json:"localizedName"`
	Proportion    *float64 `json:"proportion"`
}
type ProspectView struct {
	At               string         `json:"at"`
	Content          string         `json:"content"`
	ContentLocalized string         `json:"contentLocalized"`
	Remaining        *float64       `json:"remaining"`
	Core             string         `json:"core"`
	Materials        []MaterialView `json:"materials"`
}
type RefinedView struct {
	Name          string `json:"name"`
	LocalizedName string `json:"localizedName"`
	Units         uint64 `json:"units"`
}
type RefinedEvent struct {
	At            string `json:"at"`
	Name          string `json:"name"`
	LocalizedName string `json:"localizedName"`
}
type MiningView struct {
	Evidence        string         `json:"evidence"`
	System          string         `json:"system"`
	StartedAt       string         `json:"startedAt"`
	LastEventAt     string         `json:"lastEventAt"`
	ElapsedSeconds  float64        `json:"elapsedSeconds"`
	Prospected      uint64         `json:"prospected"`
	Cracked         uint64         `json:"cracked"`
	LimpetsLaunched uint64         `json:"limpetsLaunched"`
	Truncated       bool           `json:"truncated"`
	Prospects       []ProspectView `json:"prospects"`
	Refined         []RefinedView  `json:"refined"`
	RecentRefined   []RefinedEvent `json:"recentRefined"`
}
type state struct {
	v               View
	verified        bool
	file, path, fid string
	offset          int64
	sequence        uint64
	last            time.Time
	missions        map[string]MissionCard
	missionOrder    []string
	terminal        map[string]bool
}
type Model struct {
	states map[string]*state
	order  []string
	active string
}

func NewModel() *Model        { m := &Model{}; m.ResetSource(); return m }
func (m *Model) ResetSource() { m.states = map[string]*state{}; m.order = nil; m.active = "" }
func empty(evidence string) View {
	return View{Evidence: evidence, Commander: CommanderView{Evidence: evidence, Ranks: []RankView{}, Reputation: []ReputationView{}, Engineers: []EngineerProgressView{}}, Missions: MissionView{Evidence: evidence, Active: []MissionCard{}, Recent: []MissionCard{}}, Mining: MiningView{Evidence: evidence, Prospects: []ProspectView{}, Refined: []RefinedView{}, RecentRefined: []RefinedEvent{}}}
}
func ptr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func cloneCard(c MissionCard) MissionCard {
	c.Count = ptr(c.Count)
	c.Progress = ptr(c.Progress)
	c.Reward = ptr(c.Reward)
	return c
}
func Clone(v View) View {
	v.Commander.Ranks = append([]RankView{}, v.Commander.Ranks...)
	for i := range v.Commander.Ranks {
		v.Commander.Ranks[i].Rank = ptr(v.Commander.Ranks[i].Rank)
		v.Commander.Ranks[i].Progress = ptr(v.Commander.Ranks[i].Progress)
	}
	v.Commander.Reputation = append([]ReputationView{}, v.Commander.Reputation...)
	v.Commander.Engineers = append([]EngineerProgressView{}, v.Commander.Engineers...)
	for i := range v.Commander.Engineers {
		v.Commander.Engineers[i].Rank = ptr(v.Commander.Engineers[i].Rank)
		v.Commander.Engineers[i].RankProgress = ptr(v.Commander.Engineers[i].RankProgress)
	}
	v.Missions.Active = append([]MissionCard{}, v.Missions.Active...)
	for i := range v.Missions.Active {
		v.Missions.Active[i] = cloneCard(v.Missions.Active[i])
	}
	v.Missions.Recent = append([]MissionCard{}, v.Missions.Recent...)
	for i := range v.Missions.Recent {
		v.Missions.Recent[i] = cloneCard(v.Missions.Recent[i])
	}
	v.Mining.Prospects = append([]ProspectView{}, v.Mining.Prospects...)
	for i := range v.Mining.Prospects {
		p := &v.Mining.Prospects[i]
		p.Remaining = ptr(p.Remaining)
		p.Materials = append([]MaterialView{}, p.Materials...)
		for j := range p.Materials {
			p.Materials[j].Proportion = ptr(p.Materials[j].Proportion)
		}
	}
	v.Mining.Refined = append([]RefinedView{}, v.Mining.Refined...)
	v.Mining.RecentRefined = append([]RefinedEvent{}, v.Mining.RecentRefined...)
	return v
}
func (m *Model) View(key string) View {
	if s := m.states[key]; s != nil {
		return Clone(s.v)
	}
	return empty("unknown")
}
func clean(v any) string {
	s, ok := v.(string)
	if !ok || !utf8.ValidString(s) || len(s) > 256 {
		return ""
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return strings.TrimSpace(s)
}
func num(v any, min, max float64) *float64 {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	f, e := n.Float64()
	if e != nil || math.IsInf(f, 0) || math.IsNaN(f) || f < min || f > max {
		return nil
	}
	return &f
}
func integer(v any, max float64) *float64 {
	f := num(v, 0, max)
	if f != nil && math.Trunc(*f) != *f {
		return nil
	}
	return f
}
func id(v any) string {
	n, ok := v.(json.Number)
	if !ok || strings.ContainsAny(string(n), "+-.eE") {
		return ""
	}
	u, e := strconv.ParseUint(string(n), 10, 64)
	if e != nil || u == 0 {
		return ""
	}
	return strconv.FormatUint(u, 10)
}
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func bump(p *uint64, n uint64) {
	const limit = uint64(1e12)
	if *p >= limit || n > limit-*p {
		*p = limit
	} else {
		*p += n
	}
}
func (s *state) invalidate() {
	s.verified = false
	s.v = empty("unverified")
	s.missions = nil
	s.missionOrder = nil
	s.terminal = nil
}
func (m *Model) ApplyEvent(key string, e journal.Event) {
	if key == "" {
		return
	}
	s := m.states[key]
	if s == nil {
		if len(m.order) >= 8 {
			delete(m.states, m.order[0])
			m.order = m.order[1:]
		}
		s = &state{v: empty("unknown")}
		m.states[key] = s
		m.order = append(m.order, key)
	}
	if e.Type == "Fileheader" {
		s.invalidate()
		s.v = empty("unknown")
		if m.active == key {
			m.active = ""
		}
		return
	}
	if e.Type == "LoadGame" {
		if s.file != "" && e.Position.FileID == s.file && e.Position.Path == s.path && e.Position.Offset <= s.offset {
			s.invalidate()
			return
		}
		// New explicit identity/session is the sole recovery boundary.
		if m.active != "" && m.active != key {
			if old := m.states[m.active]; old != nil {
				old.invalidate()
			}
		}
		*s = state{v: empty("unknown"), verified: e.FID != "" && e.Position.FileID != "", file: e.Position.FileID, path: e.Position.Path, fid: e.FID, offset: e.Position.Offset, sequence: e.Position.Sequence, last: e.Timestamp, missions: map[string]MissionCard{}, terminal: map[string]bool{}}
		m.active = key
		if !s.verified {
			return
		}
		s.v.Evidence = "journal_last_known"
		s.v.Commander.Evidence = "journal_last_known"
		s.v.Commander.Session = SessionView{StartedAt: stamp(e.Timestamp), LastEventAt: stamp(e.Timestamp), GameState: "loaded"}
		s.v.Mining.StartedAt = stamp(e.Timestamp)
		return
	}
	if !s.verified || m.active != key {
		return
	}
	if e.Position.FileID == "" || e.Position.FileID != s.file || e.Position.Path != s.path || e.Position.Offset <= s.offset || ((s.sequence != 0 || e.Position.Sequence != 0) && e.Position.Sequence != s.sequence+1) || (!e.Timestamp.IsZero() && !s.last.IsZero() && e.Timestamp.Before(s.last)) {
		s.invalidate()
		return
	}
	s.offset = e.Position.Offset
	s.sequence = e.Position.Sequence
	if e.FID != "" && e.FID != s.fid {
		s.invalidate()
		return
	}
	if !e.Timestamp.IsZero() {
		s.last = e.Timestamp
		s.v.Commander.Session.LastEventAt = stamp(e.Timestamp)
	}
	if len(e.Raw) > journal.MaxRecordBytes {
		s.v.Evidence = "partial"
		return
	}
	d := json.NewDecoder(bytes.NewReader(e.Raw))
	d.UseNumber()
	var data map[string]any
	if d.Decode(&data) != nil || data == nil || d.Decode(new(any)) != io.EOF {
		s.v.Evidence = "partial"
		return
	}
	s.commander(e, data)
	switch e.Type {
	case "Missions", "MissionAccepted", "MissionCompleted", "MissionFailed", "MissionAbandoned", "MissionRedirected", "CargoDepot":
		if e.Timestamp.IsZero() {
			s.v.Missions.Evidence = "partial"
		} else {
			s.mission(e, data)
		}
	}
	switch e.Type {
	case "Location", "FSDJump", "StartJump":
		s.v.Mining.Prospects = []ProspectView{}
		s.v.Mining.System = ""
		if e.Type != "StartJump" {
			s.v.Mining.System = clean(data["StarSystem"])
		}
	case "ProspectedAsteroid", "MiningRefined", "AsteroidCracked", "LaunchDrone":
		if e.Timestamp.IsZero() {
			s.v.Mining.Evidence = "partial"
		} else {
			s.mining(e, data)
		}
	case "Shutdown":
		s.v.Commander.Session.GameState = "shutdown"
		s.verified = false
		s.v.Evidence = "waiting_game"
	}
}
func (s *state) commander(e journal.Event, d map[string]any) {
	c := &s.v.Commander
	switch e.Type {
	case "EngineerProgress":
		if e.Timestamp.IsZero() {
			c.Evidence = "partial"
			return
		}
		s.engineers(d)
	case "Rank", "Progress":
		for _, category := range []string{"Combat", "Trade", "Explore", "Empire", "Federation", "CQC", "Soldier", "Exobiologist"} {
			raw, ok := d[category]
			if !ok {
				continue
			}
			max := float64(100)
			if e.Type == "Rank" {
				max = 30
			}
			n := integer(raw, max)
			if n == nil {
				c.Evidence = "partial"
				continue
			}
			i := 0
			for i < len(c.Ranks) && c.Ranks[i].Category != category {
				i++
			}
			if i == len(c.Ranks) {
				c.Ranks = append(c.Ranks, RankView{Category: category})
			}
			if e.Type == "Rank" {
				c.Ranks[i].Rank = n
			} else {
				c.Ranks[i].Progress = n
			}
		}
		sort.Slice(c.Ranks, func(i, j int) bool { return c.Ranks[i].Category < c.Ranks[j].Category })
	case "Reputation":
		for _, f := range []string{"Empire", "Federation", "Alliance"} {
			raw, ok := d[f]
			if !ok {
				continue
			}
			n := num(raw, -100, 100)
			if n == nil {
				c.Evidence = "partial"
				continue
			}
			i := 0
			for i < len(c.Reputation) && c.Reputation[i].Faction != f {
				i++
			}
			if i == len(c.Reputation) {
				c.Reputation = append(c.Reputation, ReputationView{Faction: f})
			}
			c.Reputation[i].Value = *n
		}
	case "FSDJump":
		bump(&c.Session.Jumps, 1)
		if n := num(d["JumpDist"], 0, 1e6); n != nil {
			c.Session.DistanceLY = math.Min(1e12, c.Session.DistanceLY+*n)
		} else if _, ok := d["JumpDist"]; ok {
			c.Evidence = "partial"
		}
	case "MarketBuy", "MarketSell":
		if n := integer(d["Count"], 1e9); n != nil {
			if e.Type == "MarketBuy" {
				bump(&c.Session.BoughtUnits, uint64(*n))
			} else {
				bump(&c.Session.SoldUnits, uint64(*n))
			}
		} else {
			c.Evidence = "partial"
		}
	case "Music":
		switch clean(d["MusicTrack"]) {
		case "MainMenu":
			c.Session.GameState = "main_menu"
		case "NoTrack":
			c.Session.GameState = "in_game"
		}
	}
}
func missionCard(d map[string]any, old MissionCard) MissionCard {
	for k, p := range map[string]*string{"Name": &old.Name, "LocalisedName": &old.LocalizedName, "Name_Localised": &old.LocalizedName, "Faction": &old.Faction, "DestinationSystem": &old.DestinationSystem, "Commodity": &old.Commodity} {
		if raw, ok := d[k]; ok {
			*p = clean(raw)
		}
	}
	for k, p := range map[string]**float64{"Count": &old.Count, "Progress": &old.Progress, "Reward": &old.Reward} {
		if raw, ok := d[k]; ok {
			*p = integer(raw, 1e12)
		}
	}
	if raw, ok := d["Expiry"]; ok {
		old.Expiry = ""
		if t, e := time.Parse(time.RFC3339Nano, clean(raw)); e == nil {
			old.Expiry = stamp(t)
		}
	}
	return old
}
func (s *state) publishMissions() {
	s.v.Missions.Active = []MissionCard{}
	for _, k := range s.missionOrder {
		if c, ok := s.missions[k]; ok {
			s.v.Missions.Active = append(s.v.Missions.Active, c)
		}
	}
}
func (s *state) mission(e journal.Event, d map[string]any) {
	v := &s.v.Missions
	if e.Type == "Missions" {
		active, ok := d["Active"].([]any)
		if !ok || len(active) > 4096 {
			v.Evidence = "partial"
			return
		}
		next := map[string]MissionCard{}
		order := []string{}
		valid := true
		truncated := false
		for _, raw := range active {
			obj, ok := raw.(map[string]any)
			if !ok {
				valid = false
				continue
			}
			k := id(obj["MissionID"])
			if k == "" {
				valid = false
				continue
			}
			if _, ok := next[k]; ok {
				continue
			}
			if len(next) >= 64 {
				truncated = true
				continue
			}
			c := s.missions[k]
			c.State = "active"
			next[k] = c
			order = append(order, k)
		}
		if !valid {
			v.Evidence = "partial"
			return
		}
		s.missions = next
		s.missionOrder = order
		v.SnapshotKnown = true
		v.Truncated = truncated
		v.Evidence = "journal_last_known"
		if truncated {
			v.Evidence = "partial"
		}
		s.publishMissions()
		return
	}
	k := id(d["MissionID"])
	if k == "" {
		v.Evidence = "partial"
		return
	}
	c, exists := s.missions[k]
	if e.Type == "MissionAccepted" {
		if s.terminal[k] {
			return
		}
		if !exists {
			if len(s.missions) >= 64 {
				v.Truncated = true
				v.Evidence = "partial"
				return
			}
			s.missionOrder = append(s.missionOrder, k)
		}
		c = missionCard(d, c)
		c.State = "active"
		s.missions[k] = c
	} else if e.Type == "MissionRedirected" || e.Type == "CargoDepot" {
		if !exists {
			return
		}
		c = missionCard(d, c)
		s.missions[k] = c
	} else {
		if s.terminal[k] {
			return
		}
		c = missionCard(d, c)
		switch e.Type {
		case "MissionCompleted":
			c.State = "completed"
		case "MissionFailed":
			c.State = "failed"
		case "MissionAbandoned":
			c.State = "abandoned"
		}
		delete(s.missions, k)
		for i, x := range s.missionOrder {
			if x == k {
				s.missionOrder = append(s.missionOrder[:i], s.missionOrder[i+1:]...)
				break
			}
		}
		// A bounded terminal barrier prevents accepted replay from reviving recently closed IDs.
		if len(s.terminal) >= 128 {
			s.invalidate()
			return
		}
		s.terminal[k] = true
		v.Recent = append([]MissionCard{c}, v.Recent...)
		if len(v.Recent) > 16 {
			v.Recent = v.Recent[:16]
		}
	}
	if v.Evidence != "partial" {
		v.Evidence = "journal_last_known"
	}
	s.publishMissions()
}
func (s *state) mining(e journal.Event, d map[string]any) {
	v := &s.v.Mining
	v.Evidence = "journal_last_known"
	v.LastEventAt = stamp(e.Timestamp)
	if v.StartedAt != "" {
		start, err := time.Parse(time.RFC3339Nano, v.StartedAt)
		if err == nil && !e.Timestamp.Before(start) {
			v.ElapsedSeconds = e.Timestamp.Sub(start).Seconds()
		}
	}
	switch e.Type {
	case "ProspectedAsteroid":
		bump(&v.Prospected, 1)
		p := ProspectView{At: stamp(e.Timestamp), Content: clean(d["Content"]), ContentLocalized: clean(d["Content_Localised"]), Remaining: num(d["Remaining"], 0, 100), Core: clean(d["MotherlodeMaterial"]), Materials: []MaterialView{}}
		if arr, ok := d["Materials"].([]any); ok {
			for i, raw := range arr {
				if i >= 64 {
					v.Truncated = true
					break
				}
				obj, ok := raw.(map[string]any)
				if !ok {
					v.Evidence = "partial"
					continue
				}
				n := clean(obj["Name"])
				if n == "" {
					v.Evidence = "partial"
					continue
				}
				p.Materials = append(p.Materials, MaterialView{Name: n, LocalizedName: clean(obj["Name_Localised"]), Proportion: num(obj["Proportion"], 0, 100)})
			}
		} else {
			v.Evidence = "partial"
		}
		v.Prospects = append([]ProspectView{p}, v.Prospects...)
		if len(v.Prospects) > 16 {
			v.Prospects = v.Prospects[:16]
			v.Truncated = true
		}
	case "MiningRefined":
		name := clean(d["Type"])
		if name == "" {
			v.Evidence = "partial"
			return
		}
		label := clean(d["Type_Localised"])
		i := 0
		for i < len(v.Refined) && v.Refined[i].Name != name {
			i++
		}
		if i == len(v.Refined) {
			if i >= 64 {
				v.Truncated = true
				return
			}
			v.Refined = append(v.Refined, RefinedView{Name: name, LocalizedName: label})
		}
		bump(&v.Refined[i].Units, 1)
		v.RecentRefined = append([]RefinedEvent{{At: stamp(e.Timestamp), Name: name, LocalizedName: label}}, v.RecentRefined...)
		if len(v.RecentRefined) > 64 {
			v.RecentRefined = v.RecentRefined[:64]
			v.Truncated = true
		}
	case "AsteroidCracked":
		bump(&v.Cracked, 1)
	case "LaunchDrone":
		bump(&v.LimpetsLaunched, 1)
	}
}

// EngineerProgress arrays replace the observed roster; individual events update
// only the NPC named by Engineer. EngineerID and all other fields stay private.
func (s *state) engineers(d map[string]any) {
	c := &s.v.Commander
	records := []any{d}
	snapshot := false
	if raw, present := d["Engineers"]; present {
		var ok bool
		records, ok = raw.([]any)
		if !ok || len(records) > 4096 {
			c.Evidence = "partial"
			return
		}
		snapshot = true
	}
	next := c.Engineers
	if snapshot {
		next = []EngineerProgressView{}
	}
	for _, raw := range records {
		obj, ok := raw.(map[string]any)
		if !ok {
			c.Evidence = "partial"
			continue
		}
		name := clean(obj["Engineer"])
		if name == "" {
			c.Evidence = "partial"
			continue
		}
		index := 0
		for index < len(next) && next[index].Name != name {
			index++
		}
		if index == len(next) {
			if len(next) >= 64 {
				c.Evidence = "partial"
				continue
			}
			next = append(next, EngineerProgressView{Name: name})
		}
		v := &next[index]
		if raw, present := obj["Progress"]; present {
			v.Progress = ""
			switch p := clean(raw); p {
			case "Known", "Invited", "Unlocked":
				v.Progress = p
			default:
				c.Evidence = "partial"
			}
		}
		if raw, present := obj["Rank"]; present {
			v.Rank = integer(raw, 5)
			if v.Rank == nil {
				c.Evidence = "partial"
			}
		}
		if raw, present := obj["RankProgress"]; present {
			v.RankProgress = num(raw, 0, 1)
			if v.RankProgress == nil {
				c.Evidence = "partial"
			}
		}
	}
	c.Engineers = next
}
