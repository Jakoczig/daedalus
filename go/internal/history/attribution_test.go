package history

import (
	"context"
	"database/sql"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"testing"
)

func TestMalformedLineAtCheckpointBoundaryBreaksCommanderAttribution(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	src := journal.Source{ID: "malformed-boundary", Identity: "synthetic"}
	cp, _ := s.Resume(src)
	load, _ := journal.ParseLine([]byte(`{"event":"LoadGame","FID":"SYNTHETIC-A"}`), journal.SourcePosition{FileID: src.ID, Sequence: 1, Offset: 0, Length: 100})
	cp.Sequence = 2
	cp.Offset = 200 // second complete line was malformed, so no parsed event exists.
	if err = s.Commit(context.Background(), cp, []journal.Record{{Event: load, Phase: journal.Replay}}, nil); err != nil {
		t.Fatal(err)
	}
	jump, _ := journal.ParseLine([]byte(`{"event":"FSDJump","StarSystem":"Unattributed","SystemAddress":9007199254740993}`), journal.SourcePosition{FileID: src.ID, Sequence: 3, Offset: 200, Length: 100})
	cp.Sequence = 3
	cp.Offset = 300
	if err = s.Commit(context.Background(), cp, []journal.Record{{Event: jump, Phase: journal.Live}}, nil); err != nil {
		t.Fatal(err)
	}
	var fid sql.NullString
	if err = s.reader.QueryRow("SELECT commander_fid FROM journal_events WHERE type='FSDJump'").Scan(&fid); err != nil {
		t.Fatal(err)
	}
	if fid.Valid {
		t.Fatal("malformed checkpoint preserved stale FID", fid.String)
	}
}
