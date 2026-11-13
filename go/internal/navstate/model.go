// Package navstate projects journal-attested plotted routes. No file IO or network.
package navstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/sidecar"
)

const MaxHops = 2048

type Hop struct {
	Name                   string   `json:"name"`
	StarClass              string   `json:"starClass"`
	DistanceFromPreviousLY *float64 `json:"distanceFromPreviousLY"`
	IsCurrent              bool     `json:"isCurrent"`
	IsNext                 bool     `json:"isNext"`
}
type NavigationView struct {
	CurrentSystem          string `json:"currentSystem"`
	TargetSystem           string `json:"targetSystem"`
	DestinationSystem      string `json:"destinationSystem"`
	RouteState             string `json:"routeState"`
	TargetEvidence         string `json:"targetEvidence"`
	RemainingJumps         *int   `json:"remainingJumps"`
	ReportedRemainingJumps *int   `json:"reportedRemainingJumps"`
	Hops                   []Hop  `json:"hops"`
	Truncated              bool   `json:"truncated"`
}
type routeHop struct {
	name, class, address string
	pos                  [3]float64
}
type route struct {
	stamp       time.Time
	fingerprint [sha256.Size]byte
	ambiguous   bool
	hops        []routeHop
	owner       string
	epoch       uint64
}
type sessionState struct {
	verified                                               bool
	epoch                                                  uint64
	reset, marker, barrier                                 time.Time
	route                                                  *route
	state                                                  string
	currentName, currentAddress, targetName, targetAddress string
	arrival                                                time.Time
	reported                                               *int
	markerLive, targetLive                                 bool
	sourceFile                                             string
	sequence                                               uint64
	clearUnknown                                           bool
}

// Model is owned by the enclosing service's mutex. Returned views own all memory.
type Model struct {
	sessions       map[string]*sessionState
	order          []string
	active         string
	candidate      *route
	sourceValid    bool
	overflow       bool
	serial         uint64
	claimedThrough time.Time
}

func NewModel() *Model { m := &Model{}; m.ResetSource(); return m }
func (m *Model) ResetSource() {
	m.sessions = map[string]*sessionState{}
	m.order = nil
	m.active = ""
	m.candidate = nil
	m.sourceValid = false
	m.overflow = false
	m.claimedThrough = time.Time{}
}
func (m *Model) state(key string) *sessionState {
	if s := m.sessions[key]; s != nil {
		return s
	}
	if len(m.order) >= 64 {
		delete(m.sessions, m.order[0])
		m.order = m.order[1:]
	}
	s := &sessionState{state: "no_route"}
	m.sessions[key] = s
	m.order = append(m.order, key)
	return s
}
func (m *Model) ApplyEvent(key string, e journal.Event) { m.ApplyEventPhase(key, e, journal.Live) }

// Explicit phase keeps replayed evidence last-known, never a fresh game target.
func (m *Model) ApplyEventPhase(key string, e journal.Event, phase journal.Phase) {
	m.active = key
	s := m.state(key)
	// Completed records rejected by the reader still consume a sequence number.
	// Their category/identity is unknowable: do not bridge that source gap.
	if e.Position.Sequence != 0 {
		if s.sequence != 0 && (e.Position.FileID != s.sourceFile || e.Position.Sequence != s.sequence+1) {
			s.verified = false
			s.route = nil
			s.marker = time.Time{}
			s.currentName = ""
			s.currentAddress = ""
			s.targetName = ""
			s.targetAddress = ""
			s.reported = nil
			s.state = "unverified"
		}
		defer func() { s.sequence = e.Position.Sequence; s.sourceFile = e.Position.FileID }()
	}
	switch e.Type {
	case "Shutdown":
		s.verified = false
		s.targetName = ""
		s.targetAddress = ""
		s.reported = nil
		s.state = "waiting_game"
		s.route = nil
	case "LoadGame", "Fileheader":
		m.serial++
		*s = sessionState{epoch: m.serial, reset: e.Timestamp, state: "unverified", verified: e.Type == "LoadGame" && key != "" && e.FID != "" && !e.Timestamp.IsZero()}
	case "NavRouteClear":
		s.clearUnknown = e.Timestamp.IsZero()
		s.targetName = ""
		s.targetAddress = ""
		s.reported = nil
		s.route = nil
		s.marker = time.Time{}
		s.state = "cleared"
		if e.Timestamp.After(s.barrier) {
			s.barrier = e.Timestamp
		}
	case "NavRoute":
		if !s.verified || s.clearUnknown || e.Timestamp.IsZero() || !e.Timestamp.After(s.reset) || !e.Timestamp.After(s.barrier) {
			return
		}
		s.marker = e.Timestamp
		s.markerLive = phase == journal.Live
		s.state = "unverified"
		s.route = nil
		m.correlate(key, s)
	case "Location", "FSDJump", "CarrierJump":
		s.targetName = ""
		s.targetAddress = ""
		s.reported = nil
		if !safeText(e.SystemName) || e.SystemAddress == "" {
			s.currentName = ""
			s.currentAddress = ""
			return
		}
		s.currentName = e.SystemName
		s.currentAddress = e.SystemAddress
		s.arrival = e.Timestamp
		s.targetName = ""
		s.targetAddress = ""
		s.reported = nil
	case "StartJump":
		// A jump is pending, not arrived. An existing FSDTarget stays distinct.
		if s.targetName != "" {
			s.targetLive = false
		}
	case "FSDTarget":
		if !s.verified || e.Timestamp.IsZero() || !e.Timestamp.After(s.reset) || e.Timestamp.Before(s.arrival) {
			return
		}
		var target struct {
			Name                  string
			SystemAddress         json.Number
			RemainingJumpsInRoute *int
		}
		d := json.NewDecoder(bytes.NewReader(e.Raw))
		d.UseNumber()
		if d.Decode(&target) != nil || !safeText(target.Name) {
			return
		}
		address, err := exactID(target.SystemAddress)
		if err != nil {
			return
		}
		s.targetName = target.Name
		s.targetAddress = address
		s.targetLive = phase == journal.Live
		s.reported = nil
		if target.RemainingJumpsInRoute != nil && *target.RemainingJumpsInRoute >= 0 && *target.RemainingJumpsInRoute <= MaxHops {
			n := *target.RemainingJumpsInRoute
			s.reported = &n
		}

	}
}

