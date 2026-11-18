package providers

// Local diagnostics are explicitly invoked by the owner OS account. They create
// no server/journal reader, never refresh or write credentials, and only emit
// value-scrubbed structural fixtures. The desktop RPC uses the same scrubber.
import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jakoczig/eridion/go/internal/privatefs"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var auditKinds = map[string]bool{"profile": true, "market": true, "shipyard": true, "fleetcarrier": true, "communitygoals": true}
var auditSlot = regexp.MustCompile(`^(Armour|PowerPlant|MainEngines|FrameShiftDrive|FrameShiftDriveBooster|FuelTank|LifeSupport|PowerDistributor|Radar|Sensors|PlanetaryApproachSuite|CargoHatch|[A-Za-z]*Hardpoint[0-9]+|Slot[0-9]+_Size[0-9]+)$`)
var auditStaticFields = func() map[string]bool {
	out := map[string]bool{}
	for _, k := range strings.Fields(`commander lastStarport lastSystem loadout loadouts ship ships suit suits alive capabilities AllowCobraMkIV Horizons Odyssey credits currentShipId debt docked id mercCoins name onfoot rank builder combat cqc crime empire exobiologist explore federation learner power service soldier trade free starsystem systemaddress station value cargo hull modules total unloaned shipName shipID health integrity paintwork shield shieldup cockpitBreached oxygenRemaining launchBays module engineer WorkInProgress_modifications specialModifications engineerId engineerName recipeLevel recipeLocDescription recipeLocName recipeName locDescription locName on priority LessIsGood dir displayValue OutfittingFieldType_FSDOptimalMass OutfittingFieldType_Integrity OutfittingFieldType_Mass OutfittingFieldType_PowerDraw OutfittingFieldType_ShieldGenStrength OutfittingFieldType_BootTime activeCommunityGoals activityType bulletin expiry market_name news objective qty starsystem_name target_qty title commodities economies exported imported outpostType prohibited services buyPrice categoryname demand demandBracket legality meanPrice sellPrice statusFlags stock stockBracket categoryName shipyard_list discount_list unavailable_list basevalue sku cost category proportion status type shipyard outfitting refuel repair rearm contacts exploration commodities dock stationmenu shop engineer crewlounge techbroker searchrescue socialspace carrierfuel blackmarket voucherredemption`) {
		out[k] = true
	}
	return out
}()

// StructuralFixture removes all original scalar values, including personal ship
// names, balances, locations and credentials. Numeric object keys and ID values
// use one synthetic mapping so keyed fleet relationships remain testable.
func StructuralFixture(raw []byte) (json.RawMessage, error) {
	if len(raw) > 2<<20 {
		return nil, errors.New("Diagnostic response exceeds budget")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	if d.Decode(&value) != nil {
		return nil, errors.New("Diagnostic response is not JSON")
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return nil, errors.New("Diagnostic response has trailing data")
	}
	ids := map[string]string{}
	id := func(v string) string {
		if ids[v] == "" {
			ids[v] = strconv.FormatInt(9007199254740993+int64(len(ids)), 10)
		}
		return ids[v]
	}
	privateStrings := map[string]bool{}
	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case string:
			if len(x) > 0 {
				privateStrings[x] = true
			}
		case []any:
			for _, a := range x {
				collect(a)
			}
		case map[string]any:
			for _, a := range x {
				collect(a)
			}
		}
	}
	collect(value)
	var scrub func(any, string, int) (any, error)
	scrub = func(v any, key string, depth int) (any, error) {
		if depth > 32 {
			return nil, errors.New("Diagnostic structure exceeds depth budget")
		}
		switch x := v.(type) {
		case nil:
			return nil, nil
		case bool:
			return false, nil
		case json.Number:
			k := strings.ToLower(key)
			if k == "id" || strings.HasSuffix(k, "id") || strings.HasSuffix(k, "address") {
				return json.Number(id(string(x))), nil
			}
			if strings.ContainsAny(string(x), ".eE") {
				return json.Number("1.25"), nil
			}
			return json.Number("100"), nil
		case string:
			if _, e := time.Parse(time.RFC3339Nano, x); e == nil {
				return "2026-01-01T00:00:00Z", nil
			}
			if canonicalID(x) && (strings.EqualFold(key, "id") || strings.HasSuffix(strings.ToLower(key), "address") || strings.HasSuffix(strings.ToLower(key), "shipid")) {
				return id(x), nil
			}
			if key == "name" && depth == 2 {
				return "Fixture Commander", nil
			}
			return "Fixture value", nil
		case []any:
			if len(x) > 4096 {
				return nil, errors.New("Diagnostic collection exceeds budget")
			}
			out := make([]any, len(x))
			for i, a := range x {
				var e error
				out[i], e = scrub(a, key, depth+1)
				if e != nil {
					return nil, e
				}
			}
			return out, nil
		case map[string]any:
			if len(x) > 4096 {
				return nil, errors.New("Diagnostic object exceeds budget")
			}
			out := map[string]any{}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for i, k := range keys {
				safe := k
				if canonicalID(k) {
					safe = id(k)
				} else if (!auditStaticFields[k] && !auditSlot.MatchString(k)) || privateStrings[k] {
					safe = fmt.Sprintf("redactedField%d", i)
				}
				a, e := scrub(x[k], k, depth+1)
				if e != nil {
					return nil, e
				}
				out[safe] = a
			}
			return out, nil
		}
		return nil, errors.New("Unsupported diagnostic JSON type")
	}
	safe, e := scrub(value, "", 0)
	if e != nil {
		return nil, e
	}
	b, e := json.MarshalIndent(safe, "", "  ")
	if e != nil || len(b) > 2<<20 {
		return nil, errors.New("Sanitized diagnostic exceeds output budget")
	}
	return b, nil
}

