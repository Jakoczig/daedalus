package history

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"
)

type OperationQuery struct {
	PreviewHash string `json:"previewHash"`
	Kind        string `json:"kind"`
	FID         string `json:"fid"`
	ID          string `json:"id"`
	Before      string `json:"before"`
	Commodity   string `json:"commodity"`
	Market      string `json:"market"`
	OtherMarket string `json:"otherMarket"`
	Limit       int    `json:"limit"`
	Acknowledge bool   `json:"acknowledge"`
}
type OperationRow struct {
	ID      string          `json:"id"`
	Entity  string          `json:"entity"`
	Kind    string          `json:"kind"`
	At      string          `json:"at"`
	State   string          `json:"state"`
	Title   string          `json:"title"`
	Details json.RawMessage `json:"details"`
}
type OperationPage struct {
	PreviewHash string            `json:"previewHash"`
	Totals      map[string]string `json:"totals,omitempty"`
	Current     []OperationRow    `json:"current,omitempty"`
	Rows        []OperationRow    `json:"rows"`
	Next        string            `json:"next"`
	Notice      string            `json:"notice"`
	Reference   []EffectiveFact   `json:"reference,omitempty"`
}

func (s *Store) Operations(ctx context.Context, fid string, q OperationQuery) (OperationPage, error) {
	out := OperationPage{Rows: []OperationRow{}, Notice: "Local journal observations / last known, not a live account or market feed. Missing historical payloads are unavailable; explicit verified original-journal backfill may recover them. Rendering never fetches providers or uploads data."}
	if len(fid) > 128 || len(q.ID) > 128 || len(q.Commodity) > 128 || len(q.Market) > 20 || len(q.OtherMarket) > 20 {
		return out, errors.New("Invalid operation filters")
	}
	if q.Before != "" && identifier(q.Before) == "" {
		return out, errors.New("Invalid page cursor")
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 50
	}
	before := q.Before
	if before == "" {
		before = "9223372036854775807"
	}
	// All identifiers and JSON numbers are decimal strings. SQL identifiers are
	// fixed here, never interpolated from browser-supplied table/column names.
	query := ""
	args := []any{}
	switch q.Kind {
	case "fleet":
		query = `SELECT cast(rowid AS TEXT),ship_id,'Fleet',observed_at,state,model||' '||name,json_set(metadata_json,'$.system',system_name,'$.marketId',market_id) FROM fleet_ships WHERE commander_fid=? AND rowid<cast(? AS INTEGER) AND (?='' OR model LIKE '%'||?||'%' OR name LIKE '%'||?||'%') ORDER BY rowid DESC LIMIT ?`
		args = []any{fid, before, q.Commodity, q.Commodity, q.Commodity, q.Limit + 1}
	case "loadouts":
		query = `SELECT cast(id AS TEXT),ship_id,'Loadout',coalesce(observed_at,''),'historical_witnessed','Exact witnessed fitted modules',raw_json FROM ship_loadout_snapshots WHERE commander_fid=? AND ship_id=? AND id<cast(? AS INTEGER) ORDER BY id DESC LIMIT ?`
		args = []any{fid, q.ID, before, q.Limit + 1}
	case "modules":
		query = `SELECT cast(rowid AS TEXT),storage_slot,'StoredModules',observed_at,'last_known_snapshot',module_symbol,metadata_json FROM stored_module_observations WHERE commander_fid=? AND event_id=(SELECT event_id FROM stored_module_snapshots WHERE commander_fid=? ORDER BY observed_at DESC,event_id DESC LIMIT 1) AND rowid<cast(? AS INTEGER) ORDER BY rowid DESC LIMIT ?`
		args = []any{fid, fid, before, q.Limit + 1}
	case "fleet-history":
		query = `SELECT cast(rowid AS TEXT),ship_id,event_type,observed_at,'witnessed',event_type,metadata_json FROM fleet_ship_events WHERE commander_fid=? AND (?='' OR ship_id=?) AND rowid<cast(? AS INTEGER) ORDER BY rowid DESC LIMIT ?`
		args = []any{fid, q.ID, q.ID, before, q.Limit + 1}
	case "carrier":
		query = `SELECT cast(f.rowid AS TEXT),f.carrier_id,'Owned carrier',f.observed_at,'last_known',f.field,f.value_json FROM carrier_facts f JOIN carrier_owned o USING(commander_fid,carrier_id) WHERE f.commander_fid=? AND (?='' OR f.carrier_id=?) AND f.rowid<cast(? AS INTEGER) ORDER BY f.rowid DESC LIMIT ?`
		args = []any{fid, q.ID, q.ID, before, q.Limit + 1}
	case "carrier-history":
		query = `SELECT cast(c.event_id AS TEXT),c.carrier_id,c.event_type,c.observed_at,'witnessed_not_income',c.event_type,c.content_json FROM carrier_observations c JOIN carrier_owned o USING(commander_fid,carrier_id) WHERE c.commander_fid=? AND c.event_id<cast(? AS INTEGER) ORDER BY c.event_id DESC LIMIT ?`
		args = []any{fid, before, q.Limit + 1}
	case "trades":
		query = `SELECT cast(event_id AS TEXT),market_id,action,observed_at,CASE WHEN gross_spread IS NULL THEN 'gross_receipts_or_expenditure_cost_basis_unknown' ELSE 'game_reported_basis_gross_spread_excludes_overheads' END,commodity,json_object('units',units,'unitPrice',unit_price,'totalCredits',total_credits,'avgPricePaid',avg_price_paid,'grossSpread',gross_spread,'marketId',market_id,'systemAddress',system_address,'flags',json(flags),'source',source) FROM trade_ledger WHERE commander_fid=? AND event_id<cast(? AS INTEGER) AND (?='' OR commodity=?) AND (?='' OR market_id=?) ORDER BY event_id DESC LIMIT ?`
		args = []any{fid, before, q.Commodity, q.Commodity, q.Market, q.Market, q.Limit + 1}
	case "mining-sales":
		query = `SELECT cast(event_id AS TEXT),commodity,'Attributed mining sale',observed_at,'gross_receipts_cost_basis_unknown',commodity,json_object('units',units,'receipts',receipts,'session',session_id,'evidence',evidence) FROM trade_mining_summary WHERE commander_fid=? AND event_id<cast(? AS INTEGER) AND (?='' OR commodity=?) ORDER BY event_id DESC LIMIT ?`
		args = []any{fid, before, q.Commodity, q.Commodity, q.Limit + 1}
	case "mining":
		query = `SELECT cast(event_id AS TEXT),system_address,'MiningRefined',observed_at,'one_witnessed_refined_tonne_not_a_sale',commodity,json_object('units','1','session',session_id) FROM mining_refined WHERE commander_fid=? AND event_id<cast(? AS INTEGER) AND (?='' OR commodity=?) ORDER BY event_id DESC LIMIT ?`
		args = []any{fid, before, q.Commodity, q.Commodity, q.Limit + 1}
	case "module-events", "goals", "powerplay", "squadron", "crew", "wing":
		query = `SELECT cast(rowid AS TEXT),entity_id,event_type,observed_at,'last_known_not_live',event_type,content_json FROM commander_operations WHERE commander_fid=? AND category=? AND rowid<cast(? AS INTEGER) AND (?='' OR entity_id=?) ORDER BY rowid DESC LIMIT ?`
		category := q.Kind
		if category == "module-events" {
			category = "modules"
		}
		args = []any{fid, category, before, q.ID, q.ID, q.Limit + 1}
	case "markets":
		query = `SELECT cast(id AS TEXT),market_id,kind,observed_at,'historical_snapshot',station_name,json_object('system',system_name,'source',source,'fetchedAt',fetched_at,'expiresAt',expires_at) FROM market_observations WHERE commander_fid=? AND id<cast(? AS INTEGER) ORDER BY id DESC LIMIT ?`
		args = []any{fid, before, q.Limit + 1}
	case "commodities":
		query = `SELECT cast(c.rowid AS TEXT),m.market_id,'Commodity',m.observed_at,'observed_quote_not_guaranteed',c.commodity,json_object('buyPrice',c.buy_price,'sellPrice',c.sell_price,'meanPrice',c.mean_price,'stock',c.stock,'demand',c.demand,'station',m.station_name,'system',m.system_name,'source',m.source,'fetchedAt',m.fetched_at,'expiresAt',m.expires_at) FROM commodity_quotes c JOIN market_observations m ON m.id=c.observation_id WHERE m.commander_fid=? AND c.rowid<cast(? AS INTEGER)`
		args = []any{fid, before}
		if q.Market != "" {
			query += " AND m.market_id=?"
			args = append(args, q.Market)
		}
		if q.Commodity != "" {
			query += " AND c.commodity=?"
			args = append(args, q.Commodity)
		}
		query += " ORDER BY c.rowid DESC LIMIT ?"
		args = append(args, q.Limit+1)

	case "comparison":
		return s.marketComparison(ctx, fid, q)
	default:
		return out, errors.New("Unsupported operations view")
	}
	module := map[string]string{"fleet": "fleet", "loadouts": "fleet", "modules": "fleet", "fleet-history": "fleet", "module-events": "fleet", "carrier": "carrier", "carrier-history": "carrier", "trades": "economy", "mining": "economy", "mining-sales": "economy", "markets": "economy", "commodities": "economy", "goals": "commander-operations", "powerplay": "commander-operations", "squadron": "commander-operations", "crew": "commander-operations", "wing": "commander-operations"}[q.Kind]
	if !s.BuiltinEnabled(module) {
		out.Notice = "Built-in module disabled; history retained"
		return out, nil
	}
	rows, e := s.reader.QueryContext(ctx, query, args...)
	if e != nil {
		return out, e
	}
	bytes := 0
	for rows.Next() {
		var r OperationRow
		var raw string
		if e = rows.Scan(&r.ID, &r.Entity, &r.Kind, &r.At, &r.State, &r.Title, &raw); e != nil {
			rows.Close()
			return out, e
		}
		if len(out.Rows) == q.Limit {
			if len(out.Rows) > 0 {
				out.Next = out.Rows[len(out.Rows)-1].ID
			}
			break
		}
		// Exact final engineering remains intact in SQLite. A single exceptionally
		// large historical build is explicitly withheld rather than silently trimmed.
		if len(raw) > 64<<10 {
			r.Details = json.RawMessage(`{"notice":"Large record withheld; inspect private Data/History export"}`)
		} else {
			r.Details = exactOperationJSON(raw)
		}
		if bytes+len(r.Details) > 256<<10 {
			if len(out.Rows) > 0 {
				out.Next = out.Rows[len(out.Rows)-1].ID
			}
			break
		}
		bytes += len(r.Details)
		out.Rows = append(out.Rows, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if q.Kind == "trades" {
		sums := map[string]*big.Int{"purchaseExpenditure": new(big.Int), "saleReceipts": new(big.Int), "knownBasisGrossSpread": new(big.Int)}
		for _, r := range out.Rows {
			data := object(r.Details)
			key := "purchaseExpenditure"
			if r.Kind == "MarketSell" {
				key = "saleReceipts"
			}
			if n, ok := new(big.Int).SetString(text(data["totalCredits"]), 10); ok {
				sums[key].Add(sums[key], n)
			}
			if n, ok := new(big.Int).SetString(text(data["grossSpread"]), 10); ok {
				sums["knownBasisGrossSpread"].Add(sums["knownBasisGrossSpread"], n)
			}
		}
		out.Totals = map[string]string{"scope": "selected bounded page only; not lifetime profit"}
		for k, v := range sums {
			out.Totals[k] = v.String()
		}
	}
	if q.Kind == "fleet" || q.Kind == "goals" || q.Kind == "squadron" {
		out.Reference, _ = s.EffectiveFacts(ctx, fid, "commander", time.Now())
	}
	if q.Kind == "goals" || q.Kind == "powerplay" || q.Kind == "crew" || q.Kind == "squadron" {
		current, err := s.reader.QueryContext(ctx, "SELECT cast(rowid AS TEXT),entity_id,field,observed_at,content_json FROM commander_current WHERE commander_fid=? AND category=? ORDER BY observed_at DESC,event_id DESC LIMIT 100", fid, q.Kind)
		if err != nil {
			return out, err
		}
		total := 0
		for current.Next() {
			var r OperationRow
			var raw string
			if err = current.Scan(&r.ID, &r.Entity, &r.Title, &r.At, &raw); err != nil {
				current.Close()
				return out, err
			}
			total += len(raw)
			if total > 128<<10 {
				break
			}
			r.Kind = q.Kind
			r.State = "last_witnessed_not_live"
			r.Details = exactOperationJSON(raw)
			out.Current = append(out.Current, r)
		}
		err = current.Err()
		current.Close()
		if err != nil {
			return out, err
		}
	}
	if q.Kind == "carrier" && len(out.Rows) == 0 {
		out.Notice = "No owned carrier observed. Docking or riding another carrier does not establish ownership."
	}
	return out, nil
}
func (s *Store) marketComparison(ctx context.Context, fid string, q OperationQuery) (OperationPage, error) {
	out := OperationPage{Rows: []OperationRow{}, Notice: "No source-backed price comparison. Select two different observed markets and a commodity. Quoted gross spread is not a guaranteed opportunity or net profit."}
	if !s.BuiltinEnabled("economy") || identifier(q.Market) == "" || identifier(q.OtherMarket) == "" || q.Market == q.OtherMarket || q.Commodity == "" {
		return out, nil
	}
	type quote struct{ market, at, buy, sell, source, station string }
	values := []quote{}
	for _, market := range []string{q.Market, q.OtherMarket} {
		var r quote
		r.market = market
		var buy, sell sql.NullString
		e := s.reader.QueryRowContext(ctx, `SELECT m.observed_at,c.buy_price,c.sell_price,m.source,m.station_name FROM market_observations m JOIN commodity_quotes c ON c.observation_id=m.id WHERE m.commander_fid=? AND m.market_id=? AND c.commodity=? AND m.kind='Market' AND m.id=(SELECT id FROM market_observations WHERE commander_fid=m.commander_fid AND market_id=m.market_id AND kind='Market' ORDER BY observed_at DESC,id DESC LIMIT 1) ORDER BY m.observed_at DESC,m.id DESC LIMIT 1`, fid, market, q.Commodity).Scan(&r.at, &buy, &sell, &r.source, &r.station)
		if e == sql.ErrNoRows {
			return out, nil
		}
		if e != nil {
			return out, e
		}
		r.buy, r.sell = buy.String, sell.String
		values = append(values, r)
	}
	if values[0].buy == "" || values[0].buy == "0" || values[1].sell == "" {
		return out, nil
	}
	// Exact integer quote subtraction, without a FIFO or cargo acquisition claim.
	spread := grossSpread(json.Number(values[1].sell), json.Number("1"), json.Number(values[0].buy))
	out.Rows = append(out.Rows, OperationRow{ID: "comparison", Entity: q.Commodity, Kind: "Two-market quote", At: values[1].at, State: "gross_quoted_spread_per_tonne", Title: q.Commodity, Details: json.RawMessage(encoded(map[string]any{"buyMarket": q.Market, "buyStation": values[0].station, "buyObservedAt": values[0].at, "buyPrice": values[0].buy, "buySource": values[0].source, "sellMarket": q.OtherMarket, "sellStation": values[1].station, "sellObservedAt": values[1].at, "sellPrice": values[1].sell, "sellSource": values[1].source, "grossQuotedSpread": spread, "nominalCargoReturn": "Unavailable: cargo acquisition and sell opportunity not established"}))})
	return out, nil
}

// Export only the explicitly previewed bounded page; browser cannot choose paths.
func operationCursor(s string) bool {
	_, e := strconv.ParseUint(s, 10, 63)
	return e == nil && !strings.HasPrefix(s, "+")
}

func exactOperationJSON(raw string) json.RawMessage {
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return json.RawMessage(`null`)
	}
	return json.RawMessage(encoded(v))
}
