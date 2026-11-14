// Test-only fixture host. This separate command is never packaged with DAEDALUS.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/frontend"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"github.com/Jakoczig/daedalus/go/internal/webapp"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

type overlay struct {
	fs.FS
	reference string
}

func (o overlay) Open(name string) (fs.File, error) {
	if name == "__legacy-reference.html" && o.reference != "" {
		return os.Open(o.reference)
	}
	return o.FS.Open(name)
}

type fixtureTransport []byte

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != "www.edsm.net" || r.Method != "GET" {
		return nil, fmt.Errorf("unexpected fixture request")
	}
	var data map[string]any
	decoder := json.NewDecoder(bytes.NewReader(f))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	data["id"] = 77
	name := r.URL.Query().Get("systemName")
	if name != "Fixture Alpha" {
		data["name"] = name
		data["id64"] = uint64(9007199254740994)
		data["bodies"] = []any{map[string]any{"bodyId": 0, "name": name, "type": "Star", "subType": "M star"}}
	}
	if strings.HasSuffix(r.URL.Path, "/system") {
		data["coords"] = map[string]float64{"x": 3, "y": 4, "z": 0}
	}
	if strings.HasSuffix(r.URL.Path, "/stations") {
		delete(data, "id64")
	}
	raw, _ := json.Marshal(data)
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
}
func main() {
	fixture := flag.String("fixture", "", "explicit fabricated fixture directory")
	stage := flag.String("stage", "G2D", "G2C, G2D or G2E test stage")
	reference := flag.String("reference", "", "generated synthetic legacy reference HTML")
	communityFixture := flag.Bool("community-fixture", false, "explicit synthetic EDSM transport; never contacts provider")
	flag.Parse()
	if *fixture == "" || (*stage != "G2D" && *stage != "G2C" && *stage != "G2E") {
		panic("explicit synthetic fixture/stage required")
	}
	root, err := os.MkdirTemp("", "daedalus-visual-synthetic-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	journalDir := filepath.Join(root, "journals")
	os.Mkdir(journalDir, 0700)
	pairs := [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}}
	if *stage == "G2E" {
		pairs = append(pairs, [2]string{"Status.json", "Status.json"}, [2]string{"Cargo.json", "Cargo.json"})
	}
	for _, pair := range pairs {
		raw, err := os.ReadFile(filepath.Join(*fixture, pair[0]))
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(filepath.Join(journalDir, pair[1]), raw, 0600); err != nil {
			panic(err)
		}
	}
	assets, err := frontend.FS()
	if err != nil {
		panic(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, deadline := context.WithTimeout(ctx, 5*time.Minute)
	defer deadline()
	var communityClient *http.Client
	if *communityFixture {
		if *stage != "G2E" || !(strings.HasSuffix(filepath.Clean(*fixture), "g2f") || strings.HasSuffix(filepath.Clean(*fixture), "g2g") || strings.HasSuffix(filepath.Clean(*fixture), "g2h")) {
			panic("community fixture requires explicit G2F synthetic directory")
		}
		raw, err := os.ReadFile(filepath.Join(*fixture, "EDSM.json"))
		if err != nil {
			panic(err)
		}
		communityClient = &http.Client{Transport: fixtureTransport(raw)}
		if err = privatefs.Write(filepath.Join(root, "profile"), "community.json", []byte(`{"enabled":true}`)); err != nil {
			panic(err)
		}
	}
	server, err := webapp.StartLocalWithOptions(ctx, overlay{assets, *reference}, webapp.Options{CommunityClient: communityClient, StorageDir: filepath.Join(root, "profile"), EnableJournal: true, EnableNavigation: true, EnableExploration: true, EnableDashboard: *stage == "G2D" || *stage == "G2E", EnableParity: *stage == "G2E", JournalDir: journalDir, PollInterval: 500 * time.Millisecond})
	if err != nil {
		panic(err)
	}
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if filepath.Base(*fixture) != "g2g" {
				continue
			}
			kind := scanner.Text()
			at := time.Now().UTC().Format(time.RFC3339Nano)
			var e any
			if kind == "jump" {
				e = map[string]any{"event": "StartJump", "JumpType": "Hyperspace", "timestamp": at, "StarSystem": "Fixture Beta", "SystemAddress": uint64(9007199254740994)}
			} else if kind == "arrival" {
				e = map[string]any{"event": "FSDJump", "timestamp": at, "StarSystem": "Fixture Alpha", "SystemAddress": uint64(9007199254740993), "StarPos": []int{3, 4, 5}}
			}
			if e != nil {
				raw, _ := json.Marshal(e)
				f, err := os.OpenFile(filepath.Join(journalDir, "Journal.20260101.01.log"), os.O_WRONLY|os.O_APPEND, 0600)
				if err == nil {
					_, _ = f.Write(append(raw, '\n'))
					_ = f.Close()
				}
			}
		}
		cancel()
	}()
	fmt.Println("VISUAL_FIXTURE_URL=" + server.URL())
	<-ctx.Done()
	closeCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := server.Close(closeCtx); err != nil {
		panic(err)
	}
}
