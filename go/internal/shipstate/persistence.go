// Code generated for private durable reducer mementos; not a transport DTO.
// Regenerate deliberately when reducer state changes; CacheFormat invalidates old caches.
package shipstate

import (
	"bytes"
	"encoding/gob"
)

const CacheFormat = "6f40f18dd22595f6891fa925237571cad5ac5f8fca3b710432adca0cff1350c8"

type persistedstate struct {
	FV        View
	FVerified bool
	FFile     string
	FId       string
	FOffset   int64
	FSequence uint64
}

func (v state) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedstate{FV: v.v, FVerified: v.verified, FFile: v.file, FId: v.id, FOffset: v.offset, FSequence: v.sequence})
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
	v.id = p.FId
	v.offset = p.FOffset
	v.sequence = p.FSequence
	return nil
}

type persistedModel struct {
	FStates map[string]*state
	FOrder  []string
}

func (v Model) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedModel{FStates: v.states, FOrder: v.order})
	return b.Bytes(), err
}
func (v *Model) GobDecode(raw []byte) error {
	var p persistedModel
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.states = p.FStates
	v.order = p.FOrder
	return nil
}
