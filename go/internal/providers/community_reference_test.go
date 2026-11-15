package providers

import (
	"net/url"
	"strings"
	"testing"
)

func TestCommunityReferencesUseDocumentedBoundedReadContracts(t *testing.T) {
	for _, c := range [][3]string{{"elitebgs", "systems", "Fixture Alpha"}, {"elitebgs", "factions", "Fixture Faction"}, {"elitebgs", "stations", "Fixture Port"}, {"elitebgs", "ticks", ""}, {"canonn", "guardian", "9007199254740993"}, {"raven", "system", "9007199254740993"}, {"raven", "project", "00000000-0000-0000-0000-000000000001"}} {
		raw, err := communityEndpoint(c[0], c[1], c[2], 1)
		if err != nil {
			t.Fatal(c, err)
		}
		u, _ := url.Parse(raw)
		if !allowedEndpoint(u) {
			t.Fatal("documented reference denied", raw)
		}
		if c[0] == "canonn" && (!strings.Contains(raw, "_limit=32") || !strings.Contains(raw, "system.id64=9007199254740993")) {
			t.Fatal("unbounded/unfiltered science feed")
		}
	}
	if _, err := communityEndpoint("raven", "project", "../../api/cmdr/private", 1); err == nil {
		t.Fatal("path injection")
	}
	if _, err := normalizeCommunity("elitebgs", "systems", "Fixture Alpha", []byte(`{"docs":[{"name":"Other"}]}`)); err == nil {
		t.Fatal("wrong BGS entity")
	}
	snapshot, err := normalizeCommunity("elitebgs", "systems", "Fixture Alpha", []byte(`{"docs":[{"name":"Fixture Alpha","system_address":"9007199254740993","updated_at":"2026-11-15T00:00:00Z"}],"page":1,"pages":1}`))
	if err != nil || len(snapshot.Facts) < 3 {
		t.Fatal(snapshot, err)
	}
	if _, err := normalizeCommunity("canonn", "guardian", "9007199254740993", []byte(`[{"siteID":1,"system":{"id64":"9007199254740994"}}]`)); err == nil {
		t.Fatal("wrong science system")
	}
	snapshot, err = normalizeCommunity("canonn", "guardian", "9007199254740993", []byte(`[{"siteID":1,"system":{"id64":"9007199254740993","systemName":"Fixture","edsmCoordX":1,"edsmCoordY":2,"edsmCoordZ":3},"discoveredBy":{"name":"PRIVATE"}}]`))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range snapshot.Facts {
		if strings.Contains(f.Value, "PRIVATE") {
			t.Fatal("private contributor emitted")
		}
	}
}
