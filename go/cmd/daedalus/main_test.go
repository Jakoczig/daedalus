package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSmokeAndStartupFailure(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}, "exploration/system.html": {Data: []byte("system")}}
	var output bytes.Buffer
	launch := func(context.Context, string, func()) error { t.Fatal("smoke launched desktop"); return nil }
	if err := run(context.Background(), []string{"--smoke-test"}, assets, launch, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "G1A offline smoke passed\n" {
		t.Fatalf("output %q", output.String())
	}
	if err := run(context.Background(), []string{"--smoke-test"}, fstest.MapFS{"index.html": {Data: []byte("index")}}, launch, &output); err == nil {
		t.Fatal("missing deep link accepted")
	}
	if err := run(context.Background(), []string{"--smoke-test"}, nil, launch, &output); err == nil {
		t.Fatal("missing assets accepted")
	}
	var localURL string
	launch = func(_ context.Context, url string, stop func()) error {
		localURL = url
		response, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return errors.New("native startup failed")
	}
	if err := run(context.Background(), []string{"--data-dir", t.TempDir(), "--journal-dir", t.TempDir()}, assets, launch, &output); err == nil {
		t.Fatal("startup failure hidden")
	}
	if response, err := http.Get(localURL); err == nil {
		response.Body.Close()
		t.Fatal("orphan listener")
	}
}

func TestNormalRuntimeProvidesOwnerRemoteSettings(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}}
	var output bytes.Buffer
	launch := func(_ context.Context, url string, _ func()) error {
		req, _ := http.NewRequest("POST", url+"/auth/session", nil)
		req.Header.Set("Origin", url)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if !strings.Contains(string(body), `"stage":"G2B"`) {
			t.Fatal("normal runtime has no G2B navigation/survey stage", string(body))
		}
		return nil
	}
	if err := run(context.Background(), []string{"--data-dir", t.TempDir(), "--journal-dir", t.TempDir()}, assets, launch, &output); err != nil {
		t.Fatal(err)
	}
}

func TestSmokeUsesNoPersonalJournals(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}, "exploration/system.html": {Data: []byte("system")}}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"--smoke-test", "--journal-dir", t.TempDir()}, assets, func(context.Context, string, func()) error { t.Fatal("smoke launched desktop"); return nil }, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "G1A offline smoke passed\n" {
		t.Fatal("smoke ceased to be isolated/offline")
	}
}

func TestSyntheticJournalSmokeAndExplicitSource(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}, "exploration/system.html": {Data: []byte("system")}}
	var output bytes.Buffer
	launch := func(context.Context, string, func()) error {
		t.Fatal("headless fixture smoke launched window")
		return nil
	}
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "testdata", "g2a", "replay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "Journal.20260101.01.log"), raw, 0600)
	if err := run(context.Background(), []string{"--journal-smoke-test", "--journal-dir", dir}, assets, launch, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "G2A synthetic journal smoke passed") {
		t.Fatal("no actual journal smoke result")
	}
	if err := run(context.Background(), []string{"--journal-smoke-test"}, assets, launch, &output); err == nil {
		t.Fatal("journal smoke attempted default personal discovery")
	}
}

func TestNativeOfflineSmokeIgnoresJournalDirectory(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}}
	checked := false
	launch := func(_ context.Context, url string, _ func()) error {
		req, _ := http.NewRequest("POST", url+"/auth/session", nil)
		req.Header.Set("Origin", url)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		cookies := res.Cookies()
		res.Body.Close()
		req, _ = http.NewRequest("POST", url+"/owner/journal", strings.NewReader(`{"action":"status"}`))
		req.Header.Set("Origin", url)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatal("native offline smoke enabled journal reading", res.StatusCode)
		}
		checked = true
		return errors.New("deliberate test window stop")
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"--desktop-smoke-test", "--journal-dir", t.TempDir()}, assets, launch, &out); err == nil || !checked {
		t.Fatal("native isolation was not checked")
	}
}
