package webapp

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"github.com/Jakoczig/daedalus/go/internal/remotetls"
)

func TestDamagedRemoteStateKeepsLocalOwnerAvailable(t *testing.T) {
	for _, damage := range []string{"malformed remote.json", "missing remote.json", "missing TLS file", "missing TLS marker", "both TLS files missing", "malformed TLS identity", "unsafe remote permissions"} {
		t.Run(damage, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := remotetls.Load(dir, net.ParseIP("127.0.0.1"), time.Now()); err != nil {
				t.Fatal(err)
			}
			token := strings.Repeat("a", 64)
			state := remoteState{Version: 1, Enabled: true, Address: "127.0.0.1", Port: 0, Devices: []device{{ID: strings.Repeat("b", 32), Name: "Old viewer", Hash: tokenHash(token), Expires: time.Now().Add(time.Hour)}}}
			raw, _ := json.Marshal(state)
			if err := privatefs.Write(dir, "remote.json", raw); err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "malformed remote.json":
				if err := privatefs.Write(dir, "remote.json", []byte(`{"enabled":true,`)); err != nil {
					t.Fatal(err)
				}
			case "missing remote.json":
				if err := os.Remove(filepath.Join(dir, "remote.json")); err != nil {
					t.Fatal(err)
				}
			case "missing TLS file":
				if err := os.Remove(filepath.Join(dir, "tls.json")); err != nil {
					t.Fatal(err)
				}
			case "missing TLS marker":
				if err := os.Remove(filepath.Join(dir, "tls.initialized")); err != nil {
					t.Fatal(err)
				}
			case "both TLS files missing":
				for _, name := range []string{"tls.json", "tls.initialized"} {
					if err := os.Remove(filepath.Join(dir, name)); err != nil {
						t.Fatal(err)
					}
				}
			case "malformed TLS identity":
				if err := privatefs.Write(dir, "tls.json", []byte(`{}`)); err != nil {
					t.Fatal(err)
				}
			case "unsafe remote permissions":
				makeUnsafeRemoteFile(t, filepath.Join(dir, "remote.json"))
			}
			before := profileContents(t, dir)
			server, err := StartLocalWithStorage(context.Background(), fixtureFS(), dir)
			if err != nil {
				t.Fatalf("optional remote damage prevented local startup: %v", err)
			}
			defer server.Close(context.Background())
			owner := ownerClient(t, server)
			response, err := owner.Get(server.URL())
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatal("local assets unavailable")
			}
			conn, ctx := dialOwner(t, server)
			if len(callRPC(t, ctx, conn, "getLoadingStatus")["message"]) == 0 {
				t.Fatal("local RPC unavailable")
			}
			status := ownerAction(t, server, owner, map[string]any{"action": "status"})
			if string(status["enabled"]) != "false" || string(status["devices"]) != "[]" || string(status["recoveryRequired"]) != "true" {
				t.Fatal("unsafe recovery status", status)
			}
			var notice string
			json.Unmarshal(status["notice"], &notice)
			if !strings.Contains(notice, "back up") || !strings.Contains(notice, "Local desktop") {
				t.Fatal("missing actionable local recovery notice", notice)
			}
			server.remote.mu.Lock()
			_, valid := server.remote.findLocked(token)
			remoteHTTP := server.remote.http
			server.remote.mu.Unlock()
			if valid || remoteHTTP != nil {
				t.Fatal("invalid remote state restored access")
			}
			for _, action := range []string{"enable", "pair", "disable"} {
				code, _, _ := remotePost(t, owner, server.URL()+"/owner/remote", server.URL(), map[string]any{"action": action, "address": "127.0.0.1", "port": 0})
				if code != 503 {
					t.Fatal("quarantined remote state accepted mutation", action, code)
				}
			}
			if second, err := StartLocalWithStorage(context.Background(), fixtureFS(), dir); err == nil {
				second.Close(context.Background())
				t.Fatal("recovery dropped exclusive profile lock")
			}
			if err := server.Close(context.Background()); err != nil {
				t.Fatal(err)
			}
			after := profileContents(t, dir)
			if string(before) != string(after) {
				t.Fatal("damaged profile files were changed or regenerated")
			}
		})
	}
}

func profileContents(t *testing.T, dir string) []byte {
	t.Helper()
	snapshot := map[string]string{}
	for _, name := range []string{"remote.json", "tls.json", "tls.initialized"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			snapshot[name] = string(raw)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(snapshot)
	return raw
}

func TestLockedProfileStillPreventsCompetingLocalOwner(t *testing.T) {
	dir := t.TempDir()
	release, err := privatefs.Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if server, err := StartLocalWithStorage(context.Background(), fixtureFS(), dir); err == nil {
		server.Close(context.Background())
		t.Fatal("locked profile became another local owner")
	}
}
