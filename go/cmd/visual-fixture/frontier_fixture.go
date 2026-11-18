package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/providers"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type frontierFixtureVault struct {
	values map[string]providers.Credential
}

func (v *frontierFixtureVault) Load() (map[string]providers.Credential, error) { return v.values, nil }
func (v *frontierFixtureVault) Save(values map[string]providers.Credential) error {
	v.values = values
	return nil
}

type frontierFixtureTransport struct{ root string }

func (f frontierFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != "GET" || r.URL.Host != "companion.orerve.net" || r.Header.Get("Authorization") != "Bearer SYNTHETIC-ACCESS" {
		return nil, fmt.Errorf("unexpected synthetic Frontier request")
	}
	kind := strings.TrimPrefix(r.URL.Path, "/")
	if kind != "profile" && kind != "market" && kind != "shipyard" && kind != "fleetcarrier" && kind != "communitygoals" {
		return nil, fmt.Errorf("unsupported synthetic endpoint")
	}
	raw, e := os.ReadFile(filepath.Join(f.root, "frontier-live-"+kind+"-structure.json"))
	if e != nil {
		return nil, e
	}
	status := 200
	if kind == "fleetcarrier" {
		status = 204
		raw = nil
	}
	if kind == "profile" {
		var data map[string]any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if e = d.Decode(&data); e != nil {
			return nil, e
		}
		data["commander"].(map[string]any)["name"] = "PRIVATE_SYNTHETIC"
		raw, e = json.Marshal(data)
		if e != nil {
			return nil, e
		}
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
}
func frontierFixtureOptions(root string, clock func() time.Time) providers.Options {
	h := sha256.Sum256([]byte("SYNTHETIC-G2B"))
	scope := hex.EncodeToString(h[:]) + "/frontier"
	return providers.Options{Manual: true, Now: clock, Client: &http.Client{Transport: frontierFixtureTransport{root}}, Vault: &frontierFixtureVault{values: map[string]providers.Credential{scope: {Commander: "PRIVATE_SYNTHETIC", JournalFID: "SYNTHETIC-G2B", AccessToken: "SYNTHETIC-ACCESS", RefreshToken: "SYNTHETIC-REFRESH", ExpiresAt: time.Now().Add(time.Hour)}}}}
}
