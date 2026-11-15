// Code generated for private durable reducer mementos; not a transport DTO.
// Regenerate deliberately when reducer state changes; CacheFormat invalidates old caches.
package activitystate

import (
	"bytes"
	"encoding/gob"
	time "time"
)

const CacheFormat = "77da7963386e68726cb78b040285e00330ed1037c8c66235ae617d620279ecc4"

type persistedstate struct {
	FV            View
	FVerified     bool
	FFile         string
	FPath         string
	FFid          string
	FOffset       int64
	FSequence     uint64
	FLast         time.Time
	FMissions     map[string]MissionCard
	FMissionOrder []string
	FTerminal     map[string]bool
}

func (v state) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedstate{FV: v.v, FVerified: v.verified, FFile: v.file, FPath: v.path, FFid: v.fid, FOffset: v.offset, FSequence: v.sequence, FLast: v.last, FMissions: v.missions, FMissionOrder: v.missionOrder, FTerminal: v.terminal})
	return b.Bytes(), err
}
func (v *state) GobDecode(raw []byte) error {
	var p persistedstate
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.v = p.FV
	v.verified = p.FVerified
	v.file = p.FFile
	v.path = p.FPath
	v.fid = p.FFid
	v.offset = p.FOffset
	v.sequence = p.FSequence
	v.last = p.FLast
	v.missions = p.FMissions
	v.missionOrder = p.FMissionOrder
	v.terminal = p.FTerminal
	return nil
}

type persistedModel struct {
	FStates map[string]*state
	FOrder  []string
	FActive string
}

func (v Model) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedModel{FStates: v.states, FOrder: v.order, FActive: v.active})
	return b.Bytes(), err
}
func (v *Model) GobDecode(raw []byte) error {
	var p persistedModel
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.states = p.FStates
	v.order = p.FOrder
	v.active = p.FActive
	return nil
}
