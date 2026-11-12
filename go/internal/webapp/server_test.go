package webapp

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSingleLoopbackLifecycle(t *testing.T) {
	occupied, _ := net.Listen("tcp4", "127.0.0.1:3300")
	if occupied != nil {
		defer occupied.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := StartLocal(ctx, fixtureFS())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	if !strings.HasPrefix(server.URL(), "http://127.0.0.1:") || strings.HasSuffix(server.URL(), ":3300") {
		t.Fatal("unsafe/fixed listener", server.URL())
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			response, err := client.Get(server.URL() + "/exploration/system")
			if err != nil {
				t.Error(err)
				return
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			if string(body) != "<html>exploration</html>" {
				t.Error("wrong shared-server asset")
			}
		})
	}
	wg.Wait()
	// Retire this fixture's idle/speculative dials before timing graceful shutdown.
	// net/http can otherwise give unused StateNew sockets a five-second grace.
	client.CloseIdleConnections()
	cancel()
	select {
	case <-server.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not close server")
	}
	if _, err := client.Get(server.URL()); err == nil {
		t.Fatal("listener survived shutdown")
	}
	if err := server.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartupRejectsMissingAssetsAndCanceledContext(t *testing.T) {
	if _, err := StartLocal(context.Background(), nil); err == nil {
		t.Fatal("missing assets accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := StartLocal(ctx, fixtureFS()); err == nil {
		t.Fatal("canceled startup accepted")
	}
}
