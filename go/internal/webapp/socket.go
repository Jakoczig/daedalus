package webapp

import (
	"context"
	"encoding/json"
	"github.com/Jakoczig/eridion/go/internal/catalog"
	"github.com/Jakoczig/eridion/go/internal/gamestate"
	"github.com/Jakoczig/eridion/go/internal/paritystate"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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
	var journalRequested atomic.Bool
	var contextRequested atomic.Bool
	var navRequested, surveyRequested, navDelivered, surveyDelivered atomic.Bool
	var shipRequested, shipDelivered atomic.Bool
	var workspaceRequested, workspaceDelivered atomic.Bool
	navigationReady := func() {
		if principal.role == "owner" && (!s.g2e || (workspaceRequested.Load() && workspaceDelivered.Load())) && (!s.g2d || (shipRequested.Load() && shipDelivered.Load())) && navRequested.Load() && surveyRequested.Load() && navDelivered.Load() && surveyDelivered.Load() {
			s.navigationReadyOnce.Do(func() { close(s.navigationReady) })
		}
	}
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
	if s.game != nil && s.g2e {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			var previous any
			for {
				select {
				case <-readCtx.Done():
					return
				case <-ticker.C:
					if !contextRequested.Load() {
						continue
					}
					v := s.contextView(principal)
					if !reflect.DeepEqual(previous, v) {
						previous = v
						if send(map[string]any{"name": "goNavigationContextChanged", "message": v}) != nil {
							return
						}
					}
				}
			}
		}()
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
					if principal.role == "owner" && journalRequested.Load() && summary.CurrentSystem != "" {
						s.journalReadyOnce.Do(func() { close(s.journalReady) })
					}
				}
			}
		}()
	}
	if s.g2b && s.game != nil {
		changes, unsubscribe := s.game.SubscribeProjections(readCtx)
		var previous *gamestate.Projections
		defer unsubscribe()
		go func() {
			for {
				select {
				case <-readCtx.Done():
					return
				case p, ok := <-changes:
					if !ok {
						return
					}
					if (!s.g2e || previous == nil || !reflect.DeepEqual(previous.Navigation, p.Navigation)) && send(map[string]any{"name": "goNavigationChanged", "message": p.Navigation}) != nil {
						conn.CloseNow()
						return
					}
					if len(p.Navigation.Hops) > 0 && navRequested.Load() {
						navDelivered.Store(true)
					}
					if (!s.g2e || previous == nil || !reflect.DeepEqual(previous.Exploration, p.Exploration)) && send(map[string]any{"name": "goExplorationChanged", "message": p.Exploration}) != nil {
						conn.CloseNow()
						return
					}
					if len(p.Exploration.Bodies) > 0 && surveyRequested.Load() {
						surveyDelivered.Store(true)
					}
					if s.g2d && shipRequested.Load() && (!s.g2e || previous == nil || !reflect.DeepEqual(previous.Ship, p.Ship)) {
						if send(map[string]any{"name": "goShipChanged", "message": p.Ship}) != nil {
							conn.CloseNow()
							return
						}
						if p.Ship.ShipType != "" {
							shipDelivered.Store(true)
						}
					}
					if s.g2e && workspaceRequested.Load() && p.Workspace != nil {
						var old *gamestate.WorkspaceView
						if previous != nil {
							old = previous.Workspace
						}
						if !sameStaticWorkspace(old, p.Workspace) {
							if send(map[string]any{"name": "goWorkspaceChanged", "message": p.Workspace}) != nil {
								conn.CloseNow()
								return
							}
						} else if !reflect.DeepEqual(old.Parity.Status, p.Workspace.Parity.Status) {
							if send(map[string]any{"name": "goFlightChanged", "message": p.Workspace.Parity.Status}) != nil {
								conn.CloseNow()
								return
							}
						}
						if len(p.Workspace.Parity.Modules) > 0 {
							workspaceDelivered.Store(true)
						}
					}
					previous = &p
					navigationReady()

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
		navigationNonempty, surveyNonempty, shipNonempty, workspaceNonempty := false, false, false, false
		switch request.Name {
		case "getLoadingStatus":
			response["message"] = s.loadingStatus()
		case "hostInfo":
			response["message"] = map[string]any{"urls": []string{principal.url}}
		case "getGoShipName":
			if principal.role != "owner" || s.game == nil {
				response["error"] = "Local owner required"
			} else {
				response["message"] = map[string]string{"shipName": s.game.Ship().ShipName, "shipType": catalog.ShipDisplayName(s.game.Ship().ShipType)}
			}
		case "getGoGalaxyLayers":
			if s.providers == nil {
				response["message"] = map[string]any{"points": []any{}}
			} else {
				response["message"] = s.providers.Layers()
			}
		case "getGoOperations", "exportGoOperations", "backfillGoOperationsSource":
			if principal.role != "owner" {
				response["error"] = "Local owner required"
				break
			}
			if value, err := s.operationsRPC(readCtx, request.Name, request.Message); err != nil {
				response["error"] = err.Error()
			} else {
				response["message"] = value
			}
		case "getGoFieldView":
			response["message"] = s.fieldView(principal.role == "owner")
		case "getGoBioScan":
			var req struct {
				Body string `json:"body"`
			}
			if s.game == nil || strictDecode(request.Message, &req) != nil || len(req.Body) > 20 {
				response["error"] = "Invalid BioScan body"
			} else if v, e := s.game.FieldBio(req.Body, principal.role == "owner"); e != nil {
				response["error"] = "BioScan unavailable"
			} else {
				response["message"] = v
			}
		case "getGoBuiltinModules", "setGoBuiltinModule", "importGoItinerary", "deleteGoItinerary", "getGoItinerary", "editGoItinerary", "copyGoItineraryNext", "exportGoItinerary", "getGoBGSTally", "exportGoBGSTally":
			if principal.role != "owner" {
				response["error"] = "Local owner required"
				break
			}
			if v, e := s.fieldRPC(readCtx, request.Name, request.Message); e != nil {
				response["error"] = e.Error()
			} else {
				response["message"] = v
			}
		case "getGoConstruction", "getGoSaleLinks", "confirmGoOrganicSale", "getGoNearest", "getGoSourceFacts", "getGoDataStatus", "queryGoHistory", "backupGoHistory", "exportGoHistory", "stageGoRestore", "rebuildGoHistory", "reimportGoSource", "getGoPublicationPolicy", "setGoPublicationPolicy", "setGoPublicationException":
			if principal.role != "owner" {
				response["error"] = "Local owner required"
				break
			}
			value, err := s.historyRPC(readCtx, request.Name, request.Message)
			if err != nil {
				response["error"] = err.Error()
			} else {
				response["message"] = value
			}
		case "previewGoHistorical", "confirmGoHistorical", "getGoProviders", "configureGoProvider", "setGoProviderCredential", "disconnectGoProvider", "readGoProvider", "connectGoFrontier", "completeGoFrontierReturn", "setGoFrontierClient":
			if principal.role != "owner" {
				response["error"] = "Owner permission required"
				break
			}
			if s.providers == nil {
				response["error"] = "Providers unavailable"
				break
			}
			value, err := s.providerRPC(readCtx, request.Name, request.Message)
			if err != nil {
				response["error"] = err.Error()
			} else {
				response["message"] = value
			}
		case "getGoNavigationContext":
			if !s.g2e || s.game == nil {
				response["error"] = "Navigation context unavailable"
			} else {
				contextRequested.Store(true)
				response["message"] = s.contextView(principal)
			}
		case "getGoSystem", "lookupGoSystem":
			if !s.g2e || s.game == nil || s.community == nil {
				response["error"] = "System lookup unavailable"
				break
			}
			if request.Name == "lookupGoSystem" && principal.role != "owner" {
				response["error"] = "Local owner required"
				break
			}
			var q struct {
				Name string `json:"name"`
			}
			if strictDecode(request.Message, &q) != nil || !cleanSystemName(q.Name) {
				response["error"] = "Invalid system name"
				break
			}
			response["message"] = s.systemView(readCtx, q.Name, request.Name == "lookupGoSystem")
		case "getGoAccounts", "setGoAccounts", "getGoCommanderOverview", "setGoAvatar":
			if principal.role != "owner" {
				response["error"] = "Local owner required"
				break
			}
			value, err := s.accountRPC(readCtx, request.Name, request.Message)
			if err != nil {
				response["error"] = err.Error()
			} else {
				response["message"] = value
			}
		case "getGoLauncher", "setGoCompanion", "desktopAction":
			if principal.role != "owner" || s.companion == nil {
				response["error"] = "Local owner required"
				break
			}
			if request.Name == "setGoCompanion" {
				if err := s.saveDisplayConfig(request.Message); err != nil {
					response["error"] = err.Error()
					break
				}
			}
			if request.Name == "desktopAction" {
				var q struct {
					Action string `json:"action"`
				}
				if strictDecode(request.Message, &q) != nil {
					response["error"] = "Invalid desktop action"
					break
				}
				if q.Action != "dashboard" && q.Action != "launcher" && q.Action != "hide" && q.Action != "closeDashboard" && q.Action != "closeLauncher" && q.Action != "quit" {
					response["error"] = "Unsupported desktop action"
					break
				}
				if s.companion.action == nil {
					response["error"] = "Native desktop unavailable"
					break
				}
				if err := s.companion.action(q.Action); err != nil {
					response["error"] = err.Error()
					break
				}
			}
			response["message"] = s.launcherView()
			if request.Name == "getGoLauncher" {
				s.launcherOnce.Do(func() { close(s.launcherReady) })
			}
		case "getGoCommunity", "refreshGoCommunity", "setGoCommunityEnabled":
			if s.community == nil || s.game == nil {
				response["error"] = "Community enrichment unavailable"
				break
			}
			if request.Name != "getGoCommunity" && principal.role != "owner" {
				response["error"] = "Local owner required"
				break
			}
			if request.Name == "setGoCommunityEnabled" {
				var value struct {
					Enabled *bool `json:"enabled"`
				}
				decoder := json.NewDecoder(strings.NewReader(string(request.Message)))
				decoder.DisallowUnknownFields()
				if decoder.Decode(&value) != nil || value.Enabled == nil {
					response["error"] = "Expected explicit enabled setting"
					break
				}
				if err := s.community.SetEnabled(*value.Enabled); err != nil {
					response["error"] = err.Error()
					break
				}
			}
			name, address := s.game.SystemIdentity()
			if request.Name == "refreshGoCommunity" {
				response["message"] = s.community.Refresh(readCtx, name, address)
			} else {
				response["message"] = s.community.View(name, address)
			}
		case "getGoEngineeredStatistics":
			if !s.g2e || s.game == nil {
				response["error"] = "Native statistics unavailable"
			} else {
				response["message"] = s.game.EngineeredStatistics()
			}
		case "getGoWorkspace":
			if !s.g2e || s.game == nil {
				response["error"] = "Workspace not enabled"
			} else {
				workspaceRequested.Store(true)
				value := s.game.Workspace()
				workspaceNonempty = len(value.Parity.Modules) > 0
				response["message"] = value
			}
		case "getGoShip":
			if !s.g2d || s.game == nil {
				response["error"] = "Ship journal view not enabled"
			} else {
				shipRequested.Store(true)
				value := s.game.Ship()
				shipNonempty = value.ShipType != ""
				response["message"] = value
			}
		case "getGoNavigation":
			if !s.g2b || s.game == nil {
				response["error"] = "Navigation not enabled"
			} else {
				navRequested.Store(true)
				value := s.game.Navigation()
				navigationNonempty = len(value.Hops) > 0
				response["message"] = value
			}
		case "getGoExploration":
			if !s.g2b || s.game == nil {
				response["error"] = "Exploration not enabled"
			} else {
				surveyRequested.Store(true)
				value := s.game.Exploration()
				surveyNonempty = len(value.Bodies) > 0
				response["message"] = value
			}
		case "getJournalSummary":
			if s.game == nil {
				response["error"] = "Journal ingestion disabled"
			} else {
				journalRequested.Store(true)
				response["message"] = s.game.Snapshot()
			}
		default:
			response["error"] = "Not implemented in this offline preview"
		}
		if send(response) != nil {
			return
		}
		if navigationNonempty {
			navDelivered.Store(true)
		}
		if surveyNonempty {
			surveyDelivered.Store(true)
		}
		if shipNonempty {
			shipDelivered.Store(true)
		}
		if workspaceNonempty {
			workspaceDelivered.Store(true)
		}
		navigationReady()
		if request.Name == "getLoadingStatus" && principal.role == "owner" {
			s.readyOnce.Do(func() { close(s.clientReady) })
		}
		if request.Name == "getJournalSummary" && principal.role == "owner" && s.game != nil && s.game.Snapshot().CurrentSystem != "" {
			s.journalReadyOnce.Do(func() { close(s.journalReady) })
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
		if r.URL.Path == "/owner/frontier-callback" {
			s.frontierCallback(w, r)
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

// Status ticks must not resend static loadouts, inventories or system bodies.
func sameStaticWorkspace(a, b *gamestate.WorkspaceView) bool {
	if a == nil || b == nil {
		return false
	}
	x, y := *a, *b
	x.Parity.Status = paritystate.StatusView{}
	y.Parity.Status = paritystate.StatusView{}
	return reflect.DeepEqual(x, y)
}
