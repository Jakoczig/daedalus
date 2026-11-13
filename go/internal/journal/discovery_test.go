package journal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeDir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func gameDir(root string) string {
	return filepath.Join(root, "Frontier Developments", "Elite Dangerous")
}

func TestDiscoverWindowsMovedSavedGames(t *testing.T) {
	saved := filepath.Join(t.TempDir(), "Moved Saved Games")
	want := makeDir(t, gameDir(saved))
	os.WriteFile(filepath.Join(want, "Status.json"), []byte(`{}`), 0600)
	got, err := Discover(context.Background(), DiscoveryOptions{SavedGames: saved})
	if err != nil || !got.Found || got.Directory != want || got.Source != "saved_games" {
		t.Fatalf("moved Saved Games not discovered: %+v %v", got, err)
	}
}

func TestDiscoverLinuxSteamFlatpakExtraLibraryAndWineOverride(t *testing.T) {
	for _, kind := range []string{"native", "flatpak", "extra library", "wine override", "old vdf"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "Steam")
			library := root
			if kind == "flatpak" {
				root = filepath.Join(base, ".var", "app", "com.valvesoftware.Steam", "data", "Steam")
				library = root
			}
			if kind == "extra library" || kind == "old vdf" {
				library = filepath.Join(base, "Other Library")
				makeDir(t, filepath.Join(root, "steamapps"))
				path := strings.ReplaceAll(library, `\`, `\\`)
				vdf := `"libraryfolders" { "1" { "path" "` + path + `" "apps" { "359320" "1" } } }`
				if kind == "old vdf" {
					vdf = `"LibraryFolders" { "1" "` + path + `" }`
				}
				if err := os.WriteFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"), []byte(vdf), 0600); err != nil {
					t.Fatal(err)
				}
			}
			want := makeDir(t, gameDir(filepath.Join(library, "steamapps", "compatdata", "359320", "pfx", "drive_c", "users", "different-player", "Saved Games")))
			os.WriteFile(filepath.Join(want, "Status.json"), []byte(`{}`), 0600)
			opts := DiscoveryOptions{Roots: []string{root}}
			if kind == "wine override" {
				want = makeDir(t, filepath.Join(base, "CustomWine", "journals"))
				opts.Override = want
			}
			got, err := Discover(context.Background(), opts)
			if err != nil || !got.Found || got.Directory != want {
				t.Fatalf("discovery: %+v %v", got, err)
			}
		})
	}
}

func TestDiscoverAbsentAndNewlyCreated(t *testing.T) {
	saved := filepath.Join(t.TempDir(), "Saved Games")
	opts := DiscoveryOptions{SavedGames: saved}
	got, err := Discover(context.Background(), opts)
	if err != nil || got.Found || len(got.Candidates) == 0 {
		t.Fatalf("missing optional source: %+v %v", got, err)
	}
	want := makeDir(t, gameDir(saved))
	os.WriteFile(filepath.Join(want, "Status.json"), []byte(`{}`), 0600)
	got, err = Discover(context.Background(), opts)
	if err != nil || !got.Found || got.Directory != want {
		t.Fatal("late creation not found")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Discover(ctx, opts); err == nil {
		t.Fatal("canceled discovery continued")
	}
}

func TestBadOwnerOverrideDoesNotCrashApp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-created")
	got, err := Discover(context.Background(), DiscoveryOptions{Override: path})
	if err == nil || got.Found || got.Directory != path {
		t.Fatal("invalid override lacks recoverable result")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("discovery wrote game directory")
	}
	if err := ValidateDir("relative-path"); err == nil {
		t.Fatal("relative source accepted")
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, []byte("fixture"), 0600)
	if err := ValidateDir(file); err == nil {
		t.Fatal("file accepted as directory")
	}
}

func TestProtectedJournalOverride(t *testing.T) {
	profile := t.TempDir()
	dir := t.TempDir()
	if got, err := LoadOverride(profile); err != nil || got != "" {
		t.Fatal("fresh override", err)
	}
	if err := SaveOverride(profile, dir); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadOverride(profile); err != nil || got != dir {
		t.Fatal("override not durable", err)
	}
	if err := SaveOverride(profile, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("bad override persisted")
	}
	if got, _ := LoadOverride(profile); got != dir {
		t.Fatal("invalid change replaced good preference")
	}
	if err := SaveOverride(profile, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadOverride(profile); got != "" {
		t.Fatal("override not cleared")
	}
}

func TestDiscoverSkipsEmptyAutomaticCandidate(t *testing.T) {
	root := t.TempDir()
	saved := makeDir(t, filepath.Join(root, "MovedSaved"))
	makeDir(t, gameDir(saved))
	want := makeDir(t, gameDir(filepath.Join(root, "steamapps", "compatdata", "359320", "pfx", "drive_c", "users", "fixture", "Saved Games")))
	os.WriteFile(filepath.Join(want, "Status.json"), []byte(`{}`), 0600)
	got, err := Discover(context.Background(), DiscoveryOptions{SavedGames: saved, Roots: []string{root}})
	if err != nil || got.Directory != want || !got.Found {
		t.Fatalf("empty default masked real source: %+v %v", got, err)
	}
}

func TestDiscoverDeduplicatesQueuedLibraries(t *testing.T) {
	base := t.TempDir()
	root := makeDir(t, filepath.Join(base, "Steam"))
	var vdf strings.Builder
	vdf.WriteString(`"libraryfolders" {`)
	var want string
	for i := range 16 {
		library := filepath.Join(base, fmt.Sprintf("Library%d", i))
		fmt.Fprintf(&vdf, ` "%d" { "path" "%s" }`, i, strings.ReplaceAll(library, `\`, `\\`))
		if i == 15 {
			want = makeDir(t, gameDir(filepath.Join(library, "steamapps", "compatdata", "359320", "pfx", "drive_c", "users", "fixture", "Saved Games")))
			os.WriteFile(filepath.Join(want, "Status.json"), []byte(`{}`), 0600)
		}
	}
	vdf.WriteString(`}`)
	for _, folder := range []string{"steamapps", "config"} {
		makeDir(t, filepath.Join(root, folder))
		os.WriteFile(filepath.Join(root, folder, "libraryfolders.vdf"), []byte(vdf.String()), 0600)
	}
	got, err := Discover(context.Background(), DiscoveryOptions{Roots: []string{root, root}})
	if err != nil || !got.Found || got.Directory != want {
		t.Fatalf("duplicate queue masked valid library: %+v %v", got, err)
	}
}
