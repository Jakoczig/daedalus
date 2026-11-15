package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/journal"
)

func TestGECActualCoordinatesContractCategoriesAndCache(t *testing.T) {
	raw, err := os.ReadFile("testdata/gec-rare-sanitized.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if dec.Decode(&records) != nil {
		t.Fatal("fixture")
	}
	records = append(records, map[string]any{"name": "Fixture nebula", "galMapSearch": "Fixture System", "type": "Sights and Scenery", "type2": "Nebulae", "coordinates": []int{1, 2, 3}, "descriptionHtml": "PRIVATE_UNTRUSTED"})
	for _, coords := range []any{[]any{"1", 2, 3}, []int{1, 2}, []int{1000001, 0, 0}, nil, map[string]int{"x": 1, "y": 2, "z": 3}} {
		records = append(records, map[string]any{"name": "Invalid", "coordinates": coords})
	}
	records = append(records, map[string]any{"name": "Old fabricated shape", "coords": map[string]int{"x": 1, "y": 2, "z": 3}})
	payload, _ := json.Marshal(records)
	calls := 0
	m, now := testManager(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/gec/json/rare" {
			t.Fatalf("verbose full feed requested: %s", r.URL.Path)
		}
		return reply(string(payload), 200), nil
	})
	if err := m.Configure("edastro", Config{Read: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Read(context.Background(), "edastro", "gec", ""); err != nil {
		t.Fatal(err)
	}
	layers := m.Layers()
	if len(layers.Points) != 2 || layers.Points[0].Coordinates != [3]float64{334.969, -55.4375, 23014.3} || layers.Points[1].Category != "nebula" || layers.Points[1].SystemName != "Fixture System" {
		t.Fatalf("real GEC contract lost: %+v", layers)
	}
	encoded, _ := json.Marshal(layers)
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), "descriptionHtml") {
		t.Fatal("unallowlisted provider fields retained")
	}
	*now = now.Add(2 * time.Minute)
	if _, err := m.Read(context.Background(), "edastro", "gec", ""); err != nil || calls != 1 {
		t.Fatalf("fresh cache fetched again: %d %v", calls, err)
	}
	*now = now.Add(24 * time.Hour)
	if _, err := m.Read(context.Background(), "edastro", "gec", ""); err != nil || calls != 2 {
		t.Fatalf("expired cache did not refresh: %d %v", calls, err)
	}
	m.Configure("edastro", Config{})
	if len(m.Layers().Points) != 0 {
		t.Fatal("consent-off retained shared layers")
	}
}
func TestGECRareRetainsResponseBudgetAndRejectsInvalidFeed(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Repeat(" ", 2<<20) + "[]"))}, nil
	})
	m.Configure("edastro", Config{Read: true})
	if _, err := m.Read(context.Background(), "edastro", "gec", ""); err == nil || len(m.Layers().Points) != 0 {
		t.Fatal("oversized response accepted")
	}
}
func TestEDAstroNoNewObservationsIsNotErrorOrSuccessfulUpload(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) {
		return reply(`{"status":401,"message":"No new observations"}`, 200), nil
	})
	m.Configure("edastro", Config{Upload: true, Categories: []string{"biology"}})
	m.Observe(event(t, `{"timestamp":"2026-11-14T12:00:00Z","event":"ScanOrganic","ScanType":"Log","Species":"Fixture","SystemAddress":1,"Body":0}`), journal.Live)
	m.dispatch("edastro")
	s := m.Status()[2]
	if s.Sent != 0 || len(s.Counts) != 0 || s.LastError != "" || s.NoNewObservations != 1 || s.State != "no_new_observations" || s.Queue != 0 {
		t.Fatalf("401 misclassified: %+v", s)
	}
	if accepted("edastro", []byte(`{"status":401}`)) {
		t.Fatal("no-new response counted as accepted upload")
	}
}

func TestEDAstroHTTPUnauthorizedRemainsFailure(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) {
		return reply(`{"status":401}`, http.StatusUnauthorized), nil
	})
	if err := m.Configure("edastro", Config{Upload: true, Categories: []string{"biology"}}); err != nil {
		t.Fatal(err)
	}
	m.Observe(event(t, `{"timestamp":"2026-11-14T12:00:00Z","event":"ScanOrganic","ScanType":"Log","Species":"Fixture","SystemAddress":1,"Body":0}`), journal.Live)
	m.dispatch("edastro")
	s := m.Status()[2]
	if s.Sent != 0 || s.NoNewObservations != 0 || s.LastError == "" || s.State == "no_new_observations" {
		t.Fatalf("HTTP authentication failure hidden: %+v", s)
	}
}
