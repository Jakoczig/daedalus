package privatefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityAdoptsExistingProfileWithoutCopyingSecurityState(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "daedalus-go")
	if err := Write(old, "remote.json", []byte("preserved")); err != nil {
		t.Fatal(err)
	}
	got, err := IdentityProfile(root)
	if err != nil || got != old {
		t.Fatalf("%s %v", got, err)
	}
	unlock, err := Lock(got)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if second, e := Lock(old); e == nil {
		second()
		t.Fatal("second owner")
	}
	again, e := IdentityProfile(root)
	if e != nil || again != old {
		t.Fatal("restart lost profile", e)
	}
	if b, _ := Read(old, "remote.json"); string(b) != "preserved" {
		t.Fatal("security state modified")
	}
	unlock()
	if err := os.Rename(old, old+"-recovery"); err != nil {
		t.Fatal(err)
	}
	if _, e := IdentityProfile(root); e == nil {
		t.Fatal("missing adopted profile silently replaced")
	}
	if err := os.Rename(old+"-recovery", old); err != nil {
		t.Fatal(err)
	}
	if got, e := IdentityProfile(root); e != nil || got != old {
		t.Fatal("recovery failed", e)
	}
}
func TestIdentityNewProfileAndInvalidManifest(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "eridion-go")
	got, e := IdentityProfile(root)
	if e != nil || got != want {
		t.Fatal(got, e)
	}
	if e = Write(want, "identity-v1.json", []byte(`{"version":1,"storage":"../../other"}`)); e != nil {
		t.Fatal(e)
	}
	if _, e = IdentityProfile(root); e == nil {
		t.Fatal("arbitrary storage accepted")
	}
}

func TestIdentityResumesInterruptedManifestWithoutDeletingDiagnosticFile(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "daedalus-go")
	current := filepath.Join(root, "eridion-go")
	if e := Prepare(old); e != nil {
		t.Fatal(e)
	}
	if e := Write(current, ".write-interrupted", []byte("unfinished")); e != nil {
		t.Fatal(e)
	}
	if got, e := IdentityProfile(root); e != nil || got != old {
		t.Fatal("interrupted adoption not resumed", got, e)
	}
	if b, e := Read(current, ".write-interrupted"); e != nil || string(b) != "unfinished" {
		t.Fatal("diagnostic file lost")
	}
}
