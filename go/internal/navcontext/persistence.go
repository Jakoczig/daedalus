// Code generated for private durable reducer mementos; not a transport DTO.
// Regenerate deliberately when reducer state changes; CacheFormat invalidates old caches.
package navcontext

import (
	"bytes"
	"encoding/gob"
	time "time"
)

const CacheFormat = "dfa33ce9cb23f5412c7d9fec205cd4d4fdb9c59098da42a8a0ef9cf1c9f11ad5"

type persistedstate struct {
	FVerified bool
	FArrived  bool
	FSession  time.Time
	FLast     time.Time
	FModeAt   time.Time
	FStatusAt time.Time
	FSeq      uint64
	FFile     string
	FCurrent  *Point
	FPoints   []Point
	FMode     string
	FEpisode  string
}

func (v state) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedstate{FVerified: v.verified, FArrived: v.arrived, FSession: v.session, FLast: v.last, FModeAt: v.modeAt, FStatusAt: v.statusAt, FSeq: v.seq, FFile: v.file, FCurrent: v.current, FPoints: v.points, FMode: v.mode, FEpisode: v.episode})
	return b.Bytes(), err
}
func (v *state) GobDecode(raw []byte) error {
	var p persistedstate
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.verified = p.FVerified
	v.arrived = p.FArrived
	v.session = p.FSession
	v.last = p.FLast
	v.modeAt = p.FModeAt
	v.statusAt = p.FStatusAt
	v.seq = p.FSeq
	v.file = p.FFile
	v.current = p.FCurrent
	v.points = p.FPoints
	v.mode = p.FMode
	v.episode = p.FEpisode
	return nil
}

type persistedModel struct {
	FStates map[string]*state
	FOrder  []string
	FSerial uint64
}

func (v Model) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedModel{FStates: v.states, FOrder: v.order, FSerial: v.serial})
	return b.Bytes(), err
}
func (v *Model) GobDecode(raw []byte) error {
	var p persistedModel
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.states = p.FStates
	v.order = p.FOrder
	v.serial = p.FSerial
	return nil
}
