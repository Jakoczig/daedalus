package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/daedalus/go/internal/history"
	"github.com/Jakoczig/daedalus/go/internal/privatefs"
	"os"
	"path/filepath"
	"time"
)

// Every caller is explicitly owner-authorized in socket.go, before decoding paths.
func (s *Server) historyRPC(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.game == nil {
		return nil, errors.New("Native history unavailable")
	}
	db, notice := s.game.History()
	if name == "getGoDataStatus" {
		value := map[string]any{"summary": s.game.Snapshot(), "notice": notice, "commanders": []any{}, "sources": []any{}, "database": nil, "backupFolder": filepath.Join(s.profile, "backups")}
		_, e := os.Stat(filepath.Join(s.profile, "restore.pending.json"))
		value["restorePending"] = e == nil
		if db != nil {
			value["database"] = db.Stats()
			if v, e := db.Commanders(); e == nil {
				value["commanders"] = v
			}
			if v, e := db.Sources(); e == nil {
				value["sources"] = v
			}
		}
		return value, nil
	}
	var req struct {
		Sale        string  `json:"sale"`
		Observation string  `json:"observation"`
		Radius      float64 `json:"radius"`
		Address     string  `json:"address"`
		Service     string  `json:"service"`
		FID         string  `json:"fid"`
		Kind        string  `json:"kind"`
		Before      string  `json:"before"`
		Limit       int     `json:"limit"`
		Path        string  `json:"path"`
		Source      string  `json:"source"`
		Acknowledge bool    `json:"acknowledge"`
		Mode        string  `json:"mode"`
		Provider    string  `json:"provider"`
		Category    string  `json:"category"`
		Enabled     bool    `json:"enabled"`
	}
	if strictDecode(raw, &req) != nil {
		return nil, errors.New("Invalid owner history request")
	}
	if name == "stageGoRestore" {
		if !req.Acknowledge {
			return nil, errors.New("Explicit restore acknowledgment required; history will change and publication permissions will be paused")
		}
		if s.providers != nil {
			s.providers.CancelPublications()
		}
		if err := history.StageRecovery(ctx, s.profile, req.Path); err != nil {
			return nil, errors.New("Backup could not be verified/staged. Select a private compatible SQLite backup; existing data was retained")
		}
		return map[string]any{"restartRequired": true, "notice": "Verified restore staged. Quit and restart DAEDALUS. Current history is preserved; uploads require renewed consent."}, nil
	}
	if db == nil {
		return nil, errors.New("Database is not ready; check recovery status")
	}
	switch name {
	case "getGoNearest":
		return db.Nearest(ctx, req.Address, req.Service, req.Radius, time.Now())
	case "getGoSaleLinks":
		return db.SaleLinks(req.FID)
	case "confirmGoOrganicSale":
		if err := db.ConfirmOrganicSale(req.FID, req.Sale, req.Observation, req.Acknowledge); err != nil {
			return nil, err
		}
		return db.SaleLinks(req.FID)
	case "getGoSourceFacts":
		return db.EffectiveFacts(ctx, req.FID, "commander", time.Now())
	case "queryGoHistory":
		if req.Limit == 0 {
			req.Limit = 25
		}
		return db.Page(ctx, req.FID, req.Kind, req.Before, req.Limit)
	case "backupGoHistory", "exportGoHistory":
		if !req.Acknowledge {
			return nil, errors.New("Backups/exports contain private commander and location history; acknowledge before exporting")
		}
		destination := req.Path
		if destination == "" {
			dir := filepath.Join(s.profile, "backups")
			if err := privatefs.Prepare(dir); err != nil {
				return nil, errors.New("Private backup directory unavailable")
			}
			destination = filepath.Join(dir, "manual-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".sqlite")
		}
		hash, err := db.Backup(ctx, destination)
		if err != nil {
			return nil, errors.New("Backup/export failed. Choose a new file in a private local folder; original database was retained")
		}
		return map[string]string{"path": destination, "sha256": hash, "notice": "Consistent SQLite export verified; contains private game history, no stored API credentials"}, nil
	case "rebuildGoHistory", "reimportGoSource":
		if !req.Acknowledge {
			return nil, errors.New("Explicit rebuild/reimport acknowledgment required")
		}
		if s.providers != nil {
			s.providers.CancelPublications()
		}
		source := ""
		if name == "reimportGoSource" {
			source = req.Source
			if source == "" {
				return nil, errors.New("Choose a quarantined source")
			}
		}
		if err := s.game.RebuildDerived(ctx, source); err != nil {
			return nil, errors.New("Derived rebuild/recovery failed; immutable history and committed cache retained")
		}
		return map[string]bool{"rebuilt": true}, nil
	case "getGoPublicationPolicy":
		return s.providers.PublicationPolicy(req.FID)
	case "setGoPublicationPolicy":
		if !req.Acknowledge {
			return nil, errors.New("Review exploration timing and recipient permissions before confirming")
		}
		if err := s.providers.SetPublicationPolicy(req.FID, req.Mode); err != nil {
			return nil, err
		}
		return s.providers.PublicationPolicy(req.FID)
	case "setGoPublicationException":
		if err := s.providers.SetLiveException(req.FID, req.Provider, req.Category, req.Enabled, req.Acknowledge); err != nil {
			return nil, err
		}
		return s.providers.PublicationPolicy(req.FID)
	}
	return nil, errors.New("Unsupported owner history action")
}
