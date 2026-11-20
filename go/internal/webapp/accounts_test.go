package webapp

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/history"
	"github.com/Jakoczig/eridion/go/internal/providers"
	"github.com/coder/websocket"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPairedAccountGrantCannotFollowGlobalProfile(t *testing.T) {
	s, owner, client, remoteURL, code := remoteFixture(t)
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code}); status != 200 {
		t.Fatal("pair", status)
	}
	conn := pairedSocket(t, client, remoteURL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, name := range []string{"getGoAccounts", "setGoAccounts", "getGoCommanderOverview", "setGoAvatar", "chooseGoAvatar", "setGoFrontierClient", "completeGoFrontierReturn"} {
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
	_ = ownerAction(t, s, owner, map[string]any{"action": "pair"})
	s.remote.mu.Lock()
	initialPairing := s.remote.pairing
	s.remote.mu.Unlock()
	s.remote.selectAccount("SYNTHETIC-A") // adopt the pre-account grant to A
	if reply := callRPC(t, ctx, conn, "getLoadingStatus"); len(reply["error"]) != 0 {
		t.Fatal("initial same-profile binding dropped a valid socket")
	}
	s.remote.mu.Lock()
	if s.remote.pairing != initialPairing {
		t.Error("initial binding invalidated an owner-issued pairing attempt")
	}
	s.remote.mu.Unlock()
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

func TestHistoryRestoreCannotTemporarilySelectAnotherViewerAccount(t *testing.T) {
	s, _, _, _, _ := remoteFixture(t)
	s.remote.selectAccount("SYNTHETIC-SELECTED-B")
	s.restoreProviderHistory(nil, history.ProviderSeed{FID: "SYNTHETIC-READER-A"})
	s.remote.mu.Lock()
	defer s.remote.mu.Unlock()
	if s.remote.state.Account != tokenHash("SYNTHETIC-SELECTED-B") {
		t.Fatal("history restore replaced the selected viewer account with the cached reader account")
	}
}

func TestAmbiguousLegacyGrantDoesNotBindFirstCommanderAndSurvivesRestart(t *testing.T) {
	s, owner, client, remoteURL, code := remoteFixture(t)
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code}); status != 200 {
		t.Fatal(status)
	}
	u, err := url.Parse(remoteURL)
	if err != nil {
		t.Fatal(err)
	}
	cookies := client.Jar.Cookies(u)
	if len(cookies) != 1 {
		t.Fatal("missing paired token")
	}
	oldToken := cookies[0].Value
	// Simulate a clean v1 shutdown and restart: the original A grant has no FID.
	s.remote.mu.Lock()
	s.remote.state.Active = false
	if e := s.remote.saveLocked(); e != nil {
		t.Fatal(e)
	}
	s.remote.mu.Unlock()
	restarted := &remoteManager{dir: s.remote.dir, now: time.Now, connections: map[*websocket.Conn]string{}}
	if e := restarted.load(); e != nil {
		t.Fatal(e)
	}
	restarted.mu.Lock()
	_, before := restarted.findLocked(oldToken)
	restarted.mu.Unlock()
	if before {
		t.Fatal("unverified original token accepted before association")
	}
	// Neither first active B nor later owner selection A is proof of grant ownership.
	restarted.selectAccount("SYNTHETIC-B")
	restarted.mu.Lock()
	if len(restarted.state.Devices) != 1 || restarted.state.Devices[0].Account != "" {
		t.Fatal("ambiguous A grant silently assigned to first B")
	}
	state := restarted.state
	restarted.mu.Unlock()
	restarted.selectAccount("SYNTHETIC-A")
	restarted.mu.Lock()
	if restarted.state.Devices[0].Account != "" {
		t.Fatal("owner selection silently authorized ambiguous grant")
	}
	restarted.state.Active = false
	if e := restarted.saveLocked(); e != nil {
		t.Fatal(e)
	}
	restarted.mu.Unlock()
	again := &remoteManager{dir: s.remote.dir, now: time.Now, connections: map[*websocket.Conn]string{}}
	if e := again.load(); e != nil {
		t.Fatal("quarantined grant cannot restart", e)
	}
	if again.state.Devices[0].Account != "" || again.state.Devices[0].Hash != state.Devices[0].Hash {
		t.Fatal("restart lost or rebound trust metadata")
	}
	// Install the restarted state in the live fixture, keeping the actual TLS/auth paths.
	s.remote.mu.Lock()
	s.remote.state = again.state
	s.remote.mu.Unlock()
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil); status != http.StatusUnauthorized {
		t.Fatal("legacy A recovered selected account", status)
	}

	pairing := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	json.Unmarshal(pairing["code"], &code)
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code}); status != 200 {
		t.Fatal("explicit re-pair", status)
	}
	conn := pairedSocket(t, client, remoteURL)
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if r := callRPC(t, ctx, conn, "getLoadingStatus"); len(r["error"]) != 0 {
		t.Fatal("new grant unusable")
	}
	s.remote.mu.Lock()
	id := s.remote.state.Devices[1].ID
	oldID := s.remote.state.Devices[0].ID
	s.remote.mu.Unlock()
	ownerAction(t, s, owner, map[string]any{"action": "revoke", "deviceId": id})
	if status, _, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil); status != 401 {
		t.Fatal("revoked grant recovered", status)
	}
	ownerAction(t, s, owner, map[string]any{"action": "revoke", "deviceId": oldID})
	s.remote.mu.Lock()
	defer s.remote.mu.Unlock()
	if len(s.remote.state.Devices) != 0 {
		t.Fatal("quarantined grant cannot be revoked")
	}
}

