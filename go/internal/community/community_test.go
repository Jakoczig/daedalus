package community

import (
	"context"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOptInCacheIdentityAndAllowedFields(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Scheme != "https" || r.URL.Host != "www.edsm.net" || r.URL.Query().Get("systemName") != "Fixture Alpha" || len(r.URL.Query()) != 1 || r.Header.Get("Authorization") != "" {
			t.Error("unsafe outbound request", r.URL)
		}
		payload := `{"name":"Fixture Alpha","id64":9007199254740993,"bodyCount":2,"bodies":[{"id":123,"bodyId":0,"name":"Fixture Alpha","type":"Star","subType":"G star"},{"bodyId":1,"name":"Fixture Alpha 1","type":"Planet","subType":"Rocky body","isLandable":true,"radius":1000,"surfaceTemperature":200,"parents":[{"Star":0}],"commander":"PRIVATE","wasMapped":true}],"stations":[{"name":"Fixture Station","type":"Coriolis Starport","body":{"id":1},"distanceToArrival":42,"updateTime":{"information":"2026-01-01"},"marketId":9007199254740993}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(payload)), Header: http.Header{}}, nil
	})}
	m := New("", client)
	if m.Refresh(context.Background(), "Fixture Alpha", "9007199254740993").Status != "disabled" || calls.Load() != 0 {
		t.Fatal("default made a request")
	}
	if err := m.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	v := m.Refresh(context.Background(), "Fixture Alpha", "9007199254740993")
	if v.Status != "cached" || len(v.Bodies) != 2 || len(v.Stations) != 1 || v.Bodies[1].BodyID != "1" || v.Bodies[1].DSSMapped {
		t.Fatalf("bad projection %#v", v)
	}
	m.Refresh(context.Background(), "Fixture Alpha", "9007199254740993")
	if calls.Load() != 2 {
		t.Fatal("TTL cache not shared")
	}
	v.Bodies[1].Name = "PRIVATE_MUTATION"
	if m.View("Fixture Alpha", "9007199254740993").Bodies[1].Name != "Fixture Alpha 1" {
		t.Fatal("mutable cache")
	}
	if len(m.View("Fixture Alpha", "9007199254740994").Bodies) != 0 {
		t.Fatal("wrong system identity shared")
	}
	if err := m.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if len(m.View("Fixture Alpha", "9007199254740993").Bodies) != 0 {
		t.Fatal("disabled leaked cached data")
	}
}
func TestIdentityMismatchPayloadLimitAndCooldown(t *testing.T) {
	for _, payload := range []string{`{"name":"Wrong","id64":1}`, `{"name":"Fixture Alpha","id64":9007199254740992}`, strings.Repeat("x", (2<<20)+1)} {
		calls := 0
		m := New("", &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(payload)), Header: http.Header{}}, nil
		})})
		m.SetEnabled(true)
		v := m.Refresh(context.Background(), "Fixture Alpha", "9007199254740993")
		if v.Status != "unavailable" || len(v.Bodies) != 0 {
			t.Fatal("untrusted response accepted")
		}
		m.Refresh(context.Background(), "Fixture Alpha", "9007199254740993")
		if calls != 1 {
			t.Fatal("failure request storm")
		}
	}
}
func TestDisableCancelsInflightAndNeverRestoresData(t *testing.T) {
	entered := make(chan struct{})
	m := New("", &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})})
	m.SetEnabled(true)
	done := make(chan View)
	go func() { done <- m.Refresh(context.Background(), "Fixture Alpha", "1") }()
	<-entered
	m.SetEnabled(false)
	select {
	case v := <-done:
		if v.Enabled || len(v.Bodies) != 0 {
			t.Fatal("disabled response restored")
		}
	case <-time.After(time.Second):
		t.Fatal("request not cancelled")
	}
}

func TestCacheExpiryAndBoundedSystems(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	m := New("", &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		raw := `{"name":"` + r.URL.Query().Get("systemName") + `","id64":1,"bodies":[],"stations":[]}`
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(raw))}, nil
	})})
	m.now = func() time.Time { return at }
	m.SetEnabled(true)
	m.Refresh(context.Background(), "Fixture Alpha", "1")
	at = at.Add(6 * time.Hour)
	if m.View("Fixture Alpha", "1").Status != "stale" {
		t.Fatal("TTL never expires")
	}
	m.Refresh(context.Background(), "Fixture Alpha", "1")
	if calls != 4 {
		t.Fatal("expired data not reloaded")
	}
	for i := 0; i < 17; i++ {
		at = at.Add(time.Minute)
		m.Refresh(context.Background(), fmt.Sprintf("Fixture %d", i), "1")
	}
	if len(m.cache) != 16 || len(m.order) != 16 || len(m.View("Fixture Alpha", "1").Bodies) > 0 {
		t.Fatal("cache bounds")
	}
}
func TestMalformedPreferencesStayOfflineAndPersistExplicitChoice(t *testing.T) {
	dir := t.TempDir()
	if err := privatefs.Write(dir, "community.json", []byte("broken")); err != nil {
		t.Fatal(err)
	}
	m := New(dir, nil)
	if m.View("Fixture", "1").Enabled || m.View("Fixture", "1").Notice == "" {
		t.Fatal("corrupt preference enabled or no warning")
	}
	raw, _ := privatefs.Read(dir, "community.json")
	if string(raw) != "broken" {
		t.Fatal("damaged setting overwritten")
	}
	if err := m.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	m.Close()
	n := New(dir, nil)
	defer n.Close()
	if !n.View("Fixture", "1").Enabled {
		t.Fatal("explicit choice not restored")
	}
}

func TestDocumentedStationShapeBindsProviderIDNotGameAddress(t *testing.T) {
	for _, id := range []int{77, 78} {
		m := New("", &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			raw := `{"name":"Fixture Alpha","id":77,"id64":9007199254740993,"bodies":[]}`
			if strings.HasSuffix(r.URL.Path, "/stations") {
				raw = fmt.Sprintf(`{"name":"Fixture Alpha","id":%d,"stations":[{"name":"Documented station","type":"Coriolis Starport"}]}`, id)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(raw))}, nil
		})})
		m.SetEnabled(true)
		v := m.Refresh(context.Background(), "Fixture Alpha", "9007199254740993")
		if (len(v.Stations) == 1) != (id == 77) {
			t.Fatal("documented provider ID binding", id, v)
		}
	}
}

func TestOwnerNamedLookupIsBoundedAndKeepsExactCoordinates(t *testing.T) {
	calls := 0
	m := New("", &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		raw := `{"id":77,"id64":9007199254740993,"name":"Fixture","bodies":[],"stations":[]}`
		if r.URL.Path == "/api-v1/system" {
			raw = `{"id":77,"id64":9007199254740993,"name":"Fixture","coords":{"x":3,"y":4,"z":5}}`
		}
		if r.URL.Path == "/api-system-v1/stations" {
			raw = `{"id":77,"name":"Fixture","stations":[{"name":"Port"}]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(raw))}, nil
	})})
	if m.Lookup(context.Background(), "Fixture").Enabled || calls != 0 {
		t.Fatal("lookup before opt-in")
	}
	m.SetEnabled(true)
	v := m.Lookup(context.Background(), "Fixture")
	if v.SystemAddress != "9007199254740993" || v.Coordinates == nil || (*v.Coordinates)[1] != 4 || len(v.Stations) != 1 || calls != 3 {
		t.Fatal(v, calls)
	}
	m.Named("Fixture")
	m.Lookup(context.Background(), "Fixture")
	if calls != 3 {
		t.Fatal("cached lookup repeated network")
	}
	if m.Lookup(context.Background(), "Other").Status == "cached" || calls != 3 {
		t.Fatal("global cooldown bypass")
	}
}
