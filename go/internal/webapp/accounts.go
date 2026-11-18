package webapp

import (
	"context"
	"encoding/json"
	"errors"
)

func (s *Server) accountRPC(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.game == nil {
		return nil, errors.New("Journal service unavailable")
	}
	if name == "setGoAvatar" {
		if e := s.saveAvatar(raw); e != nil {
			return nil, e
		}
		return map[string]bool{"saved": true}, nil
	}
	if name == "getGoCommanderOverview" {
		v, e := s.game.CommanderOverview(ctx)
		if e == nil {
			fid, _ := v["fid"].(string)
			v["avatar"] = s.avatarURIFor(fid)
		}
		return v, e
	}
	if name == "setGoAccounts" {
		s.accountMu.Lock()
		defer s.accountMu.Unlock()
		var q struct {
			Multiple bool   `json:"multiple"`
			Selected string `json:"selected"`
		}
		if strictDecode(raw, &q) != nil {
			return nil, errors.New("Invalid account selection")
		}
		// Verify before touching any credential namespace.
		found := !q.Multiple
		for _, c := range s.game.AccountCards() {
			if c.FID == q.Selected {
				found = true
			}
		}
		if !found {
			return nil, errors.New("Select a verified local journal account")
		}
		previousSettings := s.game.Accounts()
		target := q.Selected
		if !q.Multiple {
			target = ""
		}
		previousPin := ""
		if previousSettings.Multiple {
			previousPin = previousSettings.Selected
		}
		if s.providers != nil {
			s.providers.PinAccount(target)
		}
		if e := s.game.SetAccounts(q.Multiple, q.Selected); e != nil {
			if s.providers != nil {
				s.providers.PinAccount(previousPin)
			}
			return nil, e
		}
		if s.providers != nil {
			s.providers.PinAccount(target)
			if fid := s.game.FieldCommander(); fid != "" {
				if e := s.providers.ActivateAccount(fid, false); e != nil {
					_ = s.game.SetAccounts(previousSettings.Multiple, previousSettings.Selected)
					s.providers.PinAccount(previousPin)
					return nil, e
				}
			}
		}
	}
	cards := []map[string]any{}
	for _, c := range s.game.AccountCards() {
		cards = append(cards, map[string]any{"fid": c.FID, "name": c.Name, "shipName": c.ShipName, "shipType": c.ShipType, "system": c.System, "active": c.Active, "avatar": s.avatarThumbnail(c.FID)})
	}
	return map[string]any{"settings": s.game.Accounts(), "cards": cards}, nil
}
