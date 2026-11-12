package webapp

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"github.com/Jakoczig/daedalus/go/internal/remotetls"
	"github.com/coder/websocket"
)

const viewerCookie = "__Host-daedalus_viewer"

type remoteManager struct {
	mu            sync.Mutex
	server        *Server
	assets        http.Handler
	dir           string
	release       func()
	state         remoteState
	http          *http.Server
	url           string
	bundle        remotetls.Bundle
	pairing       *pairingCode
	now           func() time.Time
	epoch         uint64
	connections   map[*websocket.Conn]string
	closed, fault bool
	notice        string
	attempts      int
	attemptStart  time.Time
}

func newRemote(s *Server, assets fs.FS, dir string) (*remoteManager, error) {
	release, err := privatefs.Lock(dir)
	if err != nil {
		return nil, err
	}
	m := &remoteManager{server: s, assets: NewStaticHandler(assets), dir: dir, release: release, now: time.Now, connections: map[*websocket.Conn]string{}}
	err = m.load()
	if err != nil {
		release()
		return nil, err
	}
	return m, nil
}
func validAddress(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && ip.To4() != nil && (ip.IsPrivate() || ip.IsLoopback())
}
func (m *remoteManager) enableLocked(address string, port int) error {
	if m.closed || m.fault {
		return errors.New("remote service unavailable")
	}
	if !validAddress(address) || port < 0 || port > 65535 || (port < 1024 && !(port == 0 && net.ParseIP(address).IsLoopback())) {
		return errors.New("select a local private IPv4 address and port 1024–65535")
	}
	if m.http != nil {
		return errors.New("disable remote displays before changing the address")
	}
	bundle, err := remotetls.Load(m.dir, net.ParseIP(address), m.now())
	if err != nil {
		return errors.New("cannot load private display certificate")
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		return errors.New("cannot bind selected remote address")
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	m.state.Enabled = true
	m.state.Active = true
	m.state.Address = address
	m.state.Port = actualPort
	if err = m.saveLocked(); err != nil {
		listener.Close()
		m.failLocked()
		return errors.New("cannot persist remote settings")
	}
	m.epoch++
	epoch := m.epoch
	m.url = "https://" + listener.Addr().String()
	m.bundle = bundle
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { m.serve(epoch, w, r) })
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.closed || m.fault || m.epoch != epoch {
			return nil, errors.New("remote service unavailable")
		}
		if m.now().Add(7 * 24 * time.Hour).After(m.bundle.Expires) {
			renewed, err := remotetls.Load(m.dir, net.ParseIP(address), m.now())
			if err != nil {
				return nil, errors.New("display certificate renewal failed")
			}
			m.bundle = renewed
		}
		return &m.bundle.Certificate, nil
	}}
	m.http = &http.Server{Handler: handler, TLSConfig: tlsConfig, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: log.New(io.Discard, "", 0)}
	remoteHTTP := m.http
	go func() {
		_ = remoteHTTP.Serve(tls.NewListener(&limitedListener{Listener: listener, capacity: make(chan struct{}, 64)}, tlsConfig))
	}()
	return nil
}

type limitedListener struct {
	net.Listener
	capacity chan struct{}
}
type countedConnection struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *countedConnection) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
func (l *limitedListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.capacity <- struct{}{}:
			return &countedConnection{Conn: conn, release: func() { <-l.capacity }}, nil
		default:
			conn.Close()
		}
	}
}

func (m *remoteManager) dropLocked(id string) {
	for conn, deviceID := range m.connections {
		if id == "" || id == deviceID {
			delete(m.connections, conn)
			go conn.CloseNow()
		}
	}
}
func (m *remoteManager) stopLocked() {
	m.epoch++
	m.pairing = nil
	m.dropLocked("")
	if m.http != nil {
		_ = m.http.Close()
		m.http = nil
	}
	m.url = ""
}
func (m *remoteManager) failLocked() {
	m.fault = true
	m.state.Enabled = false
	m.notice = "Private storage failed. Restart and explicitly enable/pair again."
	m.stopLocked()
}
func (m *remoteManager) close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	m.stopLocked()
	var err error
	if !m.fault {
		m.state.Active = false
		err = m.saveLocked()
	}
	m.release()
	return err
}

