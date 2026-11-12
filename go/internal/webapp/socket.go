package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

func loadingStatus() map[string]any {
	return map[string]any{"loadingComplete": true, "loadingInProgress": false, "numberOfFiles": 0, "numberOfEventsImported": 0, "numberOfLogLines": 0, "logSizeInBytes": 0, "offline": true, "stage": "G1A"}
}

func (s *Server) socket(w http.ResponseWriter, r *http.Request) {
	token := s.sessions.token(r)
	if !s.sameOrigin(r) || !s.sessions.valid(token) {
		http.Error(w, "Forbidden", 403)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	s.socketMu.Lock()
	if s.closing {
		s.socketMu.Unlock()
		_ = conn.CloseNow()
		return
	}
	s.sockets[conn] = struct{}{}
	s.socketMu.Unlock()
	defer func() { _ = conn.CloseNow(); s.socketMu.Lock(); delete(s.sockets, conn); s.socketMu.Unlock() }()
	conn.SetReadLimit(1 << 20)
	send := func(value any) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return conn.Write(ctx, websocket.MessageText, data)
	}
	if send(map[string]any{"name": "loadingProgress", "message": loadingStatus()}) != nil {
		return
	}
	for {
		kind, data, err := conn.Read(context.Background())
		if err != nil {
			return
		}
		if !s.sessions.valid(token) {
			_ = conn.Close(websocket.StatusPolicyViolation, "Session expired")
			return
		}
		var request struct {
			RequestID string          `json:"requestId"`
			Name      string          `json:"name"`
			Message   json.RawMessage `json:"message"`
		}
		if kind != websocket.MessageText || json.Unmarshal(data, &request) != nil || len(request.RequestID) > 128 || len(request.Name) > 128 {
			if send(map[string]any{"error": "Invalid request"}) != nil {
				return
			}
			continue
		}
		response := map[string]any{"requestId": request.RequestID, "name": request.Name}
		switch request.Name {
		case "getLoadingStatus":
			response["message"] = loadingStatus()
		case "hostInfo":
			response["message"] = map[string]any{"urls": []string{s.url}}
		default:
			response["error"] = "Not implemented in G1A"
		}
		if send(response) != nil {
			return
		}
		if request.Name == "getLoadingStatus" {
			s.readyOnce.Do(func() { close(s.clientReady) })
		}
	}
}

func (s *Server) handler(assets http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		if !s.trustedPeer(r) {
			http.Error(w, "Forbidden", 403)
			return
		}
		if r.URL.Path == "/auth/session" {
			s.session(w, r)
			return
		}
		if r.Header.Get("Upgrade") != "" {
			if r.URL.Path != "/" {
				http.Error(w, "Forbidden", 403)
				return
			}
			s.socket(w, r)
			return
		}
		if len(r.URL.Path) >= 6 && r.URL.Path[:6] == "/auth/" {
			http.Error(w, "Forbidden", 403)
			return
		}
		assets.ServeHTTP(w, r)
	})
}
