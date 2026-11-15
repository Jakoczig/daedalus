package history

import (
	"context"
	"github.com/Jakoczig/daedalus/go/internal/journal"
	"testing"
)

func TestSaleOnlyLinksProvenPriorSystemObservations(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	src := journal.Source{ID: "sales", Identity: "synthetic"}
	cp, _ := s.Resume(src)
	raw := []string{
		`{"event":"LoadGame","FID":"SYNTHETIC"}`,
		`{"event":"FSDJump","StarSystem":"Fixture sold","SystemAddress":9007199254740993}`,
		`{"event":"Scan","BodyID":1,"BodyName":"Fixture sold 1","SystemAddress":9007199254740993}`,
		`{"event":"FSDJump","StarSystem":"Fixture unsold","SystemAddress":9007199254740994}`,
		`{"event":"Scan","BodyID":1,"BodyName":"Fixture unsold 1","SystemAddress":9007199254740994}`,
		`{"event":"SellExplorationData","Systems":["Fixture sold"]}`,
		`{"event":"Scan","BodyID":2,"SystemAddress":9007199254740993}`,
		`{"event":"SellOrganicData","BioData":[{"Genus":"bacterium","Species":"fixture"}]}`,
	}
	records := []journal.Record{}
	for i, r := range raw {
		e, err := journal.ParseLine([]byte(r), journal.SourcePosition{FileID: src.ID, Sequence: uint64(i + 1), Offset: int64(i * 100), Length: 100})
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, journal.Record{Event: e, Phase: journal.Replay})
	}
	cp.Offset = 800
	cp.Sequence = 8
	if err = s.Commit(context.Background(), cp, records, nil); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = s.reader.QueryRow("SELECT count(*) FROM sale_item_links WHERE confidence='proven'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("sale coverage expanded: %d %v", n, err)
	}
	var rawID int64
	s.reader.QueryRow("SELECT observation_event_id FROM sale_item_links WHERE confidence='proven'").Scan(&rawID)
	if rawID != 3 {
		t.Fatalf("wrong sold observation %d", rawID)
	}
}
