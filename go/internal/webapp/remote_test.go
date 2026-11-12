package webapp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func ownerAction(t *testing.T, s *Server, client *http.Client, value any) map[string]json.RawMessage {
	t.Helper()
	raw, _ := json.Marshal(value)
	req, _ := http.NewRequest("POST", s.URL()+"/owner/remote", strings.NewReader(string(raw)))
	req.Header.Set("Origin", s.URL())
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("owner action %v: %d %s", value, res.StatusCode, data)
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(data, &result) != nil {
		t.Fatalf("invalid owner result: %s", data)
	}
	return result
}
func remoteFixture(t *testing.T) (*Server, *http.Client, *http.Client, string, string) {
	t.Helper()
	s, err := StartLocalWithStorage(context.Background(), fixtureFS(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(context.Background()) })
	owner := ownerClient(t, s)
	status := ownerAction(t, s, owner, map[string]any{"action": "status"})
	if string(status["enabled"]) != "false" {
		t.Fatal("remote enabled by default")
	}
	status = ownerAction(t, s, owner, map[string]any{"action": "enable", "address": "127.0.0.1", "port": 0})
	var remoteURL, ca string
	json.Unmarshal(status["url"], &remoteURL)
	json.Unmarshal(status["ca"], &ca)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(ca)) {
		t.Fatal("missing root")
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 4 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, Proxy: nil}}
	t.Cleanup(client.CloseIdleConnections)
	pairing := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	var code string
	json.Unmarshal(pairing["code"], &code)
	return s, owner, client, remoteURL, code
}
func remotePost(t *testing.T, client *http.Client, endpoint, origin string, body any) (int, []byte, []*http.Cookie) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(string(raw)))
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data, res.Cookies()
}
func pairedSocket(t *testing.T, client *http.Client, remoteURL string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(remoteURL, "https"), &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {remoteURL}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func TestRemotePairingBoundaryAndRevocation(t *testing.T) {
	s, owner, client, remoteURL, code := remoteFixture(t)
	status, _, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, map[string]any{"role": "owner"})
	if status != 401 {
		t.Fatal("anonymous remote bootstrap minted owner", status)
	}
	for _, origin := range []string{"", "null", "https://evil.example"} {
		status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", origin, map[string]any{"code": code})
		if status != 403 {
			t.Fatal("untrusted origin accepted", status)
		}
	}
	status, data, cookies := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code, "name": "Tablet", "role": "owner"})
	if status != 200 || !strings.Contains(string(data), `"role":"viewer"`) {
		t.Fatalf("pair: %d %s", status, data)
	}
	if len(cookies) != 1 || cookies[0].Name != "__Host-daedalus_viewer" || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe viewer cookie", cookies)
	}
	status, _, _ = remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code})
	if status == 200 {
		t.Fatal("code reused")
	}
	status, data, _ = remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil)
	if status != 200 || !strings.Contains(string(data), `"role":"viewer"`) {
		t.Fatal("viewer recovery failed")
	}
	status, _, _ = remotePost(t, client, remoteURL+"/owner/remote", remoteURL, map[string]any{"action": "pair"})
	if status != 403 {
		t.Fatal("viewer owner action accepted")
	}
	conn := pairedSocket(t, client, remoteURL)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var host struct{ URLs []string }
	json.Unmarshal(callRPC(t, ctx, conn, "hostInfo")["message"], &host)
	if len(host.URLs) != 1 || host.URLs[0] != remoteURL {
		t.Fatal("owner URL leaked", host)
	}
	for _, name := range []string{"setPreferences", "getPreferences", "createPairingCode", "constructor", "__proto__", "testMessage", "beginCapiAuth"} {
		if len(callRPC(t, ctx, conn, name)["error"]) == 0 {
			t.Fatal("privileged RPC accepted", name)
		}
	}
	list := ownerAction(t, s, owner, map[string]any{"action": "status"})
	var devices []struct{ ID, Name string }
	json.Unmarshal(list["devices"], &devices)
	if len(devices) != 1 || devices[0].Name != "Tablet" {
		t.Fatal("device list invalid", devices)
	}
	if strings.Contains(string(list["devices"]), cookies[0].Value) || strings.Contains(string(list["devices"]), "token") {
		t.Fatal("device secret leaked")
	}
	ownerAction(t, s, owner, map[string]any{"action": "rename", "deviceId": devices[0].ID, "name": "Laptop"})
	ownerAction(t, s, owner, map[string]any{"action": "revoke", "deviceId": devices[0].ID})
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("revoked socket survived")
	}
	status, _, _ = remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil)
	if status != 401 {
		t.Fatal("revoked cookie recovered")
	}
}

