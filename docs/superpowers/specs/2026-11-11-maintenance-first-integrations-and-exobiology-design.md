# DAEDALUS Terminal: maintenance-first integration and exobiology design

**Status:** Proposed for maintainer review (no implementation approved yet)  
**Date:** 2026-11-11  
**Repository:** [Jakoczig/daedalus](https://github.com/Jakoczig/daedalus)  
**Upstream:** [Varzeki/daedalus](https://github.com/Varzeki/daedalus) (fork of iaincollins/icarus)  
**First milestone:** Correct, reliable and secure integrations; validated exobiology criteria  
**Long-term aim:** One public, maintained Elite Dangerous companion application, optimized for a second display and remote tablets, in place of several concurrently running companion applications.

## Intent and why

Daedalus already provides a Windows/Node/Go service and a multi-client React second-screen interface, plus exploration, engineering, exobiology, mining, and partially implemented integrations. Its maintainer wants a **public community project**. The priority is to repair and update existing behavior using the authoritative services' documentation and proven EDMarketConnector (EDMC) implementations before introducing any new features. Neutron plotting and other EDMC-plugin-inspired functionality are future work, not blockers to this maintenance milestone.

## Goals and explicit non-goals

**Goals**

1. Make existing Inara, EDSM, EDDN, and Frontier CAPI integrations accurate against their current public protocols, with correct event mapping, validation, retry, rate limiting, consent, and observable status.
2. Establish replayable, automated, fixture-based contract tests and Windows smoke tests before real uploads.
3. Verify Daedalus's exobiology prediction data and algorithms against current SrvSurvey / EDMC BioScan rules and representative journal observations; document provenance and uncertainty.
4. Preserve the existing responsive second-screen experience; ensure that no user must run EDMC concurrently to obtain functionality that Daedalus provides.
5. Prepare repeatable, community-reviewable public releases with reproducible change evidence.

**Non-goals for the first milestone**

- Implementing the coming-soon neutron plotter, trade planner, fleet manager, controls, or other placeholder features.
- Migrating Next.js, React, Go, or the packaging stack purely for modernization.
- Copying Python EDMC plugins into Node at runtime, installing EDMC as a dependency, or scraping private websites.
- Guaranteeing a statistical probability for a biological species without calibration data.
- Sending game controls or simulated input from a remote tablet.
- Automatically uploading historical journals, beta data, or Legacy galaxy data to live services without their explicit support.

## Approaches considered; selected approach

1. **Incremental maintenance of Daedalus (selected):** preserve current client/service/launcher architecture; isolate providers behind small adapter modules, add a durable queue and test harness, repair one provider at a time. Low user disruption, but requires untangling existing service code.
2. **Copy or embed EDMC plugins:** reuses mature logic faster in principle but requires a Python plugin runtime, duplicates game-journal reads, complicates updates and multi-device behavior; rejected for the one-app objective.
3. **Full rewrite:** could yield cleaner modularity but would discard substantial working functionality, increase regression risk, and delay practical fixes; rejected for this phase.

EDMC, its plugin ecosystem, SrvSurvey and BioScan remain **reference implementations and data/protocol sources**. Any re-used code or data must carry appropriate attribution and compatible license notices. Preserve upstream GPL/third-party obligations and separately audit game audio and copyrighted assets before public distribution.

## Existing architecture and integration state (inspected 2026-11-11)

- Client: `src/client/components/settings.js`, `src/client/lib/socket.js` (React/Next).
- Service dispatch: `src/service/main.js`, `src/service/lib/event-handlers.js`.
- External integrations: `src/service/lib/event-handlers/integrations.js`, currently a single class for EDSM, EDDN, Inara and CAPI.
- Journal ingress: `src/service/lib/elite-log.js`, incremental readers with memory/database ingestion and event checksum.
- Data: `src/service/lib/bio-predictor.js`, `src/service/lib/data/bio-criteria.json`, `src/service/lib/exploration-value.js`.
- Existing CI: `.github/workflows/ci.yml`; JavaScript lint is currently non-blocking and provider contract tests are absent.

These are code-review findings, not live authenticated-service test results.

## Verified defects and priorities

### P0: Inara (first functional repair)

Official documentation: https://inara.cz/elite/inara-api-docs/ and https://inara.cz/elite/inara-api-devguide/  
Reference: https://github.com/EDCD/EDMarketConnector/blob/main/plugins/inara.py

- `testInara()` checks `header.eventStatus === 100` but documented success is `200`; additionally evaluate each `events[i].eventStatus` and distinguish soft errors/warnings.
- Request header sends `isDeveloped`, whereas the documented property is `isBeingDeveloped`. Mark test builds correctly.
- `_queueForInara()` sends undocumented `addCommanderFlightLog` for FSDJump/Location. Use `addCommanderTravelFSDJump` for an actual FSDJump; `setCommanderTravelLocation` for an initial Location, plus separate dock/land/carrier events only when supported by journal facts.
- `starsystemCoords` is sent as an object; Inara expects a numeric `[X,Y,Z]` array. Avoid undocumented fields, including indiscriminate `starsystemAddress`.
- Advertised materials sync is not implemented. Add snapshot-driven `setCommanderInventoryMaterials` only after a complete Materials.json snapshot, and test empty/partial files; similarly scope actual cargo/ships/ranks/credits in separate increments with appropriate Inara methods.
- `_flushInara()` removes queued events before confirmed delivery and drops them if credentials are absent. Queue must survive failed sends/restarts and must not silently discard credentials-missing work.
- Honor Inara's Legacy/beta restrictions, maximum event age, batching, low request frequency, duplicate suppression and commander identity. Ignore fake CQC/training locations and multicrew state where inaccurate.
- Validate both HTTP and API-level success per item; show actionable errors without exposing credentials. Avoid needless credits-log spam: use EDMC-like significant-change and session boundary policy.
- Coordinate an application identifier with Inara's developer before public production activation; no live sync without individual user opt-in.

### P0/P1: EDSM

Official documentation: https://www.edsm.net/en/api-journal-v1  
Reference: https://github.com/EDCD/EDMarketConnector/blob/main/plugins/edsm.py

- `_sendToEdsm()` omits mandatory `fromGameVersion` and `fromGameBuild` fields. Capture from Fileheader/LoadGame and attach to each request.
- Respect Live-only support; avoid sending beta/Legacy data.
- Batch appropriate journal records; process success, transient and permanent application-level responses. Maintain the service's supported/discarded events list and handle response-specific outcomes.
- Keep EDMS lookup (used by cartography/exobiology) distinct from user-authenticated journal upload: read-only lookup must still function when upload is disabled.

### P0/P1: EDDN

Official documentation: https://github.com/EDCD/EDDN/tree/live/schemas and https://github.com/EDCD/EDDN/blob/master/docs/Developers.md  
Reference: https://github.com/EDCD/EDMarketConnector/blob/main/plugins/eddn.py

- Current code routes multiple types, including FSSDiscoveryScan/NavBeaconScan, through a generic `journal/1` schema. Choose the actual *live* schema appropriate to each event.
- Current journal messages often lack schema-required `StarSystem`, `StarPos`, `SystemAddress`, or other context. Augment using authoritative current journal state; do not invent missing values. Do not confuse EDSM's underscore-prefixed transient fields with EDDN schema fields.
- Strip disallowed localized/personal/private data according to live schema and sender guidelines, not by a generic unconditional copy of the full journal object.
- Validate each outgoing message locally against the current canonical JSON schema before upload; test gzip/transport and response handling.
- Respect supported beta/Legacy destinations and policies. Use opt-in data sharing and a persistent delivery queue; do not overwhelm the EDDN gateway.

### P1: Frontier CAPI

Reference: https://github.com/EDCD/EDMarketConnector/blob/main/companion.py

- Audit OAuth2 PKCE, registration/ownership of the client ID, registered custom-protocol callback, CSRF state, token expiry/rotation, and application shutdown during login.
- Verify token owner matches the current in-game commander/FID before using profile or other CAPI data. Avoid mixing commanders, sessions and account credentials.
- Do not distribute another application's credentials, or request users to paste tokens into chat/repository issues. Keep tokens in OS-appropriate secure local storage, never in public WebSocket responses, logs or error reports.
- Treat CAPI as read-only until account validation, rate limiting and explicit consent have been tested. Never depend on CAPI for functionality already available accurately in journals unless it adds needed coverage.

### P0: WebSocket security prerequisite to public release

- `src/service/main.js` currently exposes an unauthenticated WebSocket RPC endpoint that can access `getPreferences`, including third-party credentials. Do not expose this unchanged beyond trusted local environments.
- Introduce authenticated, least-privilege, scoped remote clients. Keep secret fields server-side and redact them from preferences responses and diagnostics. Protect preference mutation, OAuth callbacks and any future Windows clipboard action.
- Default network exposure to local-only or explicit paired-device opt-in; document firewall implications. Test negative/unauthorized access cases.

## Exobiology correctness and scientific labeling

Reference sources:

- SrvSurvey criteria specification: https://github.com/njthomson/SrvSurvey/blob/main/SrvSurvey/bio-criteria/readme.md
- SrvSurvey rules: https://github.com/njthomson/SrvSurvey/tree/main/SrvSurvey/bio-criteria
- SrvSurvey algorithm: https://github.com/njthomson/SrvSurvey/blob/main/SrvSurvey/BioPredictor.cs
- EDMC BioScan independent implementation: https://github.com/Silarn/EDMC-BioScan

Daedalus bundles 21 genus trees in `bio-criteria.json`. A confirmed stale criterion: Daedalus's Bacterium Informem / Rocky body branch has malformed `atmosComp [Nitrogen >= 100] | SulphurDioxide >= 28 | Argon >= 0.06]` text; SrvSurvey fixed it to `atmosComp [Nitrogen >= 100 | SulphurDioxide >= 28 | Argon >= 0.06]` (SrvSurvey PR #1028, July 2026). This is a **known candidate for a regression test**, not yet a validated game-world outcome.

Plan:
1. Record the exact upstream SrvSurvey commit and per-file hashes of source criteria. Generate an inventory of rule additions, changes, and removals versus bundled Daedalus data. Validate the rules parser and tree expansion before importing.
2. Compare representative differences against BioScan. When sources disagree, use observed journal/codex datasets, and preserve the disagreement in tests or documentation rather than choosing an unexplained override.
3. Test atmosphere, pressure, gravity, star-parent luminosity, galactic region, nebula proximity, Guardian bubbles, volcanic composition, EDSM rounding, incomplete metadata, and species/variant filtering. Missing data must yield an explicit unknown/low-confidence state, not a false certainty.
4. Validate the displayed percentages: label them **heuristic scores** unless empirically calibrated against a representative positive/negative dataset, including base rates and known sampling bias. Prefer likely/possible/unknown and disclose why a species matched or failed a criterion.
5. Distinguish predicted potential value, observed signals, discovered species, first-footfall eligibility, and actual sold exploration value. Preserve reproducibility with fixture-based journal replays.
6. Document provenance and license for every imported or transformed data source; changes require tests and human review, not blind upstream auto-sync.

## Adapter and reliability design

Keep the current CommonJS Node service and client/launcher boundaries. Refactor incrementally, not in a single high-risk migration:

- An integration coordinator consumes only **new** journal events and validated JSON snapshots; adapters transform into each provider's documented contract.
- Per-provider transport with injected HTTP client/clock, strict request/response validation, and no real external calls in automated tests.
- Durable, bounded outbound queue and acknowledgement ledger (prefer using the existing NeDB dependency unless a measured requirement justifies replacing it); per-provider and commander identity partitions; deterministic event IDs from source fingerprint plus event identity; retry transient failures with exponential backoff/jitter; quarantine permanent rejections for inspection.
- Snapshot events and delta events have distinct semantics. Do not erase a full remote materials inventory from an incomplete local snapshot. Do not replay stale events outside the destination's allowed age/window.
- Provider-level opt-in, health, time since last successful sync, queued count, last sanitized error, test mode, and manual retry/reconnect where safe.
- No secrets in browser WebSocket payloads, analytics, HTTP logs, crash dumps, test fixtures or issue reports. Do not cache API credentials in state forwarded to remote tablets.
- Journal files are the preferred authoritative local source; CAPI/EDSM are supplementary, and third-party APIs are never assumed to have perfect coverage.

## Verification and phased delivery

1. **Baseline inventory:** record supported/unsupported event mappings, current defects, dependency/license inventory, Windows smoke-test steps, and before-fix fixtures.
2. **Security + tests:** enforce CI (not advisory), add Node built-in test runner, mocks, canonical schema fixtures, negative auth/secret leakage tests, and deterministic journal replay. Existing UI functionality must remain available on a connected second screen.
3. **Inara PR:** fix documented event/header/status bugs, add delivery semantics, then expand to materials/credits/ranks/ships/cargo one independently verified mapping at a time.
4. **EDSM PR:** game metadata, valid event batching, response validation, lifecycle and retry.
5. **EDDN PR:** live schema routing/validation, privacy filtering, correct augmentation, verified transport/queue.
6. **CAPI PR:** PKCE + account identity + secure token handling + consent; verify registered client ID and custom scheme before enabling public builds.
7. **Bio PRs:** rule parser regression suite, source-diff evidence, candidate rule updates, confirmation against recorded data, confidence labeling.
8. **Release candidate:** fresh Windows install/upgrade/uninstall, localhost and paired-device smoke tests, real-game replay, opt-in manual authenticated service tests in a development identity, privacy review, build/release checks and community documentation.

### Minimum acceptance criteria

- CI fails on malformed provider payloads, unsupported API events, unhandled response statuses, token leakage, missing required EDSM metadata, wrong EDDN schemas, and regression fixtures.
- No lost events after a simulated network timeout, process restart, disabled integration, or temporary endpoint failure; no duplicate event replay after successful acknowledgement.
- Each provider can be individually enabled/disabled, clearly reports its verified status, and has no unintended outbound telemetry when disabled.
- No real production API write during unit tests or CI; manually verified production accounts only after opt-in, API provider requirements and registration are satisfied.
- Bio criteria differences and runtime compatibility are reviewed with versioned provenance and repeatable tests; percentages are defensible or relabeled honestly.
- Public Windows builds pass fresh-install, game-running, multiple-browser/device and offline scenarios; licenses and game-audio distribution rights have been reviewed.

## Future phase, explicitly deferred: a true one-app second-screen workflow

Use [EDMC-NeutronDancer](https://github.com/dwomble/EDMC-NeutronDancer), [EDMC-SpanshTools](https://github.com/wuuthradd/EDMC-SpanshTools), and the current [EDMC Plugin Registry](https://github.com/EDCD/EDMC-Plugin-Registry) as functional references for a Daedalus-native Spansh neutron/galaxy route planner, step-by-step route progress, destination copy, and system discovery.

**Critical second-screen nuance:** copying text in a tablet browser copies to the **tablet's** clipboard, not to the Windows game PC. Later provide an explicit authenticated `Copy next waypoint on PC` server action (and convenient plain text to copy on-device), never an unauthenticated arbitrary command runner or automatic in-game input injector. Route planning must understand ship jump limits, neutron boosts (including 6x-capable ships when applicable), fuel and safe star information and support offline/resume behavior.

## Remaining decisions

- **Already selected by maintainer:** public community fork; incremental approach; existing integrations and prediction accuracy before new features; EDMC/plugins as comparison references; one-app objective with second-screen support.
- **Default engineering decisions proposed:** preserve current stack until maintenance is stable; maintain separate feature PRs; use fixture-driven tests and a small durable outbox; require explicit user opt-in for network data uploads.
- **Maintainer approval needed:** approve this scope before product-code changes; choose later whether remote pairing must support devices off the home LAN (not needed for the first maintenance release). Actual live Inara/CAPI confirmation requires provider-side registration and user-owned test credentials kept outside chat/repo.

This design is intentionally limited to the maintenance-first milestone. New features should receive their own design and implementation plan after the existing integrations and predictions are reliable.
