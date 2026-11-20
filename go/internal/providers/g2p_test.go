package providers

import (
	"context"
	"github.com/Jakoczig/eridion/go/internal/history"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"net/http"
	"testing"
)

func TestG2PSelectedVerifiedAccountCanAuthorizeAfterShutdown(t *testing.T) {
	db, err := history.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m, _ := testManager(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("authorization prerequisite check made network request")
		return nil, nil
	})
	if err = m.AttachHistory(db, history.ProviderSeed{FID: "SYNTHETIC-A", Name: "Fixture CMDR"}); err != nil {
		t.Fatal(err)
	}
	m.opts.RequireHistory = true
	if err = m.Configure("frontier", Config{Read: true}); err != nil {
		t.Fatal(err)
	}
	m.Observe(event(t, `{"event":"Shutdown"}`), journal.Live)
	if _, err = m.BeginAuthorization("http://127.0.0.1:12345"); err != nil {
		t.Fatal("verified cached account must be able to authorize", err)
	}
	if _, err = m.Read(context.Background(), "frontier", "profile", "", 1); err == nil {
		t.Fatal("authorization must not grant unverified live account reads")
	}
	m.PinAccount("SYNTHETIC-B")
	if _, err = m.BeginAuthorization("http://127.0.0.1:12345"); err == nil {
		t.Fatal("pending different profile authorized old namespace")
	}
}
