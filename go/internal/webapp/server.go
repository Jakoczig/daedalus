package webapp

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type Server struct {
	http        *http.Server
	url         string
	done        chan struct{}
	once        sync.Once
	closeErr    error
	sessions    *sessionStore
	socketMu    sync.Mutex
	sockets     map[*websocket.Conn]struct{}
	closing     bool
	clientReady chan struct{}
	readyOnce   sync.Once
}

func StartLocal(ctx context.Context, assets fs.FS) (*Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if assets == nil {
		return nil, errors.New("frontend assets missing")
	}
	if _, err := fs.Stat(assets, "index.html"); err != nil {
		return nil, errors.New("frontend assets missing")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("cannot start local service")
	}
	server := &Server{url: "http://" + listener.Addr().String(), done: make(chan struct{}), clientReady: make(chan struct{})}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	server.sessions = newSessionStore(port, time.Now)
	server.sockets = make(map[*websocket.Conn]struct{})
	server.http = &http.Server{Handler: server.handler(NewStaticHandler(assets)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() { _ = server.http.Serve(listener) }()
	go func() {
		select {
		case <-ctx.Done():
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Close(closeCtx)
		case <-server.done:
		}
	}()
	return server, nil
}

func (s *Server) URL() string           { return s.url }
func (s *Server) Done() <-chan struct{} { return s.done }

// ClientReady is signaled only after an authenticated loading-status RPC.
func (s *Server) ClientReady() <-chan struct{} { return s.clientReady }
func (s *Server) Close(ctx context.Context) error {
	s.once.Do(func() {
		s.sessions.clear()
		s.socketMu.Lock()
		s.closing = true
		connections := make([]*websocket.Conn, 0, len(s.sockets))
		for conn := range s.sockets {
			connections = append(connections, conn)
		}
		s.socketMu.Unlock()
		for _, conn := range connections {
			_ = conn.CloseNow()
		}
		s.closeErr = s.http.Shutdown(ctx)
		if s.closeErr != nil {
			_ = s.http.Close()
		}
		close(s.done)
	})
	return s.closeErr
}
