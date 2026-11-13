package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func navigationSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, pair := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2b", pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, pair[1]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func TestG2BNavigationSmokeRequiresExplicitSyntheticSource(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}, "nav/route.html": {Data: []byte("route")}, "exploration/system.html": {Data: []byte("survey")}}
	var out bytes.Buffer
	launch := func(context.Context, string, func()) error { t.Fatal("headless launched a window"); return nil }
	for _, mode := range []string{"--navigation-smoke-test", "--desktop-navigation-smoke-test"} {
		if err := run(context.Background(), []string{mode}, assets, launch, &out); err == nil {
			t.Fatal("mode discovered personal journals", mode)
		}
	}
	profile := t.TempDir()
	if err := run(context.Background(), []string{"--navigation-smoke-test", "--journal-dir", navigationSource(t), "--data-dir", profile}, assets, launch, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "G2B synthetic navigation and survey smoke passed") {
		t.Fatal("no actual G2B verification")
	}
	if entries, err := os.ReadDir(profile); err != nil || len(entries) != 0 {
		t.Fatal("smoke used supplied user profile", err)
	}
}
func TestG2BNativeNavigationRequestsBothOwnerProjections(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("index")}, "nav/route.html": {Data: []byte("route")}, "exploration/system.html": {Data: []byte("survey")}}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var out bytes.Buffer
	launch := func(ctx context.Context, location string, stop func()) error {
		parsed, err := url.Parse(location)
		if err != nil || parsed.Path != "/nav/route" {
			t.Fatal("native smoke did not load actual route page", location)
		}
		base := parsed.Scheme + "://" + parsed.Host
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar}
		defer client.CloseIdleConnections()
		req, _ := http.NewRequestWithContext(ctx, "POST", base+"/auth/session", nil)
		req.Header.Set("Origin", base)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var bootstrap struct {
			Prototype struct {
				Stage           string
				NavigationSmoke bool
			}
		}
		err = json.NewDecoder(res.Body).Decode(&bootstrap)
		res.Body.Close()
		if err != nil || bootstrap.Prototype.Stage != "G2B" || !bootstrap.Prototype.NavigationSmoke {
			t.Fatal("native views not enabled", bootstrap, err)
		}
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http"), &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {base}}})
		if err != nil {
			t.Fatal(err)
		}
		defer conn.CloseNow()
		for _, name := range []string{"getGoNavigation", "getGoExploration"} {
			for {
				if conn.Write(ctx, websocket.MessageText, []byte(`{"requestId":"smoke","name":"`+name+`"}`)) != nil {
					t.Fatal("request failed")
				}
				valid := false
				for {
					_, raw, err := conn.Read(ctx)
					if err != nil {
						t.Fatal(err)
					}
					var reply struct {
						RequestID string
						Message   struct {
							Hops   []any
							Bodies []any
						}
					}
					json.Unmarshal(raw, &reply)
					if reply.RequestID == "smoke" {
						valid = len(reply.Message.Hops) > 0 || len(reply.Message.Bodies) > 0
						break
					}
				}
				if valid {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("empty projections")
				case <-time.After(time.Millisecond):
				}
			}
		}
		<-ctx.Done()
		return nil
	}
	if err := run(ctx, []string{"--desktop-navigation-smoke-test", "--journal-dir", navigationSource(t)}, assets, launch, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "G2B native navigation and survey bootstrap and shutdown passed") {
		t.Fatal("native dual-view readiness not witnessed", out.String())
	}
}
