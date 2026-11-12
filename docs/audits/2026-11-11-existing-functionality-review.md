# Existing DAEDALUS functionality: source-level maintenance audit

**As of:** 2026-11-11  
**Target:** [Jakoczig/daedalus](https://github.com/Jakoczig/daedalus), main at `06a69635623c5bfacff51785a33fbb5f09948c9d`  
**Intent:** Public, maintained second-screen companion that can replace running several external apps simultaneously. Prioritize correcting existing behavior before adding features.  
**Evidence class:** Source inspection plus upstream specifications. Not a substitute for a Windows gameplay test or a comprehensive vulnerability assessment.

## Review method

Read JavaScript client, service, and Go launcher components from the fork; compared relevant behavior with live Inara API docs, EDDN schema documentation, Elite Dangerous journal format, EDMarketConnector's current core plugins, SrvSurvey criteria and BioScan rules, and current ship listings. For a defect not reproducible without real journal or hardware, the acceptance test is specified rather than claimed passed. Existing and planned features were classified independently.

### Findings by priority

| ID | Risk/priority | Area and source | Observed source behavior | Next verification / expected correction |
|---|---|---|---|---|
| SEC-01 | **P0 security** | `src/service/main.js`, `src/service/lib/event-handlers.js` | HTTP/WebSocket server listens without explicit bind host; its RPC dispatcher accepts handler names without authenticating or authorizing a client. `getPreferences` returns raw preferences containing third-party keys and refresh tokens, and `setPreferences` is remotely callable. | Test unauthorized LAN/loopback request against synthetic secrets; require paired sessions/least privilege, sanitized preference view, server-side secrets and explicit trusted-device controls before releases. |
| UPDATE-01 | **P0 release** | `src/app/updater.go`, `src/app/main.go`, `src/client/pages/launcher.js` | The update URL still targets `Varzeki/daedalus`; the launcher *does* expose and invoke manual update APIs. Download chooses the first GitHub release asset, does not check HTTP status/integrity/signature, and `InstallUpdate` ignores download errors before launching elevated binary. Only the automatic check on startup is commented out. | Test manual update path with synthetic release metadata; point at owned release channel, verify exact expected installer and a release SHA-256 or Windows signature, never execute if verification fails. Disable manual update until secure. |
| API-01 | **P1 functionality** | `src/service/lib/event-handlers/integrations.js` | Inara uses undocumented `addCommanderFlightLog`, object coordinates, `isDeveloped` rather than `isBeingDeveloped`, wrong success status check in the test and a lossy in-memory queue. Actual materials sync absent although UI promises it. | Implement approved Inara plan, with replayable fixtures and documented per-event response semantics. |
| API-02 | **P1 functionality** | Same file | EDSM journal upload lacks required game version/build and has no durable network queue. EDDN forces multiple specialized events into journal/1 and passes raw event fields that may fail schema/privacy validation. | Compare EDSM and EDDN official contracts plus EDMC, then add per-provider contracts, transport and schema fixtures. |
| API-03 | **P1 privacy/identity** | Same file, `src/service/main.js`, `src/app/main.go` | Frontier CAPI uses PKCE and a custom URL callback, but client registration and linked FID validation are not demonstrated; refresh token is kept in plaintext Preferences.json and available to `getPreferences`. | Validate allowed client ID and callback registration, session/FID match, scope/state/token rotation and credential isolation. |
| SHIPS-01 | **P1 data** | `src/service/data/edcd/fdevids/shipyard.json`, `src/service/lib/event-handlers/ship-status.js` | Bundled ship catalog is missing a range of modern ships, including Caspian Explorer, Anaconda Mk II, Mandalay, Type-8, and Panther Clipper Mk II. Unknown ship models fall back to internal journal symbols. Recent balance changes may also invalidate cached module/ship data. | Compare exact latest EDCD/FDevIDs release and current journal symbols, update mappings with generated provenance and regression fixtures for recent ships. Do not invent module identifiers. |
| POWER-01 | **P1 correctness** | `src/service/lib/event-handlers/powerplay.js` | Hardcoded `SHIP_PAD_SIZE` labels Diamondback Explorer Medium (it is Small), Mandalay Small (it is Medium), and Type-8 Large (it is Medium). Pad-sensitive recommendations can be wrong. | Unit test pad values from authoritative ship catalogs and preserve correct large-pad filtering. |
| POWER-02 | **P1 reliability** | Same file | `searchSpanshSystems` calls `execFileSync('curl.exe',...)`; blocking operation can stall the Node event loop for up to 20 seconds, is Windows-only, and has no explicit status diagnostics. Nearby-results cache key omits current power and whether a large pad is required. | Replace with async HTTP (injected client and timeout), test network failures, rapid change of pledged power, and ship change. |
| BIO-01 | **P1 correctness** | `src/service/lib/data/bio-criteria.json`, `src/service/lib/bio-predictor.js` | The bundled Bacterium Informem / Rocky `atmosComp` rule is malformed compared with SrvSurvey's July 2026 fix (PR #1028). `parseClause` accepts a partial regex match and `walkCriteria` skips invalid clauses as though they were comments. | Add strict source-rule validator, explicit comment handling, a fixture for the fixed branch and versioned source sync with diff report. |
| BIO-02 | **P1 scientific labeling** | `src/service/lib/exploration-value.js`, `src/service/lib/event-handlers/exploration.js` | Heuristic hit-count-based Bayes weights are displayed as percentages; a predicted species with `probability >= 100` receives `isConfirmed: true` even when `isScanned: false`. This conflates a prediction with a confirmed observation. | Separate environmental compatibility, relative estimate, DSS genus confirmation and species sample confirmation. Calibrate or relabel probabilities and values. |
| CLIENT-01 | **P1 multi-device UX** | `src/client/lib/wishlist.js` | Engineering wishlists, known ships and active-ship selection are persisted exclusively in each web client's `localStorage`. PC and tablet cannot share the same wishlist even though they connect to one Daedalus service. | Move authoritative wishlist state to service-side per-CMDR and per-ship storage; expose revision-safe CRUD over authenticated channels; migrate existing localStorage data once with consent. |
| RELEASE-01 | **P1 supply-chain** | `.github/workflows/release.yml` | Release workflow fetches the latest yt-dlp binary and ffmpeg ZIP from mutable URLs without pinned versions or hash verification, then packages resulting artifacts. | Pin versions and SHA-256, verify before packaging; generate release provenance and test on Windows. |
| CI-01 | **P1 regression** | `.github/workflows/ci.yml`, `package.json` | Lint job continues on errors, runs `npx standard ... || true`; provider/mapping tests are not in npm scripts; no reliable regression gate on future API changes. | Add Node `node:test` suites with deterministic fixtures, remove error suppression for new tests and lint gate only after baseline debt is controlled. |
| MINING-01 | **P2 correctness** | `src/service/lib/event-handlers/mining.js` | Despite a comment describing preferences override, target commodities and threshold are constant values. A jump resets mined/refined counters but preserves `sessionStart`; some event handlers update limpet counts without an immediate broadcast. | Replay journal fixtures for prospector, refinery, ring scans, limpets, system jumps, login; test rate/hour, target preferences and cargo consistency. |
| MODULE-01 | **P2 compatibility** | `src/service/lib/event-handlers/ship-status.js` | Unknown engineering blueprints log an error and use `continue` within the module loop. The module then bypasses normal enrichment and may show incomplete metadata. | Graceful fallback that retains raw engineering info and still enriches module regardless of catalog freshness. |
| STATUS-01 | **P2 correctness** | `src/service/lib/event-handlers/ship-status.js` | Last-known ship is a single in-memory global; ships and other commanders aren't persisted for fleet use. Off-ship state depends on that process cache. | Test logging in as another CMDR, ship swaps, on-foot/SRV, and restart; eventually partition persistent state by FID/ShipID. |
| JOURNAL-01 | **P2 coverage** | `src/service/lib/elite-log.js`, `src/service/lib/elite-json.js` | Existing incremental reader and deduplication work, but native/hybrid/poll fallback and restart/rotation/error paths are not covered by an automated replay test. Snapshot files are watched and loaded asynchronously. | Add rotation, partial-line, truncation, duplicate, rapid-change, read-error, and start-mid-session tests before any ingestion refactor. |
| CONTROL-01 | **P2 compatibility** | `src/service/lib/event-handlers/keybinds.js`, `src/client/pages/controls/button-pane.js` | Reads modern bind files but the custom button pane is explicitly a placeholder. Installer and game-location discovery rely on Windows paths and a regexp for bindings. | Fixture-test Steam/Epic presets, T.16000M/VKB/other HOTAS bind names and malformed XML; postpone button-pane implementation until security is in place. |
| SCREEN-01 | **P2 coverage** | `src/client/pages` | At least 16 routed screens are stubs (neutron plotter, expedition log, fleet/carrier/ship stats, commander overview/session/wing/community goals, routes/commodities/PTN/FCOC, missions overview/local activity, custom buttons). Mining status/rings/log/stats are real implementations in v2.2.0; the old mining plan is stale. | Keep honest feature status matrix; do not claim a feature works merely because a route exists. Validate finished screens independently before starting deferred features. |
| AUDIO-01 | **P1 public redistribution** | `game_voicelines`, `resources`, `README.md` | Distribution includes game-sourced COVAS voice lines and other assets; GPL-3.0 on project code does **not** automatically authorize redistribution of game recordings. | Audit asset provenance, license and right to redistribute, remove or separate unsupported media for public packages. |
| API-04 | **Needs runtime verification** | `src/service/lib/event-handlers/galnet.js`, `src/service/lib/event-handlers/mining.js` | GalNet uses a fixed Frontier CMS endpoint; mining price lookup uses Ardent Insight API v2 and assumes the first result's sell price. Neither was independently validated in this review against live responses. | Add response-shape/timeout/rate-limit fixtures, freshness labels and graceful offline state; validate against current official/provider contracts before relying on prices/articles. |

### P0/P1 release blockers

1. SEC-01: no unauthenticated remote credential access.
2. UPDATE-01: never install the upstream project's or unverified executable through a community fork.
3. API-01..03: no unexpected external telemetry, invalid API events, or silent data loss.
4. CI-01 and RELEASE-01: release must be test-gated, reproducible and integrity-checked.
5. BIO-01..02 and SHIPS-01: displayed predictions, ship identity, and known data must be faithful to the latest applicable game versions.
6. AUDIO-01: public asset distribution rights must be verified.

### Functional inventory

**Implemented but needs behavioral checks:** journal streaming; Live/Status JSON ingestion; multi-display socket/UI; in-game system map and navigation; exploration route and system valuation; biological prediction/scanning; engineering material and wishlists; mining panels and pricing; Powerplay state and nearby commodity search; GalNet and multimedia; keyboard/joystick bindings; COVAS overlays; EDSM cartographic lookups; landing pads; installation/updating.

**Present but broken/incomplete (higher priority):** Inara/EDSM/EDDN/CAPI, older ship metadata and pad sizes, exobiology criteria and epistemic labels, cross-device wishlist state, secure sharing, release update channel.

**Clearly not yet implemented:** `/nav/neutron-plotter`, `/exploration/log`, `/ship/carrier`, `/ship/fleet`, `/ship/stats`, `/cmdr/overview`, `/cmdr/session`, `/cmdr/community-goals`, `/cmdr/wing`, `/trade/routes`, `/trade/commodities`, `/trade/ptn`, `/trade/fcoc`, `/missions/overview`, `/missions/local-activity`, `/controls/button-pane` (sixteen checked stubs).

### Primary evidence / references

- Official API: https://inara.cz/elite/inara-api-docs/
- EDSM journal API: https://www.edsm.net/en/api-journal-v1
- EDDN live schemas and developer guide: https://github.com/EDCD/EDDN/tree/live/schemas
- EDMarketConnector (Inara, EDSM, EDDN, CAPI): https://github.com/EDCD/EDMarketConnector
- SrvSurvey rule descriptions: https://github.com/njthomson/SrvSurvey/tree/main/SrvSurvey/bio-criteria
- SrvSurvey July 2026 malformed rule correction: https://github.com/njthomson/SrvSurvey/pull/1028
- Independent biological criteria: https://github.com/Silarn/EDMC-BioScan
- Inara ship list: https://inara.cz/elite/ships/
- NEWP ship catalog: https://newp.io/ships
- Journal format: https://elite-journal.readthedocs.io/en/latest/File%20Format.html

## Audit completeness and limitations

This is a **first-pass source audit** of high-impact paths, not line-by-line certification of every file. No full project clone, npm dependency install, live Inara/CAPI authentication, running Elite Dangerous session, Windows installer execution, or executable asset verification was possible in this environment. Keep all findings tagged as **source-observed** or **runtime-verified** in future tickets.

On a Windows test system, record anonymized journal/JSON fixtures; never commit CMDR API keys, FID, private token URLs, or real session data. Add a consent-safe sample dataset for contributors. Initial implementation should make CI and Inara trustworthy, then expand to EDSM/EDDN/CAPI, predictions, and the remaining findings. New features such as Spansh neutron plotting must wait until the maintenance blockers clear.
