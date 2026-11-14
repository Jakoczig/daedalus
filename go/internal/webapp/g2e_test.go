package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestG2EWorkspaceOptInAndViewerAllowlist(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"SYNTHETIC_PRIVATE","Commander":"PRIVATE_COMMANDER"}` + "\n" + `{"event":"Location","timestamp":"2026-01-01T00:00:02Z","StarSystem":"Fixture Alpha","SystemAddress":9007199254740993}` + "\n" + `{"event":"Loadout","timestamp":"2026-01-01T00:00:03Z","Ship":"asp","ShipID":9007199254740993,"ShipName":"PRIVATE_SHIP","ShipIdent":"PRIVATE_IDENT","Modules":[{"Slot":"FrameShiftDrive","Item":"int_hyperdrive_size5_class5","Health":0.9,"On":true}]}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "Journal.20260101.01.log"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, _, v, u, _ := startGameFixture(t, dir, true, "Fixture Alpha", true, true, true)
	c := pairedSocket(t, v, u)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reply := callRPC(t, ctx, c, "getGoWorkspace")
	body := reply["message"]
	onlyKeys(t, body, "parity activity")
	if !strings.Contains(string(body), "FrameShiftDrive") || strings.Contains(string(body), "PRIVATE_") || strings.Contains(string(body), "9007199254740993") {
		t.Fatal("invalid public workspace", string(body))
	}
	for _, rpc := range []string{"setModules", "toggleSwitch", "sellCargo", "execute", "setRole"} {
		if len(callRPC(t, ctx, c, rpc)["error"]) == 0 {
			t.Fatal("viewer host action", rpc)
		}
	}
	legacy, _, _, _, _ := g2dFixture(t)
	lc, lctx := dialOwner(t, legacy)
	if len(callRPC(t, lctx, lc, "getGoWorkspace")["error"]) == 0 {
		t.Fatal("G2D implicitly enabled workspace")
	}
	if s.prototype()["stage"] != "G2E" {
		t.Fatal("stage marker")
	}
}

