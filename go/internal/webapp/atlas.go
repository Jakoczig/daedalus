package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/atlas"
	"net/http"
	"strings"
)

func (s *Server) atlasRPC(ctx context.Context, name string, raw json.RawMessage, owner bool) (any, error) {
	if name != "getGoAtlasView" && !owner {
		return nil, errors.New("Local owner required")
	}
	if s.atlas == nil {
		if name == "getGoAtlasView" {
			return atlas.View{Tiles: []atlas.Tile{}}, nil
		}
		return nil, errors.New("Atlas cache unavailable")
	}
	switch name {
	case "getGoAtlasStatus":
		return s.atlas.Status(), nil
	case "getGoAtlasView":
		var q atlas.ViewRequest
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Invalid atlas viewport")
		}
		return s.atlas.View(q, owner)
	case "configureGoAtlas":
		var q atlas.Preferences
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Invalid atlas preferences")
		}
		if e := s.atlas.Configure(q); e != nil {
			return nil, e
		}
	case "checkGoAtlas":
		if e := s.atlas.Check(ctx, false); e != nil {
			return nil, e
		}
	case "installGoAtlas":
		if e := s.atlas.StartInstall(); e != nil {
			return nil, e
		}
	case "rollbackGoAtlas":
		if e := s.atlas.Rollback(); e != nil {
			return nil, e
		}
	case "clearGoAtlasDetails":
		if e := s.atlas.ClearDetails(); e != nil {
			return nil, e
		}
	default:
		return nil, errors.New("Unknown atlas operation")
	}
	return s.atlas.Status(), nil
}
func (s *Server) atlasAsset(w http.ResponseWriter, r *http.Request, authenticated bool) {
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !authenticated {
		http.Error(w, "Forbidden", 403)
		return
	}
	if r.Method != "GET" || r.URL.RawQuery != "" || r.URL.RawPath != "" || s.atlas == nil {
		http.NotFound(w, r)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/atlas/assets/"), "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	b, e := s.atlas.Asset(parts[0], parts[1])
	if e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(b)
}
