package navstate

import (
	"crypto/sha256"
	"testing"
	"time"
)

func TestRestoredRouteLinksRequireExactExistingOwnershipAndContent(t *testing.T) {
	stamp := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	old := route{stamp: stamp, fingerprint: sha256.Sum256([]byte("verified route")), owner: "synthetic-a", epoch: 3, hops: []routeHop{{name: "Fixture", address: "9007199254740993"}}}
	copied := old
	m := NewModel()
	m.candidate = &copied
	m.sourceValid = true
	m.active = "synthetic-a"
	s := &sessionState{verified: true, epoch: 3, reset: stamp.Add(-time.Hour), barrier: stamp.Add(-time.Hour), marker: stamp, route: &old}
	m.sessions["synthetic-a"] = s
	m.RestoreLinks()
	if s.route != m.candidate {
		t.Fatal("verified persisted alias not restored")
	}
	prior := old
	changed := old
	changed.fingerprint = sha256.Sum256([]byte("changed file with unchanged timestamp"))
	m.candidate = &changed
	s.route = &prior
	m.RestoreLinks()
	if s.route == m.candidate {
		t.Fatal("old marker verified changed content")
	}
	changed = old
	changed.owner = "synthetic-b"
	m.candidate = &changed
	m.RestoreLinks()
	if s.route == m.candidate {
		t.Fatal("route restored across commander ownership")
	}
}
