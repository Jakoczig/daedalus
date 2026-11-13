# DAEDALUS G2B — Local Navigation and Current-System Exploration

**Date:** 2026-11-13  
**Status:** Scoped design for the next *draft, non-release* Go milestone after G2A game-to-laptop acceptance  
**Implementation plan:** [G2B TDD plan](../plans/2026-11-13-g2b-native-navigation-exploration.md)  
**Dependency:** [G2A draft PR #6](https://github.com/Jakoczig/daedalus/pull/6), head `07c2084a170ffe99d1ebe5d1c0e218127fb7923e` at scoping  
**Parent:** [Go-first Windows/Linux design](./2026-11-12-go-first-windows-linux-rebuild-design.md)

## Intent and confirmed context

The maintainer validated G2A with the *real Windows game*: correct journal discovery, prior location restoration, real jump detection, correct destination, and the new system appearing automatically on an already-paired laptop without refresh. G2A's Windows/Linux CI, Linux race, synthetic native smoke and G1B security regressions also passed. Next, turn the shared journal pipeline into two actually useful read-only screens on both PC and laptop: **in-game plotted navigation route** and **journal-verified current-system exploration survey**.

Navigation and exploration are valuable for this maintainer, particularly observing a route and scans off the main Elite screen, but game-map control, server-enriched maps, neutron route optimization and speculative biological estimates are not part of G2B. Keep the old Daedalus and all unfinished Go pages available but honestly marked unfinished. **No merge, public release, deployment, or legacy replacement**.

## Approaches and chosen boundary

1. **Full port of Node navigation/map/exobiology/EDSM in one PR — rejected.** The old `/nav/map` and `/exploration/system` depend on a rich `getSystem` service, EDSM and inference/credit valuation, while G2A provides only a six-field safe summary. Porting everything would conflate provider consent and data completeness, and risk inventing scan/bio certainty.
2. **Route-only monitor — useful but insufficient** for the accepted goal of navigation *and* exploration.
3. **Two journal-only Go projections, fed by the existing single source service — selected.** Implement a plotted-route panel and current-system body/scan survey with explicit confidence/freshness states. Add richer old-style system maps, predictors, neutron plotting and provider enrichment in separately scoped later work.

No additional journal watcher, Node/EDMC/Python runtime, remote arbitrary-file API, background website requests or game commands.

## UI / user-visible deliverables

Use existing exported React routes **`/nav/route`** and **`/exploration/system`** in the **G2B** limited preview shell, on both native PC and paired secondary browser. Preserve all 62 routes and G2A source-status/owner settings. The other routes, including `/nav/map`, remain visibly unsupported until migrated; the presence of navigation links is not evidence that their logic works.

**Navigation screen**:
- Current / last-known system; currently targeted *next jump* from a recent `FSDTarget` (or next matched route entry if target unavailable), route destination, remaining hops (only when defensible), star classes and ordered waypoint names, calculated individual hop distances where both coordinates are available and finite, actual route-data freshness label.
- When possible, scroll/mark current and next waypoint; after a real `FSDJump` advance on actual arrival, **never at `StartJump`**.
- Safe user-initiated **Copy system name** action copies text into the **viewer device's** clipboard, never remotely controls the gaming PC. Browsers can deny clipboard access; show a normal fallback. Do not send keypresses to Elite or write files.
- Show `No route`, `Route unverified/stale`, `Route cleared`, `Waiting for game` or `Partial route` honestly. If fresh `FSDTarget` does not match the plotted route, show as a separate target and mark route/target disagreement rather than silently claiming a valid next hop.
- No neutron/white-dwarf fuel or safety predictions, fuel-economy/engineering models, automatic game targeting, or Spansh calls. Star class is observational only.

**Current-system exploration screen**:
- System name and explicit `journal-only` label. From `FSSDiscoveryScan` show reported body count and progress when actually present; from `FSSAllBodiesFound` show scanner-completion evidence, never conflate a body-list count with the system's known total.
- Per-body rows from `Scan` keyed by current `SystemAddress` + exact `BodyID`, with a safe subset: body name, star/planet class, landability if present, atmosphere and volcanism if present, terraformable flag only when explicitly recorded, observed surface gravity with documented conversion to g, arrival distance, previously-discovered/mapped flags labeled as source facts, and commander-confirmed DSS mapping only when `SAAScanComplete` indicates it.
- From `FSSBodySignals`/`SAASignalsFound` show *confirmed signal counts* and optional genus names only when actually present. Genus ≠ confirmed species; no inferred biological species, first footfall, predicted payout, mapping credits or complete-planet-list claims.
- Unscanned/unreported body fields are **unknown**, not zero/false; partial scans, stale session, disconnected source and incomplete systems have distinct labels. A game jump switches to the target system view immediately at arrival, without showing the previous system's bodies.
- Read-only on laptop. No raw journal record, commander/FID, file paths, chat, mission/cargo/materials, local lat/lon, personal scan coordinates, or entire `Status.json` documents in viewer payloads.

## Source contracts and freshness

**Existing Go code:** `go/internal/journal/parser.go` exposes `Event{Type, Timestamp, FID, SystemName, SystemAddress, Raw, Position}` with internal exact-ID strings; `go/internal/journal/reader.go` provides `Replay`/`Live`; `go/internal/sidecar/snapshots.go` exposes valid bounded `NavRoute` snapshots; `go/internal/gamestate/service.go` owns the one source reader, commander partitions and safe summary. Extend this one service in focused files; do not start parallel readers, and do not modify the old `src/service/lib/event-handlers/{nav-route,system,exploration}.js`.

**Authoritative journal reference:** [Elite Dangerous Travel journal](https://elite-journal.readthedocs.io/en/latest/Travel.html), [Exploration journal](https://elite-journal.readthedocs.io/en/latest/Exploration.html), [NavRoute schema](https://jixxed.github.io/ed-journal-schemas/NavRoute.html). EDMC explicitly notes historical journal documentation calling the event `Route` instead of `NavRoute` and warns not every route cancellation is reported; treat current live evidence and sidecar validity conservatively: https://github.com/EDCD/EDMarketConnector/blob/main/PLUGINS.md.

### Navigation evidence

The plotted route resides in `NavRoute.json`, with `event:"NavRoute"`, timestamp and `Route` entries containing `StarSystem`, `SystemAddress`, `StarPos` and `StarClass`. Journal `NavRoute` indicates route plotted, `NavRouteClear` indicates clear, and `FSDTarget` describes selected next target and `RemainingJumpsInRoute` when emitted. Journal `Location`/`FSDJump`/`CarrierJump` are actual arrivals. `Status.json` may describe the *current body target*; do not mistake that for galaxy route.

**Crucial:** G2A's sidecar cache is identity-unverified. G2B must **not** attribute an old `NavRoute.json` to a new FID, nor resurrect a route after a `NavRouteClear`. Match a validated sidecar to the appropriate current commander/session through a correlated journal `NavRoute` marker (same event timestamp; choose and test a documented conservative, deterministic match rule) and ensure it is newer than the latest clear/session reset. If the file changes before/after its marker, keep it pending until both align; if no reliable match, show **unverified** rather than an actionable route. Older serialized route at startup can be described as last-known only if it can be tied to the replayed active FID and no newer clear; never label it as *currently active game target* without fresh evidence. Missing, truncated, invalid, oversized, or timestamp-inconsistent file must not replace last valid route without a stale state.

The `Route` array is bounded (design cap 2,048 entries or stricter justified cap). A larger file must show overflow/partial warning, never silently truncate and present fake final destination. Reject nonfinite coordinates, negative/wildly implausible travel distances and out-of-range count values. Preserve `SystemAddress` as exact decimal string; compute only optional approximate Euclidean distances from finite `StarPos` floats. Prefer address over system name for matching route progression. Do not infer a remaining hop count when current position is not found; `FSDTarget.RemainingJumpsInRoute` is a separate reported value with its own freshness label.

### Exploration evidence and attribution

Journal `FSSDiscoveryScan`: `BodyCount` and `Progress` (0..1) when present. `FSSAllBodiesFound`: `SystemAddress` and `Count`. `Scan`: `BodyID`, `BodyName`, `SystemAddress`, star/planet subtype and optional physical/scan fields. `SAAScanComplete`: actual mapping action. `FSSBodySignals` and `SAASignalsFound`: signal classes/counts, possible genus strings; decode only known structures and bound all nested arrays/strings.

Attribution rule: **do not assign a body event to a commander or system by arbitrary timestamp, name-only guess or stale sidecar**. Require a matching exact `SystemAddress` and current (per-FID or isolated unknown) identity/session context for body-specific data. Missing or contradictory address must be ignored or marked unattested internally; no cross-commander contamination. Exception: `FSSDiscoveryScan` has no `SystemAddress` in the documented schema; its aggregate `BodyCount`/`Progress` may be attributed only when a verified active commander session and confirmed arrived system exist, with continuous ordered journal source context after that arrival and no ambiguous pending change of system. Otherwise mark it unverified and show `unknown`, not a guessed total. No body-specific data is attributed by this exception. Do not treat historical `WasMapped` as “you mapped it”; retain explicit source meaning and separate `SAAScanComplete`. Route changes and jumps must not erase *other commander's* private cache, but remote viewer sees only the currently active vetted system.

Bound cache to a small LRU of verified current systems per commander (e.g., no more than 8 systems total / 512 bodies per system) and route size/bytes. Invalid data skips with a safe warning, not a crash. Journal replay reconstructs current system deterministically and does not broadcast thousands of intermediate UI states; no persistence or external side effects in G2B.

## API, role security and module boundaries

Two dedicated, **typed, allowlisted** read-only websocket RPCs and coalesced broadcasts:
- `getGoNavigation` → `goNavigationChanged` with typed `NavigationView`, no raw underlying JSON/FID/canonical journal path, no `StarPos`/coordinates exposed to secondary viewers. Names/classes, optional computed LY distances, hop sequence and evidence/freshness marker only.
- `getGoExploration` → `goExplorationChanged` with typed `ExplorationView`, no commander ID, lat/lon, private freeform journal fields, `Materials` or raw `Scan`. Bounded body list and accepted signal count/class.
Preserve G2A `getJournalSummary` and `journalSummaryChanged` exactly. Add G2B opt-in stage marker only when new APIs are ready. Existing owner local-only `/owner/journal` remains restricted by same-origin/session and protected profile. Paired viewer has **read-only** access to these *narrow safe* projections, consistent with the purpose of the remote dashboard; no shell, arbitrary path, OS control or host clipboard action.

Use one bounded service with a cancellation/subscribe lifecycle. For change broadcasts, always check the principal's authorization *at send time* under the existing WebSocket writer serialization, including delayed transport close/revocation. No route/exploration notifications to unpaired clients; no sidecar blob/array through generic loading status.

## Required acceptance / tests

1. **Navigation truth:** real-world source order variation (NavRoute journal marker preceding/following sidecar), FSDTarget before/during jump, StartJump vs arrival, repeated names/different addresses, missing target, wrong/stale sidecar, clear followed by unchanged stale file, direct jump outside route, oversized route and nonfinite coordinates.
2. **Exploration truth:** no scan data before honk, observed body count vs actual known Scan rows, duplicate Scan updates keyed by exact ID, FSS progress vs all-bodies, known/unknown optional fields, Scan vs SAAScanComplete mapping semantics, bio signal vs species prediction and out-of-system events ignored. Actual `SystemAddress` above 2^53 must survive exact.
3. **Privacy/identity:** two FIDs, unknown FID, logout/new LoadGame, route sidecar from previous commander, multiple viewers/reconnect/revoke, no arbitrary details/coordinates/chat/paths/financial data in web projections, owner remains owner-only.
4. **Reliability:** G2A source lifecycle, auto-discovery, game not running, live creation, JSON rewriting, replay/rotation/restart, corruption/recovery and late pairing; no extra source reader, stale async UI responses, unbounded subscriptions or event storms. Disconnected laptop clears stale live view or clearly shows offline last-known.
5. **Platform/CI:** existing Windows/Linux Go/JS/security/native journal smoke and Linux race remain required. Add synthetic NavRoute + FSS/Scan/SAAScanComplete fixtures, Go/React assertions, new native read-only route/system smoke, repeated racing/arrival/clear/revocation cases and build artifacts with checksums; no real game data/credentials in CI.
6. **Physical maintainer test:** Windows game already paired to laptop: plot a multi-jump route and verify next hop/destination, complete one jump and check progress on both screens, clear route and verify stale route does not reappear; honk, FSS a body, and DSS map when possible; verify supported known fields display and unavailable ones remain unknown. Test disconnection/rejoin. Physical Linux/Proton remains separate outstanding acceptance.

**No game commands or galaxy map targeting**; DAEDALUS cannot programmatically select a star in Elite's Galaxy Map through journal read-only data. OS clipboard on laptop is local user action only.

## Stop / follow-up

Implementation **PR #7** (or the next available number) should be **draft stacked from current `go/g2a-journal-ingestion`**, targeting that exact branch. PRs #2–#6 remain unmerged. No release or replacement; G2B is not complete solely because CI is green. If safe attribution of `NavRoute.json` cannot be established from observed in-game behavior, ship an **unverified-only route** with explicit limitation and bring evidence to review, not a fabricated active-route claim. Do not start G2C, provider integrations, neutron plotting or exobiology estimates without separate authorization.
