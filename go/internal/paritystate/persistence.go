// Code generated for private durable reducer mementos; not a transport DTO.
// Regenerate deliberately when reducer state changes; CacheFormat invalidates old caches.
package paritystate

import (
	"bytes"
	"encoding/gob"
	shipstate "github.com/Jakoczig/eridion/go/internal/shipstate"
	time "time"
)

const CacheFormat = "7ca657b0090286de146e9539c52577c78252d039345352977abc7540169e6e76"

type persistedstate struct {
	FV           View
	FVerified    bool
	FSession     time.Time
	FBarrier     time.Time
	FMarker      time.Time
	FStatusStamp time.Time
	FFile        string
	FId          string
	FOffset      int64
	FSequence    uint64
	FLocker      bool
	FBackpack    bool
}

func (v state) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedstate{FV: v.v, FVerified: v.verified, FSession: v.session, FBarrier: v.barrier, FMarker: v.marker, FStatusStamp: v.statusStamp, FFile: v.file, FId: v.id, FOffset: v.offset, FSequence: v.sequence, FLocker: v.locker, FBackpack: v.backpack})
	return b.Bytes(), err
}
func (v *state) GobDecode(raw []byte) error {
	var p persistedstate
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.v = p.FV
	v.verified = p.FVerified
	v.session = p.FSession
	v.barrier = p.FBarrier
	v.marker = p.FMarker
	v.statusStamp = p.FStatusStamp
	v.file = p.FFile
	v.id = p.FId
	v.offset = p.FOffset
	v.sequence = p.FSequence
	v.locker = p.FLocker
	v.backpack = p.FBackpack
	return nil
}

type persistedcargoCandidate struct {
	FStamp     time.Time
	FHash      [32]byte
	FView      CargoView
	FAmbiguous bool
	FOwner     string
}

func (v cargoCandidate) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedcargoCandidate{FStamp: v.stamp, FHash: v.hash, FView: v.view, FAmbiguous: v.ambiguous, FOwner: v.owner})
	return b.Bytes(), err
}
func (v *cargoCandidate) GobDecode(raw []byte) error {
	var p persistedcargoCandidate
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.stamp = p.FStamp
	v.hash = p.FHash
	v.view = p.FView
	v.ambiguous = p.FAmbiguous
	v.owner = p.FOwner
	return nil
}

type persistedModel struct {
	FStates     map[string]*state
	FOrder      []string
	FShip       *shipstate.Model
	FActive     string
	FCandidate  *cargoCandidate
	FClaimed    time.Time
	FCargoValid bool
}

func (v Model) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedModel{FStates: v.states, FOrder: v.order, FShip: v.ship, FActive: v.active, FCandidate: v.candidate, FClaimed: v.claimed, FCargoValid: v.cargoValid})
	return b.Bytes(), err
}
func (v *Model) GobDecode(raw []byte) error {
	var p persistedModel
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.states = p.FStates
	v.order = p.FOrder
	v.ship = p.FShip
	v.active = p.FActive
	v.candidate = p.FCandidate
	v.claimed = p.FClaimed
	v.cargoValid = p.FCargoValid
	return nil
}
