package webapp

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/community"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type communityTransport func(*http.Request) (*http.Response, error)

func (f communityTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestG2FViewersCannotEnableOrTriggerProvider(t *testing.T) {
	s, _, v, u, _ := g2eFixture(t)
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
	if json.Unmarshal(reply["message"], &got) != nil || !got.Enabled {
		t.Fatal("read-only cached status unavailable")
	}
	if calls.Load() != 0 {
		t.Fatal("viewer caused external request")
	}
}
