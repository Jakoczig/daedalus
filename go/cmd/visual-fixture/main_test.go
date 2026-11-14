package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFixtureProviderPreservesExactIdentifiersAndStationBinding(t *testing.T) {
	f := fixtureTransport(`{"name":"Fixture Alpha","id64":9007199254740993,"bodies":[{"bodyId":9007199254740997}]}`)
	for _, endpoint := range []string{"bodies", "stations"} {
		req, _ := http.NewRequest("GET", "https://www.edsm.net/api-system-v1/"+endpoint+"?systemName=Fixture+Alpha", nil)
		resp, err := f.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var data struct {
			ID      json.Number `json:"id"`
			Address json.Number `json:"id64"`
			Bodies  []struct {
				ID json.Number `json:"bodyId"`
			} `json:"bodies"`
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.UseNumber()
		if err := dec.Decode(&data); err != nil {
			t.Fatal(err)
		}
		if data.ID.String() != "77" || data.Bodies[0].ID.String() != "9007199254740997" {
			t.Fatalf("fixture changed exact provider/body identifiers: %s", raw)
		}
		if endpoint == "bodies" && data.Address.String() != "9007199254740993" {
			t.Fatalf("fixture changed exact system address: %s", raw)
		}
		if endpoint == "stations" && data.Address.String() != "" {
			t.Fatalf("documented station fixture unexpectedly has id64: %s", raw)
		}
	}
}
