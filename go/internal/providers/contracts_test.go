package providers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func reply(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}
func testManager(t *testing.T, tr transportFunc) (*Manager, *time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	m := New(context.Background(), t.TempDir(), Options{Client: &http.Client{Transport: tr}, Vault: &testVault{}, Now: func() time.Time { return now }, Manual: true, InaraAppName: "FixtureApproved", FrontierClientID: "fixture-client"})
	t.Cleanup(m.Close)
	session(t, m)
	return m, &now
}

const jump = `{"timestamp":"2026-11-14T12:00:00Z","event":"FSDJump","StarSystem":"Fixture","SystemAddress":9007199254740993,"StarPos":[1,2,3],"JumpDist":12,"FuelUsed":2,"FID":"PRIVATE","Latitude":123}`

func TestG2HProviderPayloadContractsAndRedaction(t *testing.T) {
	for _, id := range []string{"edsm", "inara", "edastro", "eddn"} {
		t.Run(id, func(t *testing.T) {
			calls := 0
			m, _ := testManager(t, func(r *http.Request) (*http.Response, error) {
				calls++
				b, _ := io.ReadAll(r.Body)
				if strings.Contains(string(b), "PRIVATE") || strings.Contains(string(b), "Latitude") {
					t.Fatalf("private fields: %s", b)
				}
				switch id {
				case "edsm":
					form, _ := url.ParseQuery(string(b))
					if form.Get("apiKey") != "fixture-key" || form.Get("commanderName") != "Fixture CMDR" || form.Get("fromGameVersion") != "4.1.0.0" || !strings.Contains(form.Get("message"), "9007199254740993") {
						t.Fatal("EDSM contract", string(b))
					}
					return reply(`{"msgnum":100}`, 200), nil
				case "inara":
					v := object(b)
					a := v["events"].([]any)
					if a[0].(map[string]any)["eventName"] != "addCommanderTravelFSDJump" {
						t.Fatal("Inara event")
					}
					return reply(`{"header":{"eventStatus":200},"events":[{"eventStatus":200}]}`, 200), nil
				case "edastro":
					var a []any
					json.Unmarshal(b, &a)
					if len(a) != 2 || a[1].(map[string]any)["event"] != "ScanOrganic" {
						t.Fatal("EDAstro envelope")
					}
					return reply(`{"status":200}`, 200), nil
				case "eddn":
					v := object(b)
					if validateEDDN(v) != nil || !strings.Contains(string(b), "9007199254740993") || strings.Contains(string(b), "FuelUsed") {
						t.Fatal("EDDN live schema/identity")
					}
					return reply(``, 200), nil
				}
				panic("unexpected")
			})
			if id == "edsm" || id == "inara" {
				if e := m.SetCredential(id, Credential{APIKey: "fixture-key", Commander: "Fixture CMDR"}); e != nil {
					t.Fatal(e)
				}
			}
			category := "travel"
			raw := jump
			if id == "edastro" {
				category = "biology"
				raw = `{"timestamp":"2026-11-14T12:00:00Z","event":"ScanOrganic","ScanType":"Log","SystemAddress":9007199254740993,"Body":1,"Genus":"Bacterium","Species":"Fixture organic","Latitude":123,"Commander":"PRIVATE"}`
			}
			if e := m.Configure(id, Config{Upload: true, Categories: []string{category}}); e != nil {
				t.Fatal(e)
			}
			m.Observe(event(t, raw), journal.Live)
			m.dispatch(id)
			if calls != 1 {
				t.Fatalf("calls %d", calls)
			}
		})
	}
}
func TestG2HRetryIsBoundedAndErrorsAreNotSuccess(t *testing.T) {
	calls := 0
	m, now := testManager(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return reply(``, 429), nil
		}
		return reply(``, 200), nil
	})
	m.Configure("eddn", Config{Upload: true, Categories: []string{"travel"}})
	m.Observe(event(t, jump), journal.Live)
	m.dispatch("eddn")
	if len(m.states["eddn"].queue) != 1 || !m.states["eddn"].next.After(now.Add(time.Minute)) {
		t.Fatal("missing backoff")
	}
	m.dispatch("eddn")
	if calls != 1 {
		t.Fatal("retry before cooldown")
	}
	*now = now.Add(3 * time.Minute)
	m.dispatch("eddn")
	if calls != 2 || m.states["eddn"].sent != 1 {
		t.Fatal("retry completion")
	}
	if accepted("inara", []byte(`{"header":{"eventStatus":200},"events":[{"eventStatus":400}]}`)) || accepted("edsm", []byte(`{"msgnum":203}`)) {
		t.Fatal("error misreported as success")
	}
}
func TestG2HQueueBoundsIdentityAndLiveVersion(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network"); return nil, nil })
	m.Configure("eddn", Config{Upload: true, Categories: []string{"travel"}})
	for i := 0; i < 100; i++ {
		e := event(t, strings.Replace(jump, `"JumpDist":12`, `"JumpDist":`+strconv.Itoa(i), 1))
		m.Observe(e, journal.Live)
	}
	if len(m.states["eddn"].queue) != 64 {
		t.Fatal("queue not bounded")
	}
	m.Observe(event(t, `{"event":"LoadGame","FID":"SYNTHETIC-B","Commander":"Other"}`), journal.Live)
	m.Observe(event(t, jump), journal.Live)
	if len(m.states["eddn"].queue) != 64 {
		t.Fatal("other commander queued")
	}
	m.Configure("eddn", Config{})
	if len(m.states["eddn"].queue) != 0 {
		t.Fatal("queue not cleared")
	}
}
func TestG2HOAuthPKCEStateRedirectExpiryAndOneUse(t *testing.T) {
	calls := 0
	expectedChallenge := ""
	m, _ := testManager(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "auth.frontierstore.net" {
			t.Fatal("token endpoint")
		}
		b, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(b))
		sum := sha256.Sum256([]byte(form.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != expectedChallenge || form.Get("redirect_uri") != "http://127.0.0.1:12345/owner/frontier-callback" {
			t.Fatal("PKCE binding")
		}
		return reply(`{"access_token":"ACCESS_SECRET","refresh_token":"REFRESH_SECRET","token_type":"Bearer","expires_in":3600}`, 200), nil
	})
	m.Configure("frontier", Config{Read: true})
	if _, err := m.BeginAuthorization("http://evil.invalid:12345"); err == nil {
		t.Fatal("untrusted redirect accepted")
	}
	u, err := m.BeginAuthorization("http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	auth, _ := url.Parse(u)
	q := auth.Query()
	expectedChallenge = q.Get("code_challenge")
	if q.Get("scope") != "auth capi" || q.Get("code_challenge_method") != "S256" {
		t.Fatal("scopes/PKCE")
	}
	redirect := "http://127.0.0.1:12345/owner/frontier-callback"
	if m.CompleteAuthorization(context.Background(), redirect, "WRONG", "code") == nil || calls != 0 {
		t.Fatal("CSRF failed")
	}
	if err := m.CompleteAuthorization(context.Background(), redirect, q.Get("state"), "code"); err != nil {
		t.Fatal(err)
	}
	if m.CompleteAuthorization(context.Background(), redirect, q.Get("state"), "code") == nil || calls != 1 {
		t.Fatal("authorization reused")
	}
	raw, _ := json.Marshal(m.Status())
	if strings.Contains(string(raw), "ACCESS_SECRET") || strings.Contains(string(raw), "REFRESH_SECRET") {
		t.Fatal("token returned")
	}
	m.BeginAuthorization("http://127.0.0.1:12345")
	state := m.oauth.state
	m.oauth.expires = m.opts.Now().Add(-time.Second)
	if m.CompleteAuthorization(context.Background(), redirect, state, "code") == nil {
		t.Fatal("expired state")
	}
}
func TestG2HReadSnapshotsAndGECRemainSupplemental(t *testing.T) {
	m, _ := testManager(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/gec/json/all":
			return reply(`[{"name":"Fixture Nebula","type":["nebula"],"coords":{"x":1,"y":2,"z":3},"secret":"PRIVATE"},{"name":"Invalid","coords":{"x":"1","y":2,"z":3}}]`, 200), nil
		case "/api-logs-v1/get-logs":
			return reply(`{"msgnum":100,"logs":[{"system":"Fixture","date":"2026-11-14 12:00:00","secret":"PRIVATE"}]}`, 200), nil
		}
		t.Fatal("unexpected read endpoint")
		return nil, nil
	})
	if _, e := m.Read(context.Background(), "edastro", "gec", ""); e == nil {
		t.Fatal("default read")
	}
	m.Configure("edastro", Config{Read: true})
	v, err := m.Read(context.Background(), "edastro", "gec", "")
	if err != nil || v.Source != "edastro / gec" {
		t.Fatal(err)
	}
	l := m.Layers()
	if len(l.Points) != 1 || l.Points[0].Category != "nebula" {
		t.Fatalf("layers %+v", l)
	}
	l.Points[0].Name = "mutated"
	if m.Layers().Points[0].Name == "mutated" {
		t.Fatal("mutable cache")
	}
	m.Configure("edastro", Config{})
	if len(m.Layers().Points) > 0 {
		t.Fatal("disabled cache retained")
	}
	m.SetCredential("edsm", Credential{APIKey: "fixture", Commander: "Fixture CMDR"})
	m.Configure("edsm", Config{Read: true})
	v, err = m.Read(context.Background(), "edsm", "flightlog", "")
	if err != nil || len(v.Facts) != 1 {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "PRIVATE") {
		t.Fatal("untyped snapshot")
	}
}
func TestG2HVaultEncryptsAndRejectsCorruption(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Linux vault requires physical unlocked Secret Service; fail-closed missing-service path is tested separately")
	}
	dir := t.TempDir()
	v := newVault(dir)
	if err := v.Save(map[string]Credential{"edsm": {APIKey: "PRIVATE_KEY", Commander: "Fixture"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := privatefs.Read(dir, "provider-vault.bin")
	if err != nil || strings.Contains(string(raw), "PRIVATE_KEY") {
		t.Fatal("plaintext vault")
	}
	got, err := v.Load()
	if err != nil || got["edsm"].APIKey != "PRIVATE_KEY" {
		t.Fatal("vault roundtrip")
	}
	privatefs.Write(dir, "provider-vault.bin", []byte("corrupt"))
	if v.Save(map[string]Credential{}) == nil {
		t.Fatal("corrupt vault overwritten")
	}
	_, _ = os.Stat(dir)
}

func TestG2HBatchesPreserveEngineeringAndExactIDs(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) { return reply(``, 200), nil })
	raw := `{"timestamp":"2026-11-14T12:00:00Z","event":"Loadout","Ship":"asp","ShipID":9007199254740993,"Modules":[{"Slot":"MainEngines","Item":"engine","On":true,"Priority":2,"Engineering":{"BlueprintName":"Fixture","Level":5,"Quality":1,"ExperimentalEffect":"FixtureEffect","Modifiers":[{"Label":"PowerDraw","Value":6,"OriginalValue":5,"LessIsGood":true}]}}]}`
	e := event(t, raw)
	b, _, _, err := m.encode("inara", e, object(e.Raw), Credential{APIKey: "fixture", Commander: "Fixture CMDR"})
	if err != nil || !strings.Contains(string(b), `"isOn":true`) || !strings.Contains(string(b), `"itemPriority":2`) || !strings.Contains(string(b), `"engineering"`) || strings.Contains(string(b), `"isEnabled"`) {
		t.Fatalf("loadout contract %s %v", b, err)
	}
	queue := []queued{{body: b, generation: 1}, {body: b, generation: 1}}
	batch, body := batchRequest("inara", queue)
	if len(batch) != 2 || len(object(body)["events"].([]any)) != 2 || !strings.Contains(string(body), "9007199254740993") {
		t.Fatal("batch identity/ordering")
	}
}
func TestG2HSourceGapStopsUploadsAndSchemaRejectsPrivateFields(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network"); return nil, nil })
	m.Configure("eddn", Config{Upload: true, Categories: []string{"travel"}})
	e := event(t, jump)
	m.sourceFile = "fixture"
	m.sourceSequence = 3
	e.Position.Sequence = 5
	m.Observe(e, journal.Live)
	if len(m.states["eddn"].queue) > 0 {
		t.Fatal("source gap retained identity")
	}
	v := map[string]any{"$schemaRef": "https://eddn.edcd.io/schemas/journal/1", "header": map[string]any{"uploaderID": "fixture", "softwareName": "fixture", "softwareVersion": "1"}, "message": map[string]any{"timestamp": "2026-11-14T12:00:00Z", "event": "FSDJump", "StarSystem": "Fixture", "StarPos": []any{json.Number("1"), json.Number("2"), json.Number("3")}, "SystemAddress": json.Number("9007199254740993"), "Latitude": json.Number("1")}}
	if validateEDDN(v) == nil {
		t.Fatal("live schema accepted forbidden location")
	}
}

