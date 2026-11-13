package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/explorestate"
	"github.com/Jakoczig/daedalus/go/internal/navstate"
	"github.com/coder/websocket"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

func NavigationSmokeTest(ctx context.Context, s *Server) error {
	fail := errors.New("synthetic navigation and survey verification failed")
	if !s.g2b || s.game == nil {
		return fail
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	req, _ := http.NewRequestWithContext(ctx, "POST", s.URL()+"/auth/session", nil)
	req.Header.Set("Origin", s.URL())
	res, err := client.Do(req)
	if err != nil {
		return fail
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		return fail
	}
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.URL(), "http"), &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {s.URL()}}})
	if err != nil {
		return fail
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	rpc := func(name string, out any) error {
		if conn.Write(ctx, websocket.MessageText, []byte(`{"requestId":"navigation-smoke","name":"`+name+`"}`)) != nil {
			return fail
		}
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return fail
			}
			var reply struct {
				RequestID string
				Message   json.RawMessage
				Error     string
			}
			if json.Unmarshal(raw, &reply) != nil {
				return fail
			}
			if reply.RequestID == "navigation-smoke" {
				if reply.Error != "" || strings.Contains(string(reply.Message), "PRIVATE_") || json.Unmarshal(reply.Message, out) != nil {
					return fail
				}
				return nil
			}
		}
	}
	for {
		var n navstate.NavigationView
		var e explorestate.ExplorationView
		if rpc("getGoNavigation", &n) != nil || rpc("getGoExploration", &e) != nil {
			return fail
		}
		if n.CurrentSystem == "Fixture Alpha" && n.DestinationSystem == "Fixture Beta" && n.TargetSystem == "Fixture Beta" && len(n.Hops) == 2 && n.RemainingJumps != nil && *n.RemainingJumps == 1 && e.SystemName == "Fixture Alpha" && e.ReportedBodyCount != nil && *e.ReportedBodyCount == 12 && len(e.Bodies) == 1 && e.Bodies[0].BodyID == "9007199254740997" && e.Bodies[0].DSSMapped && e.Bodies[0].BioSignals != nil && *e.Bodies[0].BioSignals == 3 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fail
		case <-time.After(10 * time.Millisecond):
		}
	}
}
