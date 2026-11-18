package providers

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/history"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFrontierOwnedFleetShapesAndExactShipIdentity(t *testing.T) {
	for _, collection := range []string{`[{"id":9007199254740993,"name":"asp","shipName":"Fixture ship","shipID":"FIX-01","value":{"total":12000,"hull":10000,"modules":2000},"starsystem":{"name":"Fixture system","systemaddress":18446744073709551615},"station":{"name":"Fixture station"}}]`, `{"9007199254740993":{"name":"asp","shipName":"Fixture ship","shipID":"FIX-01","value":{"total":12000,"hull":10000,"modules":2000},"starsystem":{"name":"Fixture system","systemaddress":18446744073709551615},"station":{"name":"Fixture station"}}}`} {
		raw := `{"commander":{"name":"Fixture Commander","currentShipId":9007199254740993},"ship":{"id":9007199254740993,"name":"asp"},"ships":` + collection + `}`
		x, e := normalizedCommander("frontier", "profile", "SYNTHETIC-A", "Fixture Commander", object([]byte(raw)), time.Now())
		if e != nil {
			t.Fatal(e)
		}
		ships, ok := x.Fields["ships"].([]CAPIShip)
		if !ok || len(ships) != 1 || ships[0].ID != "9007199254740993" || ships[0].SystemAddress != "18446744073709551615" {
			t.Fatalf("array/keyed owned fleet not normalized: %#v", x.Fields["ships"])
		}
		if !x.Observed.IsZero() {
			t.Fatal("fetch time used as observation")
		}
	}
}
func TestFrontierCarrier204IsNotAuthenticationFailure(t *testing.T) {
	now := time.Now()
	calls := 0
	m := New(context.Background(), t.TempDir(), Options{Manual: true, Vault: &testVault{}, Now: func() time.Time { return now }, Client: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/fleetcarrier" {
			t.Fatal(r.URL.Path)
		}
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}})
	defer m.Close()
	session(t, m)
	m.mu.Lock()
	m.secrets["frontier"] = Credential{Commander: "Fixture CMDR", JournalFID: "SYNTHETIC-A", AccessToken: "fixture", RefreshToken: "fixture", ExpiresAt: now.Add(time.Hour)}
	m.mu.Unlock()
	if e := m.Configure("frontier", Config{Read: true}); e != nil {
		t.Fatal(e)
	}
	v, e := m.Read(context.Background(), "frontier", "fleetcarrier", "")
	if e != nil || v == nil || calls != 1 {
		t.Fatalf("204 rejected as failure: %v", e)
	}
	if len(v.Facts) == 0 || v.Facts[0].Value != "No owned fleet carrier" {
		t.Fatal("missing explicit no-carrier status")
	}
}

func TestStructuralDiagnosticScrubsValuesAndPreservesFleetRelationships(t *testing.T) {
	raw := []byte(`{"commander":{"name":"PRIVATE-COMMANDER","id":123456789,"currentShipId":4837281,"credits":987654321},"ships":{"4837281":{"id":4837281,"shipName":"PRIVATE-SHIP","shipID":"SECRET-IDENT","station":{"name":"PRIVATE-LOCATION"},"name":"PRIVATE-MODEL","value":{"total":7654321}}},"PrivateOnlyKey":{},"secret":"PRIVATE-TOKEN","PRIVATE-COMMANDER":{"name":"PRIVATE-COMMANDER"}}`)
	safe, e := StructuralFixture(raw)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"PRIVATE-", "PrivateOnlyKey", "SECRET-IDENT", "4837281", "123456789", "987654321", "7654321"} {
		if strings.Contains(string(safe), secret) {
			t.Fatal("private value in fixture")
		}
	}
	d := json.NewDecoder(strings.NewReader(string(safe)))
	d.UseNumber()
	var v map[string]any
	if e = d.Decode(&v); e != nil {
		t.Fatal(e)
	}
	c := v["commander"].(map[string]any)
	ships := v["ships"].(map[string]any)
	key := string(c["currentShipId"].(json.Number))
	ship := ships[key].(map[string]any)
	if string(ship["id"].(json.Number)) != key || c["name"] != "Fixture Commander" {
		t.Fatal("ID relationship or identity fixture lost")
	}
	for _, bad := range [][]byte{[]byte(`{} {}`), []byte(`not json`), []byte(strings.Repeat(" ", 2<<20+1))} {
		if _, e := StructuralFixture(bad); e == nil {
			t.Fatal("unsafe diagnostic accepted")
		}
	}
}

