package webapp

import (
	"context"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/gamestate"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type Server struct {
	http                      *http.Server
	url                       string
	done                      chan struct{}
	once                      sync.Once
	closeErr                  error
	sessions                  *sessionStore
	socketMu                  sync.Mutex
	sockets                   map[*websocket.Conn]struct{}
	closing                   bool
	clientReady               chan struct{}
	readyOnce                 sync.Once
	remote                    *remoteManager
	pingInterval, pongTimeout time.Duration
	g2b                       bool
	g2c                       bool
	g2d                       bool
	game                      *gamestate.Service
	gameCancel                context.CancelFunc
	gameDone                  chan struct{}
	gameConfigMu              sync.Mutex
	profile                   string
	journalReady              chan struct{}
	journalReadyOnce          sync.Once
	navigationReady           chan struct{}
	navigationReadyOnce       sync.Once
	navigationSmoke           bool
}

func StartLocal(ctx context.Context, assets fs.FS) (*Server, error) {
	return startLocal(ctx, assets, "")
}
func StartLocalWithStorage(ctx context.Context, assets fs.FS, dir string) (*Server, error) {
	return startLocal(ctx, assets, dir)
}
func startLocal(ctx context.Context, assets fs.FS, dir string) (*Server, error) {
	return StartLocalWithOptions(ctx, assets, Options{StorageDir: dir})
}
func StartLocalWithOptions(ctx context.Context, assets fs.FS, opts Options) (*Server, error) {
	dir := opts.StorageDir
	if opts.EnableJournal && dir == "" {
		return nil, errors.New("journal ingestion requires an exclusively locked private profile")
	}
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
	server := &Server{url: "http://" + listener.Addr().String(), done: make(chan struct{}), clientReady: make(chan struct{}), pingInterval: 20 * time.Second, pongTimeout: 10 * time.Second}
	server.journalReady = make(chan struct{})
	server.navigationReady = make(chan struct{})
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	server.sessions = newSessionStore(port, time.Now)
	server.sockets = make(map[*websocket.Conn]struct{})
	server.http = &http.Server{Handler: server.handler(NewStaticHandler(assets)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	if dir != "" {
		manager, err := newRemote(server, assets, dir)
		if err != nil {
			listener.Close()
			return nil, err
		}
		server.remote = manager
		server.startGame(ctx, opts)
		manager.mu.Lock()
		if manager.state.Enabled {
			if err := manager.enableLocked(manager.state.Address, manager.state.Port); err != nil {
				manager.notice = "Remote startup failed. Check the host address, port and certificate storage, then enable again."
				if !manager.fault {
					manager.state.Enabled = false
					manager.state.Active = false
					if err = manager.saveLocked(); err != nil {
						manager.failLocked()
					}
				}
			}
		}
		manager.mu.Unlock()
	}
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
func (s *Server) ClientReady() <-chan struct{}           { return s.clientReady }
func (s *Server) JournalClientReady() <-chan struct{}    { return s.journalReady }
func (s *Server) NavigationClientReady() <-chan struct{} { return s.navigationReady }
func (s *Server) Close(ctx context.Context) error {
	s.once.Do(func() {
		s.gameConfigMu.Lock()
		s.sessions.clear()
		var remoteErr error
		gameStopped := true
		if s.gameCancel != nil {
			s.gameCancel()
			select {
			case <-s.gameDone:
			case <-ctx.Done():
				gameStopped = false
				s.closeErr = ctx.Err()
			}
		}
		if s.remote != nil {
			if gameStopped {
				remoteErr = s.remote.close()
			} else {
				// Stop admission immediately, but retain the profile lock until the
				// old source reader exits, even after a shutdown deadline.
				s.remote.mu.Lock()
				s.remote.stopLocked()
				s.remote.mu.Unlock()
				go func() { <-s.gameDone; _ = s.remote.close() }()
			}
		}
		// Let admitted slow handlers finish, recheck their now-invalid session,
		// and return before Shutdown waits for active HTTP requests.
		s.gameConfigMu.Unlock()
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
		shutdownErr := s.http.Shutdown(ctx)
		if shutdownErr != nil {
			_ = s.http.Close()
		}
		s.closeErr = errors.Join(s.closeErr, shutdownErr, remoteErr)
		close(s.done)
	})
	return s.closeErr
}
