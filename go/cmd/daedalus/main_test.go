package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
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
	if err := run(context.Background(), []string{"--data-dir", t.TempDir()}, assets, launch, &output); err == nil {
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
		if !strings.Contains(string(body), `"stage":"G1B"`) {
			t.Fatal("normal runtime has no G1B owner settings", string(body))
		}
		return nil
	}
	if err := run(context.Background(), []string{"--data-dir", t.TempDir()}, assets, launch, &output); err != nil {
		t.Fatal(err)
	}
}
