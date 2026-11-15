package webapp

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"sync/atomic"
	"testing"
	"time"
)

func rpcMessage(t *testing.T, ctx context.Context, c *websocket.Conn, name string, msg any) map[string]json.RawMessage {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"requestId": "explicit", "name": name, "message": msg})
	if err := c.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]json.RawMessage
		if json.Unmarshal(b, &v) != nil {
			t.Fatal("frame")
		}
		if string(v["requestId"]) == `"explicit"` {
			return v
		}
	}
}
func TestG2GOwnerOnlyLifecycleAndDevicePolicy(t *testing.T) {
	s, _, viewer, url, _ := g2eFixture(t)
	var actions atomic.Int32
	s.companion.action = func(string) error { actions.Add(1); return nil }
	c := pairedSocket(t, viewer, url)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config := displayConfig{Charge: true, Jump: true, Restore: true, CloseMode: "tray"}
	for _, test := range []struct {
		name string
		msg  any
	}{{"getGoShipName", nil}, {"getGoProviders", nil}, {"configureGoProvider", map[string]any{"id": "eddn", "config": map[string]any{"upload": true, "categories": []string{"travel"}}}}, {"setGoProviderCredential", map[string]any{"id": "edsm", "credential": map[string]string{"apiKey": "PRIVATE"}}}, {"readGoProvider", map[string]string{"id": "edastro", "kind": "gec"}}, {"disconnectGoProvider", map[string]any{"id": "frontier", "remove": true}}, {"connectGoFrontier", nil}, {"desktopAction", map[string]string{"action": "quit"}}, {"setGoCompanion", config}, {"getGoLauncher", nil}, {"lookupGoSystem", map[string]string{"name": "Fixture Alpha"}}} {
		if len(rpcMessage(t, ctx, c, test.name, test.msg)["error"]) == 0 {
			t.Fatal("viewer privileged action", test.name)
		}
	}
	if actions.Load() != 0 {
		t.Fatal("viewer reached native host")
	}
	owner, ownerCtx := dialOwner(t, s)
	s.remote.mu.Lock()
	id := s.remote.state.Devices[0].ID
	s.remote.mu.Unlock()
	config.Devices = []string{id}
	if len(rpcMessage(t, ownerCtx, owner, "setGoCompanion", config)["error"]) != 0 {
		t.Fatal("owner setting rejected")
	}
	selected := s.displayPolicy(socketPrincipal{role: "viewer", deviceID: id})
	other := s.displayPolicy(socketPrincipal{role: "viewer", deviceID: "different"})
	local := s.displayPolicy(socketPrincipal{role: "owner"})
	if selected["selected"] != true || other["selected"] != false || local["selected"] != false {
		t.Fatal("untrusted role/device selection", selected, other, local)
	}
	if len(rpcMessage(t, ownerCtx, owner, "desktopAction", map[string]string{"action": "closeDashboard"})["error"]) != 0 {
		t.Fatal("dashboard close rejected")
	}
	if len(callRPC(t, ctx, c, "getGoWorkspace")["message"]) == 0 || s.game.Details().ActiveReaders != 1 {
		t.Fatal("closing optional dashboard interrupted paired service")
	}
	if len(rpcMessage(t, ownerCtx, owner, "desktopAction", map[string]string{"action": "execute"})["error"]) == 0 {
		t.Fatal("arbitrary native command")
	}
	_ = callRPC(t, ctx, c, "getGoNavigationContext")
	// Revoke without closing the owner's service; this includes the new feed.
	ownerAction(t, s, ownerClient(t, s), map[string]any{"action": "revoke", "deviceId": id})
}
