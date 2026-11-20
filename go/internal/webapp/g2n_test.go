package webapp

import (
	"encoding/json"
	"testing"
)

func TestG2NLauncherRequiresExplicitChoicePerProcess(t *testing.T) {
	s, _, viewer, url, _ := g2eFixture(t)
	cards := s.game.AccountCards()
	if len(cards) == 0 {
		t.Fatal("fixture commander missing")
	}
	fid := cards[0].FID
	if err := s.game.SetAccounts(true, fid); err != nil {
		t.Fatal(err)
	}
	opened := 0
	s.companion.action = func(action string) error {
		if action == "dashboard" {
			opened++
		}
		return nil
	}
	c, ctx := dialOwner(t, s)
	view := callRPC(t, ctx, c, "getGoAccounts")
	var accounts struct {
		ChoiceRequired bool `json:"choiceRequired"`
	}
	json.Unmarshal(view["message"], &accounts)
	if !accounts.ChoiceRequired {
		t.Fatal("persisted profile silently selected")
	}
	if len(rpcMessage(t, ctx, c, "desktopAction", map[string]string{"action": "dashboard"})["error"]) == 0 || opened != 0 {
		t.Fatal("dashboard opened before choice")
	}
	if len(rpcMessage(t, ctx, c, "setGoAccounts", map[string]any{"multiple": true, "selected": "UNVERIFIED"})["error"]) == 0 {
		t.Fatal("unverified commander accepted")
	}
	if !s.accountChoiceRequired() {
		t.Fatal("failed selection bypassed choice")
	}
	if len(rpcMessage(t, ctx, c, "setGoAccounts", map[string]any{"multiple": true, "selected": fid})["error"]) != 0 {
		t.Fatal("verified selection failed")
	}
	if len(rpcMessage(t, ctx, c, "desktopAction", map[string]string{"action": "dashboard"})["error"]) != 0 || opened != 1 {
		t.Fatal("explicit selected dashboard unavailable")
	}
	restored := &Server{game: s.game}
	if !restored.accountChoiceRequired() {
		t.Fatal("new process inherited transient choice")
	}
	v := pairedSocket(t, viewer, url)
	if len(callRPC(t, ctx, v, "refreshGoCommunityGoals")["error"]) == 0 {
		t.Fatal("viewer initiated Frontier goals read")
	}
}
