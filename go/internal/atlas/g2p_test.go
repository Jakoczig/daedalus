package atlas

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestG2PDisabledAtlasAndClearAreOfflineAndPreserveOtherProfileData(t *testing.T) {
	_, key := testFixture(t)
	profile := t.TempDir()
	root := filepath.Join(profile, "galaxy-atlas-v2")
	m := newManager(context.Background(), root, &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network"); return nil, nil })}, key, time.Now)
	defer m.Close()
	if err := os.WriteFile(filepath.Join(profile, "history.sqlite"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Configure(Preferences{Disabled: true, Automatic: true, Details: true}); err != nil {
		t.Fatal(err)
	}
	if err := m.Check(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := m.ClearAll(); err != nil {
		t.Fatal(err)
	}
	if !m.Status().Disabled || m.Status().Installed != "" {
		t.Fatal("clear did not disable atlas")
	}
	if b, err := os.ReadFile(filepath.Join(profile, "history.sqlite")); err != nil || string(b) != "preserve" {
		t.Fatal("profile data changed")
	}
}
