package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/gamestate"
	"github.com/coder/websocket"
)

// JournalSmokeTest requires the explicitly supplied fabricated replay fixture.
// It never discovers journals, starts another reader, or sends provider traffic.
func JournalSmokeTest(ctx context.Context, s *Server) error {
	fail := errors.New("synthetic journal verification failed")
	if s.game == nil {
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
	for {
		if conn.Write(ctx, websocket.MessageText, []byte(`{"requestId":"journal-smoke","name":"getJournalSummary"}`)) != nil {
			return fail
		}
		var summary gamestate.Summary
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return fail
			}
			var reply struct {
				RequestID string
				Message   json.RawMessage
			}
			if json.Unmarshal(raw, &reply) != nil {
				return fail
			}
			if reply.RequestID == "journal-smoke" {
				if json.Unmarshal(reply.Message, &summary) != nil {
					return fail
				}
				break
			}
		}
		if !summary.ImportInProgress && summary.EventsRead == 5 && summary.CurrentSystem == "Fixture Beta" && summary.LastEventType == "FSDJump" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fail
		case <-time.After(25 * time.Millisecond):
		}
	}
}
