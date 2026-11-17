package history

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkG2LOperationsQueries(b *testing.B) {
	s, e := Open(b.TempDir())
	if e != nil {
		b.Fatal(e)
	}
	defer s.Close()
	tx, e := s.writer.Begin()
	if e != nil {
		b.Fatal(e)
	}
	for i := 0; i < 1000; i++ {
		if _, e = tx.Exec("INSERT INTO fleet_ships VALUES('SYNTHETIC',?,'asp','Fixture','last_known','','','2026-01-01T00:00:00Z',1,'{}')", fmt.Sprint(i)); e != nil {
			b.Fatal(e)
		}
	}
	for i := 1; i <= 10000; i++ {
		if _, e = tx.Exec("INSERT INTO trade_ledger VALUES(?,'SYNTHETIC','session','','1','gold','MarketSell','1','10','10','1','9','{}','2026-01-01T00:00:00Z','journal')", i); e != nil {
			b.Fatal(e)
		}
	}
	for i := 1; i <= 100; i++ {
		if _, e = tx.Exec("INSERT INTO market_observations VALUES(?,'SYNTHETIC',?,'Fixture','Port','Market',NULL,'2026-01-01T00:00:00Z','','','fixture',?)", i, fmt.Sprint(i), fmt.Sprint(i)); e != nil {
			b.Fatal(e)
		}
		for j := 0; j < 500; j++ {
			if _, e = tx.Exec("INSERT INTO commodity_quotes VALUES(?,?,'10','20','15','100','50')", i, fmt.Sprintf("commodity%03d", j)); e != nil {
				b.Fatal(e)
			}
		}
	}
	for i := 1; i <= 1000; i++ {
		if _, e = tx.Exec("INSERT INTO commander_operations VALUES(?,0,'SYNTHETIC','goals',?,'CommunityGoal','2026-01-01T00:00:00Z','{}')", i, fmt.Sprint(i)); e != nil {
			b.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		b.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		q    OperationQuery
	}{{"Fleet1000", OperationQuery{Kind: "fleet", Limit: 30}}, {"Trades10000", OperationQuery{Kind: "trades", Limit: 30}}, {"Quotes50000", OperationQuery{Kind: "commodities", Commodity: "commodity001", Limit: 30}}, {"Goals1000", OperationQuery{Kind: "goals", Limit: 30}}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				p, e := s.Operations(context.Background(), "SYNTHETIC", tc.q)
				if e != nil || len(p.Rows) != 30 {
					b.Fatal(len(p.Rows), e)
				}
			}
		})
	}
}
