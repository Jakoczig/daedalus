package gamestate

// Identical test-only overlay for the two immutable baseline commits.
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkReviewFleetTradeCold(b *testing.B) {
	const records = 10000
	for iteration := 0; iteration < b.N; iteration++ {
		dir, profile := b.TempDir(), b.TempDir()
		f, e := os.Create(filepath.Join(dir, "Journal.20260101.01.log"))
		if e != nil {
			b.Fatal(e)
		}
		for i := 0; i < records; i++ {
			at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second).Format(time.RFC3339)
			var row map[string]any
			switch {
			case i == 0:
				row = map[string]any{"event": "Fileheader", "gameversion": "4.1", "part": 1}
			case i == 1 || i == 5000:
				row = map[string]any{"event": "LoadGame", "FID": fmt.Sprintf("SYNTHETIC-%d", i/5000), "Commander": "Synthetic performance fixture"}
			default:
				switch i % 20 {
				case 0:
					row = map[string]any{"event": "Loadout", "ShipID": uint64(9007199254740993), "Ship": "asp", "Modules": []any{map[string]any{"Slot": "MainEngines", "Item": "fixture", "On": true, "Priority": 0, "Engineering": map[string]any{"Modifiers": []any{map[string]any{"Label": "PowerDraw", "Value": 6}}}}}}
				case 1:
					row = map[string]any{"event": "StoredShips", "MarketID": uint64(9007199254740993), "StarSystem": "Fixture Alpha", "ShipsHere": []any{map[string]any{"ShipID": uint64(9007199254740993), "ShipType": "asp"}}, "ShipsRemote": []any{map[string]any{"ShipID": uint64(9007199254740994), "ShipType": "asp", "StarSystem": "Fixture Beta", "ShipMarketID": uint64(9007199254740994)}}}
				case 2:
					row = map[string]any{"event": "StoredModules", "Items": []any{map[string]any{"StorageSlot": 1, "Name": "fixture", "MarketID": uint64(9007199254740993)}}}
				case 3:
					row = map[string]any{"event": "ShipyardTransfer", "ShipID": uint64(9007199254740994), "TransferPrice": 100, "TransferTime": 60}
				case 4:
					row = map[string]any{"event": "CarrierStats", "CarrierID": uint64(18446744073709551615), "FuelLevel": 250, "Finance": map[string]any{"CarrierBalance": 1000000}, "SpaceUsage": map[string]any{"Cargo": 50, "FreeSpace": 20000}}
				case 5:
					row = map[string]any{"event": "CarrierJumpRequest", "CarrierID": uint64(18446744073709551615), "SystemName": "Fixture Beta", "SystemAddress": uint64(9007199254740994)}
				case 6:
					row = map[string]any{"event": "CarrierBankTransfer", "CarrierID": uint64(18446744073709551615), "Deposit": 100, "CarrierBalance": 1000100}
				case 7:
					row = map[string]any{"event": "CarrierTradeOrder", "CarrierID": uint64(18446744073709551615), "Commodity": "gold", "PurchaseOrder": 5, "Price": 10}
				case 8:
					row = map[string]any{"event": "MarketBuy", "MarketID": uint64(9007199254740993), "Type": "gold", "Count": 2, "BuyPrice": 10, "TotalCost": 20}
				case 9:
					row = map[string]any{"event": "MarketSell", "MarketID": uint64(9007199254740993), "Type": "gold", "Count": 2, "SellPrice": 20, "TotalSale": 40, "AvgPricePaid": 10}
				case 10:
					row = map[string]any{"event": "MiningRefined", "Type": "platinum"}
				case 11:
					row = map[string]any{"event": "Cargo", "Vessel": "Ship", "Count": 0, "Inventory": []any{}}
				case 12:
					row = map[string]any{"event": "CommunityGoal", "CurrentGoals": []any{map[string]any{"CGID": uint64(9007199254740993), "PlayerContribution": i}}}
				case 13:
					row = map[string]any{"event": "Powerplay", "Power": "Fixture", "Rank": 1, "Merits": i}
				case 14:
					row = map[string]any{"event": "CrewHire", "Name": "PRIVATE_SYNTHETIC_CREW", "CrewID": 1}
				case 15:
					row = map[string]any{"event": "WingJoin", "Others": []string{"PRIVATE_SYNTHETIC_WING"}}
				case 16:
					row = map[string]any{"event": "Market", "MarketID": uint64(9007199254740993), "StationName": "Fixture Port", "StarSystem": "Fixture Alpha"}
				case 17:
					row = map[string]any{"event": "ModuleStore", "ShipID": uint64(9007199254740993), "StoredItem": "fixture", "Slot": "Slot01"}
				case 18:
					row = map[string]any{"event": "ShipyardSwap", "ShipID": uint64(9007199254740993), "ShipType": "asp", "StoreShipID": uint64(9007199254740994)}
				default:
					row = map[string]any{"event": "Location", "StarSystem": "Fixture Alpha", "SystemAddress": uint64(9007199254740993)}
				}
			}
			row["timestamp"] = at
			raw, _ := json.Marshal(row)
			if _, e = f.Write(append(raw, '\n')); e != nil {
				b.Fatal(e)
			}
		}
		if e = f.Close(); e != nil {
			b.Fatal(e)
		}
		service, e := NewService(Options{StorageDir: profile, Directory: dir, EnableParity: true, PollInterval: time.Millisecond})
		if e != nil {
			b.Fatal(e)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		started := time.Now()
		go func() {
			defer close(done)
			if e := service.Run(ctx); e != nil {
				b.Error(e)
			}
		}()
		deadline := time.Now().Add(3 * time.Minute)
		for {
			v := service.Snapshot()
			if v.Stage == "DB_ERROR" || time.Now().After(deadline) {
				cancel()
				<-done
				b.Fatal(v)
			}
			if !v.ImportInProgress && v.EventsRead == records {
				break
			}
			time.Sleep(time.Millisecond)
		}
		elapsed := time.Since(started)
		cancel()
		<-done
		b.ReportMetric(float64(elapsed.Microseconds())/1000, "cold-ms")
		b.ReportMetric(float64(service.Details().ReaderBytes), "journal-bytes")
		b.ReportMetric(records/elapsed.Seconds(), "events/s")
	}
}
