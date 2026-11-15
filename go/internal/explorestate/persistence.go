// Code generated for private durable reducer mementos; not a transport DTO.
// Regenerate deliberately when reducer state changes; CacheFormat invalidates old caches.
package explorestate

import (
	"bytes"
	"encoding/gob"
)

const CacheFormat = "7c36e0ef1c3d4247c46c4144908a6ff92e1df9df3d7dbdbf3b5707fc360df50b"

type persistedorganismState struct {
	FView     OrganismView
	FKey      string
	FLastKind string
}

func (v organismState) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedorganismState{FView: v.view, FKey: v.key, FLastKind: v.lastKind})
	return b.Bytes(), err
}
func (v *organismState) GobDecode(raw []byte) error {
	var p persistedorganismState
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.view = p.FView
	v.key = p.FKey
	v.lastKind = p.FLastKind
	return nil
}

type persistedsurveyCache struct {
	FKey     string
	FAddress string
	FView    ExplorationView
}

func (v surveyCache) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedsurveyCache{FKey: v.key, FAddress: v.address, FView: v.view})
	return b.Bytes(), err
}
func (v *surveyCache) GobDecode(raw []byte) error {
	var p persistedsurveyCache
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.key = p.FKey
	v.address = p.FAddress
	v.view = p.FView
	return nil
}

type persistedcontextState struct {
	FActiveOrganic *organismState
	FVerified      bool
	FArrived       bool
	FPending       bool
	FContinuous    bool
	FSeen          bool
	FAddress       string
	FName          string
	FFileID        string
	FOffset        int64
	FSequence      uint64
}

func (v contextState) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedcontextState{FActiveOrganic: v.activeOrganic, FVerified: v.verified, FArrived: v.arrived, FPending: v.pending, FContinuous: v.continuous, FSeen: v.seen, FAddress: v.address, FName: v.name, FFileID: v.fileID, FOffset: v.offset, FSequence: v.sequence})
	return b.Bytes(), err
}
func (v *contextState) GobDecode(raw []byte) error {
	var p persistedcontextState
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.activeOrganic = p.FActiveOrganic
	v.verified = p.FVerified
	v.arrived = p.FArrived
	v.pending = p.FPending
	v.continuous = p.FContinuous
	v.seen = p.FSeen
	v.address = p.FAddress
	v.name = p.FName
	v.fileID = p.FFileID
	v.offset = p.FOffset
	v.sequence = p.FSequence
	return nil
}

type persistedsystemKey struct {
	FCommander string
	FAddress   string
}

func (v systemKey) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedsystemKey{FCommander: v.commander, FAddress: v.address})
	return b.Bytes(), err
}
func (v *systemKey) GobDecode(raw []byte) error {
	var p persistedsystemKey
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.commander = p.FCommander
	v.address = p.FAddress
	return nil
}

type persistedbodyState struct {
	FView      BodyView
	FScanned   bool
	FOrganisms []*organismState
}

func (v bodyState) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedbodyState{FView: v.view, FScanned: v.scanned, FOrganisms: v.organisms})
	return b.Bytes(), err
}
func (v *bodyState) GobDecode(raw []byte) error {
	var p persistedbodyState
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.view = p.FView
	v.scanned = p.FScanned
	v.organisms = p.FOrganisms
	return nil
}

type persistedsystemState struct {
	FName         string
	FCount        *int
	FScannerCount *int
	FProgress     *float64
	FAll          bool
	FPartial      bool
	FOverflow     bool
	FBodies       map[string]*bodyState
}

func (v systemState) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedsystemState{FName: v.name, FCount: v.count, FScannerCount: v.scannerCount, FProgress: v.progress, FAll: v.all, FPartial: v.partial, FOverflow: v.overflow, FBodies: v.bodies})
	return b.Bytes(), err
}
func (v *systemState) GobDecode(raw []byte) error {
	var p persistedsystemState
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.name = p.FName
	v.count = p.FCount
	v.scannerCount = p.FScannerCount
	v.progress = p.FProgress
	v.all = p.FAll
	v.partial = p.FPartial
	v.overflow = p.FOverflow
	v.bodies = p.FBodies
	return nil
}

type persistedModel struct {
	FSessions map[string]*contextState
	FOrder    []string
	FSystems  map[systemKey]*systemState
	FLru      []systemKey
}

func (v Model) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedModel{FSessions: v.sessions, FOrder: v.order, FSystems: v.systems, FLru: v.lru})
	return b.Bytes(), err
}
func (v *Model) GobDecode(raw []byte) error {
	var p persistedModel
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.sessions = p.FSessions
	v.order = p.FOrder
	v.systems = p.FSystems
	v.lru = p.FLru
	return nil
}
