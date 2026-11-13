package webapp

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func g2bFixture(t *testing.T) (*Server, *http.Client, *http.Client, string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, item := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2b", item[0]))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, item[1]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, owner, viewer, url, dir := startGameFixture(t, dir, true, "Fixture Alpha")
	deadline := time.Now().Add(3 * time.Second)
	for len(s.game.Navigation().Hops) != 2 || len(s.game.Exploration().Bodies) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("G2B fixture did not settle")
		}
		time.Sleep(time.Millisecond)
	}
	return s, owner, viewer, url, dir
}
func onlyKeys(t *testing.T, raw json.RawMessage, keys string) map[string]json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, k := range strings.Fields(keys) {
		allowed[k] = true
	}
	if len(fields) != len(allowed) {
		t.Fatalf("schema changed: %s", raw)
	}
	for key := range fields {
		if !allowed[key] {
			t.Fatal("unallowlisted field", key)
		}
	}
	return fields
}
func assertG2BSafe(t *testing.T, name string, raw json.RawMessage) {
	t.Helper()
	if len(raw) > 256<<10 {
		t.Fatal("public payload exceeds budget")
	}
	for _, private := range []string{"SystemAddress", "StarPos", "FID", "PRIVATE_", "Commander", "Latitude", "Longitude", "Materials", "Journal.", "9007199254740993"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("private field/data leaked", private)
		}
	}
	if name == "getGoNavigation" || name == "goNavigationChanged" {
		v := onlyKeys(t, raw, "currentSystem targetSystem destinationSystem routeState targetEvidence remainingJumps reportedRemainingJumps hops truncated")
		var hops []json.RawMessage
		json.Unmarshal(v["hops"], &hops)
		for _, h := range hops {
			onlyKeys(t, h, "name starClass distanceFromPreviousLY isCurrent isNext")
		}
	} else {
		v := onlyKeys(t, raw, "systemName surveyState reportedBodyCount fssProgress allBodiesFound scannerBodyCount bodies truncated")
		var bodies []json.RawMessage
		json.Unmarshal(v["bodies"], &bodies)
		for _, b := range bodies {
			fields := onlyKeys(t, b, "name kind bodyId atmosphere volcanism landable terraformable gravityG distanceLS bioSignals geoSignals previouslyDiscovered previouslyMapped dssMapped genuses")
			var id string
			if json.Unmarshal(fields["bodyId"], &id) != nil || id == "" {
				t.Fatal("BodyID not an exact string")
			}
		}
	}
}
func TestG2BViewSchemaExcludesPrivateFields(t *testing.T) {
	s, _, viewer, url, _ := g2bFixture(t)
	owner, ctx := dialOwner(t, s)
	remote := pairedSocket(t, viewer, url)
	for _, conn := range []*websocket.Conn{owner, remote} {
		for _, name := range []string{"getGoNavigation", "getGoExploration"} {
			r := callRPC(t, ctx, conn, name)
			if len(r["error"]) > 0 {
				t.Fatal(string(r["error"]))
			}
			assertG2BSafe(t, name, r["message"])
		}
	}
	if stage := callRPC(t, ctx, owner, "getLoadingStatus"); !strings.Contains(string(stage["message"]), `"stage":"G2B"`) {
		t.Fatal("G2B not opt-in")
	}
	assertSafeSummary(t, callRPC(t, ctx, owner, "getJournalSummary")["message"])
}
func TestG2BPairedViewerReadOnly(t *testing.T) {
	s, _, viewer, url, _ := g2bFixture(t)
	conn := pairedSocket(t, viewer, url)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, name := range []string{"setTarget", "copyToHostClipboard", "plotRoute", "getRawNavRoute", "getSystem", "getRawScan"} {
		if len(callRPC(t, ctx, conn, name)["error"]) == 0 {
			t.Fatal("viewer host/legacy action accepted", name)
		}
	}
	status, _, _ := remotePost(t, viewer, url+"/owner/journal", url, map[string]any{"action": "status"})
	if status != 403 {
		t.Fatal("remote owner path exposed")
	}
	// Anonymous clients do not acquire even read-only game projections.
	raw, res, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(url, "https"), &websocket.DialOptions{HTTPClient: &http.Client{Transport: viewer.Transport}, HTTPHeader: http.Header{"Origin": {url}}})
	if raw != nil {
		raw.CloseNow()
	}
	if err == nil || res == nil || res.StatusCode != 403 {
		t.Fatal("unpaired socket admitted", err)
	}
	_ = s
}
func TestG2BOneJournalReaderForMultipleViewers(t *testing.T) {
	s, _, viewer, url, _ := g2bFixture(t)
	starts := s.game.Details().ReaderStarts
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for range 3 {
		conn := pairedSocket(t, viewer, url)
		callRPC(t, ctx, conn, "getGoNavigation")
		callRPC(t, ctx, conn, "getGoExploration")
	}
	if s.game.Details().ReaderStarts != starts || s.game.Details().ActiveReaders != 1 {
		t.Fatal("viewers started another reader")
	}
}
func TestG2BRevokedViewerReceivesNoNewNavigationOrSurvey(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		name := "registered"
		if delayed {
			name = "delayed_close"
		}
		t.Run(name, func(t *testing.T) {
			s, owner, viewer, url, dir := g2bFixture(t)
			conn := pairedSocket(t, viewer, url)
			ownerConn, _ := dialOwner(t, s)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			callRPC(t, ctx, conn, "getGoNavigation")
			callRPC(t, ctx, conn, "getGoExploration")
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
			writeGameEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:09Z","Name":"Fixture Before Revoke","SystemAddress":42}`)
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
			writeGameEvent(t, dir, `{"event":"FSDTarget","timestamp":"2026-01-01T00:00:10Z","Name":"NEW_AFTER_REVOKE_TARGET","SystemAddress":43}`)
			writeGameEvent(t, dir, `{"event":"Scan","SystemAddress":9007199254740993,"BodyID":2,"BodyName":"NEW_AFTER_REVOKE_BODY","PlanetClass":"Rocky body"}`)
			// Both producers must be live after revoke, independently observed by owner.
			seenNav, seenSurvey := false, false
			for !seenNav || !seenSurvey {
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
				if frame.Name == "goNavigationChanged" && strings.Contains(string(raw), "NEW_AFTER_REVOKE_TARGET") {
					seenNav = true
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
					if strings.Contains(string(f.raw), "NEW_AFTER_REVOKE") {
						t.Fatalf("new data authorized after revoke: %s", f.raw)
					}
					var frame struct {
						Name    string
						Message json.RawMessage
					}
					if json.Unmarshal(f.raw, &frame) != nil {
						t.Fatal("invalid frame")
					}
					if frame.Name == "goNavigationChanged" || frame.Name == "goExplorationChanged" {
						assertG2BSafe(t, frame.Name, frame.Message)
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

// Guard the default G2A constructor: new RPCs are not advertised or streamed.
func TestG2BG2AStageRemainsCompatible(t *testing.T) {
	s, _, _, _, _ := gameFixture(t)
	conn, ctx := dialOwner(t, s)
	if len(callRPC(t, ctx, conn, "getGoNavigation")["error"]) == 0 {
		t.Fatal("G2A implicitly opted in")
	}
}

func TestG2BNativeReadinessRequiresBothOwnerViews(t *testing.T) {
	s, _, viewer, url, _ := g2bFixture(t)
	remote := pairedSocket(t, viewer, url)
	owner, ctx := dialOwner(t, s)
	callRPC(t, ctx, remote, "getGoNavigation")
	callRPC(t, ctx, remote, "getGoExploration")
	select {
	case <-s.NavigationClientReady():
		t.Fatal("viewer satisfied native readiness")
	default:
	}
	callRPC(t, ctx, owner, "getGoNavigation")
	select {
	case <-s.NavigationClientReady():
		t.Fatal("single view satisfied dual-view readiness")
	default:
	}
	callRPC(t, ctx, owner, "getGoExploration")
	select {
	case <-s.NavigationClientReady():
	case <-time.After(time.Second):
		t.Fatal("both owner projections were not acknowledged")
	}
}
