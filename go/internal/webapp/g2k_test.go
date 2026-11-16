package webapp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestG2KOwnerSelectedCSVPreservesBytesAndRequiresExportConsent(t *testing.T) {
	s, _, _, _, _ := g2eFixture(t)
	raw := []byte("\ufeffSystem Name\nFixture Alpha\nFixture Beta\n")
	payload, _ := json.Marshal(map[string]any{"name": "Byte fixture", "csvBase64": base64.StdEncoding.EncodeToString(raw)})
	if _, err := s.fieldRPC(context.Background(), "importGoItinerary", payload); err != nil {
		t.Fatal(err)
	}
	db, _ := s.game.History()
	p, err := db.Itinerary(s.game.FieldCommander(), "")
	if err != nil || p.Hash != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("file hash changed", p, err)
	}
	if _, err = s.fieldRPC(context.Background(), "exportGoItinerary", json.RawMessage(`{}`)); err == nil {
		t.Fatal("unacknowledged export")
	}
	calls := 0
	s.copyWaypoint = func(text string) error {
		calls++
		if text != "Fixture Beta" {
			t.Fatal(text)
		}
		return nil
	}
	if _, err = s.fieldRPC(context.Background(), "copyGoItineraryNext", json.RawMessage(`{}`)); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
}

func TestG2KViewerCannotOperatePlansClipboardTallyOrExports(t *testing.T) {
	s, _, v, u, _ := g2eFixture(t)
	c := pairedSocket(t, v, u)
	defer c.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, name := range []string{"getGoBuiltinModules", "setGoBuiltinModule", "importGoItinerary", "getGoItinerary", "editGoItinerary", "copyGoItineraryNext", "exportGoItinerary", "getGoBGSTally", "exportGoBGSTally"} {
		if len(callRPC(t, ctx, c, name)["error"]) == 0 {
			t.Fatal("viewer authorized", name)
		}
	}
	r := callRPC(t, ctx, c, "getGoFieldView")
	if len(r["error"]) != 0 {
		t.Fatal(r)
	}
	b, _ := json.Marshal(s.fieldView(false))
	for _, secret := range []string{"SYNTHETIC", "commander_fid", s.profile, "source_hash", "itinerary_waypoints"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("field projection leaked private metadata", secret)
		}
	}
}
