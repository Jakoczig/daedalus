package webapp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/journal"
	"github.com/coder/websocket"
)

func writeGameEvent(t *testing.T, dir, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "Journal.20260101.01.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}
func gameFixture(t *testing.T) (*Server, *http.Client, *http.Client, string, string) {
	t.Helper()
	dir := t.TempDir()
	writeGameEvent(t, dir, `{"event":"LoadGame","FID":"SYNTHETIC-PRIVATE","Commander":"PRIVATE_COMMANDER"}`)
	writeGameEvent(t, dir, `{"event":"Location","StarSystem":"Fixture System","SystemAddress":9223372036854775807}`)
	s, err := StartLocalWithOptions(context.Background(), fixtureFS(), Options{StorageDir: t.TempDir(), JournalDir: dir, EnableJournal: true, Discovery: journal.DiscoveryOptions{Roots: []string{}}, PollInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(context.Background()) })
	owner := ownerClient(t, s)
	status := ownerAction(t, s, owner, map[string]any{"action": "enable", "address": "127.0.0.1", "port": 0})
	var remoteURL, ca string
	json.Unmarshal(status["url"], &remoteURL)
	json.Unmarshal(status["ca"], &ca)
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(ca))
	jar, _ := cookiejar.New(nil)
	viewer := &http.Client{Jar: jar, Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{RootCAs: roots}}}
	t.Cleanup(viewer.CloseIdleConnections)
	issued := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	var code string
	json.Unmarshal(issued["code"], &code)
	result, _, _ := remotePost(t, viewer, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code, "name": "Fixture tablet"})
	if result != 200 {
		t.Fatal("pairing", result)
	}
	deadline := time.Now().Add(3 * time.Second)
	for s.game.Snapshot().CurrentSystem != "Fixture System" {
		if time.Now().After(deadline) {
			t.Fatal("journal did not replay")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return s, owner, viewer, remoteURL, dir
}
func assertSafeSummary(t *testing.T, raw []byte) {
	t.Helper()
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("invalid summary")
	}
	allowed := map[string]bool{"sourceState": true, "currentSystem": true, "lastEventType": true, "lastEventAt": true, "importInProgress": true, "eventsRead": true}
	if len(fields) != len(allowed) {
		t.Fatalf("unexpected public schema: %s", raw)
	}
	for key := range fields {
		if !allowed[key] {
			t.Fatal("private field", key)
		}
	}
	for _, secret := range []string{"SYNTHETIC-PRIVATE", "PRIVATE_COMMANDER", "CHAT_SENTINEL", "Journal.", "FID", "directory", "Raw"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("private journal data leaked", secret)
		}
	}
}
func TestGameFeedOwnerViewerSanitizedRPC(t *testing.T) {
	s, _, viewer, url, _ := gameFixture(t)
	ownerConn, ctx := dialOwner(t, s)
	remoteConn := pairedSocket(t, viewer, url)
	for _, conn := range []*websocket.Conn{ownerConn, remoteConn} {
		reply := callRPC(t, ctx, conn, "getJournalSummary")
		if len(reply["error"]) != 0 {
			t.Fatal(string(reply["error"]))
		}
		assertSafeSummary(t, reply["message"])
		if !strings.Contains(string(reply["message"]), "Fixture System") {
			t.Fatal("current system missing")
		}
	}
	status := callRPC(t, ctx, ownerConn, "getLoadingStatus")
	if !strings.Contains(string(status["message"]), `"stage":"G2A"`) || !strings.Contains(string(status["message"]), `"offline":false`) {
		t.Fatal("dishonest live stage")
	}
}
func TestViewerCannotChangeJournalDirectory(t *testing.T) {
	s, owner, viewer, url, _ := gameFixture(t)
	for _, action := range []string{"status", "selectDir", "clearOverride"} {
		status, _, _ := remotePost(t, viewer, url+"/owner/journal", url, map[string]any{"action": action, "directory": t.TempDir()})
		if status != 403 {
			t.Fatal("viewer filesystem control accepted", status)
		}
	}
	for _, origin := range []string{"", "null", "https://evil.invalid"} {
		status, _, _ := remotePost(t, owner, s.URL()+"/owner/journal", origin, map[string]any{"action": "status"})
		if status != 403 {
			t.Fatal("owner CSRF accepted", status)
		}
	}
	status, body, _ := remotePost(t, owner, s.URL()+"/owner/journal", s.URL(), map[string]any{"action": "status"})
	if status != 200 || !strings.Contains(string(body), "Directory") {
		t.Fatal("owner source details unavailable", status)
	}
	conn := pairedSocket(t, viewer, url)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, name := range []string{"selectJournalDirectory", "getLogEntries", "getRawJournal", "ReceiveText", "constructor"} {
		if len(callRPC(t, ctx, conn, name)["error"]) == 0 {
			t.Fatal("unsafe RPC accepted", name)
		}
	}
}
func TestRawReceiveTextAndFIDNeverBroadcast(t *testing.T) {
	s, _, viewer, url, dir := gameFixture(t)
	conn := pairedSocket(t, viewer, url)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	callRPC(t, ctx, conn, "getJournalSummary")
	before := s.game.Snapshot().EventsRead
	writeGameEvent(t, dir, `{"event":"ReceiveText","Message":"CHAT_SENTINEL","FID":"SYNTHETIC-PRIVATE","StarSystem":"NOT_A_SYSTEM"}`)
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var frame struct {
			Name    string
			Message json.RawMessage
		}
		json.Unmarshal(raw, &frame)
		if frame.Name == "journalSummaryChanged" {
			assertSafeSummary(t, frame.Message)
			var v struct{ EventsRead uint64 }
			json.Unmarshal(frame.Message, &v)
			if v.EventsRead > before {
				break
			}
		}
	}
}
func TestViewerReconnectDoesNotStartSecondReader(t *testing.T) {
	s, _, viewer, url, _ := gameFixture(t)
	starts := s.game.Details().ReaderStarts
	for range 3 {
		conn := pairedSocket(t, viewer, url)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		callRPC(t, ctx, conn, "getJournalSummary")
		cancel()
		conn.CloseNow()
	}
	if s.game.Details().ReaderStarts != starts || s.game.Details().ActiveReaders != 1 {
		t.Fatal("browser started another source reader")
	}
}
func TestRevocationStopsGameFeed(t *testing.T) {
	s, owner, viewer, url, dir := gameFixture(t)
	conn := pairedSocket(t, viewer, url)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	callRPC(t, ctx, conn, "getJournalSummary")
	listed := ownerAction(t, s, owner, map[string]any{"action": "status"})
	var devices []struct{ ID string }
	json.Unmarshal(listed["devices"], &devices)
	ownerAction(t, s, owner, map[string]any{"action": "revoke", "deviceId": devices[0].ID})
	writeGameEvent(t, dir, `{"event":"FSDJump","StarSystem":"Fixture After Revoke"}`)
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("revoked subscription remained active")
	}
}
func TestBadJournalOverrideAndShutdownKeepOwnerSafe(t *testing.T) {
	profile := t.TempDir()
	s, err := StartLocalWithOptions(context.Background(), fixtureFS(), Options{StorageDir: profile, JournalDir: filepath.Join(t.TempDir(), "missing"), EnableJournal: true, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal("optional journal blocked desktop", err)
	}
	defer s.Close(context.Background())
	owner := ownerClient(t, s)
	status, _, _ := remotePost(t, owner, s.URL()+"/owner/journal", s.URL(), map[string]any{"action": "selectDir", "directory": "relative"})
	if status != 400 {
		t.Fatal("invalid directory accepted")
	}
	good := t.TempDir()
	status, _, _ = remotePost(t, owner, s.URL()+"/owner/journal", s.URL(), map[string]any{"action": "selectDir", "directory": good})
	if status != 200 {
		t.Fatal("valid source selection failed", status)
	}
	if saved, err := journal.LoadOverride(profile); err != nil || saved != good {
		t.Fatal("source not protected/durable", err)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.game.Details().ActiveReaders != 0 {
		t.Fatal("reader survived shutdown")
	}
	restarted, err := StartLocalWithOptions(context.Background(), fixtureFS(), Options{StorageDir: profile, EnableJournal: true, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close(context.Background())
	res, err := http.Get(restarted.URL())
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
}

func TestSlowJournalSelectionCannotWriteAfterProfileReleased(t *testing.T) {
	profile, original, target := t.TempDir(), t.TempDir(), t.TempDir()
	if err := journal.SaveOverride(profile, original); err != nil {
		t.Fatal(err)
	}
	s, err := StartLocalWithOptions(context.Background(), fixtureFS(), Options{StorageDir: profile, EnableJournal: true, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	owner := ownerClient(t, s)
	value, _ := json.Marshal(map[string]any{"action": "selectDir", "directory": target})
	body := &gatedBody{entered: make(chan struct{}), release: make(chan struct{}), Reader: strings.NewReader(string(value))}
	var once sync.Once
	release := func() { once.Do(func() { close(body.release) }) }
	defer release()
	req := httptest.NewRequest("POST", s.URL()+"/owner/journal", body)
	req.Header.Set("Origin", s.URL())
	parsed, _ := url.Parse(s.URL())
	for _, cookie := range owner.Jar.Cookies(parsed) {
		req.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); s.ownerJournal(response, req) }()
	<-body.entered
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	replacement, err := StartLocalWithOptions(context.Background(), fixtureFS(), Options{StorageDir: profile, EnableJournal: true, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close(context.Background())
	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stale owner request stuck")
	}
	if saved, _ := journal.LoadOverride(profile); saved != original {
		t.Fatal("stale handler changed another owner's locked profile")
	}
}

func TestJournalNativeReadinessRequiresOwnerCardRequest(t *testing.T) {
	s, _, viewer, url, _ := gameFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	remote := pairedSocket(t, viewer, url)
	callRPC(t, ctx, remote, "getJournalSummary")
	select {
	case <-s.JournalClientReady():
		t.Fatal("viewer spoofed native journal readiness")
	default:
	}
	owner, ownerCtx := dialOwner(t, s)
	callRPC(t, ownerCtx, owner, "getJournalSummary")
	select {
	case <-s.JournalClientReady():
	case <-ctx.Done():
		t.Fatal("native card request not observed")
	}
}
