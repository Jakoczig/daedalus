package history

import (
	"context"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/journal"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func operationFixture(t testing.TB, lines ...string) *Store {
	t.Helper()
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	src := journal.Source{ID: "ops", Identity: "synthetic"}
	cp, _ := s.Resume(src)
	all := append([]string{`{"event":"Fileheader","timestamp":"2026-01-01T00:00:00Z","gameversion":"4.1"}`, `{"event":"LoadGame","timestamp":"2026-01-01T00:00:01Z","FID":"SYNTHETIC"}`}, lines...)
	for i, raw := range all {
		if !containsTimestamp(raw) {
			raw = raw[:len(raw)-1] + fmt.Sprintf(`,"timestamp":"2026-01-01T00:01:%02dZ"}`, i)
		}
		e, err := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: src.ID, Sequence: uint64(i + 1), Offset: cp.Offset, Length: int64(len(raw) + 1)})
		if err != nil {
			t.Fatal(err)
		}
		cp.Sequence++
		cp.Offset += int64(len(raw) + 1)
		if err = s.Commit(context.Background(), cp, []journal.Record{{Event: e, Phase: journal.Replay}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func containsTimestamp(s string) bool {
	for i := 0; i+11 <= len(s); i++ {
		if s[i:i+11] == `"timestamp"` {
			return true
		}
	}
	return false
}
func TestG2LOperationsAtomicFleetCarrierEconomy(t *testing.T) {
	s := operationFixture(t,
		`{"event":"Loadout","ShipID":9007199254740993,"Ship":"asp","Modules":[]}`,
		`{"event":"ShipyardNew","NewShipID":9007199254740994,"ShipType":"asp"}`,
		`{"event":"ShipyardTransfer","ShipID":9007199254740993,"TransferPrice":100,"TransferTime":60}`,
		`{"event":"CarrierJumpRequest","CarrierID":18446744073709551615,"SystemName":"Pending"}`,
		`{"event":"CarrierStats","CarrierID":18446744073709551615,"FuelLevel":100,"Finance":{"CarrierBalance":9007199254740993}}`,
		`{"event":"CarrierJump","MarketID":123,"StarSystem":"Unrelated","SystemAddress":321}`,
		`{"event":"MarketBuy","MarketID":9007199254740993,"Type":"gold","Count":2,"BuyPrice":10,"TotalCost":20}`,
		`{"event":"MarketSell","MarketID":9007199254740993,"Type":"gold","Count":2,"SellPrice":30,"TotalSale":60,"AvgPricePaid":10}`,
		`{"event":"MarketSell","Type":"gold","Count":1,"TotalSale":30,"AvgPricePaid":0}`,
		`{"event":"CommunityGoal","CurrentGoals":[{"CGID":18446744073709551615,"Title":"Synthetic goal","PlayerContribution":12}]}`,
		`{"event":"CrewHire","Name":"PRIVATE_CREW","CrewID":18446744073709551615}`)
	if s.Stats().Schema != 6 {
		t.Errorf("need schema 6: %v", s.Stats())
	}
	var state string
	if e := s.reader.QueryRow("SELECT state FROM fleet_ships WHERE commander_fid='SYNTHETIC' AND ship_id='9007199254740993'").Scan(&state); e != nil || state != "in_transit" {
		t.Fatal("transfer not arrival", state, e)
	}
	var n int
	if e := s.reader.QueryRow("SELECT count(*) FROM carrier_facts WHERE field='location'").Scan(&n); e != nil || n != 0 {
		t.Fatal("unrelated carrier claimed", n, e)
	}
	var spread string
	if e := s.reader.QueryRow("SELECT gross_spread FROM trade_ledger WHERE action='MarketSell' AND avg_price_paid='10'").Scan(&spread); e != nil || spread != "40" {
		t.Fatal("cost basis", spread, e)
	}
	if e := s.reader.QueryRow("SELECT count(*) FROM trade_ledger WHERE action='MarketSell' AND gross_spread IS NULL").Scan(&n); e != nil || n != 1 {
		t.Fatal("unknown basis", n, e)
	}
	if e := s.reader.QueryRow("SELECT count(*) FROM commander_operations WHERE entity_id='18446744073709551615'").Scan(&n); e != nil || n != 2 {
		t.Fatal("exact CG/Crew identity", n, e)
	}
}

func TestG2LMarketRequiresDockMarkerIdentityAndNeverNeedsUploadConsent(t *testing.T) {
	s := operationFixture(t, `{"event":"Location","StarSystem":"Synthetic A","SystemAddress":9007199254740993}`, `{"event":"Docked","MarketID":9007199254740993,"StarSystem":"Synthetic A","StationName":"Port A"}`, `{"event":"Market","MarketID":9007199254740993,"StarSystem":"Synthetic A","StationName":"Port A"}`)
	var raw string
	var offset int64
	var seq uint64
	if e := s.reader.QueryRow("SELECT raw_relevant_json,byte_offset,record_sequence FROM journal_events WHERE type='Market'").Scan(&raw, &offset, &seq); e != nil {
		t.Fatal(e)
	}
	event, e := journal.ParseLine([]byte(raw), journal.SourcePosition{FileID: "ops", Offset: offset, Sequence: seq})
	if e != nil {
		t.Fatal(e)
	}
	good := []byte(`{"event":"Market","timestamp":"2026-01-01T00:01:04Z","MarketID":9007199254740993,"StarSystem":"Synthetic A","StationName":"Port A","Items":[{"Name":"gold","BuyPrice":null,"SellPrice":30,"Stock":5,"Demand":10}]}`)
	if e = s.SaveSidecar("SYNTHETIC", event, "Market", good); e != nil {
		t.Fatal(e)
	}
	var n int
	if e = s.reader.QueryRow("SELECT count(*) FROM commodity_quotes WHERE commodity='gold' AND buy_price IS NULL").Scan(&n); e != nil || n != 1 {
		t.Fatal("market local cache", n, e)
	}
	bad := []byte(`{"event":"Market","timestamp":"2026-01-01T00:01:04Z","MarketID":123,"StarSystem":"Synthetic B","StationName":"Port B","Items":[]}`)
	if e = s.SaveSidecar("SYNTHETIC", event, "Market", bad); e == nil {
		t.Fatal("stale other-market sidecar accepted")
	}
	if e = s.SaveSidecar("OTHER", event, "Market", good); e == nil {
		t.Fatal("other commander accepted")
	}
}

func TestG2LCopiedSourceIsNotDoubleAccountedAndPageIsExact(t *testing.T) {
	s := operationFixture(t, `{"event":"MarketSell","MarketID":18446744073709551615,"Type":"gold","Count":2,"TotalSale":9007199254740995,"AvgPricePaid":1}`)
	rows, e := s.reader.Query("SELECT raw_relevant_json,byte_offset,byte_length,record_sequence FROM journal_events ORDER BY event_id")
	if e != nil {
		t.Fatal(e)
	}
	records := []journal.Record{}
	for rows.Next() {
		var raw string
		var p journal.SourcePosition
		if e = rows.Scan(&raw, &p.Offset, &p.Length, &p.Sequence); e != nil {
			t.Fatal(e)
		}
		p.FileID = "copy"
		ev, e := journal.ParseLine([]byte(raw), p)
		if e != nil {
			t.Fatal(e)
		}
		records = append(records, journal.Record{Event: ev, Phase: journal.Replay})
	}
	rows.Close()
	cp, e := s.Resume(journal.Source{ID: "copy", Identity: "synthetic-copy"})
	if e != nil {
		t.Fatal(e)
	}
	last := records[len(records)-1].Event.Position
	cp.Sequence = last.Sequence
	cp.Offset = last.Offset + last.Length
	if e = s.Commit(context.Background(), cp, records, nil); e != nil {
		t.Fatal(e)
	}
	var n int
	s.reader.QueryRow("SELECT count(*) FROM trade_ledger").Scan(&n)
	if n != 1 {
		t.Fatal("copied journal counted twice", n)
	}
	page, e := s.Operations(context.Background(), "SYNTHETIC", OperationQuery{Kind: "trades", Limit: 1})
	if e != nil || len(page.Rows) != 1 {
		t.Fatal(page, e)
	}
	if string(page.Rows[0].Details) == "" || !strings.Contains(string(page.Rows[0].Details), `"9007199254740993"`) {
		t.Fatal("exact financial spread lost", page)
	}
	other, e := s.Operations(context.Background(), "OTHER", OperationQuery{Kind: "trades"})
	if e != nil || len(other.Rows) != 0 {
		t.Fatal("commander isolation", other, e)
	}
}
func TestG2LVerifiedOriginalBackfillAndRollback(t *testing.T) {
	s := operationFixture(t, `{"event":"CrewHire","Name":"PRIVATE_CREW","CrewID":123}`)
	var raw string
	var id, offset, length int64
	if e := s.reader.QueryRow("SELECT event_id,raw_relevant_json,byte_offset,byte_length FROM journal_events WHERE type='CrewHire'").Scan(&id, &raw, &offset, &length); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "Journal.synthetic.log")
	bytes := make([]byte, offset)
	bytes = append(bytes, []byte(raw+"\n")...)
	if e := os.WriteFile(path, bytes, 0600); e != nil {
		t.Fatal(e)
	}
	s.writer.Exec("UPDATE journal_sources SET canonical_path=? WHERE source_id='ops'", path)
	s.writer.Exec("DELETE FROM commander_operations WHERE event_id=?", id)
	s.writer.Exec("DELETE FROM operation_sources WHERE event_id=?", id)
	s.writer.Exec("UPDATE journal_events SET raw_relevant_json='{}' WHERE event_id=?", id)
	if n, e := s.RecoverOperations(context.Background(), "ops"); e != nil || n != 1 {
		t.Fatal("verified recovery", n, e)
	}
	if n, e := s.RecoverOperations(context.Background(), "ops"); e != nil || n != 0 {
		t.Fatal("recovery not idempotent", n, e)
	}
	s.writer.Exec("UPDATE journal_events SET raw_relevant_json='{}' WHERE event_id=?", id)
	if e := os.WriteFile(path, append(bytes[:offset], []byte(strings.Repeat("x", int(length)))...), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.RecoverOperations(context.Background(), "ops"); e == nil {
		t.Fatal("changed original accepted")
	}
	s.reader.QueryRow("SELECT raw_relevant_json FROM journal_events WHERE event_id=?", id).Scan(&raw)
	if raw != "{}" {
		t.Fatal("failed backfill committed", raw)
	}
}

func TestG2LMiningReceiptAttributionRequiresEmptyCargoAndNoMixedSource(t *testing.T) {
	s := operationFixture(t, `{"event":"Cargo","Vessel":"Ship","Count":0,"Inventory":[]}`, `{"event":"MiningRefined","Type":"gold"}`, `{"event":"MarketSell","Type":"gold","Count":1,"TotalSale":500,"AvgPricePaid":0}`, `{"event":"CollectCargo","Type":"gold"}`, `{"event":"MiningRefined","Type":"gold"}`, `{"event":"MarketSell","Type":"gold","Count":1,"TotalSale":500,"AvgPricePaid":0}`)
	var n int
	if e := s.reader.QueryRow("SELECT count(*) FROM trade_mining_summary").Scan(&n); e != nil || n != 1 {
		t.Fatal("mixed cargo cannot become mining revenue", n, e)
	}
	var sum string
	s.reader.QueryRow("SELECT receipts FROM trade_mining_summary").Scan(&sum)
	if sum != "500" {
		t.Fatal(sum)
	}
}

func TestG2LCarrierRequestNeedsMatchingOwnedArrivalAndOldStatsDoNotWin(t *testing.T) {
	s := operationFixture(t, `{"event":"CarrierStats","CarrierID":18446744073709551615,"FuelLevel":100,"Finance":{"CarrierBalance":20}}`, `{"event":"CarrierBankTransfer","CarrierID":18446744073709551615,"CarrierBalance":30,"Deposit":10}`, `{"event":"CarrierStats","timestamp":"2025-01-01T00:00:00Z","CarrierID":18446744073709551615,"Finance":{"CarrierBalance":5}}`, `{"event":"CarrierJumpRequest","CarrierID":18446744073709551615,"SystemAddress":9007199254740993,"SystemName":"Fixture A"}`, `{"event":"CarrierJump","MarketID":123,"SystemAddress":9007199254740993,"StarSystem":"Fixture A"}`, `{"event":"CarrierJump","MarketID":18446744073709551615,"SystemAddress":9007199254740993,"StarSystem":"Fixture A"}`)
	var value string
	s.reader.QueryRow("SELECT value_json FROM carrier_facts WHERE field='finance.CarrierBalance'").Scan(&value)
	if value != `"30"` {
		t.Fatal("stale snapshot overwrote finance", value)
	}
	s.reader.QueryRow("SELECT value_json FROM carrier_facts WHERE field='jumpRequest'").Scan(&value)
	if !strings.Contains(value, "confirmed_matching_owned_arrival") {
		t.Fatal("matching arrival not confirmed", value)
	}
}
func TestG2LModuleToggleRetainsAndBackfillsOnce(t *testing.T) {
	s := operationFixture(t, `{"event":"ShipyardNew","NewShipID":1,"ShipType":"asp"}`)
	if e := s.SetBuiltinModule("fleet", false); e != nil {
		t.Fatal(e)
	}
	page, e := s.Operations(context.Background(), "SYNTHETIC", OperationQuery{Kind: "fleet"})
	if e != nil || len(page.Rows) != 0 {
		t.Fatal(page, e)
	}
	for i := 0; i < 2; i++ {
		if e = s.SetBuiltinModule("fleet", true); e != nil {
			t.Fatal(e)
		}
	}
	var n int
	s.reader.QueryRow("SELECT count(*) FROM fleet_ship_events").Scan(&n)
	if n != 1 {
		t.Fatal("toggle duplicated or lost history", n)
	}
}

func TestG2LOlderActiveShipAndEmptyModuleSnapshot(t *testing.T) {
	s := operationFixture(t, `{"event":"Loadout","ShipID":2,"Ship":"asp","Modules":[]}`, `{"event":"Loadout","timestamp":"2025-01-01T00:00:00Z","ShipID":1,"Ship":"asp","Modules":[]}`, `{"event":"StoredModules","Items":[{"Name":"fixture","StorageSlot":1}]}`, `{"event":"StoredModules","Items":[]}`)
	var n int
	s.reader.QueryRow("SELECT count(*) FROM fleet_ships WHERE state='known_active'").Scan(&n)
	if n != 1 {
		t.Fatal("two active ships from older loadout", n)
	}
	page, e := s.Operations(context.Background(), "SYNTHETIC", OperationQuery{Kind: "modules"})
	if e != nil || len(page.Rows) != 0 {
		t.Fatal("empty snapshot resurrected modules", page, e)
	}
}
