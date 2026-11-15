package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/providers"
	"net/http"
)

func (s *Server) providerRPC(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if name == "previewGoHistorical" || name == "confirmGoHistorical" {
		var selection providers.HistorySelection
		if strictDecode(raw, &selection) != nil {
			return nil, errors.New("Invalid historical selection")
		}
		return s.providers.Historical(ctx, selection, name == "confirmGoHistorical")
	}
	switch name {
	case "getGoProviders":
		return s.providers.Status(), nil
	case "connectGoFrontier":
		if s.openAuthorization == nil {
			return nil, errors.New("System-browser authorization requires the native desktop")
		}
		u, err := s.providers.BeginAuthorization(s.url)
		if err != nil {
			return nil, err
		}
		if s.openAuthorization(u) != nil {
			return nil, errors.New("Cannot open system browser")
		}
		return map[string]bool{"started": true}, nil
	}
	var req struct {
		ID         string               `json:"id"`
		Config     providers.Config     `json:"config"`
		Credential providers.Credential `json:"credential"`
		Remove     bool                 `json:"remove"`
		Kind       string               `json:"kind"`
		Page       int                  `json:"page"`
		Query      string               `json:"query"`
	}
	if strictDecode(raw, &req) != nil {
		return nil, errors.New("Invalid provider request")
	}
	var err error
	switch name {
	case "configureGoProvider":
		err = s.providers.Configure(req.ID, req.Config)
	case "setGoProviderCredential":
		err = s.providers.SetCredential(req.ID, req.Credential)
	case "disconnectGoProvider":
		err = s.providers.Disconnect(req.ID, req.Remove)
	case "readGoProvider":
		page := req.Page
		if page == 0 {
			page = 1
		}
		return s.providers.Read(ctx, req.ID, req.Kind, req.Query, page)
	default:
		return nil, errors.New("Unsupported provider action")
	}
	if err != nil {
		return nil, err
	}
	return s.providers.Status(), nil
}
func (s *Server) frontierCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if s.providers == nil || r.Method != "GET" || len(r.URL.RawQuery) > 4096 {
		http.Error(w, "Invalid authorization callback", 400)
		return
	}
	q := r.URL.Query()
	if len(q["state"]) != 1 || len(q["code"]) != 1 || q.Get("error") != "" {
		http.Error(w, "Authorization declined or invalid. Start again in owner settings.", 400)
		return
	}
	if s.providers.CompleteAuthorization(r.Context(), s.url+"/owner/frontier-callback", q.Get("state"), q.Get("code")) != nil {
		http.Error(w, "Authorization failed. Check approval, callback registration and vault, then start again.", 400)
		return
	}
	_, _ = w.Write([]byte("Frontier connected. Return to DAEDALUS owner settings. Tokens were not sent to this page."))
}
