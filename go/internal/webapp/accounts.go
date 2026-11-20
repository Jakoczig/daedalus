package webapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
)

func (s *Server) accountRPC(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.game == nil {
		return nil, errors.New("Journal service unavailable")
	}
	if name == "setGoAvatar" {
		return nil, errors.New("Use Change portrait in the native launcher")
	}
	if name == "chooseGoAvatar" {
		var q struct {
			FID string `json:"fid"`
		}
		if strictDecode(raw, &q) != nil || s.chooseAvatar == nil {
			return nil, errors.New("Native launcher portrait selection required")
		}
		found := false
		for _, c := range s.game.AccountCards() {
			if c.FID == q.FID {
				found = true
			}
		}
		if !found {
			return nil, errors.New("Choose a verified local commander")
		}
		data, err := s.chooseAvatar()
		if err != nil {
			return nil, err
		}
		image, err := avatarPNG(base64.StdEncoding.EncodeToString(data))
		if err != nil {
			return nil, err
		}
		if err = privatefs.Write(s.profile, "avatar-"+tokenHash(q.FID)+".png", image); err != nil {
			return nil, err
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
	confirmedChoice := false
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
		confirmedChoice = !q.Multiple || previousSettings.Multiple
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
	// Successful explicit selection (including reselecting the persisted profile)
	// confirms this launch. Enabling multi-account mode still requires card choice.
	if name == "setGoAccounts" {
		s.accountConfirmed.Store(confirmedChoice)
	}
	cards := []map[string]any{}
	for _, c := range s.game.AccountCards() {
		cards = append(cards, map[string]any{"fid": c.FID, "name": c.Name, "shipName": c.ShipName, "shipType": c.ShipType, "system": c.System, "active": c.Active, "avatar": s.avatarThumbnail(c.FID)})
	}
	return map[string]any{"settings": s.game.Accounts(), "cards": cards, "choiceRequired": s.accountChoiceRequired()}, nil
}

func (s *Server) accountChoiceRequired() bool {
	return s.game != nil && s.game.Accounts().Multiple && !s.accountConfirmed.Load()
}
