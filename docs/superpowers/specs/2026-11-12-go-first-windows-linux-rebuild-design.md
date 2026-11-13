# DAEDALUS: Go-First Cross-Platform Desktop Rebuild

**Date:** 2026-11-12  
**Status:** Proposed for maintainer approval; architecture direction selected  
**Repository:** https://github.com/Jakoczig/daedalus  
**Previous direction:** [performance-first native modules](./2026-11-12-performance-first-native-modules-design.md) — superseded by this design  
**Related work:** [Phase 1A security PR #2](https://github.com/Jakoczig/daedalus/pull/2); [maintenance and feature audit PR #1](https://github.com/Jakoczig/daedalus/pull/1)  

## Decision and reason to migrate now

The maintainer uses DAEDALUS on a secondary laptop while Elite Dangerous runs on the game PC. The project will become a public, resource-efficient, modular companion that can replace parallel companion applications such as EDMC and its selected plugins. **Full native desktop support is required for Windows and Linux**; remote clients must keep working over an authenticated web interface.

The maintainer explicitly chose **beginning the backend migration to Go now, before adding more functionality**, rather than expanding the Node.js server and later rewriting it. This changes the previously approved Node-native proposal. The choice is motivated by lower ongoing architectural and duplication costs, not a demonstrated benchmark of Go versus Node.

**Selected target:** Go backend and module registry, Wails v3 desktop shells for Windows/Linux, **existing React static web UI retained initially**, authenticated Go-hosted browser interface. The Windows Wails runtime uses WebView2 and Linux uses GTK4/WebKitGTK 6.0 (actual distribution requirements must be checked). Target maintained, tagged stable Wails v3; pin an exact tag after toolchain verification. Go >=1.26 as required by current stable v3 docs.

**Immediate policy:** freeze new Node-specific product features, particularly Phase 1B Inara implementation in JavaScript; use the current service and Phase 1A security PR as validated behavior/test references. Urgent security fixes for users of the legacy Windows version can continue separately. Do not discard the old code until parity gates pass. This is a Go-first rewrite of the **runtime**, not an immediate rewrite of every page and its styling.

## Goals

1. One installed DAEDALUS application/service on the gaming PC; no EDMC, Python interpreter or Node.js process required at **runtime**.
2. Full native desktop experience on **Windows and Linux**, including window management, journal discovery, secure storage, module settings, updates and packaging where the OS permits. A local browser and remote secondary laptop/tablet continue to work.
3. Shared, accurate journal/JSON ingestion and event bus: **one watcher per source**, one authoritative per-CMDR state cache, no repeated third-party uploads across multiple display clients.
4. All **currently implemented** useful Daedalus features must be audited and either preserved, replaced with a verified equivalent, or explicitly and separately approved for retirement. Existing placeholders do not become retroactive parity requirements.
5. Rebuild Inara, EDSM, EDDN, Frontier CAPI and exobiology directly to the current provider/game contracts **in Go**, using EDMC, SrvSurvey and BioScan as independently checked references.
6. Performance-first native modules: enable/disable actual backend work independently from per-device hide/show UI settings. Maintain clear source/author/license credits and compatibility.
7. Automated Windows and Linux builds, deterministic offline contract tests, manual gameplay and secondary-screen acceptance, reproducible performance measurements, safe updates and rollback.

## Non-goals, anti-goals and constraints

- No big-bang replacement of the UI, release channel or all existing features in a single PR.
- No shipping an untested Go placeholder that claims parity with working exploration/engineering/mining features.
- No automatic execution of arbitrary EDMC plugins, unverified GitHub code or platform shell commands.
- No assumption that Go is universally faster: RSS, private bytes, WebView renderer memory, browser CPU and processing latency must be benchmarked against the real old application and EDMC workflows.
- No real provider uploads or authentication in CI. Beta/Legacy restrictions, registered client IDs, copyright, GPL-3.0-or-later and attribution continue to apply.
- No unpaired LAN/Internet access to game data. Preserve Phase 1A's separate owner/viewer roles and secret redaction.
- No aggressive OS automation or remote input injection; future PC clipboard requests require a narrowly scoped host capability.
- No automatic migration of credentials if secure handling cannot be guaranteed.
- No unnecessary dependency on Electron, separate Chromium install, Python interpreter, an external Node server or database daemon.

## Approaches considered

**A. Stay with Node, add features now and migrate someday (rejected).** Minimizes immediate effort but every future backend module would need to be ported; duplicates work and prolongs the weaker platform integration.

**B. Full big-bang Go/Rust + new frontend rewrite (rejected).** Reimplements known-good behaviors, UI and integrations at once with a high risk of regressions, delayed usefulness and source drift. Language choice alone does not ensure lower total RAM usage.

**C. Begin a Go-first, parity-gated replacement now (selected).** Establish cross-platform Go + Wails architecture and deterministic parity checks before new features; retain existing React frontend, static assets and the legacy Node implementation as a temporary oracle. Replace the runtime in vertical slices; ship only after native Windows/Linux and browser flows pass. This captures the maintainer's early-migration benefit while containing risk.

**D. Rust/Tauri instead (not selected).** Potentially smaller runtime overhead in hot paths but creates more migration complexity without evidence that DAEDALUS's mostly event/HTTP workload is CPU-bound. Revisit only if measurements justify it.

## Proposed architecture

```
Game PC (Windows native or Linux Steam/Proton)
  Wails v3 desktop window(s) -- WebView2 / GTK4+WebKitGTK
            |
            v
  Single Go runtime / app server (no Node/Python runtime)
      journal + JSON file detectors/watchers
      canonical parser and per-CMDR state
      typed, cancelable event hub / module registry
      durable private preferences + secrets
      outbound provider adapters + rate-limited outbox
      local HTTP + authenticated WebSocket + remote TLS/pairing
            |
            +--> local desktop UI (existing React static frontend)
            +--> secondary laptop/tablet browser (same UI and shared data)
```

**Packaging:** Single Go backend running within the Wails desktop process by default; it should not spawn an independent identical data reader for each window. A separately buildable headless Go service is optional for advanced setups but not a prerequisite for using the full desktop app. Compile the same domain packages for both entrypoints.

**Frontend:** Retain existing static Next/React pages initially, including the 62-page exported app, navigation, style system, card components and familiar user workflows. JS is a *build-time* tool and a UI language, not a Node service requirement. Fix Node-specific imports or browser assumptions behind a transport adapter; do not rewrite every component just for stylistic consistency. In the long run, consider a smaller UI framework only if profiling supports the cost.

**Go packages (illustrative boundaries, names adjustable by implementer):**
- `internal/journal`: platform journal discovery, incremental tail/rotation, explicit Fileheader, replay.
- `internal/gamestate`: snapshots, FID/ship/system partitioning, 64-bit identifiers.
- `internal/events`: publish immutable normalized events; subscriptions have explicit teardown.
- `internal/modules`: manifest, enable/disable, capabilities, dependency graph, per-device UI visibility.
- `internal/web`: compatible HTTP/WebSocket protocol, authenticated viewer/owner sessions and pairing.
- `internal/providers`: Inara/EDSM/EDDN/CAPI and read-only Spansh/other APIs as separate adapters.
- `internal/outbox`: durable, per-provider/per-CMDR queued records, dedup, retry and ack. Choose embedded file/SQLite implementation based on cross-platform correctness and benchmark, not based on preference alone.
- `internal/platform`: OS-specific window, browser, media, clipboard, path resolution, secure storage and update actions; explicitly detect unsupported features.
- `cmd/daedalus`: Wails native desktop executable; optional `cmd/daedalus-headless` for supported web-only deployment.

## Contract compatibility and migration safety

Keep the **wire protocol** and client UI behavior backward-compatible while migrating (e.g. `getShipStatus`, `getExplorationBiologicals`, `loadingProgress`, `newLogEntry`). Where old behavior is demonstrably wrong, version the protocol/change tests and adapt the frontend deliberately rather than perpetuating a bug.

Treat the existing Node service as a reference and behavior oracle, **not** a permanent prerequisite or a normative source for broken third-party contracts. Build a captured, sanitized journal fixture and a deterministic dual-run comparison harness. Inputs: Fileheader, LoadGame, Location, FSDJump, NavRoute, Materials/Cargo/Status JSON, Shipyard/Modules, mining events, bio scan, Powerplay, session stop/restart. Outputs: state snapshots, RPC envelopes and broadcast sequences. Normalize non-semantic field order/timestamps; identify real semantic differences.

**Important:** JavaScript's numeric precision problem with 64-bit game identifiers is not a reason to silently change them in Go. Use exact `json.Number` or int64/uint64 as appropriate and safe JSON serialization; test high-value SystemAddress and MarketID values. Do not round-trip through JavaScript Number when a decimal string is required. Use schema-aware translation to maintain browser compatibility.

Phase 1A security fixes have value as test cases and policy, regardless of code language: anonymous/CSRF/Origin rejection, secrets never returned, owner vs viewer rights, device revocation, no leaked WAV/video/private media, safe permissioned actions. Port these tests to Go at the same time as the boundary; avoid reintroducing the vulnerabilities fixed in PR #2.

## Windows + Linux native desktop support

**Windows:** Wails v3/WebView2, multi-window handling, optional always-on-top, actual Saved Games known folder (not hardcoded `C:\Users\x`), update paths, native notifications and later host clipboard. Package installer with integrity/signature verification. Native media helpers/voice resources require independent tests and correct redistribution licenses.

**Linux:** Wails v3 using GTK4/WebKitGTK 6.0. Set an explicit supported distribution/architecture matrix based on dependency availability, initially x86-64 modern distros; test both X11 and Wayland for window management, HiDPI, focus, tray and overlay behavior. A desktop window is a first-class supported mode, not merely a web page.

**Linux game discovery:** Detect Elite Dangerous journal files inside Steam Play/Proton compatdata 359320, including normal Steam, Flatpak Steam, alternate Steam library locations and user-selected custom Wine/Heroic prefixes. Check path existence and accessible journal patterns; do not assume one fixed `~/.steam` location. Offer folder picker and persist override. Verify behavior when Steam has not launched game yet and on journal rotation.

**Linux specific features:** The Windows media session and COVAS/voice helper implementations do not magically work in Linux. Design portable equivalents or show clearly unavailable capability; don't silently spawn PowerShell/WMI/Windows DLLs. Update/install mechanics are OS-specific and may need different signed/verified packages. For public stable release, meet feature parity in game-critical views on both OSes and clearly document optional OS-specific media limitations. User-selected data directory and desktop environment compatibility get test coverage.

**Remote displays:** Default local listener restricted to loopback. Optional HTTPS remote listener, explicit pairing/revocation and read-only viewer role. A Wails app must not make the local native UI dependent on a second free-floating public listener. One process must handle multiple authorized clients and windows without duplicate journal watchers.

## Modular feature model

A bundled module is native Go business logic plus optionally lazy-loaded React UI and a manifest with: feature ID, provider/data dependencies, platform capabilities, ownership/author/attribution, licensing, privacy/upload behavior, default enablement and supported game versions. No arbitrary code is downloaded to enable a bundled feature.

Three independent states:
1. **Bundled/available** (part of signed/verified application build).
2. **Enabled** (actual event subscriptions, timers, cache and network upload; upload has separate explicit consent).
3. **Visible on this device** (UI navigation/cards local to Windows desktop, laptop, tablet, etc.).

Disabling a module must cancel subscriptions/timers and free module-specific caches with tests. Hiding it from a laptop must *not* disable desired PC-side BGS tracking. Profiles can preselect visibility (Exploration, Engineering, Mining, BGS, Trading), but not silently enable credentialed uploads or host control.

**Priority after backend reaches parity:** native Spansh/neutron planning and safe next-system-copy workflow for secondary laptop; verified bio rules; BGS-Tally-style event tally; additional overlays; true module marketplace/data-only rule packs only once secure distribution and compatibility exists. For downloaded native executable modules, create a separate security model and signed packages first; auto-update checks must never be “run latest GitHub executable at startup.”

## Migration stages and stop/ship gates

### Stage G0 — Inventory, baselines and toolchain
- Preserve working Windows v2.2.0 source, Phase 1A security branch, fixture data and existing feature matrix; do not create Go code until this design is approved.
- Record process-group RAM/CPU, idle and journal-replay latency on Windows desktop + second laptop, including Node, WebView and browser processes. Document source of measurements.
- Probe Wails v3 on CI Windows and Linux and selected target distros, including `wails3 doctor` results and GTK/WebKit dependencies. Pin tested versions.
- Define exact game-critical parity gates and OS support matrix.

### Stage G1 — Executable cross-platform skeleton
- New Go workspace, Wails desktop application, embed existing static React UI and bind a local authenticated Go HTTP/WebSocket server. **No separate Node process at runtime**. Introduce OS-specific tests and Windows/Linux CI builds.
- Native window open/close, multiple local windows, crash recovery, remote HTTPS/pairing and local web access must work with synthetic state.
- Keep interfaces minimal and tested. No third-party upload credentials/live traffic.

### Stage G2 — Reliable game ingestion/security + major UI parity
- Journal discovery on Windows and Proton/Linux, incremental tailing, partial lines, rotation, JSON snapshot watcher and exactly-once *local event identification*.
- Auth boundary equivalent to Phase 1A, secret isolation, clipboard/OS controls unavailable to viewers without explicit grants.
- Migrate source-correct state systems one at a time: navigation/system/map, ship/status/cargo, engineering/materials, exploration/bio, mining, Powerplay, GalNet/media as supported. Existing client pages use a compatibility adapter. Don't deprecate legacy until tests and manual acceptance show parity for implemented pages.
- Screens may be placeholders during internal development but must be correctly labeled and must not be released as replacement functionality.

### Stage G3 — Direct Go provider correctness
- Inara, EDSM and EDDN conform to current official event formats with immutable fixture tests, rate limits, consent, durable outbox, non-lossy retry and commander identity.
- CAPI must validate its registered client identity, OAuth/PKCE callback, per-CMDR profile and secure refresh token handling on both OSes. Keep provider opt-in independent.
- Correct exobiology source-rule mismatches and false certainty, ship catalog/pad sizes and Powerplay API behavior before a stable release.

### Stage G4 — Windows/Linux release candidate
- Package native signed/verified Windows and supported Linux builds. Include straightforward journal discovery, first-run onboarding and same-device privacy controls.
- Run full automated tests on Windows and Linux, mock network for all CI, manual real-game and laptop/device acceptance, cross-platform native window and Proton paths. Compare actual resource usage to baseline; disclose any regression.
- Migrate preferences and secrets with backup/rollback, never automatically erase old installation. Release notes must distinguish fully supported modules, platform-specific limitations and future work.

### Stage G5 — New first-party modules
- Only after parity/stability: Spansh neutron routing (neutron safety, jumprange, PC clipboard), BGS, more colonisation/carrier/trading features, overlay modules and optional trusted data-pack delivery.

**Stop conditions:** no release if credential leakage, unauthenticated client access, duplicate uploads, loss of journal events, broken existing feature parity, missing Linux executable startup, untested installer or unverified external asset integrity.

## Performance benchmark design

Measurement must include *total* app resource footprint: Go process, WebView2/WebKitGTK renderers, background helpers and laptop browser, not just the Go heap. Compare (A) original DAEDALUS, (B) migrated DAEDALUS, and (C) original DAEDALUS + EDMC plugins for equivalent tasks.

Capture: cold start, steady idle, idle with 2 browsers, replay at controlled event rate, 99th-percentile event-to-screen latency, CPU, process-group resident/private memory, network bytes/requests and file watcher count. Test module enable/disable cost and repeated reconnects; stress bounded memory and queues. Do not promise precise resource savings before data exists.

## Security, licensing and operational safeguards

- Cross-platform HTTPS pairing, authenticated websocket RPC, least-privilege roles, same-origin/cookie controls, safe local bind defaults and no unsanitized preference/token exposure.
- Separate read-only queries from actions affecting OS/game/remote endpoints; explicitly grant PC clipboard copy only on a user click with a validated system name.
- Third-party official API contracts are authoritative, EDMC implementations references; all source/data imports require attribution and compatibility review, retaining GPL obligations.
- Native audio and game voiceline redistribution rights, dependency advisories, pinned update/asset hashes and installer integrity remain public-release blockers from earlier audits.
- Data loss prevention: backup state before migrating preferences, preserve old data until successful rollback-tested import; one commander/FID partition, no auto-enabling uploads.
- Development Windows binary and Linux binary CI must both be green prior to declaring any stage 'complete'; a compile alone does not establish gameplay support.

## Important implementation decisions delegated to engineering

- Exact Go package/module layout and embedded persistence engine after filesystem and cross-platform benchmarks.
- Wails window API specifics and hosting integration for current static React frontend.
- Select a modern, maintained websocket and file-watch library, or use standard library appropriately; keep dependency count low.
- Supported Linux packaging format order after build-matrix validation; no arbitrary claims of universal distro support.
- Concrete performance thresholds after reproducible baseline data.
- Feature migration order within G2, but do not silently drop the working media/engineering/mining pages. OS-specific helper gaps must be separately recorded.

## Approval checkpoint

The maintainer has already selected: **native Linux + Windows desktops, Go-first migration beginning now, retain the already working second-screen interface, prioritize resources and reliable APIs, and postpone new plugins until the runtime is sound**.

This doc is the proposed detailed implementation boundary. Maintainer approval of this updated document is required before Codex starts editing executable/application code. After approval, produce a short PR-by-PR Codex implementation plan with milestone G1 stopping after a tested cross-platform desktop skeleton. Do **not** ask Codex to reimplement all feature categories in a single large task.

## References

- Wails stable status: https://wails.io/status/
- Wails desktop compatibility: https://wails.io/faq/
- Wails installation and Go / Linux requirements: https://wails.io/quick-start/installation/
- EDMC Linux journal docs: https://github.com/EDCD/EDMarketConnector/wiki/Installation-%26-Setup
- ED journal format: https://elite-journal.readthedocs.io/en/latest/File%20Format.html
- Current Daedalus BUILD.md: https://github.com/Jakoczig/daedalus/blob/main/BUILD.md
- Current corrected security branch: https://github.com/Jakoczig/daedalus/pull/2
