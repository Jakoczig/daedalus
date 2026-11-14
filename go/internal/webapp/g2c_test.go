package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestG2CViewerBodyDetailsAndBiologyAreAllowlisted(t *testing.T) {
	dir := t.TempDir()
	for _, item := range [][2]string{{"replay.jsonl", "Journal.20260101.01.log"}, {"NavRoute.json", "NavRoute.json"}} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/testdata/g2c", item[0]))
		if err != nil {
			t.Fatal(err)
		}
		if os.WriteFile(filepath.Join(dir, item[1]), raw, 0600) != nil {
			t.Fatal("fixture write")
		}
	}
	s, _, viewer, url, _ := startGameFixture(t, dir, true, "Fixture Alpha", true)
	deadline := time.Now().Add(3 * time.Second)
	for {
		v := s.game.Exploration()
		complete := false
		for _, body := range v.Bodies {
			if len(body.Organisms) > 0 && body.Organisms[0].Complete {
				complete = true
			}
		}
		if complete {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("organic replay did not settle")
		}
		time.Sleep(time.Millisecond)
	}
	conn := pairedSocket(t, viewer, url)
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reply := callRPC(t, ctx, conn, "getGoExploration")
	raw := reply["message"]
	for _, secret := range []string{"PRIVATE_", "FID", "Commander", "Latitude", "Longitude", "SystemAddress", "Materials", "Journal.", "fixture-genus"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("private metadata leaked", secret)
		}
	}
	top := onlyKeys(t, raw, "systemName surveyState reportedBodyCount fssProgress allBodiesFound scannerBodyCount bodies truncated")
	var bodies []json.RawMessage
	json.Unmarshal(top["bodies"], &bodies)
	seen := false
	for _, body := range bodies {
		var optional map[string]json.RawMessage
		json.Unmarshal(body, &optional)
		keys := "name kind bodyId atmosphere volcanism landable terraformable gravityG distanceLS bioSignals geoSignals previouslyDiscovered previouslyMapped dssMapped genuses"
		if optional["details"] != nil {
			keys += " details"
		}
		if optional["organisms"] != nil {
			keys += " organisms"
		}
		fields := onlyKeys(t, body, keys)
		if d := fields["details"]; d != nil {
			detail := onlyKeys(t, d, "isStar radiusKM temperatureK pressureATM massEarths massSolar ageMY orbitalDays rotationHours semiMajorAU eccentricity axialTiltDegrees tidalLock luminosity parents rings composition atmosphereComposition")
			var parents []json.RawMessage
			json.Unmarshal(detail["parents"], &parents)
			for _, p := range parents {
				onlyKeys(t, p, "kind bodyId")
			}
			var rings []json.RawMessage
			json.Unmarshal(detail["rings"], &rings)
			for _, r := range rings {
				onlyKeys(t, r, "name class innerKM outerKM")
			}
		}
		if o := fields["organisms"]; o != nil {
			var organisms []json.RawMessage
			json.Unmarshal(o, &organisms)
			for _, o := range organisms {
				onlyKeys(t, o, "genus species variant samples stage complete lastObservedAt")
			}
			seen = strings.Contains(string(o), "Fixture Bacterium") && strings.Contains(string(o), `"complete":true`)
		}
	}
	if !seen {
		t.Fatal("viewer lost confirmed organic evidence", string(raw))
	}
	loading := callRPC(t, ctx, conn, "getLoadingStatus")
	if !strings.Contains(string(loading["message"]), `"stage":"G2C"`) {
		t.Fatal("wrong prototype")
	}
	for _, name := range []string{"setTarget", "selectJournalDir", "execute", "runCommand"} {
		if response := callRPC(t, ctx, conn, name); response["error"] == nil {
			t.Fatal("viewer acquired host action", name)
		}
	}
	// Source configuration remains owner-only on both remote and local listener.
	code, _, _ := remotePost(t, viewer, url+"/owner/journal", url, map[string]any{"action": "status"})
	if code != http.StatusForbidden {
		t.Fatal("viewer acquired owner configuration", code)
	}
}
