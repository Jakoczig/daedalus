package webapp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestG2MBActivityContextUsesSelectedAccountAndStrictEnum(t *testing.T) {
	s, _, _, _, _ := g2eFixture(t)
	for _, scope := range []string{"commander", "missions", "powerplay", "colonisation"} {
		raw, _ := json.Marshal(map[string]any{"context": scope})
		if _, err := s.fieldRPC(context.Background(), "getGoBGSTally", raw); err != nil {
			t.Fatal(scope, err)
		}
	}
	for _, raw := range []string{`{"context":"unknown"}`, `{"context":"missions' OR 1=1"}`, `{"context":"missions","fid":"OTHER_ACCOUNT"}`, `{"context":"missions","role":"owner"}`} {
		if _, err := s.fieldRPC(context.Background(), "getGoBGSTally", json.RawMessage(raw)); err == nil {
			t.Fatal("Invalid/foreign-account context authorized", raw)
		}
	}
	if _, err := s.fieldRPC(context.Background(), "exportGoBGSTally", json.RawMessage(`{"context":"missions"}`)); err == nil {
		t.Fatal("Context export skipped acknowledgment")
	}
}