func TestRemoteCookiesHostOriginAndTLS(t *testing.T) {
	s, owner, client, remoteURL, _ := remoteFixture(t)
	local, _ := url.Parse(s.URL())
	remote, _ := url.Parse(remoteURL)
	for _, cookie := range owner.Jar.Cookies(local) {
		client.Jar.SetCookies(remote, []*http.Cookie{cookie})
	}
	status, _, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil)
	if status != 401 {
		t.Fatal("local owner cookie authorized TLS")
	}
	for _, header := range []struct{ name, value string }{{"Host", "evil.example"}, {"X-Forwarded-For", "127.0.0.1"}, {"X-Forwarded-Proto", "https"}, {"Forwarded", "for=127.0.0.1"}} {
		req, _ := http.NewRequest("GET", remoteURL, nil)
		if header.name == "Host" {
			req.Host = header.value
		} else {
			req.Header.Set(header.name, header.value)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatal("forged host/proxy accepted", header)
		}
	}
	untrusted := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	if res, err := untrusted.Get(remoteURL); err == nil {
		res.Body.Close()
		t.Fatal("untrusted CA accepted")
	}
	res, err := client.Get(strings.Replace(remoteURL, "https:", "http:", 1))
	if err == nil {
		res.Body.Close()
		if res.StatusCode == 200 {
			t.Fatal("plaintext service exposed")
		}
	}
}

