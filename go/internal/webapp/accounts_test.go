package webapp

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"strings"
	"testing"
	"time"
)

func TestPairedAccountGrantCannotFollowGlobalProfile(t *testing.T) {
	s, _, client, remoteURL, code := remoteFixture(t)
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code}); status != 200 {
		t.Fatal("pair", status)
	}
	conn := pairedSocket(t, client, remoteURL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, name := range []string{"getGoAccounts", "setGoAccounts", "getGoCommanderOverview", "setGoAvatar", "setGoFrontierClient", "completeGoFrontierReturn"} {
		raw, _ := json.Marshal(map[string]any{"name": name, "requestId": name, "message": map[string]any{}})
		if e := conn.Write(ctx, websocket.MessageText, raw); e != nil {
			t.Fatal(e)
		}
		for {
			_, data, e := conn.Read(ctx)
			if e != nil {
				t.Fatal(e)
			}
			var reply map[string]any
			_ = json.Unmarshal(data, &reply)
			if reply["requestId"] == name {
				if reply["error"] == nil || reply["message"] != nil {
					t.Fatalf("viewer accessed private account RPC %s", name)
				}
				break
			}
		}
	}
	s.remote.selectAccount("SYNTHETIC-A") // adopt the pre-account grant to A
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil); status != 200 {
		t.Fatal("existing grant not adopted")
	}
	fresh := pairedSocket(t, client, remoteURL)
	s.remote.selectAccount("SYNTHETIC-B")
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil); status != 401 {
		t.Fatal("A grant followed B", status)
	}
	// No newly authorized request or data notification can pass the invalid grant.
	data := []byte(`{"name":"getJournalSummary","requestId":"after-switch"}`)
	_ = fresh.Write(ctx, websocket.MessageText, data)
	for {
		_, raw, e := fresh.Read(ctx)
		if e != nil {
			if ctx.Err() != nil {
				t.Fatal("socket was not actively closed", e)
			}
			break
		}
		if strings.Contains(string(raw), "after-switch") {
			t.Fatal("foreign-account response reached previous grant")
		}
	}
	s.remote.mu.Lock()
	if len(s.remote.state.Devices) != 1 || s.remote.state.Devices[0].Account != tokenHash("SYNTHETIC-A") {
		t.Error("original grant lost or reassigned")
	}
	s.remote.mu.Unlock()
}
