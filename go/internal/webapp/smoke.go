package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// SmokeTest uses real HTTP/WebSocket traffic against the exact desktop service.
// It reads only embedded assets and synthetic status; no provider is contacted.
func SmokeTest(ctx context.Context, s *Server, assets fs.FS) error {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	fail := errors.New("offline service verification failed")
	paths := []string{"/", "/exploration/system"}
	fs.WalkDir(assets, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasPrefix(path, "_next/") && (strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css")) {
			paths = append(paths, "/"+path)
		}
		return nil
	})
	for _, path := range paths {
		req, _ := http.NewRequestWithContext(ctx, "GET", s.URL()+path, nil)
		response, err := client.Do(req)
		if err != nil {
			return fail
		}
		count, readErr := io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || readErr != nil || count == 0 {
			return fail
		}
	}
	request, _ := http.NewRequestWithContext(ctx, "POST", s.URL()+"/auth/session", nil)
	request.Header.Set("Origin", s.URL())
	response, err := client.Do(request)
	if err != nil {
		return fail
	}
	var session struct{ Role string }
	err = json.NewDecoder(response.Body).Decode(&session)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || session.Role != "owner" {
		return fail
	}
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.URL(), "http"), &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": {s.URL()}}})
	if err != nil {
		return fail
	}
	defer socket.CloseNow()
	if socket.Write(ctx, websocket.MessageText, []byte(`{"requestId":"smoke","name":"getLoadingStatus","message":{}}`)) != nil {
		return fail
	}
	for {
		_, data, err := socket.Read(ctx)
		if err != nil {
			return fail
		}
		var reply struct {
			RequestID string
			Message   struct {
				Offline                bool
				Stage                  string
				NumberOfEventsImported int
			}
		}
		if json.Unmarshal(data, &reply) != nil {
			return fail
		}
		if reply.RequestID == "smoke" {
			if !reply.Message.Offline || (reply.Message.Stage != "G1A" && reply.Message.Stage != "G1B") || reply.Message.NumberOfEventsImported != 0 {
				return fail
			}
			return nil
		}
	}
}
