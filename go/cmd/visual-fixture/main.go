// Test-only fixture host. This separate command is never packaged with DAEDALUS.
package main

import (
	"bytes"
	"context"
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
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(f))}, nil
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
		if *stage != "G2E" || !strings.HasSuffix(filepath.Clean(*fixture), "g2f") {
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
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	fmt.Println("VISUAL_FIXTURE_URL=" + server.URL())
	<-ctx.Done()
	closeCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := server.Close(closeCtx); err != nil {
		panic(err)
	}
}
