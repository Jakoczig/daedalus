package journal

import (
	"bytes"
	"encoding/json"
	"io"
	"time"
	"unicode"
	"unicode/utf8"
)

// Startup is a bounded private reducer memento, never a transport DTO. Only
// snapshot events are retained; no game activity, provider dispatch or history
// insert is synthesized. Exported fields permit atomic checkpoint persistence.
type Startup struct {
	Header            Event
	Last              SourcePosition
	LastAt, HighWater time.Time
	CommanderFID      string
	Snapshots         []Event
	Records, Bytes    int
	Valid             bool
}

// StartupBatch cannot be constructed with snapshots outside this package.
// Its identity is the subsequently verified LoadGame, not the previous account.
type StartupBatch struct {
	load      Event
	snapshots []Event
}

func (b StartupBatch) Events(load Event) []Event {
	if load.Type != "LoadGame" || load.FID != b.load.FID || load.Timestamp != b.load.Timestamp || load.Position != b.load.Position {
		return nil
	}
	return b.snapshots
}
func startupIdentity(fid string) bool {
	if fid == "" || !utf8.ValidString(fid) || utf8.RuneCountInString(fid) > 128 {
		return false
	}
	for _, r := range fid {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func startupSnapshot(kind string) bool {
	switch kind {
	case "Rank", "Progress", "Reputation", "EngineerProgress", "Materials":
		return true
	}
	return false
}

// Strict JSON rejects duplicate object properties, incomplete data and excessive
// nesting before any startup evidence can become attributable.
func uniqueStartupJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 16 {
			return false
		}
		tok, err := d.Token()
		if err != nil {
			return false
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return false
				}
				name, ok := k.(string)
				if !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		}
		return false
	}
	if !value(0) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}
func (s *Startup) Observe(e Event) StartupBatch {
	empty := StartupBatch{}
	if e.Type == "Fileheader" {
		high := s.HighWater
		*s = Startup{HighWater: high}
		if e.Position.FileID == "" || e.Position.Path == "" || e.Position.Offset != 0 || e.Position.Sequence != 1 || e.Position.Length <= 0 || e.Timestamp.IsZero() || e.Timestamp.Before(high) {
			return empty
		}
		s.Header = e
		s.Valid = true
	}
	if !s.Valid {
		return empty
	}
	valid := e.Position.FileID == s.Header.Position.FileID && e.Position.Path == s.Header.Position.Path && e.Position.Length >= int64(len(e.Raw)) && e.Position.Length > 0 && !e.Timestamp.IsZero() && !e.Timestamp.Before(s.LastAt)
	if s.Records > 0 {
		valid = valid && e.Position.Sequence == s.Last.Sequence+1 && e.Position.Offset == s.Last.Offset+s.Last.Length
	}
	s.Records++
	s.Bytes += len(e.Raw)
	if !valid || s.Records > 64 || s.Bytes > 512<<10 {
		s.Valid = false
		s.Snapshots = nil
		return empty
	}
	s.Last = e.Position
	s.LastAt = e.Timestamp
	if e.Type == "Commander" || startupSnapshot(e.Type) || e.Type == "LoadGame" {
		parsed, err := ParseLine(e.Raw, e.Position)
		if err != nil || parsed.Type != e.Type || !parsed.Timestamp.Equal(e.Timestamp) || parsed.FID != e.FID || !uniqueStartupJSON(e.Raw) {
			s.Valid = false
			s.Snapshots = nil
			return empty
		}
	}
	if e.FID != "" && e.Type != "Commander" && e.FID != s.CommanderFID {
		s.Valid = false
		s.Snapshots = nil
		return empty
	}
	switch {
	case e.Type == "Commander":
		if s.CommanderFID != "" || !startupIdentity(e.FID) {
			s.Valid = false
			s.Snapshots = nil
			return empty
		}
		s.CommanderFID = e.FID
	case startupSnapshot(e.Type):
		if s.CommanderFID == "" {
			s.Valid = false
			return empty
		}
		for _, old := range s.Snapshots {
			if old.Type == e.Type {
				s.Valid = false
				s.Snapshots = nil
				return empty
			}
		}
		s.Snapshots = append(s.Snapshots, e)
	case e.Type == "LoadGame":
		if !startupIdentity(e.FID) || e.FID != s.CommanderFID || e.Timestamp.Before(s.HighWater) || len(s.Snapshots) == 0 || e.Timestamp.Sub(s.Snapshots[0].Timestamp) > 2*time.Minute {
			s.Valid = false
			s.Snapshots = nil
			return empty
		}
		batch := StartupBatch{load: e, snapshots: s.Snapshots}
		*s = Startup{HighWater: e.Timestamp}
		return batch
	case e.Type == "Shutdown" || e.Type == "Location" || e.Type == "Loadout" || e.Type == "FSDJump" || e.Type == "Died":
		s.Valid = false
		s.Snapshots = nil
	}
	return empty
}
