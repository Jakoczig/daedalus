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

func g2dFixture(t *testing.T) (*Server, *http.Client, *http.Client, string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, p := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2d", p[0]))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, p[1]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, o, v, u, d := startGameFixture(t, dir, true, "Fixture Alpha", true, true)
	deadline := time.Now().Add(3 * time.Second)
	for s.game.Ship().FuelMainT == nil || *s.game.Ship().FuelMainT != 16 {
		if time.Now().After(deadline) {
			t.Fatal("ship replay not settled")
		}
		time.Sleep(time.Millisecond)
	}
	return s, o, v, u, d
}
func TestG2DViewerShipAllowlistAndReadOnly(t *testing.T) {
	s, _, v, u, _ := g2dFixture(t)
	c := pairedSocket(t, v, u)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reply := callRPC(t, ctx, c, "getGoShip")
	raw := reply["message"]
	onlyKeys(t, raw, "shipType evidence hullHealth fuelMainT fuelCapacityT cargoCapacityT maxJumpRangeLY unladenMassT docked landed")
	if !strings.Contains(string(raw), `"hullHealth":0.82`) || strings.Contains(string(raw), "PRIVATE_") || strings.Contains(string(raw), "123") {
		t.Fatal("ship public contract", string(raw))
	}
	for _, rpc := range []string{"setShip", "deployLandingGear", "setJournalDirectory", "generatePairingCode"} {
		if len(callRPC(t, ctx, c, rpc)["error"]) == 0 {
			t.Fatal("viewer action", rpc)
		}
	}
	owner, ownerCtx := dialOwner(t, s)
	r := callRPC(t, ownerCtx, owner, "getGoShip")
	if string(r["message"]) != string(raw) {
		t.Fatal("owner/viewer contract differs")
	}
	legacy, _, _, _, _ := g2bFixture(t)
	lc, lctx := dialOwner(t, legacy)
	if len(callRPC(t, lctx, lc, "getGoShip")["error"]) == 0 {
		t.Fatal("legacy stage opted in")
	}
}
func TestG2DNativeReadinessRequiresOwnerShip(t *testing.T) {
	s, _, v, u, _ := g2dFixture(t)
	c, ctx := dialOwner(t, s)
	callRPC(t, ctx, c, "getGoNavigation")
	callRPC(t, ctx, c, "getGoExploration")
	select {
	case <-s.NavigationClientReady():
		t.Fatal("ship omitted from terminal readiness")
	default:
	}
	viewer := pairedSocket(t, v, u)
	defer viewer.CloseNow()
	callRPC(t, ctx, viewer, "getGoShip")
	select {
	case <-s.NavigationClientReady():
		t.Fatal("viewer acknowledged owner ship")
	default:
	}
	callRPC(t, ctx, c, "getGoShip")
	select {
	case <-s.NavigationClientReady():
	case <-ctx.Done():
		t.Fatal("owner terminal never ready")
	}
	snapshot := s.game.Ship()
	*snapshot.FuelMainT = 900
	if *s.game.Ship().FuelMainT != 16 {
		t.Fatal("ship view aliases service")
	}
	raw, _ := json.Marshal(s.game.Ship())
	if strings.Contains(string(raw), "900") {
		t.Fatal("mutated ship")
	}
}

func TestG2DRevokedViewerReceivesNoNewShip(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		name := "registered"
		if delayed {
			name = "delayed_close"
		}
		t.Run(name, func(t *testing.T) {
			s, owner, viewer, url, dir := g2dFixture(t)
			conn := pairedSocket(t, viewer, url)
			ownerConn, _ := dialOwner(t, s)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			callRPC(t, ctx, conn, "getGoNavigation")
			callRPC(t, ctx, conn, "getGoExploration")
			callRPC(t, ctx, conn, "getGoShip")
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
			writeGameEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:29Z","Name":"Fixture Before Revoke","SystemAddress":42}`)
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
			writeGameEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:30Z","Name":"NEW_AFTER_REVOKE_TARGET","SystemAddress":43}`)
			writeGameEvent(t, dir, `{"event":"Scan","SystemAddress":9007199254740993,"BodyID":2,"BodyName":"NEW_AFTER_REVOKE_BODY","PlanetClass":"Rocky body"}`)
			writeGameEvent(t, dir, `{"event":"ScanOrganic","timestamp":"2026-01-01T00:00:31Z","SystemAddress":9007199254740993,"Body":2,"Genus":"Bacterium","Species":"NEW_AFTER_REVOKE_BIO","ScanType":"Log"}`)
			writeGameEvent(t, dir, `{"event":"HullDamage","Health":0.271828}`)
			// All producers must be live after revoke, independently observed by owner.
			seenNav, seenSurvey, seenBio, seenShip := false, false, false, false
			for !seenNav || !seenSurvey || !seenBio || !seenShip {
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