func TestLiveStructuralFrontierContractsRemainSeparateFromJournalEvidence(t *testing.T) {
	now := time.Now()
	for _, kind := range []string{"profile", "market", "shipyard", "communitygoals"} {
		raw, e := os.ReadFile(filepath.Join("testdata", "frontier-live-"+kind+"-structure.json"))
		if e != nil {
			t.Fatal(e)
		}
		var x history.ExternalFacts
		if kind == "profile" {
			x, e = normalizedCommander("frontier", kind, "SYNTHETIC-A", "Fixture Commander", object(raw), now)
		} else {
			x, e = normalizeCAPISnapshot(kind, "SYNTHETIC-A", object(raw), now)
		}
		if e != nil || !x.IdentityVerified || !x.Observed.IsZero() {
			t.Fatalf("%s contract/observed-time: %v", kind, e)
		}
		if kind == "profile" {
			ships := x.Fields["ships"].([]CAPIShip)
			if len(ships) != 2 || ships[0].LoadoutComplete || ships[1].LoadoutComplete {
				t.Fatal("stored loadout completeness inferred")
			}
			current := x.Fields["ship"].(CAPIShip)
			if !current.CurrentReference || len(current.Modules) != 3 {
				t.Fatal("verified current modules lost")
			}
		}
		if kind == "shipyard" {
			if x.Fields["ships"] != nil || x.Fields["shipyard"] == nil {
				t.Fatal("station stock became owned ships")
			}
		}
		if kind == "communitygoals" {
			g := x.Fields["communityGoals"].(map[string]any)
			if g["personalParticipationVerified"] != false {
				t.Fatal("global goal became personal participation")
			}
		}
	}
	if _, e := normalizeCAPISnapshot("communitygoals", "SYNTHETIC-A", object([]byte(`{"activeCommunityGoals":null}`)), now); e == nil {
		t.Fatal("malformed global snapshot accepted")
	}
	x, e := normalizeCAPISnapshot("communitygoals", "SYNTHETIC-A", object([]byte(`{"activeCommunityGoals":[]}`)), now)
	if e != nil || len(x.Fields["communityGoals"].(map[string]any)["global"].([]map[string]any)) != 0 {
		t.Fatal("known empty global snapshot lost")
	}
}
func TestFrontierKeyedIdentityConflictCannotReattributeOwnedShip(t *testing.T) {
	raw := `{"commander":{"name":"Fixture Commander","currentShipId":1},"ship":{"id":2,"name":"asp"},"ships":{"1":{"id":2,"name":"asp"},"3":{"id":3,"name":"asp"}}}`
	x, e := normalizedCommander("frontier", "profile", "SYNTHETIC-A", "Fixture Commander", object([]byte(raw)), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	ships := x.Fields["ships"].([]CAPIShip)
	if len(ships) != 1 || ships[0].ID != "3" || x.Fields["ship"].(CAPIShip).CurrentReference {
		t.Fatal("conflicting IDs silently reconciled")
	}
}

func TestDiagnosticCannotGuessMultiAccountSelection(t *testing.T) {
	for _, tc := range []struct {
		fid, active, selected string
		multiple              bool
		others                int
		want                  bool
	}{{"A", "A", "", false, 0, true}, {"A", "A", "", false, 1, false}, {"A", "A", "A", true, 1, true}, {"A", "A", "B", true, 1, false}, {"A", "B", "A", true, 1, false}, {"", "A", "", false, 0, false}} {
		if got := auditNamespaceAllowed(tc.fid, hash(tc.active), tc.multiple, tc.selected, tc.others); got != tc.want {
			t.Fatal("ambiguous diagnostic namespace accepted/rejected", tc)
		}
	}
}
