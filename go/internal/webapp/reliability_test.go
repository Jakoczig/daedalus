package webapp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"
)

func TestUnauthenticatedTLSCapacityDoesNotBlockOwner(t *testing.T) {
	s, owner, _, remoteURL, _ := remoteFixture(t)
	address := strings.TrimPrefix(remoteURL, "https://")
	connections := []net.Conn{}
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	for range 64 {
		conn, err := net.DialTimeout("tcp4", address, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, conn)
	}
	extra, err := net.DialTimeout("tcp4", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer extra.Close()
	extra.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := extra.Read(make([]byte, 1)); err == nil {
		t.Fatal("excess TLS admission survived")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("excess handshake retained until timeout")
	}
	ownerAction(t, s, owner, map[string]any{"action": "status"})
}

func TestIdleExpiryClosesSocketWithoutRPC(t *testing.T) {
	s, _, client, remoteURL, code := remoteFixture(t)
	s.pingInterval = 10 * time.Millisecond
	s.pongTimeout = time.Second
	status, _, _ := remotePost(t, client, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code})
	if status != 200 {
		t.Fatal(status)
	}
	conn := pairedSocket(t, client, remoteURL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	callRPC(t, ctx, conn, "getLoadingStatus")
	s.remote.mu.Lock()
	expired := time.Now().Add(13 * time.Hour)
	s.remote.now = func() time.Time { return expired }
	s.remote.mu.Unlock()
	if _, _, err := conn.Read(ctx); err == nil || ctx.Err() != nil {
		t.Fatal("idle expired socket did not close promptly", err)
	}
}

func TestLongIdleMultipleViewersAndNetworkRecovery(t *testing.T) {
	t.Parallel()
	s, owner, first, remoteURL, code := remoteFixture(t)
	status, _, _ := remotePost(t, first, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code, "name": "Laptop"})
	if status != 200 {
		t.Fatal(status)
	}
	pairing := ownerAction(t, s, owner, map[string]any{"action": "pair"})
	json.Unmarshal(pairing["code"], &code)
	jar, _ := cookiejar.New(nil)
	second := &http.Client{Jar: jar, Transport: first.Transport, Timeout: 4 * time.Second}
	status, _, _ = remotePost(t, second, remoteURL+"/auth/pair", remoteURL, map[string]any{"code": code, "name": "Tablet"})
	if status != 200 {
		t.Fatal(status)
	}
	a := pairedSocket(t, first, remoteURL)
	b := pairedSocket(t, second, remoteURL)
	// Readers must remain active to process control frames, as browser WebSockets do.
	// An outstanding read in each goroutine keeps pong handling active while idle.
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	callRPC(t, ctx, a, "getLoadingStatus")
	callRPC(t, ctx, b, "getLoadingStatus")
	timer := time.NewTimer(65 * time.Second)
	defer timer.Stop()
	failures := make(chan error, 2)
	go func() { _, _, err := a.Read(ctx); failures <- err }()
	go func() { _, _, err := b.Read(ctx); failures <- err }()
	select {
	case err := <-failures:
		t.Fatal("healthy idle connection closed", err)
	case <-timer.C:
	}
	// Simulate a dropped browser connection, then recover the durable cookie and
	// connect a fresh socket to the same single service/state instance.
	a.CloseNow()
	b.CloseNow()
	for range 2 {
		select {
		case <-failures:
		case <-time.After(2 * time.Second):
			t.Fatal("reader leaked")
		}
	}
	status, data, _ := remotePost(t, first, remoteURL+"/auth/session", remoteURL, nil)
	if status != 200 || !json.Valid(data) {
		t.Fatal("session recovery failed", status)
	}
	recovered := pairedSocket(t, first, remoteURL)
	reply := callRPC(t, ctx, recovered, "getLoadingStatus")
	if len(reply["error"]) != 0 {
		t.Fatal("recovered read failed")
	}
	list := ownerAction(t, s, owner, map[string]any{"action": "status"})
	var devices []map[string]any
	json.Unmarshal(list["devices"], &devices)
	if len(devices) != 2 {
		t.Fatal("reconnect duplicated devices or service")
	}
}
