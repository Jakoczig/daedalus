package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/atlas"
)

func (s *Server) spatialRPC(ctx context.Context, name string, raw json.RawMessage, owner bool) (any, error) {
	if name != "getGoSpatialView" && !owner {
		return nil, errors.New("Local owner required")
	}
	if s.spatial == nil {
		if name == "getGoSpatialView" {
			return atlas.SpatialView{Points: [][3]float64{}, Notice: "Spatial cache unavailable; use density overview"}, nil
		}
		return nil, errors.New("Spatial cache unavailable")
	}
	switch name {
	case "getGoSpatialView":
		var q atlas.SpatialQuery
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Invalid spatial query")
		}
		return s.spatial.View(q, owner)
	case "getGoSpatialStatus":
	case "configureGoSpatial":
		var q atlas.SpatialPreferences
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Invalid spatial settings")
		}
		if e := s.spatial.Configure(q); e != nil {
			return nil, e
		}
	case "installGoBubble":
		var q struct {
			SHA256 string `json:"sha256"`
			Bytes  int64  `json:"bytes"`
		}
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Confirm Bubble size")
		}
		if e := s.spatial.StartBubble(q.SHA256, q.Bytes); e != nil {
			return nil, e
		}
	case "installGoSpatial":
		if e := s.spatial.StartUpdate(name == "installGoBubble", false); e != nil {
			return nil, e
		}
	case "rollbackGoSpatial":
		if e := s.spatial.Rollback(); e != nil {
			return nil, e
		}
	case "retryGoSpatial":
		s.spatial.Retry()
	case "removeGoBubble":
		var q struct {
			Confirm bool `json:"confirm"`
		}
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Confirm Bubble removal")
		}
		if e := s.spatial.RemoveBubble(q.Confirm); e != nil {
			return nil, e
		}
	default:
		return nil, errors.New("Unknown spatial operation")
	}
	return s.spatial.Status(), nil
}
