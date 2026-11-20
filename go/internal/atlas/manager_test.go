package atlas

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	Public, Pointer, Index, Overview, Tile string
	Catalogs                               map[string]string
}

func testFixture(t testing.TB) (fixture, []byte) {
	t.Helper()
	b, e := os.ReadFile("testdata/publisher-fixture.json")
	if e != nil {
		t.Fatal(e)
	}
	var f fixture
	json.Unmarshal(b, &f)
	key, _ := hex.DecodeString(f.Public)
	return f, key
}
func un64(s string) []byte { b, _ := base64.StdEncoding.DecodeString(s); return b }

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestInstallOfflineAndFailureRecovery(t *testing.T) {
	f, key := testFixture(t)
	calls := 0
	bad := false
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "eridion.fitoserver.org" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatal("private request")
		}
		p := r.URL.Path
		data := un64(f.Pointer)
		if strings.HasSuffix(p, "index.json") {
			data = un64(f.Index)
		}
		if strings.HasSuffix(p, "overview.zip") {
			data = un64(f.Overview)
			if bad {
				data = data[:len(data)-5]
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Etag": []string{"fixture"}}, Body: io.NopCloser(strings.NewReader(string(data))), ContentLength: int64(len(data))}, nil
	})}
	now := func() time.Time { return time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC) }
	dir := t.TempDir()
	m := newManager(context.Background(), dir, client, key, now)
	defer m.Close()
	if calls != 0 || m.Status().Installed != "" {
		t.Fatal("startup network")
	}
	if e := m.Check(context.Background(), false); e != nil {
		t.Fatal(e)
	}
	if e := m.Install(context.Background()); e != nil {
		t.Fatal(e)
	}
	if m.Status().Installed == "" {
		t.Fatal("not installed")
	}
	before := calls
	m.Close()
	m = newManager(context.Background(), dir, client, key, now)
	defer m.Close()
	if calls != before || m.Status().Installed == "" {
		t.Fatal("offline cache restart")
	}
	bad = true
	if e := m.Install(context.Background()); e == nil {
		t.Fatal("accepted truncated update")
	}
	if m.Status().Installed == "" {
		t.Fatal("lost last good atlas")
	}
}
