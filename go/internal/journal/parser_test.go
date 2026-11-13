package journal

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseJournalUTF8CRLFAndFileheader(t *testing.T) {
	for _, kind := range []string{"Fileheader", "fileheader"} {
		e, err := ParseLine([]byte(`{"timestamp":"2026-01-01T12:00:00.1234567Z","event":"`+kind+`","gameversion":"synthetic","build":"fixture"}`+"\r"), SourcePosition{Path: "private.log", FileID: "identity", Offset: 42})
		if err != nil || e.Type != "Fileheader" || e.GameVersion != "synthetic" || e.GameBuild != "fixture" || e.Position.Offset != 42 || e.Timestamp.Nanosecond() != 123456700 {
			t.Fatalf("envelope: %+v %v", e, err)
		}
	}
	e, err := ParseLine([]byte(`{"event":"Location","StarSystem":"Fixture Ω"}`), SourcePosition{})
	if err != nil || e.SystemName != "Fixture Ω" || !e.Timestamp.IsZero() {
		t.Fatal("UTF8/missing timestamp rejected", err)
	}
}
func TestParseExactSystemAndMarketIDs(t *testing.T) {
	e, err := ParseLine([]byte(`{"event":"Location","SystemAddress":9223372036854775807,"MarketID":9007199254740993}`), SourcePosition{})
	if err != nil || e.SystemAddress != "9223372036854775807" || e.MarketID != "9007199254740993" {
		t.Fatal("ID rounded", e, err)
	}
	e, err = ParseLine([]byte(`{"event":"Location","SystemAddress":18446744073709551615}`), SourcePosition{})
	if err != nil || e.SystemAddress != "18446744073709551615" {
		t.Fatal("uint64 lost", err)
	}
	for _, id := range []string{"-1", "1.5", "1e6", "18446744073709551616"} {
		if _, err := ParseLine([]byte(`{"event":"Location","SystemAddress":`+id+`}`), SourcePosition{}); err == nil {
			t.Fatal("non-uint64 ID accepted", id)
		}
	}
}
func TestParseDuplicateTimestampNotDedupKey(t *testing.T) {
	line := []byte(`{"event":"Location","timestamp":"2026-01-01T00:00:00Z"}`)
	a, _ := ParseLine(line, SourcePosition{Offset: 0})
	b, _ := ParseLine(line, SourcePosition{Offset: 80})
	if a.Position == b.Position || a.Timestamp != b.Timestamp {
		t.Fatal("timestamp replaced source provenance")
	}
}
func TestParseMalformedCompleteLine(t *testing.T) {
	for _, line := range []string{"", `[]`, `{}`, `{"event":"Location"`, `{"event":"Location"} {}`, `{"event":42}`, `{"event":"Location","timestamp":"bad"}`, string(bytes.Repeat([]byte("x"), (1<<20)+1))} {
		if _, err := ParseLine([]byte(line), SourcePosition{Path: "SECRET_PATH"}); err == nil || strings.Contains(err.Error(), "SECRET_PATH") {
			t.Fatal("malformed record accepted or diagnostic leaked path")
		}
	}
}
func TestParseUnknownFutureEvent(t *testing.T) {
	e, err := ParseLine([]byte(`{"event":"FutureEvent","Unknown":{"ID":9007199254740993}}`), SourcePosition{})
	if err != nil || e.Type != "FutureEvent" || !bytes.Contains(e.Raw, []byte("9007199254740993")) {
		t.Fatal("future event lost", err)
	}
}
func TestSyntheticFixturesContainNoPrivateData(t *testing.T) {
	root := filepath.Join("..", "..", "..", "tests", "testdata", "g2a")
	raw, err := os.ReadFile(filepath.Join(root, "replay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		e, err := ParseLine(line, SourcePosition{})
		if err != nil {
			t.Fatal(err)
		}
		if e.FID != "" && !strings.HasPrefix(e.FID, "SYNTHETIC-") {
			t.Fatal("nonfixture identity")
		}
		var value map[string]any
		json.Unmarshal(line, &value)
		for _, key := range []string{"Commander", "StarPos", "Message", "token", "credentials"} {
			if _, ok := value[key]; ok {
				t.Fatal("private fixture field", key)
			}
		}
	}
}