func (m *remoteManager) statusLocked() map[string]any {
	addresses := []string{}
	if interfaces, err := net.Interfaces(); err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 {
				continue
			}
			items, _ := iface.Addrs()
			for _, item := range items {
				ip, _, err := net.ParseCIDR(item.String())
				if err == nil && ip.To4() != nil && ip.IsPrivate() {
					addresses = append(addresses, ip.String())
				}
			}
		}
	}
	devices := make([]map[string]any, 0, len(m.state.Devices))
	for _, d := range m.state.Devices {
		devices = append(devices, map[string]any{"id": d.ID, "name": d.Name, "expires": d.Expires})
	}
	return map[string]any{"enabled": m.http != nil && !m.fault, "url": m.url, "address": m.state.Address, "addresses": addresses, "port": m.state.Port, "devices": devices, "ca": string(m.bundle.CAPEM), "fingerprint": m.bundle.Fingerprint, "notice": m.notice}
}
func (s *Server) ownerRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || !s.sameOrigin(r) || !s.sessions.valid(s.sessions.token(r)) || s.remote == nil {
		http.Error(w, "Forbidden", 403)
		return
	}
	var request struct {
		Action   string `json:"action"`
		Address  string `json:"address"`
		Port     int    `json:"port"`
		DeviceID string `json:"deviceId"`
		Name     string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Invalid request", 400)
		return
	}
	m := s.remote
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.fault {
		http.Error(w, "Remote settings unavailable; restart and pair again", 503)
		return
	}
	var err error
	switch request.Action {
	case "status":
	case "enable":
		err = m.enableLocked(request.Address, request.Port)
	case "disable":
		m.state.Enabled = false
		m.state.Active = false
		m.state.Devices = nil
		m.pairing = nil
		if err = m.saveLocked(); err != nil {
			m.failLocked()
		} else {
			m.stopLocked()
		}
	case "pair":
		if m.http == nil {
			err = errors.New("enable remote displays first")
			break
		}
		code, e := randomToken(16)
		err = e
		if err == nil {
			expires := m.now().Add(5 * time.Minute)
			m.pairing = &pairingCode{Hash: tokenHash(code), Expires: expires}
			writeJSON(w, map[string]any{"code": code, "expires": expires})
			return
		}
	case "rename", "revoke":
		found := false
		for i, d := range m.state.Devices {
			if d.ID != request.DeviceID {
				continue
			}
			found = true
			if request.Action == "rename" {
				if !validName(request.Name) {
					err = errors.New("invalid device name")
					break
				}
				m.state.Devices[i].Name = request.Name
			} else {
				m.state.Devices = append(m.state.Devices[:i], m.state.Devices[i+1:]...)
			}
			if err = m.saveLocked(); err != nil {
				m.failLocked()
			}
			if request.Action == "revoke" {
				m.dropLocked(d.ID)
			}
			break
		}
		if !found {
			err = errors.New("device not found")
		}
	default:
		err = errors.New("unknown owner operation")
	}
	if err != nil {
		http.Error(w, "Remote settings operation failed", 400)
		return
	}
	writeJSON(w, m.statusLocked())
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func noForwarded(r *http.Request) bool {
	for _, key := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
		if len(r.Header.Values(key)) > 0 {
			return false
		}
	}
	return true
}
func matchingOrigin(r *http.Request, origin string) bool {
	return len(r.Header.Values("Origin")) == 1 && r.Header.Get("Origin") == origin
}
func (m *remoteManager) serve(epoch uint64, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	m.mu.Lock()
	origin := m.url
	active := !m.closed && !m.fault && m.http != nil && m.epoch == epoch
	m.mu.Unlock()
	if !active || r.TLS == nil || r.Host != strings.TrimPrefix(origin, "https://") || !noForwarded(r) {
		http.Error(w, "Forbidden", 403)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/owner/") {
		http.Error(w, "Forbidden", 403)
		return
	}
	if r.URL.Path == "/auth/session" || r.URL.Path == "/auth/pair" {
		if r.Method != "POST" || !matchingOrigin(r, origin) {
			http.Error(w, "Forbidden", 403)
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.closed || m.fault || m.epoch != epoch || m.http == nil {
			http.Error(w, "Forbidden", 403)
			return
		}
		var token string
		if r.URL.Path == "/auth/pair" {
			var request struct{ Code, Name, Role string }
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF {
				http.Error(w, "Invalid request", 400)
				return
			}
			if request.Name == "" {
				request.Name = "Browser display"
			}
			var err error
			token, err = m.pairLocked(request.Code, request.Name)
			if err != nil {
				http.Error(w, "Invalid, expired or rate-limited pairing", 403)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: viewerCookie, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
		} else {
			if _, err := io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 4096)); err != nil {
				http.Error(w, "Invalid request", 400)
				return
			}
			cookie, err := r.Cookie(viewerCookie)
			if err == nil {
				token = cookie.Value
			}
			if _, ok := m.findLocked(token); !ok {
				http.Error(w, "Pair this display", 401)
				return
			}
		}
		writeJSON(w, map[string]any{"role": "viewer", "prototype": map[string]any{"stage": "G1B", "offline": true}})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/auth/") {
		http.Error(w, "Forbidden", 403)
		return
	}
	if r.Header.Get("Upgrade") != "" {
		if r.URL.Path != "/" || !matchingOrigin(r, origin) {
			http.Error(w, "Forbidden", 403)
			return
		}
		cookie, err := r.Cookie(viewerCookie)
		if err != nil {
			http.Error(w, "Forbidden", 403)
			return
		}
		m.mu.Lock()
		d, valid := m.findLocked(cookie.Value)
		m.mu.Unlock()
		if !valid {
			http.Error(w, "Forbidden", 403)
			return
		}
		m.server.serveSocket(w, r, socketPrincipal{role: "viewer", url: origin, remote: m, epoch: epoch, deviceID: d.ID, token: cookie.Value, valid: func() bool {
			m.mu.Lock()
			defer m.mu.Unlock()
			_, ok := m.findLocked(cookie.Value)
			return ok && m.epoch == epoch
		}})
		return
	}
	m.assets.ServeHTTP(w, r)
}
