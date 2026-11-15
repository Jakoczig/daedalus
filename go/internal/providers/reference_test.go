package providers

import (
	"net/url"
	"testing"
	"time"
)

func TestSpanshExactIDsAndPublishedResponseEnvelopes(t *testing.T) {
	for _, kind := range []string{"dump", "system", "body", "station"} {
		endpoint, err := referenceEndpoint("spansh", kind, "18446744073709551615")
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(endpoint)
		if !allowedEndpoint(u) {
			t.Fatal("documented endpoint denied")
		}
		if _, err = referenceEndpoint("spansh", kind, "9007199254740993.0"); err == nil {
			t.Fatal("noncanonical identifier accepted")
		}
	}
	for _, path := range []string{"/api/route", "/api/system/01", "/api/station/1/extra"} {
		u, _ := url.Parse("https://spansh.co.uk" + path)
		if allowedEndpoint(u) {
			t.Fatalf("unsafe path %s", path)
		}
	}
	snapshot, err := normalizeSpansh("system", "9007199254740993", []byte(`{"record":{"id64":9007199254740993,"name":"Fixture","x":1,"y":2,"z":3}}`))
	if err != nil || len(snapshot.Facts) == 0 {
		t.Fatalf("published record envelope: %+v %v", snapshot, err)
	}
	if _, err = normalizeSpansh("system", "9007199254740993", []byte(`{"record":{"id64":9007199254740992,"name":"Wrong"}}`)); err == nil {
		t.Fatal("wrong system accepted")
	}
	if _, err = normalizeSpansh("system", "1", []byte(`{"record":{"id64":1,"name":"Bad","x":1,"y":"unknown","z":3}}`)); err == nil {
		t.Fatal("invalid coordinates accepted")
	}
}

func TestCommanderProfilesRejectFuzzyIdentity(t *testing.T) {
	for _, id := range []string{"inara", "frontier"} {
		raw := `{"commander":{"id":123,"name":"Different"}}`
		if id == "inara" {
			raw = `{"events":[{"eventData":{"commanderName":"Different"}}]}`
		}
		if _, err := normalizedCommander(id, "profile", "SYNTHETIC", "Exact", object([]byte(raw)), time.Now()); err == nil {
			t.Fatal("different commander matched")
		}
	}
}

func TestCommanderProfileFetchDoesNotInventObservationTime(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 6, 0, 0, time.UTC)
	x, err := normalizedCommander("frontier", "profile", "SYNTHETIC-A", "Exact", object([]byte(`{"timestamp":"2026-11-15T12:06:00Z","commander":{"name":"Exact","credits":100,"observedAt":"2026-11-15T12:06:00Z"}}`)), now)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Observed.IsZero() || !x.Fetched.Equal(now) || x.Fields["credits"] != "100" {
		t.Fatal("unsupported response timestamp became witnessed evidence", x)
	}
}