// Correlation is exact UTC timestamp equality, strictly after this session's
// LoadGame and latest clear. A candidate is claimed by one session epoch only.
// Both file-before-marker and marker-before-file are supported without guessing.
func (m *Model) correlate(key string, s *sessionState) {
	c := m.candidate
	if !s.verified || s.clearUnknown || !m.sourceValid || c == nil || c.ambiguous || !c.stamp.Equal(s.marker) || !c.stamp.After(s.reset) || !c.stamp.After(s.barrier) {
		return
	}
	if c.owner != "" && (c.owner != key || c.epoch != s.epoch) {
		return
	}
	c.owner = key
	c.epoch = s.epoch
	if c.stamp.After(m.claimedThrough) {
		m.claimedThrough = c.stamp
	}
	s.route = c
	s.state = "verified"
}
func (m *Model) ApplySidecar(c sidecar.Change) {
	if c.Kind != "NavRoute" {
		return
	}
	if !c.Valid {
		m.sourceValid = false
		return
	}
	r, overflow, err := parseRoute(c)
	continuous := m.sourceValid
	m.sourceValid = err == nil
	m.overflow = overflow
	if err != nil {
		return
	}
	r.fingerprint = sha256.Sum256(c.Raw)
	// Only an uninterrupted observation of the exact bytes retains its claim.
	// A journal timestamp carries no content hash or within-second ordering.
	// Once claimed (or ambiguous), that timestamp cannot attest replacement
	// content, even after a bad write, an identical repair or a duplicate marker.
	if previous := m.candidate; previous != nil {
		if continuous && previous.fingerprint == r.fingerprint {
			r = previous
		} else if previous.stamp.Equal(r.stamp) {
			r.ambiguous = true
		}
	}
	if r != m.candidate && !r.stamp.After(m.claimedThrough) {
		r.ambiguous = true
	}
	m.candidate = r
	if s := m.sessions[m.active]; s != nil {
		m.correlate(m.active, s)
	}
}
func (m *Model) View(key string) NavigationView {
	v := NavigationView{RouteState: "no_route", TargetEvidence: "unknown", Hops: []Hop{}}
	s := m.sessions[key]
	if s == nil {
		return v
	}
	v.CurrentSystem = s.currentName
	v.RouteState = s.state
	v.Truncated = m.overflow
	if s.targetName != "" {
		v.TargetSystem = s.targetName
		v.TargetEvidence = "last_known_target"
		if s.targetLive {
			v.TargetEvidence = "journal_target"
		}
		if s.reported != nil {
			n := *s.reported
			v.ReportedRemainingJumps = &n
		}
	}
	if s.state == "cleared" || s.state == "waiting_game" {
		return v
	}
	if m.sourceValid && m.candidate != nil && m.candidate.ambiguous {
		v.RouteState = "unverified"
		return v
	}
	if s.route == nil {
		if m.candidate != nil {
			v.RouteState = "unverified"
		}
		return v
	}
	if !m.sourceValid || s.route != m.candidate {
		v.RouteState = "stale"
		return v
	}
	if !s.markerLive {
		v.RouteState = "verified_last_known"
	}
	index := -1
	matches := 0
	for i, h := range s.route.hops {
		if h.address == s.currentAddress {
			index = i
			matches++
		}
	}
	if matches != 1 {
		index = -1
	}
	for i, h := range s.route.hops {
		hop := Hop{Name: h.name, StarClass: h.class, IsCurrent: i == index}
		if i > 0 {
			p := s.route.hops[i-1].pos
			d := math.Sqrt(math.Pow(h.pos[0]-p[0], 2) + math.Pow(h.pos[1]-p[1], 2) + math.Pow(h.pos[2]-p[2], 2))
			hop.DistanceFromPreviousLY = &d
		}
		v.Hops = append(v.Hops, hop)
	}
	if len(v.Hops) > 0 {
		v.DestinationSystem = v.Hops[len(v.Hops)-1].Name
	}
	if index >= 0 {
		n := len(v.Hops) - index - 1
		v.RemainingJumps = &n
	}
	next := index + 1
	if index >= 0 && next < len(v.Hops) {
		expected := s.route.hops[next]
		if s.targetName == "" {
			v.TargetSystem = expected.name
			v.TargetEvidence = "verified_route_next"
			v.Hops[next].IsNext = true
		} else if s.targetAddress == expected.address {
			v.Hops[next].IsNext = true
		} else {
			v.TargetEvidence = "route_conflict"
		}
	} else if s.targetName != "" && len(v.Hops) > 0 {
		v.TargetEvidence = "route_conflict"
	}
	if raw, err := json.Marshal(v); err != nil || len(raw) > 256<<10 {
		v.Hops = []Hop{}
		v.DestinationSystem = ""
		v.RemainingJumps = nil
		v.Truncated = true
		v.RouteState = "partial"
		if v.TargetEvidence == "verified_route_next" {
			v.TargetSystem = ""
			v.TargetEvidence = "unknown"
		}
	}
	return v
}
func safeText(s string) bool {
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > 128 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func exactID(n json.Number) (string, error) {
	s := string(n)
	if strings.ContainsAny(s, "+-.eE") {
		return "", errors.New("invalid identifier")
	}
	x, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(x, 10), nil
}
func starClass(s string) bool {
	for _, known := range strings.Split("O B A F G K M L T Y TTS AeBe W WN WNC WC WO CS C CN CJ CH CHd MS S D DA DAB DAO DAZ DAV DB DBZ DBV DO DOV DQ DC DCV DX N H X", " ") {
		if s == known {
			return true
		}
	}
	return false
}
func parseRoute(c sidecar.Change) (*route, bool, error) {
	bad := errors.New("invalid bounded route")
	if len(c.Raw) > 4<<20 || !utf8.Valid(c.Raw) {
		return nil, false, bad
	}
	var env struct {
		Event     string
		Timestamp time.Time
		Route     json.RawMessage
	}
	d := json.NewDecoder(bytes.NewReader(c.Raw))
	if d.Decode(&env) != nil || d.Decode(new(any)) != io.EOF || env.Event != "NavRoute" || env.Timestamp.IsZero() || !env.Timestamp.Equal(c.Timestamp) {
		return nil, false, bad
	}
	d = json.NewDecoder(bytes.NewReader(env.Route))
	d.UseNumber()
	token, err := d.Token()
	if err != nil || token != json.Delim('[') {
		return nil, false, bad
	}
	r := &route{stamp: env.Timestamp}
	for d.More() {
		if len(r.hops) >= MaxHops {
			return nil, true, bad
		}
		var h struct {
			StarSystem, StarClass string
			SystemAddress         json.Number
			StarPos               []any
		}
		if d.Decode(&h) != nil || !safeText(h.StarSystem) || !starClass(h.StarClass) || len(h.StarPos) != 3 {
			return nil, false, bad
		}
		address, err := exactID(h.SystemAddress)
		if err != nil {
			return nil, false, bad
		}
		v := routeHop{name: h.StarSystem, class: h.StarClass, address: address}
		for i, coordinate := range h.StarPos {
			number, ok := coordinate.(json.Number)
			if !ok {
				return nil, false, bad
			}
			x, err := number.Float64()
			if err != nil {
				return nil, false, bad
			}
			if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e6 {
				return nil, false, bad
			}
			v.pos[i] = x
		}
		if len(r.hops) > 0 {
			p := r.hops[len(r.hops)-1].pos
			if math.Sqrt(math.Pow(v.pos[0]-p[0], 2)+math.Pow(v.pos[1]-p[1], 2)+math.Pow(v.pos[2]-p[2], 2)) > 200000 {
				return nil, false, bad
			}
		}
		r.hops = append(r.hops, v)
	}
	if _, err := d.Token(); err != nil || d.Decode(new(any)) != io.EOF || len(r.hops) == 0 {
		return nil, false, bad
	}
	return r, false, nil
}
