package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/providers"
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) providerRPC(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.game != nil {
		if fid := s.game.FieldCommander(); fid != "" {
			if e := s.providers.ActivateAccount(fid, false); e != nil {
				return nil, e
			}
		}
	}

	if name == "previewGoHistorical" || name == "confirmGoHistorical" {
		var selection providers.HistorySelection
		if strictDecode(raw, &selection) != nil {
			return nil, errors.New("Invalid historical selection")
		}
		if selection.FID != s.game.FieldCommander() {
			return nil, errors.New("Switch the global commander profile before reviewing another account")
		}
		return s.providers.Historical(ctx, selection, name == "confirmGoHistorical")
	}
	switch name {
	case "setGoFrontierClient":
		var q struct {
			ClientID string `json:"clientId"`
		}
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Invalid Frontier client settings")
		}
		if e := s.providers.ConfigureFrontierClient(q.ClientID); e != nil {
			return nil, e
		}
		return map[string]bool{"saved": true}, nil
	case "completeGoFrontierReturn":
		var q struct {
			URL string `json:"url"`
		}
		if strictDecode(raw, &q) != nil || len(q.URL) > 4096 {
			return nil, errors.New("Invalid authorization return")
		}
		u, e := url.Parse(q.URL)
		if e != nil || u.Scheme != "http" || u.Host != strings.TrimPrefix(s.url, "http://") || u.Path != "/owner/frontier-callback" || u.User != nil || u.Fragment != "" {
			return nil, errors.New("Paste only the fixed local return URL from the ERIDION authorization page")
		}
		v := u.Query()
		if len(v["state"]) != 1 || len(v["code"]) != 1 || v.Get("error") != "" {
			return nil, errors.New("Invalid authorization return")
		}
		if e = s.providers.CompleteAuthorization(ctx, s.url+"/owner/frontier-callback", v.Get("state"), v.Get("code")); e != nil {
			return nil, e
		}
		return map[string]bool{"connected": true}, nil
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
	if len(q["state"]) == 1 && len(q["error"]) == 1 && q.Get("error") != "" {
		_ = s.providers.CancelAuthorization(s.url+"/owner/frontier-callback", q.Get("state"))
	}
	if len(q["state"]) != 1 || len(q["code"]) != 1 || q.Get("error") != "" {
		http.Error(w, "Authorization declined or invalid. Start again in owner settings.", 400)
		return
	}
	if s.providers.CompleteAuthorization(r.Context(), s.url+"/owner/frontier-callback", q.Get("state"), q.Get("code")) != nil {
		http.Error(w, "Authorization failed. Check approval, callback registration and vault, then start again.", 400)
		return
	}
	_, _ = w.Write([]byte("Frontier connected. Return to ERIDION owner settings. Tokens were not sent to this page."))
}
