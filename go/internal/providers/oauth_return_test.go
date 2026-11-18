package providers

import (
	"context"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHTTPSReturnRejectsAlteredPortStaleAndConcurrentFlows(t *testing.T) {
	calls := 0
	m, now := testManager(t, func(*http.Request) (*http.Response, error) {
		calls++
		return reply(`{"access_token":"fixture","refresh_token":"fixture","token_type":"Bearer","expires_in":3600}`, 200), nil
	})
	if e := m.Configure("frontier", Config{Read: true}); e != nil {
		t.Fatal(e)
	}
	first, _ := m.BeginAuthorization("http://127.0.0.1:12345")
	a, _ := url.Parse(first)
	second, _ := m.BeginAuthorization("http://127.0.0.1:23456")
	b, _ := url.Parse(second)
	for _, v := range []struct{ receiver, state string }{{"http://127.0.0.1:12345/owner/frontier-callback", a.Query().Get("state")}, {"http://127.0.0.1:23456/owner/frontier-callback", strings.Replace(b.Query().Get("state"), "23456", "34567", 1)}, {FrontierRedirectURI, b.Query().Get("state")}} {
		if m.CompleteAuthorization(context.Background(), v.receiver, v.state, "code") == nil {
			t.Fatal("unbound return accepted")
		}
	}
	*now = now.Add(6 * time.Minute)
	if m.CompleteAuthorization(context.Background(), "http://127.0.0.1:23456/owner/frontier-callback", b.Query().Get("state"), "code") == nil || calls != 0 {
		t.Fatal("expired flow exchanged")
	}
	for _, base := range []string{"http://127.0.0.1:0", "http://127.0.0.1:99999", "https://127.0.0.1:12345", "http://localhost:12345", "http://127.0.0.1:12345/?next=evil"} {
		if _, e := m.BeginAuthorization(base); e == nil {
			t.Fatal("unsafe receiver", base)
		}
	}
}

func TestOAuthAccountSwitchRejectsInflightTokenResponse(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return reply(`{"access_token":"OLD-ACCOUNT-ACCESS","refresh_token":"OLD-ACCOUNT-REFRESH","token_type":"Bearer","expires_in":3600}`, 200), nil
	})
	if e := m.ActivateAccount("SYNTHETIC-A", false); e != nil {
		t.Fatal(e)
	}
	if e := m.Configure("frontier", Config{Read: true}); e != nil {
		t.Fatal(e)
	}
	u, e := m.BeginAuthorization("http://127.0.0.1:12345")
	if e != nil {
		t.Fatal(e)
	}
	parsed, _ := url.Parse(u)
	done := make(chan error, 1)
	go func() {
		done <- m.CompleteAuthorization(context.Background(), "http://127.0.0.1:12345/owner/frontier-callback", parsed.Query().Get("state"), "fixture")
	}()
	<-entered
	if e = m.ActivateAccount("SYNTHETIC-B", false); e != nil {
		t.Fatal(e)
	}
	close(release)
	if e = <-done; e == nil {
		t.Fatal("old account token saved after switch")
	}
	if m.secrets["frontier"].AccessToken != "" {
		t.Fatal("foreign token exposed")
	}
}
func TestPrivateRegisteredClientAndCancelledState(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) { t.Fatal("network on cancelled flow"); return nil, nil })
	if e := m.ConfigureFrontierClient("3a7cd51a-f629-4fec-b0c7-967bb7e2fa53"); e != nil {
		t.Fatal(e)
	}
	m.Configure("frontier", Config{Read: true})
	u, e := m.BeginAuthorization("http://127.0.0.1:12345")
	if e != nil {
		t.Fatal(e)
	}
	q, _ := url.Parse(u)
	if q.Query().Get("client_id") != "3a7cd51a-f629-4fec-b0c7-967bb7e2fa53" {
		t.Fatal("private client ignored")
	}
	state := q.Query().Get("state")
	receiver := "http://127.0.0.1:12345/owner/frontier-callback"
	if e = m.CancelAuthorization(receiver, state); e != nil {
		t.Fatal(e)
	}
	if m.CompleteAuthorization(context.Background(), receiver, state, "fixture") == nil {
		t.Fatal("cancelled state reused")
	}
	if e = privatefs.Write(m.dir, "frontier-client.json", []byte(`{"clientId":"damaged"}`)); e != nil {
		t.Fatal(e)
	}
	if _, e = m.BeginAuthorization("http://127.0.0.1:12345"); e == nil {
		t.Fatal("damaged private client silently replaced")
	}
}

func TestChoosingProfileCancelsPendingAuthorizationBeforeActivation(t *testing.T) {
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) { t.Fatal("network after profile choice"); return nil, nil })
	m.Configure("frontier", Config{Read: true})
	u, e := m.BeginAuthorization("http://127.0.0.1:12345")
	if e != nil {
		t.Fatal(e)
	}
	q, _ := url.Parse(u)
	m.PinAccount("SYNTHETIC-B")
	if m.CompleteAuthorization(context.Background(), "http://127.0.0.1:12345/owner/frontier-callback", q.Query().Get("state"), "fixture") == nil {
		t.Fatal("old attempt survived profile choice")
	}
}