func TestG2HLinuxVaultHasNoPlaintextFallback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Secret Service path is Linux only")
	}
	t.Setenv("PATH", "")
	dir := t.TempDir()
	v := newVault(dir)
	if v.Save(map[string]Credential{"edsm": {APIKey: "PRIVATE_KEY"}}) == nil {
		t.Fatal("credential accepted without Secret Service")
	}
	if _, err := os.Stat(dir + "/provider-vault.bin"); !os.IsNotExist(err) {
		t.Fatal("vault created without OS protection")
	}
}
func BenchmarkG2HDisabledProviderObserve(b *testing.B) {
	m := New(context.Background(), b.TempDir(), Options{Vault: &testVault{}, Manual: true})
	defer m.Close()
	e, _ := journal.ParseLine([]byte(jump), journal.SourcePosition{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Observe(e, journal.Live)
	}
}

func TestG2HSupplementalSnapshotsDoNotFollowAnotherCommander(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) {
		return reply(`{"msgnum":100,"logs":[{"system":"Fixture","date":"2026-11-14"}]}`, 200), nil
	})
	m.SetCredential("edsm", Credential{APIKey: "fixture", Commander: "Fixture CMDR"})
	m.Configure("edsm", Config{Read: true})
	if _, err := m.Read(context.Background(), "edsm", "flightlog", ""); err != nil {
		t.Fatal(err)
	}
	if !m.Status()[0].Snapshot.ActiveCommander {
		t.Fatal("matching snapshot not attributed")
	}
	m.Observe(event(t, `{"event":"LoadGame","FID":"SYNTHETIC-B","Commander":"Other"}`), journal.Live)
	if m.Status()[0].Snapshot.ActiveCommander {
		t.Fatal("supplemental snapshot leaked commander attribution")
	}
}

type blockingVault struct{ started, release chan struct{} }

func (v *blockingVault) Load() (map[string]Credential, error) { return map[string]Credential{}, nil }
func (v *blockingVault) Save(map[string]Credential) error     { close(v.started); <-v.release; return nil }
func TestG2HVaultSetupCannotBlockJournalPipeline(t *testing.T) {
	v := &blockingVault{make(chan struct{}), make(chan struct{})}
	m := New(context.Background(), t.TempDir(), Options{Vault: v, Manual: true})
	defer m.Close()
	finished := make(chan struct{})
	go func() { m.SetCredential("edsm", Credential{APIKey: "fixture", Commander: "Fixture"}); close(finished) }()
	<-v.started
	defer func() { close(v.release); <-finished }()
	observed := make(chan struct{})
	go func() {
		m.Observe(event(t, `{"event":"LoadGame","FID":"SYNTHETIC-A","Commander":"Fixture"}`), journal.Replay)
		close(observed)
	}()
	select {
	case <-observed:
	case <-time.After(time.Second):
		t.Fatal("credential vault blocked the shared reader")
	}
}
