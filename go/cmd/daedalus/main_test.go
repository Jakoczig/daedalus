package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
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
	if err := run(context.Background(), nil, assets, launch, &output); err == nil {
		t.Fatal("startup failure hidden")
	}
	if response, err := http.Get(localURL); err == nil {
		response.Body.Close()
		t.Fatal("orphan listener")
	}
}
