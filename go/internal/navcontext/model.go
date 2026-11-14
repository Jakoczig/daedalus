// Package navcontext contains public galactic positions and explicit live flight
// state. Surface coordinates, commander IDs and arbitrary records never leave it.
package navcontext

import (
	"bytes"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/paritystate"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Point struct {
	Name        string      `json:"name"`
	Address     string      `json:"address"`
	Coordinates *[3]float64 `json:"coordinates,omitempty"`
	Security    string      `json:"security,omitempty"`
	Powers      []string    `json:"powers,omitempty"`
	Powerplay   string      `json:"powerplay,omitempty"`
	Source      string      `json:"source"`
}
type View struct {
	Current *Point  `json:"current"`
	Points  []Point `json:"points"`
	Mode    string  `json:"mode"`
	Episode string  `json:"episode"`
	Fresh   bool    `json:"fresh"`
	ModeAt  string  `json:"modeAt"`
}
type state struct {
	verified, arrived               bool
	session, last, modeAt, statusAt time.Time
	seq                             uint64
	file                            string
	current                         *Point
	points                          []Point
	mode, episode                   string
}
type Model struct {
	states map[string]*state
	order  []string
	serial uint64
}

func New() *Model { return &Model{states: map[string]*state{}} }
func clean(s string) string {
	if len(s) > 128 {
		return ""
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return strings.TrimSpace(s)
}
func Coordinates(raw json.RawMessage) *[3]float64 {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil
	}
	var values []any
	decoder := json.NewDecoder(bytes.NewReader(fields["StarPos"]))
	decoder.UseNumber()
	if decoder.Decode(&values) != nil || len(values) != 3 {
		return nil
	}
	var out [3]float64
	for i, v := range values {
		if len(fields["StarPos"]) > 256 {
			return nil
		}
		n, ok := v.(json.Number)
		if !ok {
			return nil
		}
		f, e := n.Float64()
		if e != nil || math.IsNaN(f) || math.IsInf(f, 0) || math.Abs(f) > 1e6 {
			return nil
		}
		out[i] = f
	}
	return &out
}
func (m *Model) Apply(key string, e journal.Event, phase journal.Phase) {
	s := m.states[key]
	if s == nil {
		if len(m.order) >= 16 {
			delete(m.states, m.order[0])
			m.order = m.order[1:]
		}
		s = &state{}
		m.states[key] = s
		m.order = append(m.order, key)
	}
	if e.Type == "Fileheader" || e.Type == "LoadGame" {
		old := s.points
		*s = state{points: old, session: e.Timestamp, verified: e.Type == "LoadGame" && key != "" && !e.Timestamp.IsZero(), seq: e.Position.Sequence, file: e.Position.FileID}
		return
	}
	if e.Position.Sequence != 0 {
		if s.seq != 0 && (e.Position.FileID != s.file || e.Position.Sequence != s.seq+1) {
			s.verified = false
			s.arrived = false
			s.current = nil
			s.mode = ""
		}
		s.seq = e.Position.Sequence
		s.file = e.Position.FileID
	}
	if !s.verified || e.Timestamp.IsZero() || e.Timestamp.Before(s.session) || e.Timestamp.Before(s.last) {
		return
	}
	s.last = e.Timestamp
	if e.Type == "Shutdown" {
		s.verified = false
		s.current = nil
		s.arrived = false
		s.mode = ""
		return
	}
	if e.Type == "Location" || e.Type == "FSDJump" || e.Type == "CarrierJump" {
		s.current = nil
		s.arrived = false
		s.mode = ""
		s.episode = ""
		if clean(e.SystemName) == "" || e.SystemAddress == "" {
			return
		}
		var d struct {
			SystemSecurityLocalised string `json:"SystemSecurity_Localised"`
			SystemSecurity          string
			Powers                  []string
			PowerplayState          string
		}
		_ = json.Unmarshal(e.Raw, &d)
		p := Point{Name: e.SystemName, Address: e.SystemAddress, Coordinates: Coordinates(e.Raw), Security: clean(d.SystemSecurityLocalised), Powerplay: clean(d.PowerplayState), Source: "journal"}
		if p.Security == "" {
			p.Security = clean(d.SystemSecurity)
		}
		for i, x := range d.Powers {
			if i >= 8 {
				break
			}
			if x = clean(x); x != "" {
				p.Powers = append(p.Powers, x)
			}
		}
		s.current = &p
		s.arrived = true
		found := false
		for i, old := range s.points {
			if old.Address == p.Address {
				s.points[i] = p
				found = true
				break
			}
		}
		if !found {
			s.points = append(s.points, p)
			if len(s.points) > 64 {
				s.points = s.points[1:]
			}
		}
		return
	}
	if e.Type == "StartJump" {
		if e.Timestamp.Before(s.statusAt) {
			return
		}
		var d struct{ JumpType string }
		_ = json.Unmarshal(e.Raw, &d)
		if phase == journal.Live && d.JumpType == "Hyperspace" && s.arrived {
			m.mode(s, "jump", e.Timestamp)
		} else {
			s.mode = ""
			s.episode = ""
		}
	}
}
func (m *Model) mode(s *state, mode string, at time.Time) {
	if s.mode == "" {
		m.serial++
		s.episode = strconv.FormatUint(m.serial, 10) + "/" + at.Format(time.RFC3339Nano)
	}
	s.mode = mode
	s.modeAt = at
}
func (m *Model) Status(key string, v paritystate.StatusView) {
	s := m.states[key]
	if s == nil || !s.verified || !s.arrived || v.State != "known" || v.Flags == nil || !v.Flags.InMainShip {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, v.ObservedAt)
	if err != nil || !at.After(s.session) || at.Before(s.modeAt) || at.Before(s.statusAt) {
		return
	}
	s.statusAt = at
	if v.Flags2 != nil && v.Flags2.FSDHyperdriveCharging {
		m.mode(s, "charge", at)
	} else if s.mode == "charge" {
		s.mode = ""
		s.episode = ""
	} else if s.mode == "jump" && v.Flags.FSDJump {
		s.modeAt = at
	}
}
func clone(p Point) Point {
	p.Powers = append([]string(nil), p.Powers...)
	if p.Coordinates != nil {
		x := *p.Coordinates
		p.Coordinates = &x
	}
	return p
}
func (m *Model) View(key string, now time.Time) View {
	v := View{Mode: "idle"}
	s := m.states[key]
	if s == nil {
		return v
	}
	for _, p := range s.points {
		v.Points = append(v.Points, clone(p))
	}
	if !s.verified || !s.arrived {
		return v
	}
	v.Fresh = true
	if s.current != nil {
		p := clone(*s.current)
		v.Current = &p
	}
	if s.mode != "" && now.Sub(s.modeAt) >= 0 && now.Sub(s.modeAt) < 60*time.Second {
		v.Mode = s.mode
		v.Episode = s.episode
		v.ModeAt = strings.SplitN(s.episode, "/", 2)[1]
	}
	return v
}

// Decode a bounded search or settings envelope without accepting unknown roles.
func Decode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
