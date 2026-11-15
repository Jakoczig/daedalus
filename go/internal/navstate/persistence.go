// Code generated for private durable reducer mementos; not a transport DTO.
// Regenerate deliberately when reducer state changes; CacheFormat invalidates old caches.
package navstate

import (
	"bytes"
	sha256 "crypto/sha256"
	"encoding/gob"
	time "time"
)

const CacheFormat = "441aff67671a331b793aa37051ead7fe67df5129df0ad5324a3e8147b0d520a7"

type persistedrouteHop struct {
	FName    string
	FClass   string
	FAddress string
	FPos     [3]float64
}

func (v routeHop) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedrouteHop{FName: v.name, FClass: v.class, FAddress: v.address, FPos: v.pos})
	return b.Bytes(), err
}
func (v *routeHop) GobDecode(raw []byte) error {
	var p persistedrouteHop
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.name = p.FName
	v.class = p.FClass
	v.address = p.FAddress
	v.pos = p.FPos
	return nil
}

type persistedroute struct {
	FStamp       time.Time
	FFingerprint [sha256.Size]byte
	FAmbiguous   bool
	FHops        []routeHop
	FOwner       string
	FEpoch       uint64
}

func (v route) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedroute{FStamp: v.stamp, FFingerprint: v.fingerprint, FAmbiguous: v.ambiguous, FHops: v.hops, FOwner: v.owner, FEpoch: v.epoch})
	return b.Bytes(), err
}
func (v *route) GobDecode(raw []byte) error {
	var p persistedroute
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.stamp = p.FStamp
	v.fingerprint = p.FFingerprint
	v.ambiguous = p.FAmbiguous
	v.hops = p.FHops
	v.owner = p.FOwner
	v.epoch = p.FEpoch
	return nil
}

type persistedsessionState struct {
	FVerified       bool
	FEpoch          uint64
	FReset          time.Time
	FMarker         time.Time
	FBarrier        time.Time
	FRoute          *route
	FState          string
	FCurrentName    string
	FCurrentAddress string
	FTargetName     string
	FTargetAddress  string
	FArrival        time.Time
	FReported       *int
	FMarkerLive     bool
	FTargetLive     bool
	FSourceFile     string
	FSequence       uint64
	FClearUnknown   bool
}

func (v sessionState) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedsessionState{FVerified: v.verified, FEpoch: v.epoch, FReset: v.reset, FMarker: v.marker, FBarrier: v.barrier, FRoute: v.route, FState: v.state, FCurrentName: v.currentName, FCurrentAddress: v.currentAddress, FTargetName: v.targetName, FTargetAddress: v.targetAddress, FArrival: v.arrival, FReported: v.reported, FMarkerLive: v.markerLive, FTargetLive: v.targetLive, FSourceFile: v.sourceFile, FSequence: v.sequence, FClearUnknown: v.clearUnknown})
	return b.Bytes(), err
}
func (v *sessionState) GobDecode(raw []byte) error {
	var p persistedsessionState
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.verified = p.FVerified
	v.epoch = p.FEpoch
	v.reset = p.FReset
	v.marker = p.FMarker
	v.barrier = p.FBarrier
	v.route = p.FRoute
	v.state = p.FState
	v.currentName = p.FCurrentName
	v.currentAddress = p.FCurrentAddress
	v.targetName = p.FTargetName
	v.targetAddress = p.FTargetAddress
	v.arrival = p.FArrival
	v.reported = p.FReported
	v.markerLive = p.FMarkerLive
	v.targetLive = p.FTargetLive
	v.sourceFile = p.FSourceFile
	v.sequence = p.FSequence
	v.clearUnknown = p.FClearUnknown
	return nil
}

type persistedModel struct {
	FSessions       map[string]*sessionState
	FOrder          []string
	FActive         string
	FCandidate      *route
	FSourceValid    bool
	FOverflow       bool
	FSerial         uint64
	FClaimedThrough time.Time
}

func (v Model) GobEncode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(persistedModel{FSessions: v.sessions, FOrder: v.order, FActive: v.active, FCandidate: v.candidate, FSourceValid: v.sourceValid, FOverflow: v.overflow, FSerial: v.serial, FClaimedThrough: v.claimedThrough})
	return b.Bytes(), err
}
func (v *Model) GobDecode(raw []byte) error {
	var p persistedModel
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return err
	}
	v.sessions = p.FSessions
	v.order = p.FOrder
	v.active = p.FActive
	v.candidate = p.FCandidate
	v.sourceValid = p.FSourceValid
	v.overflow = p.FOverflow
	v.serial = p.FSerial
	v.claimedThrough = p.FClaimedThrough
	return nil
}