// AuditNativeFrontier performs one bounded, consent-checked GET using the
// existing verified OS-vault namespace. Original profile files are read-only.
// Its separate diagnostics directory serializes this optional command and keeps
// a conservative cooldown; no raw account response is written anywhere.
func AuditNativeFrontier(ctx context.Context, profile, output, kind string) error {
	if !auditKinds[kind] {
		return errors.New("Unsupported diagnostic endpoint")
	}
	original, e := filepath.Abs(profile)
	if e != nil {
		return errors.New("Invalid profile")
	}
	dest, e := filepath.Abs(output)
	if e != nil {
		return errors.New("Invalid output")
	}
	rel, e := filepath.Rel(original, dest)
	if e != nil || rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
		return errors.New("Diagnostics must be outside the original profile")
	}
	if e = privatefs.Prepare(dest); e != nil {
		return errors.New("Cannot prepare private diagnostic output")
	}
	unlock, e := privatefs.Lock(dest)
	if e != nil {
		return errors.New("Another diagnostic is active")
	}
	defer unlock()
	p := filepath.ToSlash(filepath.Join(original, "daedalus.sqlite"))
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p}
	db, e := sql.Open("sqlite3", u.String()+"?mode=ro&_query_only=on&_busy_timeout=1000")
	if e != nil {
		return errors.New("Existing history unavailable")
	}
	defer db.Close()
	var ledgerRaw, configRaw []byte
	if db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key='provider-ledger'`).Scan(&ledgerRaw) != nil || db.QueryRowContext(ctx, `SELECT config_json FROM provider_consent WHERE provider='frontier'`).Scan(&configRaw) != nil {
		return errors.New("Existing provider consent unavailable")
	}
	var ledger providerDisk
	var config Config
	if json.Unmarshal(ledgerRaw, &ledger) != nil || json.Unmarshal(configRaw, &config) != nil || !config.Read || config.Paused || len(ledger.ActiveAccount) != 64 {
		return errors.New("Explicit selected Frontier read consent required")
	}
	values, e := newVault(original).Load()
	if e != nil {
		return errors.New("Owner OS vault unavailable")
	}
	credential := values[ledger.ActiveAccount+"/frontier"]
	if credential.JournalFID == "" || hash(credential.JournalFID) != ledger.ActiveAccount || credential.Commander == "" || credential.AccessToken == "" {
		return errors.New("Verified selected account credential required")
	}
	var expected string
	if db.QueryRowContext(ctx, `SELECT latest_name FROM commander WHERE fid=?`, credential.JournalFID).Scan(&expected) != nil || expected != credential.Commander {
		return errors.New("Verified journal identity required")
	}
	var settings struct {
		Multiple bool
		Selected string
	}
	b, readError := privatefs.Read(original, "accounts-v1.json")
	if readError != nil && !os.IsNotExist(readError) {
		return errors.New("Cannot verify current account selection")
	}
	if readError == nil && json.Unmarshal(b, &settings) != nil {
		return errors.New("Invalid account selection; original file preserved")
	}
	var others int
	if db.QueryRowContext(ctx, "SELECT count(*) FROM commander WHERE fid<>?", credential.JournalFID).Scan(&others) != nil {
		return errors.New("Cannot verify complete commander context")
	}
	if !auditNamespaceAllowed(credential.JournalFID, ledger.ActiveAccount, settings.Multiple, settings.Selected, others) {
		return errors.New("Explicit matching account selection required for a multi-account diagnostic")
	}

	now := time.Now()
	if !now.Before(credential.ExpiresAt.Add(-30 * time.Second)) {
		return errors.New("Access token expired: use the existing desktop refresh; diagnostic never changes credentials")
	}
	var last string
	rateError := db.QueryRowContext(ctx, `SELECT requested_at FROM provider_requests WHERE provider='frontier' ORDER BY id DESC LIMIT 1`).Scan(&last)
	if rateError != nil && rateError != sql.ErrNoRows {
		return errors.New("Cannot verify provider cooldown")
	}
	latest, parseError := time.Parse(time.RFC3339Nano, last)
	if last != "" && parseError != nil {
		return errors.New("Invalid saved provider cooldown")
	}

	if b, e := privatefs.Read(dest, "last-read.json"); e == nil {
		var at time.Time
		if json.Unmarshal(b, &at) == nil && at.After(latest) {
			latest = at
		}
	}
	if now.Before(latest.Add(time.Minute)) {
		return errors.New("Frontier cooldown active; wait at least one minute since the last desktop/diagnostic request")
	}
	stamp, _ := json.Marshal(now)
	if privatefs.Write(dest, "last-read.json", stamp) != nil {
		return errors.New("Cannot reserve diagnostic cooldown")
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, "GET", "https://companion.orerve.net/"+kind, nil)
	if e != nil {
		return errors.New("Invalid diagnostic request")
	}
	req.Header.Set("Authorization", "Bearer "+credential.AccessToken)
	req.Header.Set("User-Agent", software+"/"+version)
	response, e := client.Do(req)
	if e != nil {
		return errors.New("Diagnostic request failed; no private response logged")
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if e != nil || len(raw) > 2<<20 {
		return errors.New("Diagnostic response exceeds budget")
	}
	if kind == "profile" && response.StatusCode == 200 {
		data := object(raw)
		c, _ := data["commander"].(map[string]any)
		if text(c["name"]) != expected {
			return errors.New("Frontier account association failed; response discarded")
		}
	}
	if response.StatusCode != 200 && !(kind == "fleetcarrier" && response.StatusCode == 204) {
		return fmt.Errorf("Diagnostic HTTP %d; no private response retained", response.StatusCode)
	}
	var fixture json.RawMessage
	if response.StatusCode == 200 {
		fixture, e = StructuralFixture(raw)
		if e != nil {
			return e
		}
	} else {
		fixture = json.RawMessage(`null`)
	}
	report := struct {
		Endpoint string          `json:"endpoint"`
		Status   int             `json:"httpStatus"`
		Fixture  json.RawMessage `json:"sanitizedFixture"`
		Scope    string          `json:"scope"`
	}{"/" + kind, response.StatusCode, fixture, "Owner-initiated read; all scalar account values anonymized; no credential refresh/write, upload or journal reader"}
	reportBytes, _ := json.MarshalIndent(report, "", "  ")
	if privatefs.Write(dest, kind+"-structural.json", reportBytes) != nil {
		return errors.New("Cannot save sanitized structural fixture")
	}
	return nil
}

func auditNamespaceAllowed(fid, active string, multiple bool, selected string, otherCommanders int) bool {
	return fid != "" && hash(fid) == active && ((multiple && selected == fid) || (!multiple && otherCommanders == 0))
}
