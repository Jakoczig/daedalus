package journal

import (
	"context"
	"crypto/sha256"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkJournalReplay(b *testing.B) {
	const records = 50000
	data := []byte(strings.Repeat("{\"event\":\"Location\",\"StarSystem\":\"Fixture Benchmark\",\"SystemAddress\":9223372036854775807}\n", records))
	dir := b.TempDir()
	os.WriteFile(filepath.Join(dir, "Journal.20260101.01.log"), data, 0600)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r, _ := NewReader(dir, ReaderOptions{})
		ctx, cancel := context.WithCancel(context.Background())
		count := 0
		err := r.Run(ctx, func(Event, Phase) error {
			count++
			if count == records {
				cancel()
			}
			return nil
		})
		cancel()
		if err != nil || count != records {
			b.Fatal("replay incomplete", err, count)
		}
	}
	b.ReportMetric(float64(records*b.N)/b.Elapsed().Seconds(), "events/s")
}
func TestReplayNeverWritesGameSources(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Journal.20260101.01.log")
	raw := []byte("{\"event\":\"Fileheader\"}\n")
	os.WriteFile(file, raw, 0600)
	r, _ := NewReader(dir, ReaderOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Run(ctx, func(Event, Phase) error { cancel(); return nil })
	after, err := os.ReadFile(file)
	if err != nil || sha256.Sum256(raw) != sha256.Sum256(after) {
		t.Fatal("reader modified game source")
	}
}
func TestIngestionPackagesCannotPerformProviderRequestsOrSpawnRuntimes(t *testing.T) {
	for _, name := range []string{"journal", "sidecar", "gamestate"} {
		root := filepath.Join("..", name)
		files, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range files {
			if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, entry.Name()), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range file.Imports {
				value, _ := strconv.Unquote(imp.Path.Value)
				if value == "net/http" || value == "net" || value == "os/exec" {
					t.Fatal("source ingestion gained provider/runtime capability", value)
				}
			}
		}
	}
}
