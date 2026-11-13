package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

func (s *Server) loadingStatus() map[string]any {
	marker := s.prototype()
	return map[string]any{"loadingComplete": true, "loadingInProgress": false, "numberOfFiles": 0, "numberOfEventsImported": 0, "numberOfLogLines": 0, "logSizeInBytes": 0, "offline": marker["offline"], "stage": marker["stage"]}
}

func (s *Server) socket(w http.ResponseWriter, r *http.Request) {
	token := s.sessions.token(r)
	if !s.sameOrigin(r) || !s.sessions.valid(token) {
		http.Error(w, "Forbidden", 403)
		return
	}
	s.serveSocket(w, r, socketPrincipal{role: "owner", url: s.url, valid: func() bool { return s.sessions.valid(token) }})
}

type socketPrincipal struct {
	role, url, deviceID, token string
	valid                      func() bool
	remote                     *remoteManager
	epoch                      uint64
}

func (s *Server) serveSocket(w http.ResponseWriter, r *http.Request, principal socketPrincipal) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	if principal.remote != nil {
		m := principal.remote
		m.mu.Lock()
		_, valid := m.findLocked(principal.token)
		count := 0
		for _, id := range m.connections {
			if id == principal.deviceID {
				count++
			}
		}
		if !valid || m.epoch != principal.epoch || m.http == nil || len(m.connections) >= 64 || count >= 4 {
			m.mu.Unlock()
			conn.CloseNow()
			return
		}
		m.connections[conn] = principal.deviceID
		m.mu.Unlock()
		defer func() { m.mu.Lock(); delete(m.connections, conn); m.mu.Unlock() }()
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
	readCtx, cancelRead := context.WithCancel(context.Background())
	defer cancelRead()
	go func() {
		ticker := time.NewTicker(s.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-readCtx.Done():
				return
			case <-ticker.C:
				if !principal.valid() {
					conn.CloseNow()
					return
				}
				pingCtx, cancel := context.WithTimeout(readCtx, s.pongTimeout)
				err := conn.Ping(pingCtx)
				cancel()
				if err != nil {
					conn.CloseNow()
					return
				}
			}
		}
	}()
	var writeMu sync.Mutex
	send := func(value any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if !principal.valid() {
			return context.Canceled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return conn.Write(ctx, websocket.MessageText, data)
	}
	if send(map[string]any{"name": "loadingProgress", "message": s.loadingStatus()}) != nil {
		return
	}
	if s.game != nil {
		changes, unsubscribe := s.game.Subscribe(readCtx)
		defer unsubscribe()
		go func() {
			for {
				select {
				case <-readCtx.Done():
					return
				case summary, ok := <-changes:
					if !ok {
						return
					}
					if send(map[string]any{"name": "journalSummaryChanged", "message": summary}) != nil {
						conn.CloseNow()
						return
					}
				}
			}
		}()
	}
	for {
		kind, data, err := conn.Read(readCtx)
		if err != nil {
			return
		}
		if !principal.valid() {
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
			response["message"] = s.loadingStatus()
		case "hostInfo":
			response["message"] = map[string]any{"urls": []string{principal.url}}
		case "getJournalSummary":
			if s.game == nil {
				response["error"] = "Journal ingestion disabled"
			} else {
				response["message"] = s.game.Snapshot()
			}
		default:
			response["error"] = "Not implemented in this offline preview"
		}
		if send(response) != nil {
			return
		}
		if request.Name == "getLoadingStatus" && principal.role == "owner" {
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
		if strings.HasPrefix(r.URL.Path, "/owner/") {
			if r.URL.Path == "/owner/journal" {
				s.ownerJournal(w, r)
			} else {
				s.ownerRemote(w, r)
			}
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
