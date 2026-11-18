package webapp

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/community"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type communityTransport func(*http.Request) (*http.Response, error)

func (f communityTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestG2FViewersCannotEnableOrTriggerProvider(t *testing.T) {
	dir := t.TempDir()
	raw := `{"timestamp":"2026-01-01T00:00:01Z","event":"LoadGame","FID":"SYNTHETIC-PROVIDER"}` + "\n" + `{"timestamp":"2026-01-01T00:00:02Z","event":"Location","StarSystem":"Fixture Alpha","SystemAddress":9007199254740993}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "Journal.20260101.01.log"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	s, _, v, u, _ := startGameFixture(t, dir, true, "Fixture Alpha", true, true, true)
	deadline := time.Now().Add(3 * time.Second)
	for name, _ := s.game.SystemIdentity(); name != "Fixture Alpha"; name, _ = s.game.SystemIdentity() {
		if time.Now().After(deadline) {
			t.Fatal("current system identity not ready")
		}
		time.Sleep(time.Millisecond)
	}
	s.community.Close()
	var calls atomic.Int32
	s.community = community.New("", &http.Client{Transport: communityTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})})
	s.community.SetEnabled(true)
	c := pairedSocket(t, v, u)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, name := range []string{"setGoCommunityEnabled", "refreshGoCommunity"} {
		r := callRPC(t, ctx, c, name)
		if len(r["error"]) == 0 {
			t.Fatal("viewer provider action allowed", name)
		}
	}
	reply := callRPC(t, ctx, c, "getGoCommunity")
	if len(reply["error"]) > 0 {
		t.Fatal(string(reply["error"]))
	}
	var got community.View
	if json.Unmarshal(reply["message"], &got) != nil || !got.Enabled || got.SystemName != "Fixture Alpha" {
		t.Fatal("read-only cached status unavailable")
	}
	if calls.Load() != 0 {
		t.Fatal("viewer caused external request")
	}
}