func g2eFixture(t *testing.T) (*Server, *http.Client, *http.Client, string, string) {
	dir := t.TempDir()
	for _, pair := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}, {"Status.json", "Status.json"}, {"Cargo.json", "Cargo.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2e", pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, pair[1]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, o, v, u, d := startGameFixture(t, dir, true, "Fixture Alpha", true, true, true)
	deadline := time.Now().Add(3 * time.Second)
	for s.game.Workspace().Parity.Status.State != "known" {
		if time.Now().After(deadline) {
			t.Fatal("workspace status not settled", s.game.Workspace().Parity.Status.State)
		}
		time.Sleep(time.Millisecond)
	}
	return s, o, v, u, d
}
func TestG2EStatusUpdatesDoNotResendModuleOrExplorationPayloads(t *testing.T) {
	s, _, _, _, dir := g2eFixture(t)
	c, ctx := dialOwner(t, s)
	initial := callRPC(t, ctx, c, "getGoWorkspace")
	initialBytes := len(initial["message"])
	for _, fuel := range []string{"17.125", "18.375"} {
		raw := []byte(`{"event":"Status","timestamp":"2026-01-01T00:00:36Z","Flags":16777224,"Pips":[4,6,2],"Fuel":{"FuelMain":` + fuel + `}}`)
		if err := os.WriteFile(filepath.Join(dir, "Status.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		for {
			_, b, err := c.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var f struct {
				Name    string
				Message json.RawMessage
			}
			if json.Unmarshal(b, &f) != nil {
				t.Fatal("frame")
			}
			if !strings.Contains(string(f.Message), fuel) {
				continue
			}
			if fuel == "18.375" {
				if f.Name != "goFlightChanged" {
					t.Fatal("static payload resent on status-only update", f.Name)
				}
				if strings.Contains(string(b), "modules") || len(b) > 4096 {
					t.Fatal("unbounded flight frame")
				}
				t.Logf("workspace=%d bytes, flight=%d bytes", initialBytes, len(b))
			}
			break
		}
	}
}

func TestG2ERevokedViewerReceivesNoNewWorkspaceOrFlight(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		name := "registered"
		if delayed {
			name = "delayed_close"
		}
		t.Run(name, func(t *testing.T) {
			s, owner, viewer, url, dir := g2eFixture(t)
			conn := pairedSocket(t, viewer, url)
			ownerConn, _ := dialOwner(t, s)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			callRPC(t, ctx, conn, "getGoNavigation")
			callRPC(t, ctx, conn, "getGoExploration")
			callRPC(t, ctx, conn, "getGoShip")
			callRPC(t, ctx, conn, "getGoWorkspace")
			callRPC(t, ctx, ownerConn, "getGoWorkspace")
			callRPC(t, ctx, ownerConn, "getGoShip")
			type received struct {
				raw []byte
				err error
			}
			frames := make(chan received, 32)
			queued := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				var once sync.Once
				for {
					_, raw, err := conn.Read(ctx)
					select {
					case frames <- received{raw, err}:
					case <-ctx.Done():
						return
					}
					if strings.Contains(string(raw), "Fixture Before Revoke") {
						once.Do(func() { close(queued) })
					}
					if err != nil {
						return
					}
				}
			}()
			defer func() { cancel(); conn.CloseNow(); <-done }()
			writeGameEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:01:00Z","Name":"Fixture Before Revoke","SystemAddress":42}`)
			select {
			case <-queued:
			case <-ctx.Done():
				t.Fatal("no authorized pre-revoke stream barrier")
			}
			listed := ownerAction(t, s, owner, map[string]any{"action": "status"})
			var devices []struct{ ID string }
			if json.Unmarshal(listed["devices"], &devices) != nil || len(devices) != 1 {
				t.Fatal("invalid paired devices")
			}
			if delayed {
				s.remote.mu.Lock()
				removed := 0
				for c, id := range s.remote.connections {
					if id == devices[0].ID {
						delete(s.remote.connections, c)
						removed++
					}
				}
				s.remote.mu.Unlock()
				if removed != 1 {
					t.Fatal("no socket registration barrier")
				}
			}
			ownerAction(t, s, owner, map[string]any{"action": "revoke", "deviceId": devices[0].ID})
			writeGameEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:01:01Z","Name":"NEW_AFTER_REVOKE_TARGET","SystemAddress":43}`)
			writeGameEvent(t, dir, `{"event":"Scan","timestamp":"2026-01-01T00:01:02Z","SystemAddress":9007199254740993,"BodyID":2,"BodyName":"NEW_AFTER_REVOKE_BODY","PlanetClass":"Rocky body"}`)
			writeGameEvent(t, dir, `{"event":"ScanOrganic","timestamp":"2026-01-01T00:01:02Z","SystemAddress":9007199254740993,"Body":2,"Genus":"Bacterium","Species":"NEW_AFTER_REVOKE_BIO","ScanType":"Log"}`)
			writeGameEvent(t, dir, `{"event":"HullDamage","timestamp":"2026-01-01T00:01:03Z","Health":0.271828}`)
			if err := os.WriteFile(filepath.Join(dir, "Status.json"), []byte(`{"event":"Status","timestamp":"2026-01-01T00:01:04Z","Flags":16777224,"Fuel":{"FuelMain":19.777}}`), 0600); err != nil {
				t.Fatal(err)
			}
			// All producers must be live after revoke, independently observed by owner.
			seenNav, seenSurvey, seenBio, seenShip, seenWorkspace, seenFlight := false, false, false, false, false, false
			for !seenNav || !seenSurvey || !seenBio || !seenShip || !seenWorkspace || !seenFlight {
				_, raw, err := ownerConn.Read(ctx)
				if err != nil {
					t.Fatal(err)
				}
				var frame struct {
					Name    string
					Message json.RawMessage
				}
				if json.Unmarshal(raw, &frame) != nil {
					t.Fatal("invalid owner frame")
				}
				if frame.Name == "goWorkspaceChanged" && strings.Contains(string(raw), "0.271828") {
					seenWorkspace = true
				}
				if (frame.Name == "goFlightChanged" || frame.Name == "goWorkspaceChanged") && strings.Contains(string(raw), "19.777") {
					seenFlight = true
				}
				if frame.Name == "goShipChanged" && strings.Contains(string(raw), "0.271828") {
					seenShip = true
				}
				if frame.Name == "goNavigationChanged" && strings.Contains(string(raw), "NEW_AFTER_REVOKE_TARGET") {
					seenNav = true
				}
				if frame.Name == "goExplorationChanged" && strings.Contains(string(raw), "NEW_AFTER_REVOKE_BIO") {
					seenBio = true
				}
				if frame.Name == "goExplorationChanged" && strings.Contains(string(raw), "NEW_AFTER_REVOKE_BODY") {
					seenSurvey = true
				}
			}
			for {
				select {
				case f := <-frames:
					if f.err != nil {
						if ctx.Err() != nil {
							t.Fatal("deadline is not successful revocation")
						}
						goto closed
					}
					if strings.Contains(string(f.raw), "NEW_AFTER_REVOKE") || strings.Contains(string(f.raw), "0.271828") {
						t.Fatalf("new data authorized after revoke: %s", f.raw)
					}
					var frame struct {
						Name    string
						Message json.RawMessage
					}
					if json.Unmarshal(f.raw, &frame) != nil {
						t.Fatal("invalid frame")
					}
					if strings.Contains(string(frame.Message), "PRIVATE_") {
						t.Fatal("private data in queued frame")
					}
				case <-ctx.Done():
					t.Fatal("revoked socket remained open")
				}
			}
		closed:
			status, _, _ := remotePost(t, viewer, url+"/auth/session", url, nil)
			if status != 401 {
				t.Fatal("revoked session recovered")
			}
		})
	}
}