func TestRemoteCleanRestartAndPrivateHashes(t *testing.T) {
	dir := t.TempDir()
	s, err := StartLocalWithStorage(context.Background(), fixtureFS(), dir)
	if err != nil {
		t.Fatal(err)
	}
	owner := ownerClient(t, s)
	status := ownerAction(t, s, owner, map[string]any{"action": "enable", "address": "127.0.0.1", "port": 0})
	var remoteURL, ca string
	json.Unmarshal(status["url"], &remoteURL)
	json.Unmarshal(status["ca"], &ca)
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(ca))
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Transport: &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{RootCAs: roots}}, Timeout: 4 * time.Second}
	defer client.CloseIdleConnections()
	pairing := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	var code string
	json.Unmarshal(pairing["code"], &code)
	httpStatus, _, cookies := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code, "name": "Persistent"})
	if httpStatus != 200 {
		t.Fatal(httpStatus)
	}
	state, err := os.ReadFile(filepath.Join(dir, "remote.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(state), cookies[0].Value) || strings.Contains(string(state), code) {
		t.Fatal("bearer/code persisted")
	}
	if _, err := StartLocalWithStorage(context.Background(), fixtureFS(), dir); err == nil {
		t.Fatal("second backend shared private profile")
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	s, err = StartLocalWithStorage(context.Background(), fixtureFS(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	httpStatus, data, _ := remotePost(t, client, remoteURL+"/auth/session", remoteURL, nil)
	if httpStatus != 200 || !strings.Contains(string(data), `"viewer"`) {
		t.Fatal("restart did not recover viewer", httpStatus, string(data))
	}
	owner = ownerClient(t, s)
	ownerAction(t, s, owner, map[string]any{"action": "disable"})
	if res, err := client.Get(remoteURL); err == nil {
		res.Body.Close()
		t.Fatal("disabled listener survived")
	}
}

func TestPairingExpiryRateAndDeviceCapacity(t *testing.T) {
	s, owner, _, _, code := remoteFixture(t)
	m := s.remote
	m.mu.Lock()
	current := time.Now()
	m.now = func() time.Time { return current }
	current = current.Add(6 * time.Minute)
	if _, err := m.pairLocked(code, "Expired"); err == nil {
		t.Fatal("expired code accepted")
	}
	m.mu.Unlock()
	pairing := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	json.Unmarshal(pairing["code"], &code)
	m.mu.Lock()
	m.attempts = 0
	m.attemptStart = current
	for range 10 {
		if _, err := m.pairLocked(strings.Repeat("0", 32), "Guess"); err == nil {
			t.Fatal("guess accepted")
		}
	}
	if _, err := m.pairLocked(code, "Limited"); err == nil {
		t.Fatal("pairing rate limit bypassed")
	}
	current = current.Add(time.Minute)
	if _, err := m.pairLocked(code, "Allowed"); err != nil {
		t.Fatal("rate budget did not recover", err)
	}
	for len(m.state.Devices) < 32 {
		id, _ := randomToken(16)
		token, _ := randomToken(32)
		m.state.Devices = append(m.state.Devices, device{ID: id, Name: "Fixture", Hash: tokenHash(token), Expires: current.Add(time.Hour)})
	}
	m.pairing = &pairingCode{Hash: tokenHash(code), Expires: current.Add(time.Minute)}
	if _, err := m.pairLocked(code, "Overflow"); err == nil {
		t.Fatal("device capacity bypassed")
	}
	m.mu.Unlock()
}

func TestWriteFailureStopsRemoteAndPreventsRestartResurrection(t *testing.T) {
	s, owner, client, remoteURL, code := remoteFixture(t)
	status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code})
	if status != 200 {
		t.Fatal(status)
	}
	conn := pairedSocket(t, client, remoteURL)
	readCtx, readCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer readCancel()
	callRPC(t, readCtx, conn, "getLoadingStatus")
	list := ownerAction(t, s, owner, map[string]any{"action": "status"})
	var devices []struct{ ID string }
	json.Unmarshal(list["devices"], &devices)
	dir := s.remote.dir
	// A deterministic file/dir collision injects a real persistence failure while
	// preserving the on-disk active latch, which must survive shutdown.
	statePath := filepath.Join(dir, "remote.json")
	backup, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(statePath, 0700); err != nil {
		t.Fatal(err)
	}
	status, _, _ = remotePost(t, owner, s.URL()+"/owner/remote", s.URL(), map[string]any{"action": "revoke", "deviceId": devices[0].ID})
	if status == 200 {
		t.Fatal("failed revoke reported success")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("socket survived persistence failure")
	}
	if res, err := client.Get(remoteURL); err == nil {
		res.Body.Close()
		t.Fatal("listener survived write failure")
	}
	if err = os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err = privatefs.Write(dir, "remote.json", backup); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted, err := StartLocalWithStorage(context.Background(), fixtureFS(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close(context.Background())
	recoveredOwner := ownerClient(t, restarted)
	state := ownerAction(t, restarted, recoveredOwner, map[string]any{"action": "status"})
	if string(state["enabled"]) != "false" || string(state["devices"]) != "[]" {
		t.Fatal("failed revocation resurrected viewer", state)
	}
}

func TestRemoteStrictOriginsAndOwnerOnlyControl(t *testing.T) {
	s, _, client, remoteURL, code := remoteFixture(t)
	status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code})
	if status != 200 {
		t.Fatal(status)
	}
	for _, origin := range []string{"", "null", "https://evil.example"} {
		status, _, _ = remotePost(t, client, remoteURL+"/auth/session", origin, nil)
		if status != 403 {
			t.Fatal("recovery origin bypass")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		headers := http.Header{}
		if origin != "" {
			headers.Set("Origin", origin)
		}
		conn, res, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(remoteURL, "https"), &websocket.DialOptions{HTTPClient: client, HTTPHeader: headers})
		cancel()
		if conn != nil {
			conn.CloseNow()
		}
		if err == nil || res == nil || res.StatusCode != 403 {
			t.Fatal("WS origin bypass")
		}
	}
	status, _, _ = remotePost(t, http.DefaultClient, s.URL()+"/owner/remote", s.URL(), map[string]any{"action": "enable", "address": "127.0.0.1", "port": 0})
	if status != 403 {
		t.Fatal("anonymous owner settings accepted")
	}
}

func TestSavedRemoteDoesNotPublishBeforeServerInitialization(t *testing.T) {
	dir := t.TempDir()
	if err := privatefs.Prepare(dir); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(remoteState{Version: 1, Enabled: true, Address: "127.0.0.1", Port: 0})
	if err := privatefs.Write(dir, "remote.json", state); err != nil {
		t.Fatal(err)
	}
	manager, err := newRemote(&Server{}, fixtureFS(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.close()
	if manager.http != nil {
		t.Fatal("constructor published remote listener before server initialized")
	}
}
