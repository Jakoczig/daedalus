# G2B: Native Go Navigation Route and Exploration Survey Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two read-only, journal-verified, live Windows/Linux Go navigation and exploration screens to DAEDALUS, visible on the native desktop and securely paired secondary browser, without introducing provider traffic or gameplay control.

**Architecture:** Extend G2A's *single* journal + sidecar reader with focused navigation and current-system exploration projections in Go; emit separately allowlisted coalesced WebSocket view models. Use the existing React preview shell to enable only `/nav/route` and `/exploration/system`; keep all other legacy routes clearly unfinished in Go.

**Tech Stack:** Existing Go 1.26.10 / Wails 3, standard library `encoding/json` (`UseNumber`, exact IDs), existing `internal/journal`, `internal/sidecar`, `internal/gamestate`, `internal/webapp`, React client with `sendEvent`/`eventListener`, native Windows and Ubuntu CI.

**Spec:** `docs/superpowers/specs/2026-11-13-g2b-native-navigation-exploration-design.md` (and parent Go-first design) on the existing docs branch `docs/performance-first-native-modules-2026-11-12`.

## Global Constraints

- **G2A accepted**: user-reported Windows journal auto-discovery, last system, real FSDJump update and live *paired laptop without refresh* pass; [draft PR #6](https://github.com/Jakoczig/daedalus/pull/6) head `07c2084a170ffe99d1ebe5d1c0e218127fb7923e` passed Windows/Linux full CI, Linux race, native journal smoke and G1B security. G2A, G1B, G1A and security PRs all **remain draft/unmerged**.
- **Create branch `go/g2b-local-nav-exploration` from current `go/g2a-journal-ingestion`, then open a new draft PR targeting G2A**. If G2A head has changed, inspect/reconcile before branching. Never target `main` or change PRs #2–#6.
- Do not merge, release, deploy, replace original Daedalus, write Elite journal/sidecars, or start a later phase. Old Node `src/service` and original Windows launcher are untouched; no runtime Node, Python, EDMC, Inara/EDSM/EDDN/CAPI, Spansh or HTTP provider lookups.
- The Go runtime has exactly **one** source ingestion service per profile, not one per page/browser. Preserve private-profile lock, auth cookie, six-character pairing, CA quarantine, owner/listener split, bounded subscriptions and per-message revocation verification from G1B/G2A.
- Only allowlisted route/exploration DTOs go to verified owner and paired viewer. **Never expose raw event/sidecar, FID, commander name, local path, lat/lon, StarPos galaxy coordinates, chat, inventory/materials, credit balance, authentication fields or history**. Owner paths stay protected at `POST /owner/journal`.
- Parse 64-bit SystemAddress and BodyID without JS Number rounding; SystemAddress is internal as exact string. For route distance math, only `StarPos` may be validated as finite float, never interpreted as an identifier. Validate all scalar/array bounds and unknown/missing fields.
- Do not present stale NavRoute.json as active after cleared route, LoadGame/FID change or missing correlated `NavRoute` journal event. No unverified navigation/mapping, fabricated Sol fallback, bio species prediction or credit estimates.
- New routes remain **journal-only** and are not functional game galaxy-map controls. Clipboard copies to the *current viewer's* browser only after explicit click; never OS/game keypress from viewer.
- Keep all existing 62 statically exported client routes, G2A summary card and G1B remote management intact. New preview marker `G2B` only after both safe APIs and views are wired; other pages show unfinished message.
- CI must pass every current G2A, G1B and legacy Windows/Linux job, including Linux full race + native smoke, plus G2B tests, stress and new native fixture smoke. Experimental artifact names must clearly say `g2b-unsigned-local-route-survey-no-uploads`; verify ZIP hashes and no profile contents.
- All tests use **fabricated** journal and NavRoute.json files only; never commit or log real FID, real player journal, tokens, systems/coords from user's game.

## File responsibility map

**Create**
- `go/internal/navstate/model.go` and `model_test.go`: typed evidence, correlated sidecar validation, route progression and safe bounded `NavigationView`.
- `go/internal/explorestate/model.go` and `model_test.go`: body/scan/signal aggregation and safe bounded `ExplorationView` by exact system/commander.
- `go/internal/gamestate/g2b.go` and `g2b_test.go`: integrate two projections with existing reader, session changes, sidecar poll, source reset and immutable subscriptions.
- `go/internal/webapp/g2b_test.go`: narrow read-only RPC, privacy, revocation and owner/viewer behavior.
- `src/client/components/go-navigation.js`, `go-exploration-system.js`, and `tests/service/g2b-panels.test.js`: actual route/exploration views and disconnected/ready states.
- `tests/testdata/g2b/`: synthetic ordered journal and different-time sidecar fixtures, stale clearing, two commander sessions, valid FSS/Scan/DSS and malformed/overflow cases.
- `docs/testing/g2b-windows-linux-navigation-exploration-smoke.md` and `docs/handoffs/g2b-native-navigation-exploration.md`.

**Modify minimally**
- `go/internal/gamestate/service.go`: delegate from existing `apply` and sidecar ticker into navigation/exploration projections, respecting locks/lifecycle (no additional watcher).
- `go/internal/webapp/socket.go` and `gamefeed.go`: new narrow RPCs/events, existing serialized writes/per-message authentication, G2B marker and shutdown.
- `src/client/lib/socket.js`: G2B preview branch and two supported page slots; preserve G2A and original Node behavior.
- `go/cmd/daedalus/main.go` and `go/internal/webapp/journal_smoke.go`: new explicit-fixture G2B headless/native verification flags, isolated smoke profiles; keep G2A smoke behavior unchanged.
- `.github/workflows/go-desktop.yml`, `scripts/build/package-go-preview.cjs` and existing JS build tests: G2B branch/PR gates, native smoke, artifacts.

## Review Focus

1. **Old route from another commander or after clear:** must never appear as active to a second-screen viewer; tested in Tasks 1, 2, 4.
2. **Target differs from route during a jump:** next target and plotted route provenance stay distinct; progress only on `FSDJump`, not `StartJump`; tested Task 2.
3. **Scan data from prior system / missing SystemAddress:** must never show under new current system, even after restart; tested Task 3.
4. **Viewer revocation with backlog:** no post-revoke navigation/exploration frames; old queued frames cannot be misdiagnosed as new. Tested Task 4 with delayed-close fault injection and 25 repetitions.
5. **Large route / invalid nested journal values / slow laptop:** cap route, body count, payload sizes and subscribers; show `unverified`/`partial` rather than silent truncation; tested Tasks 1, 3, 4, 5.

---

### Task 1 — Route evidence parser, bounded NavRoute correlation

**Files:** Create `go/internal/navstate/model.go`, `model_test.go`, synthetic route fixture files. Read existing `go/internal/sidecar/snapshots.go`, `go/internal/journal/parser.go` and legacy `src/service/lib/event-handlers/nav-route.js` first.

**Interfaces:**
- `type Hop struct { Name, StarClass string; DistanceFromPreviousLY *float64; IsCurrent, IsNext bool }`, `type NavigationView struct { CurrentSystem, TargetSystem, DestinationSystem, RouteState, TargetEvidence string; RemainingJumps *int; Hops []Hop; Truncated bool }`. Add evidence flags if needed, but **no IDs/FID/raw coordinates in public JSON**.
- `type Model struct {...}` has `ApplyEvent(commanderKey string, event journal.Event)`, `ApplySidecar(change sidecar.Change)`, `View(commanderKey string) NavigationView`, `ResetSource()`. `commanderKey` is an internal normalized key supplied by the G2A service, never a public field.
- Validate `NavRoute.json` event/timestamp and bounded (max 2048) Route array with exact `SystemAddress` (json.Number → decimal string), 3 finite coordinates, safe 128-rune names, and known valid StarClass. Correlate sidecar with the current commander/session's latest `NavRoute` marker; stale/older/same-as-cleared file is unverified, not active. The acceptance rule MUST be documented and tested with marker-before-file and file-before-marker orders.

- [ ] **Step 1 — write failing tests** `TestRouteSidecarMarkerBeforeAndAfterWrite`, `TestRouteOldCommanderCannotReuseSidecar`, `TestRouteNavRouteClearStaysCleared`, `TestRouteRejectsOverflowNonfiniteOrWrongTimestamp`, `TestRouteExactLargeSystemAddress`. Assert old sidecar without matching active-session marker cannot yield actionable `Hops`/`DestinationSystem`. 
- [ ] **Step 2 — confirm RED**: `cd go && go test ./internal/navstate -count=1`. Expect compilation / assertions to fail.
- [ ] **Step 3 — implement** only bounded route extraction and conservative evidence correlation in `go/internal/navstate/model.go` using standard JSON decoder and existing `sidecar.Change`, no new read/watch. Use internal exact address and coordinate fields, but map explicitly to the public allowlist.
- [ ] **Step 4 — verify GREEN**: `cd go && go test -count=1 ./internal/navstate` (Windows/Linux); no full sidecar/document emitted in `NavigationView`.
- [ ] **Step 5 — commit**: `feat(go): verify plotted routes against journal evidence`.

### Task 2 — Route progress, next jump and honest state

**Files:** Extend `go/internal/navstate/model.go` and `model_test.go`.

**Interfaces:** Same `ApplyEvent`/`View`; parse `FSDTarget`, `StartJump`, `Location`, `FSDJump`, `CarrierJump`, `NavRouteClear`, `LoadGame`/`Fileheader` with internal per-commander epoch. Match current route hop by exact address, not name; calculate previous-to-next Euclidean LY only from finite `StarPos`; never fuel/scooping safety estimate. Keep journal-reported `RemainingJumpsInRoute` separate from inferred progress; do not invent remaining hops when current position unknown.

- [ ] **Step 1 — write failing tests** `TestRouteAdvancesOnlyOnConfirmedArrival`, `TestFSDTargetBeforeArrivalDoesNotMoveCurrent`, `TestRouteTargetConflictMarked`, `TestRouteUnmatchedCurrentHasUnknownRemaining`, `TestRouteDirectJumpDoesNotFakePlottedRoute`, `TestRouteClearRemovesActiveHopsAndCannotResurrect`.
- [ ] **Step 2 — confirm RED**: `cd go && go test ./internal/navstate -run 'TestRoute(Advances|Target|Unmatched|Direct|Clear)|TestFSDTarget' -count=1`.
- [ ] **Step 3 — implement** progression/target evidence in `Model`; retain only source-verified/allowlisted fields. A `StartJump` may affect an explicitly labeled target, never the current arrived system.
- [ ] **Step 4 — verify GREEN** and `go vet ./internal/navstate`.
- [ ] **Step 5 — commit**: `feat(go): advance source-verified route on arrived jumps`.

### Task 3 — Current-system journal-only FSS, body and DSS survey

**Files:** Create `go/internal/explorestate/model.go`, `model_test.go` and synthetic fixtures.

**Interfaces:**
- `type BodyView struct { Name, Kind string; BodyID string; Landable *bool; Terraformable *bool; GravityG *float64; DistanceLS *float64; BioSignals *int; GeoSignals *int; PreviouslyDiscovered *bool; PreviouslyMapped *bool; DSSMapped bool }`. Add bounded witnessed genus strings separately if present. `BodyID` crosses browser as **decimal string** (or omit if unnecessary); it is not a float.
- `type ExplorationView struct { SystemName, SurveyState string; ReportedBodyCount *int; FSSProgress *float64; AllBodiesFound bool; Bodies []BodyView; Truncated bool }`. No secrets, raw events, system ID, StarPos, lat/lon, person or path.
- `type Model struct {...}` implements `ApplyEvent(commanderKey string, event journal.Event)`, `View(commanderKey, systemAddress string) ExplorationView`, `ResetSource()`. Bound 8 verified systems across active data and 512 unique body entries per system; return explicit partial warning for overflow. Decode source event Raw via `json.Decoder.UseNumber` into typed, validated subset.
- Only apply `Scan`/`SAAScanComplete`/`SAASignalsFound`/`FSSBodySignals` when a matching nonempty exact `SystemAddress` is present and the active `commanderKey` is verified in current session; ignore conflicted/missing system identifiers rather than contaminate current system. Parse `FSSDiscoveryScan` and `FSSAllBodiesFound` conservatively, preserving `unknown` if SystemAddress or attribution cannot be safely established. `SAAScanComplete` proves the player's DSS map; `Scan.WasMapped` describes existing data and is **not** proof of a new mapping action. A signal count/genus is not a species determination.

- [ ] **Step 1 — write failing tests** `TestExplorationFSSUnknownVsReportedCounts`, `TestExplorationDeduplicatesBodiesByAddressAndBodyID`, `TestExplorationIgnoresOutOfSystemScan`, `TestExplorationKnownVsMissingLandability`, `TestExplorationDSSMappingVsPreviouslyMapped`, `TestExplorationSignalsDoNotInventSpecies`, `TestExplorationLargeIDsAndBoundedRows`, `TestExplorationDifferentFIDCannotShareBodies`.
- [ ] **Step 2 — confirm RED**: `cd go && go test ./internal/explorestate -count=1`.
- [ ] **Step 3 — implement** typed projections and bounded in-memory data, no EDSM or predictive library. Convert `SurfaceGravity` (m/s²) to Earth g with documented constant 9.80665 and validity checks; missing stays nil. No historical database.
- [ ] **Step 4 — verify GREEN** and `go vet ./internal/explorestate`.
- [ ] **Step 5 — commit**: `feat(go): aggregate confirmed current-system exploration scans`.

### Task 4 — Shared G2A service, secure API, session and revocation

**Files:** Create `go/internal/gamestate/g2b.go`, `g2b_test.go`, `go/internal/webapp/g2b_test.go`; minimally modify `go/internal/gamestate/service.go` and `go/internal/webapp/socket.go`/`gamefeed.go`.

**Interfaces:**
- G2A's `Service` retains one `journal.Reader` and one `sidecar.Reader` and owns navigation/exploration models. Add methods `Navigation() navstate.NavigationView`, `Exploration() explorestate.ExplorationView`, `SubscribeNavigation(ctx) (<-chan navstate.NavigationView, func())` and `SubscribeExploration(ctx) (<-chan explorestate.ExplorationView, func())` (or a single coalesced typed aggregate to avoid reader-lock fan-out).
- Define WebSocket names `getGoNavigation` / `goNavigationChanged` and `getGoExploration` / `goExplorationChanged`. The public messages are typed `NavigationView`/`ExplorationView` only. Retain `getJournalSummary` unchanged. Check `principal.valid()` before every write, including each stream message; preserve same Origin/session checks for local owner controls and remote viewer, exact listener split and role denial. No owner action from remote.
- If backend startup fails/rounds, G2A still starts and shows honest missing/partial source. Changing directory resets G2B caches and closes/restarts existing single reader. On logout or FID swap, old commander route/bodies must not appear for the new active FID. Backfill outputs one settled snapshot on completion rather than broadcasting thousands of intermediate replay changes.

- [ ] **Step 1 — write failing tests** `TestG2BOneJournalReaderForMultipleViewers`, `TestG2BProfileSwitchClearsOldRouteAndBodies`, `TestG2BViewSchemaExcludesPrivateFields`, `TestG2BPairedViewerReadOnly`, `TestG2BRevokedViewerReceivesNoNewNavigationOrSurvey`, `TestG2BSourceClearAndShutdownCancelSubscriptions`, `TestG2BReplayDoesNotFloodClients`. Use delayed-close fault injection to ensure the per-send authorization remains independently protective.
- [ ] **Step 2 — confirm RED**: `cd go && go test ./internal/gamestate ./internal/webapp -run 'TestG2B' -count=1`.
- [ ] **Step 3 — implement** one subscriber source and allowlisted transport; explicit G2B stage only when ready. No raw journal forwarding or new remote filesystem / clipboard RPC.
- [ ] **Step 4 — verify GREEN**: targeted tests and `cd go && go test -race -count=1 ./internal/gamestate ./internal/webapp` on Linux; legacy G1B/G2A tests must still pass.
- [ ] **Step 5 — commit**: `feat(go): publish safe read-only route and exploration models`.

### Task 5 — React actual route + exploration pages, laptop usability

**Files:** Create `src/client/components/go-navigation.js`, `src/client/components/go-exploration-system.js`, `tests/service/g2b-panels.test.js`; minimally modify `src/client/lib/socket.js` and relevant preview link navigation (not the existing Node route implementations).

**Interfaces:** When `prototype.stage === 'G2B'`, only `/nav/route` and `/exploration/system` show live Go projections; every other route stays clearly disabled and keeps existing navigation links. Pages call `getGoNavigation`/`getGoExploration` and listen for named change notifications, cancel stale async results on disconnect or route change, and handle empty/stale/cleared/partial/error states. On secondary device explicit Copy uses `navigator.clipboard.writeText` under click, no PC clipboard or game control. Retain G2A status card and local owner settings; viewers get no owner affordances.

- [ ] **Step 1 — write failing tests** `TestG2BRoutePageUsesOnlyGoProjection`, `TestG2BExplorationPageDoesNotInventSpeciesOrRevenue`, `TestG2BMissingOrClearedRouteStates`, `TestG2BDisconnectAndReconnectNoStaleResponse`, `TestG2BViewerCopyIsLocalOnly`, `TestG2BLegacySocketAndG2AStillWork`, `TestG2BOtherRoutesRemainUnimplemented`.
- [ ] **Step 2 — confirm RED**: `node --test tests/service/g2b-panels.test.js`.
- [ ] **Step 3 — implement** responsive route list + current-system body table, visible unknown/partial flags, scroll to current and next, screen-reader labels and local copy affordances; preserve G1B code and existing remote screen.
- [ ] **Step 4 — verify GREEN**: `node --test tests/service/g2b-panels.test.js`, `npm test`, `npm run lint:security`, `npm run test:go-assets` and `npm run build:go-assets`.
- [ ] **Step 5 — commit**: `feat(ui): display verified route and exploration survey on second screens`.

### Task 6 — Native test fixtures, Windows/Linux CI, artifacts and handoff

**Files:** Add `tests/testdata/g2b`, `docs/testing/g2b-windows-linux-navigation-exploration-smoke.md`, `docs/handoffs/g2b-native-navigation-exploration.md`; modify `go/cmd/daedalus/main.go` and `go/internal/webapp/journal_smoke.go` (or a focused new smoke file), `.github/workflows/go-desktop.yml`, `scripts/build/package-go-preview.cjs` and targeted JS build tests.

**Interfaces:** New explicit synthesized `--navigation-smoke-test` and `--desktop-navigation-smoke-test` with `--journal-dir` only; both isolate private profiles and read *only* provided fake route/scans. Native smoke requires authenticated owner React view to request and receive two nonempty allowlisted projections; viewer cannot spoof native ready. Existing `--smoke-test`, `--desktop-smoke-test`, `--journal-smoke-test` and `--desktop-journal-smoke-test` all remain correct. Do **not** accidentally enable journal source discovery in offline smoke modes.

- [ ] **Step 1 — write failing workflow/build smoke tests** for G2B PR base/branch, Windows/Linux native and headless route+survey smoke, privacy/no-provider traffic, both preview uploads and SHA256SUMS, Linux full race and bounded stress of route clear + FID swap + viewer revoke; add benchmark for projection replay (report, do not invent strict performance budgets).
- [ ] **Step 2 — confirm RED**: `node --test tests/build/go-workflow.test.cjs tests/build/package-go-preview.test.cjs` plus targeted Go smoke fixture test.
- [ ] **Step 3 — implement** workflow, portable synthetic fixture tool, route + FSS/Scan/SAAScanComplete native smoke and accurate experimental labels. Follow G2A's existing pinned actions, WebKit sandbox CI and no-network assumptions.
- [ ] **Step 4 — verify GREEN** with **current SHA** in Windows+Ubuntu desktop + Linux full race/native smoke, all legacy/security/JS/React asset tests and both uploaded artifact checksums. Record actual immutable CI runs and artifact digests; never claim physical gameplay from CI. Physical Windows acceptance runbook: plot route, check destination/target and per-hop progress, clear route, honk/FSS/DSS, check laptop live updates and reboot. Linux Steam/Proton field acceptance is separate. Avoid uploading any actual personal journals/screenshots with identifying data.
- [ ] **Step 5 — commit**: `ci(go): verify G2B navigation and survey across Windows and Linux`. Update handoff with commits, test results, source/identity guarantees, preview link/manifest hashes and manual limitations.

## Stop / acceptance matrix

| Criterion | Source/CI gate | Manual gate |
| --- | --- | --- |
| Accurate route, next-hop progress and clearing | Synthetic marker/sidecar, duplicate/FID/reset/large route tests | Plot, jump, clear actual in-game route |
| Honest current-system scan/body table | Synthetic FSS/Scan/DSS exact IDs, missing fields and no bio inference | Honk, FSS and DSS real system |
| Same data on PC and paired viewer | Read-only RPC schema/reconnect/revoke tests and native React smoke | Paired laptop updates without refresh |
| Security/lifecycle | Windows/Linux unit+security, Linux race, one reader and coalescing | Disable/revoke after live update |
| Portability/artifacts | Green Windows+Ubuntu CI, both experimental ZIP manifest/digest | Linux Steam/Proton field test deferred |

**Handoff:** [draft PR #7](https://github.com/Jakoczig/daedalus/pulls) (actual number may differ), targeting G2A. Include exact head SHA, CI run/job/artifact links, tests/known limitations, and a request for external G2B source review. **STOP** there. G2C, EDSM, exobiology predictions, neutron plotting, game control, merging and public release are not authorized.

## Plan self-review

- All spec outputs map to Tasks 1–6; the old richer system map/EDSM, predictive exobiology and Spansh functionality are intentionally excluded and clearly labeled, not silently dropped.
- Review Focus 1→Tasks 1/2/4, 2→Task 2, 3→Task 3, 4→Task 4, 5→Tasks 1/3/5. Every focus has a named falsifiable test.
- Type names and RPC names are consistent across backend/model/frontend. New public DTOs are explicitly separate from internal source events.
- New dependencies are unnecessary; package names and paths match current Go module and React layout. No fake “Go app parity” claims or provider API access.
- Every task has RED, implementation, GREEN, commit, with an inspectable security/test boundary; Tasks 1–3 can be reviewed independently before wiring authentication/UI.