func TestUnambiguousLegacyBindingPreservesTokenAfterRestart(t *testing.T) {
	s, _, _, _, _ := remoteFixture(t)
	m := s.remote
	m.mu.Lock()
	token := strings.Repeat("a", 64)
	m.state.Devices = []device{{ID: strings.Repeat("b", 32), Name: "Single account laptop", Expires: time.Now().Add(time.Hour), Hash: tokenHash(token)}}
	m.state.Active = false
	if e := m.saveLocked(); e != nil {
		t.Fatal(e)
	}
	m.mu.Unlock()
	next := &remoteManager{dir: m.dir, now: time.Now, connections: map[*websocket.Conn]string{}}
	if e := next.load(); e != nil {
		t.Fatal(e)
	}
	next.mu.Lock()
	if _, ok := next.findLocked(token); ok {
		t.Fatal("unverified loaded grant active before proof")
	}
	next.mu.Unlock()
	next.selectAccount("SYNTHETIC-A", true)
	next.mu.Lock()
	if _, ok := next.findLocked(token); !ok {
		t.Fatal("verified single account lost legitimate grant")
	}
	next.state.Active = false
	if e := next.saveLocked(); e != nil {
		t.Fatal(e)
	}
	next.mu.Unlock()
	again := &remoteManager{dir: m.dir, now: time.Now, connections: map[*websocket.Conn]string{}}
	if e := again.load(); e != nil {
		t.Fatal(e)
	}
	again.mu.Lock()
	defer again.mu.Unlock()
	if _, ok := again.findLocked(token); !ok {
		t.Fatal("verified same-account grant lost on restart")
	}
}

type accountTestVault struct {
	data map[string]providers.Credential
}

func (v *accountTestVault) Load() (map[string]providers.Credential, error) { return v.data, nil }
func (v *accountTestVault) Save(data map[string]providers.Credential) error {
	v.data = data
	return nil
}
func TestOwnerAccountConsentFailureRollsBackAndRetriesWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	writeGameEvent(t, dir, `{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"SYNTHETIC-A"}`)
	writeGameEvent(t, dir, `{"event":"Location","timestamp":"2026-01-01T00:00:02Z","StarSystem":"A"}`)
	writeGameEvent(t, dir, `{"event":"LoadGame","timestamp":"2026-01-01T00:00:03Z","FID":"SYNTHETIC-B"}`)
	writeGameEvent(t, dir, `{"event":"Location","timestamp":"2026-01-01T00:00:04Z","StarSystem":"B"}`)
	s, e := StartLocalWithOptions(context.Background(), fixtureFS(), Options{StorageDir: t.TempDir(), JournalDir: dir, EnableJournal: true, EnableNavigation: true, EnableExploration: true, EnableDashboard: true, EnableParity: true, ProviderOptions: providers.Options{Vault: &accountTestVault{}, Manual: true}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close(context.Background())
	deadline := time.Now().Add(3 * time.Second)
	for v := s.game.Snapshot(); v.ImportInProgress || v.EventsRead != 4; v = s.game.Snapshot() {
		if time.Now().After(deadline) {
			t.Fatal("journal replay not ready")
		}
		time.Sleep(time.Millisecond)
	}
	selectAccount := func(fid string) error {
		raw, _ := json.Marshal(map[string]any{"multiple": true, "selected": fid})
		_, e := s.accountRPC(context.Background(), "setGoAccounts", raw)
		return e
	}
	if e = selectAccount("SYNTHETIC-A"); e != nil {
		t.Fatal(e)
	}
	if e = s.providers.SetCredential("edsm", providers.Credential{APIKey: "A-PRIVATE", Commander: "A"}); e != nil {
		t.Fatal(e)
	}
	if e = s.providers.Configure("edsm", providers.Config{Read: true}); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite3", filepath.Join(s.profile, "daedalus.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(`CREATE TRIGGER fail_consent BEFORE INSERT ON app_state WHEN NEW.key='provider-ledger' BEGIN SELECT RAISE(FAIL,'injected consent failure'); END`); e != nil {
		t.Fatal(e)
	}
	if e = selectAccount("SYNTHETIC-B"); e == nil {
		t.Fatal("injected switch succeeded")
	}
	if s.game.Accounts().Selected != "SYNTHETIC-A" || s.game.FieldCommander() != "SYNTHETIC-A" {
		t.Fatal("UI selection/pin did not roll back")
	}
	// With the trigger still failing this can only succeed if A's namespace was retained.
	if e = s.providers.ActivateAccount("SYNTHETIC-A", false); e != nil {
		t.Fatal("provider scope did not roll back", e)
	}
	if e = s.providers.ActivateAccount("SYNTHETIC-B", false); e == nil {
		t.Fatal("previous owner pin not restored")
	}
	for _, v := range s.providers.Status() {
		if v.Capability.ID == "edsm" && (!v.CredentialStored || !v.Config.Read) {
			t.Fatal("A credential/consent not retained")
		}
	}
	if _, e = db.Exec(`DROP TRIGGER fail_consent`); e != nil {
		t.Fatal(e)
	}
	if e = selectAccount("SYNTHETIC-B"); e != nil {
		t.Fatal("owner retry", e)
	}
	if s.game.Accounts().Selected != "SYNTHETIC-B" {
		t.Fatal("retry selection")
	}
	for _, v := range s.providers.Status() {
		if v.Capability.ID == "edsm" && (v.CredentialStored || v.Config.Read) {
			t.Fatal("retry exposed A credential/consent to B")
		}
	}
	if e = selectAccount("SYNTHETIC-A"); e != nil {
		t.Fatal(e)
	}
	for _, v := range s.providers.Status() {
		if v.Capability.ID == "edsm" && (!v.CredentialStored || !v.Config.Read) {
			t.Fatal("successful retry lost original A namespace")
		}
	}
}
