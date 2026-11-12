package privatefs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrivateAtomicFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"first", "replacement"} {
		if err := Write(dir, "state.json", []byte(data)); err != nil {
			t.Fatal(err)
		}
		value, err := Read(dir, "state.json")
		if err != nil || string(value) != data {
			t.Fatalf("read: %s %v", value, err)
		}
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{dir, filepath.Join(dir, "state.json")} {
			info, _ := os.Stat(path)
			if info.Mode().Perm()&0077 != 0 {
				t.Fatalf("public permissions: %s %v", path, info.Mode())
			}
		}
	}
	if err := Write(dir, "../escape", []byte("bad")); err == nil {
		t.Fatal("path escape accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary files leaked")
	}
}

func TestRejectSymbolicStorage(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlink unavailable:", err)
	}
	if Prepare(link) == nil {
		t.Fatal("symlink root accepted")
	}
	dir := filepath.Join(parent, "safe")
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, "missing"), filepath.Join(dir, "state.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, "state.json"); err == nil {
		t.Fatal("symlink read accepted")
	}
	if Write(dir, "state.json", []byte("bad")) == nil {
		t.Fatal("symlink replaced")
	}
}

func TestExclusiveProfileLock(t *testing.T) {
	dir := t.TempDir()
	release, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Lock(dir); err == nil {
		second()
		t.Fatal("second process lock accepted")
	}
	release()
	release, err = Lock(dir)
	if err != nil {
		t.Fatal("lock did not release", err)
	}
	release()
}
