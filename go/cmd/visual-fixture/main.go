// Test-only fixture host. This separate command is never packaged with DAEDALUS.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Jakoczig/daedalus/go/internal/frontend"
	"github.com/Jakoczig/daedalus/go/internal/webapp"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
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
func main() {
	fixture := flag.String("fixture", "", "explicit fabricated fixture directory")
	stage := flag.String("stage", "G2D", "G2C or G2D test stage")
	reference := flag.String("reference", "", "generated synthetic legacy reference HTML")
	flag.Parse()
	if *fixture == "" || (*stage != "G2D" && *stage != "G2C") {
		panic("explicit synthetic fixture/stage required")
	}
	root, err := os.MkdirTemp("", "daedalus-visual-synthetic-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	journalDir := filepath.Join(root, "journals")
	os.Mkdir(journalDir, 0700)
	for _, pair := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
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
	server, err := webapp.StartLocalWithOptions(ctx, overlay{assets, *reference}, webapp.Options{StorageDir: filepath.Join(root, "profile"), EnableJournal: true, EnableNavigation: true, EnableExploration: true, EnableDashboard: *stage == "G2D", JournalDir: journalDir, PollInterval: 25 * time.Millisecond})
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
