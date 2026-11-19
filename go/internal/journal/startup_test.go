package journal

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStartupBindingBoundsAndIdentity(t *testing.T) {
	for _, scenario := range []string{"valid", "restart", "foreign-fid", "foreign-file", "foreign-path", "duplicate-key", "duplicate-snapshot", "gap", "truncated", "too-many", "too-large", "expired", "older"} {
		t.Run(scenario, func(t *testing.T) {
			var s Startup
			var offset int64
			var seq uint64
			stamp := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
			next := func(kind, fields string) Event {
				seq++
				raw := []byte(`{"event":"` + kind + `","timestamp":"` + stamp.Format(time.RFC3339Nano) + `"` + fields + `}`)
				e, err := ParseLine(raw, SourcePosition{Path: "/synthetic/A.log", FileID: "file-a", Sequence: seq, Offset: offset, Length: int64(len(raw) + 1)})
				if err != nil {
					t.Fatal(err)
				}
				offset += e.Position.Length
				return e
			}
			header := next("Fileheader", "")
			if scenario != "truncated" {
				s.Observe(header)
			}
			s.Observe(next("Commander", `,"FID":"SYNTHETIC-A"`))
			fields := `,"Combat":3`
			if scenario == "duplicate-key" {
				fields += `,"Combat":4`
			}
			if scenario == "too-large" {
				fields += `,"unused":"` + strings.Repeat("x", 512<<10) + `"`
			}
			rank := next("Rank", fields)
			if scenario == "gap" {
				rank.Position.Sequence++
			}
			s.Observe(rank)
			if scenario == "restart" {
				raw, _ := json.Marshal(s)
				s = Startup{}
				if err := json.Unmarshal(raw, &s); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "duplicate-snapshot" {
				s.Observe(next("Rank", `,"Combat":4`))
			}
			if scenario == "too-many" {
				for i := 0; i < 64; i++ {
					s.Observe(next("Music", ""))
				}
			}
			if scenario == "expired" {
				stamp = stamp.Add(3 * time.Minute)
			}
			load := next("LoadGame", `,"FID":"SYNTHETIC-A"`)
			switch scenario {
			case "foreign-fid":
				load = next("LoadGame", `,"FID":"SYNTHETIC-B"`)
			case "foreign-file":
				load.Position.FileID = "file-b"
			case "foreign-path":
				load.Position.Path = "/synthetic/B.log"
			case "older":
				s.HighWater = stamp.Add(time.Hour)
			}
			batch := s.Observe(load)
			want := scenario == "valid" || scenario == "restart"
			if (len(batch.Events(load)) == 1) != want {
				t.Fatalf("binding accepted=%v want=%v", len(batch.Events(load)) == 1, want)
			}
			if want {
				load.FID = "SYNTHETIC-B"
				if len(batch.Events(load)) != 0 {
					t.Fatal("batch reused for another commander")
				}
			}
		})
	}
}
