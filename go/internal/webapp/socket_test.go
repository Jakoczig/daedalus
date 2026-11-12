package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func dialOwner(t *testing.T, s *Server) (*websocket.Conn, context.Context) {
	t.Helper()
	client := ownerClient(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, strings.Replace(s.URL(), "http:", "ws:", 1), &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{s.URL()}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn, ctx
}

func callRPC(t *testing.T, ctx context.Context, conn *websocket.Conn, name string) map[string]json.RawMessage {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"requestId": "fixture", "name": name, "message": map[string]any{"role": "owner"}})
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var response map[string]json.RawMessage
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		if string(response["requestId"]) == `"fixture"` {
			return response
		}
	}
}

func TestAuthenticatedOfflineSocket(t *testing.T) {
	s, err := StartLocal(context.Background(), fixtureFS())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	select {
	case <-s.ClientReady():
		t.Fatal("ready without authenticated RPC")
	default:
	}
	conn, ctx := dialOwner(t, s)
	response := callRPC(t, ctx, conn, "getLoadingStatus")
	select {
	case <-s.ClientReady():
	case <-time.After(time.Second):
		t.Fatal("authenticated bootstrap not observed")
	}
	var status struct {
		LoadingComplete        bool `json:"loadingComplete"`
		Offline                bool `json:"offline"`
		NumberOfFiles          int  `json:"numberOfFiles"`
		NumberOfEventsImported int  `json:"numberOfEventsImported"`
	}
	if err := json.Unmarshal(response["message"], &status); err != nil || !status.LoadingComplete || !status.Offline || status.NumberOfFiles != 0 || status.NumberOfEventsImported != 0 {
		t.Fatalf("dishonest bootstrap: %+v %v", status, err)
	}
	var host struct {
		URLs []string `json:"urls"`
	}
	if json.Unmarshal(callRPC(t, ctx, conn, "hostInfo")["message"], &host) != nil || len(host.URLs) != 1 || host.URLs[0] != s.URL() {
		t.Fatal("non-local host info")
	}
	for _, name := range []string{"constructor", "__proto__", "getPreferences", "setPreferences", "beginCapiAuth", "getLogEntries", "getShipStatus", "createPairingCode", "testMessage"} {
		reply := callRPC(t, ctx, conn, name)
		if len(reply["error"]) == 0 || len(reply["message"]) != 0 {
			t.Fatalf("unimplemented/privileged RPC accepted: %s", name)
		}
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("socket survived shutdown")
	}
}

func TestSocketRejectsAnonymousAndCrossOrigin(t *testing.T) {
	s, err := StartLocal(context.Background(), fixtureFS())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	owner := ownerClient(t, s)
	for _, tc := range []struct {
		client *http.Client
		origin string
	}{{http.DefaultClient, s.URL()}, {owner, "https://evil.example"}, {owner, ""}} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, response, err := websocket.Dial(ctx, strings.Replace(s.URL(), "http:", "ws:", 1), &websocket.DialOptions{HTTPClient: tc.client, HTTPHeader: http.Header{"Origin": []string{tc.origin}}})
		cancel()
		if conn != nil {
			conn.CloseNow()
		}
		if err == nil || response == nil || response.StatusCode != 403 {
			t.Fatalf("untrusted socket accepted: %v", err)
		}
	}
}

func TestSocketFrameLimit(t *testing.T) {
	s, err := StartLocal(context.Background(), fixtureFS())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	conn, ctx := dialOwner(t, s)
	_ = conn.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", (1<<20)+1)))
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
				t.Fatalf("wrong close status: %v", err)
			}
			break
		}
	}
}
