# DAEDALUS — Performance-First Native Module Architecture

**Date:** 2026-11-12  
**Status:** Proposed design for maintainer review; architecture direction selected, detailed design not yet approved  
**Repository:** https://github.com/Jakoczig/daedalus  
**Related work:** [Phase 1A security PR #2](https://github.com/Jakoczig/daedalus/pull/2), [maintenance-first design PR #1](https://github.com/Jakoczig/daedalus/pull/1)

## 1. Purpose and user intent

The maintainer runs Elite Dangerous on a Windows PC and uses DAEDALUS on a secondary laptop as a live, interactive second screen. The goal is to replace the need for several concurrently running companion applications, specifically EDMarketConnector (EDMC) and useful plugins, while supporting community users and retaining credit to original developers.

The maintainer's deciding preference is **lower runtime CPU, memory, resource use and friction**, not literal Python-plugin compatibility. The selected approach is to **implement/port proven plugin functionality as native DAEDALUS modules in the existing application**, with source/project attribution, rather than bundle EDMC and start a separate Python plugin host. If another solution benchmarks better for a specific task, measurements may justify an exception. A JavaScript rewrite is not *inherently* faster than Python; eliminating duplicate application services, journal watchers, WebSockets, network calls and process startup is the principal expected benefit.

In DAEDALUS, “native” means modules written for its current Node.js backend, React/Next.js frontend and, only where necessary, Go/Windows host launcher—not that all logic must be in a machine-code binary or that Go must replace Node.

## 2. Goals

1. One Windows app installation and one authoritative game journal/JSON ingestion pipeline. Remote laptops/tablets use the existing browser UI without running EDMC.
2. Modular capability selection: enable/disable modules, hide/show navigation and dashboard widgets per device, and optionally select role-specific layouts (Exploration, BGS, Mining, Trading, Combat). Disabled modules should consume negligible **incremental** CPU, RAM and network bandwidth.
3. Reproduce/upgrade useful EDMC ecosystem workflows, not necessarily their UI or source code. Prioritize neutron/Spansh routing and navigation handoff, exobiology, BGS/Powerplay tally, overlay capabilities and engineering.
4. Keep existing work until functionally replaced and tested. Integrations and exobiology remain correctness-first.
5. Easy discovery of each module's purpose, source inspiration, original authors, license, changelog, permissions, update and data provenance.
6. Keep user-facing second-screen functionality practical: minimal input on the gaming PC, efficient data updates, configurable page layout and no unnecessary polling.

## 3. Non-goals and anti-goals

- **No EDMC or Python runtime dependency** in the standard Windows DAEDALUS installation.
- No universal Python-to-JavaScript byte-for-byte plugin compatibility. “One click install any GitHub repo” is *not* a near-term goal.
- No auto-execution of arbitrary downloaded plugin JavaScript/Python, and no unattended software installation from unverified GitHub releases.
- No rewriting the entire existing service, application, React stack or Go launcher just to create the module API.
- No simultaneous duplicated bio prediction algorithms that each run against the same journal data in background.
- No automatic in-game injection, uncontrolled OS clipboard access or host media controls from any unpaired browser.
- Do not equate a hidden module with a disabled module; separate visibility from processing.

## 4. Approaches considered and selected

### A. Run EDMC and Python plugins embedded
Reuse existing plugin binaries and UI integrations, gaining rapid feature compatibility, but adding a Python runtime, process lifecycle, duplicate journal readers/data stores, and often UI that does not translate to web. Complexity and IPC overhead work against the one-app goal. **Rejected as default**.

### B. Copy all plugin logic directly into the existing event handlers
Avoids external runtime but increases the current monolith's coupling, makes modules hard to disable, and is likely to leave unnecessary timers and memory resident. **Rejected**.

### C. Native DAEDALUS capability modules on a shared event bus
A small module registry plus native Node/React adapters uses existing service and host runtime, can avoid duplicate watchers and lazy-load UI/service work. Copying/modifying other projects' source code is optional, and only subject to license and attribution review; functional behavior can be independently reimplemented from public specifications and test fixtures. **Selected**.

## 5. Runtime architecture

### Host processes

- Keep the existing Windows launcher (Go/WebView2) and one Node service.
- The service alone reads Elite journal lines plus Status/Cargo/Materials/NavRoute and related JSON files, normalizes source events and maintains authoritative state.
- Run *one* central ingestion pipeline and *one* state-cache/outbox service shared by all active modules. The Node service is not responsible for rendering React views.
- Each secondary laptop/tablet is a browser client; the number of browser clients must not multiply journal file reads, API uploads or expensive background calculations.

### Module manifest (proposed)

Each bundled native module has a declarative manifest similar to:

```json
{
  "id": "navigation.neutron",
  "name": "Neutron Route Planner",
  "version": "0.1.0",
  "category": "navigation",
  "defaultEnabled": false,
  "dataNeeds": ["journal", "navroute", "ship.fsd"],
  "subscriptions": ["FSDJump", "StartJump", "Location", "Loadout"],
  "capabilities": ["spansh.read", "host.clipboard.request"],
  "credits": [
    {"name": "Spansh", "url": "https://spansh.co.uk/"},
    {"name": "EDMC-NeutronDancer", "url": "https://github.com/dwomble/EDMC-NeutronDancer"}
  ],
  "licenseNotes": "Own source plus identified third-party notices"
}
```

This is a target interface, **not an implemented schema**. Actual API interfaces will be versioned and tested before adoption. No private API keys or arbitrary filesystem paths appear in manifests.

### Lifecycle and data flow

- Module registry resolves built-in modules, dependencies and capabilities at service startup without initializing disabled modules.
- On enable: validate permissions/dependencies, hydrate only needed stored state, register event subscriptions, optionally start one scheduled task with explicit cadence, and supply initial UI snapshot.
- On disable: unsubscribe event handlers, cancel pending timers/network requests, release caches/workers, leave only minimal configuration persisted, and prevent further outbound network requests. Track residual allocations in tests/benchmarks.
- Event hub fans out immutable, typed game-data events, and may precompute shared derived facts (e.g. current system, ship identity and FSD range). One module may not mutate another module's journal entries.
- Calculate incremental deltas from each journal line, not recompute all stored history at each 250 ms poll. Expensive datasets (biological rules, galaxy data) are loaded on demand and cached within explicit memory bounds.
- Frontend uses route/module-level lazy loading and unsubscribes/pauses animations, media and background polling when the corresponding panel is not visible.
- Prefer request-scoped HTTP clients with deduplication, bounded concurrency, caching and published API rate limits. Shared provider infrastructure remains independent of the UI.
- The event bridge supports an authorized read-only display mode by default; any host-impacting action requires a distinct, audited, user-granted capability. Phase 1A's owner/viewer security boundary remains the baseline.

### Settings: three independent controls

1. **Available/bundled:** Native core capability code ships with the app. Most small modules need no separate download. Large optional data packs can remain separately downloadable later.
2. **Enabled:** Determines whether the backend performs subscriptions, calculations, uploads or timers. Integration upload requires independent explicit opt-in.
3. **Visible on this device:** Controls navigation items, pinned cards and dashboards, with per-device configurations. Hiding does not stop background processing when the user intentionally wants collection/tracking to continue.

Profiles for Exploration, Mining, Engineering, BGS and Trading merely set view preferences; the user may override every component. Per-CMDR state and account separation are critical for shared computers. The PC should expose owner-controlled module enablement, while remote devices can change local layout preferences but not escalate server privileges.

### Module credits page

For every capability, display: what it does, author(s), source/inspiration links, upstream project version or source commit if data/logic is imported, license/NOTICE and compatibility status, data collected/uploaded, host/device permissions, change history, and any known accuracy limits.

**Always separate** “inspired by/reference” from “bundles code” and “uses data”. Credit and license metadata must survive updates and public redistributions.

## 6. Update and distribution design

- **Native bundled modules:** Update together with signed/verified DAEDALUS releases. Check on startup if the user wishes, but never execute an unverified update. Provide user-visible release notes, automatic rollback and no unexpected game interruption.
- **Data-only packs:** A future minimal trusted catalog may update bio rules, game data and static reference lists using pinned versions/hashes and schema validation. Do not blindly import a new SrvSurvey rule or generate false “100% confirmed” assertions.
- **Optional separately distributed native modules:** Defer until there is a versioned API, sandbox/permission model, artifact signature verification, rollback, deactivation and compatibility-testing system. No arbitrary GitHub URL installer. Per-module version management is unnecessary for first-party modules already bundled.
- External service integrations must honor consent, rate limits and secrets isolation independently of module installation and enabled/visible state.

## 7. Planned modules and source references

| Native module (proposal) | Existing baseline | Reference projects / notes |
| --- | --- | --- |
| Navigation and system map | Implemented | Retain current route/map/list; correct outdated game data. |
| Neutron/Spansh route planning | Placeholder | Spansh, EDMC-NeutronDancer, EDMC-SpanshTools. Route editing, progression, fuel checks, next-system copy workflow. |
| Exobiology | Substantial implementation, inaccurate data/labels | SrvSurvey, EDMC-BioScan, BioInsights. Single prediction engine with provenance, matching evidence and uncertainty. |
| Engineering/materials | Substantial implementation | EDMC-Engineering, EDOMH; shared server-side wishlist, current ship/blueprint catalogs. |
| BGS / Powerplay / Colonisation tally | Partly implemented Powerplay; rest largely placeholders | BGS-Tally and EDMC-PowerPlayProgress; start from current journal events without EDMC. |
| Ship/fleet/carrier | Status/cargo implemented, others placeholders | EDMC/Frontier CAPI only where permitted, journals as authoritative, per-commander cache. |
| Mining | Four implemented panels with some bugs | EDMC-Mining-Analytics, limpet timers, current mining handler. |
| Overlay | Native landing-pad overlay; others absent | EDMCModernOverlay is a reference, not a runtime dependency. Use the existing Windows windowing/UI where feasible; measure GPU/CPU impact. |
| General media, voices, GalNet, controls | Implemented, others partially | Modular opt-in with safe host control permissions and optional download assets. |
| Community trade / missions / CGs | Mostly placeholders | Public authorized APIs and clear data-freshness labels. |

The selected project list is non-exhaustive. Each planned feature requires a separate design/verification exercise before implementation.

## 8. Next-system copy on PC: security constraint

The secondary-laptop experience should support a click to request “Copy next waypoint on gaming PC” so the player can paste into the Elite Dangerous galaxy map without manually retyping. The clipboard belongs to the **PC**, not the laptop. This requires a narrowly scoped host capability (trusted device / per-device permission grant, restricted to a validated star system string) and an explicit click. It cannot be implemented by simply allowing read-only viewers to call arbitrary RPCs or OS shell commands. The action must never silently send keypresses to the game.

This is future scope. It should not be added as part of Phase 1A security or Phase 1B Inara.

## 9. Performance validation and operational checks

Before migrating features, record baseline measurements on the **same Windows gaming PC and secondary laptop**:

- PC Node service process RSS/private bytes, CPU while idle, CPU during jump-heavy journal replay, event-to-UI latency, startup time, number of file watchers, HTTP/API call rate, network bytes and queued tasks.
- Laptop browser JS heap, CPU/frame rate, first-page interaction, off-screen polling/animation, number of WebSocket requests, and reconnect time.
- Compare equivalent functionality against **current DAEDALUS** and **DAEDALUS + EDMC plus target plugins**; observe whole process groups (including Python, helper processes, WebView2 renderer and browser), not a single Node PID.
- Repeat with modules disabled, individually enabled and all enabled, with one and multiple viewers. Ensure there is no doubling of journal watchers, duplicate uploads, unexpected idle polling or unbounded caches.
- Acceptance: no material regression in core idle CPU/memory or journal-to-screen latency; disabled module adds no recurring work or network requests; tests verify subscription cancellation, lazy loading, bounded queues and source fixture parity. Set numeric targets after collecting a reproducible baseline, not by unsupported guesswork.
- If evidence shows one hot-path algorithm is unsuitable in Node, optimize it with profiling and consider a small Go helper that reuses the existing launcher/runtime. Do not add a whole second general-purpose plugin host.

## 10. Execution order and coexistence with maintenance work

1. **Finish existing Phase 1A / 1B and integrations/data audit.** Do not introduce the new module runtime into the unfinished security/Inara PRs.
2. **Module discovery/visibility increment:** inventory existing handlers and pages, add feature IDs, navigation filters, per-device visibility profiles and owner-controlled enable/disable state. This is additive and backwards-compatible.
3. **Runtime isolation increment:** central subscription registry, lazy init/deactivation, test and benchmark instrumentation. Convert only one noncritical existing feature before migrating others.
4. **Feature delivery:** neutron/Spansh navigator, improved exobiology, BGS tally/colonisation, and optional overlay/engineering improvements, in separate independently reviewed PRs.
5. **Optional signed packs / third-party extension SDK:** only when stable modular APIs and security review exist.

Any cleanly functional existing page remains visible by default until its replacement is tested and the maintainer has decided the desired default. No placeholder must ship as a claimed functioning module.

## 11. Acceptance criteria

- A single local journal ingestion stream serves all modules and connected devices.
- Disabling a module prevents new subscriptions, periodic jobs and uploads, with corresponding automated tests.
- Hiding modules on a laptop changes only that laptop's layout; desired background tracking on PC continues.
- No EDMC/Python processes are required for native implementation; native tests remain offline by default.
- The app starts and all existing pages function without activating future modules.
- Each module displays source credits and appropriately distinguishes copied code, imported data and inspiration.
- Provider permissions and Windows host actions are constrained by Phase 1A authorization and per-device grants, not by frontend buttons alone.
- New optional features pass memory, CPU, latency, and double-ingestion tests; comparisons are documented and reproducible.
- Third-party code updates cannot silently execute from untrusted GitHub repositories.
- Public releases pass previously defined integrity, dependency, attribution, device and authenticated API gates.

## 12. Deliberate unresolved choice

**Proposed default:** bundled native modules with enable/disable and per-device visibility; startup **update check** for the application, not automatic execution of unverified module code. A curated signed optional-package system can be designed later if the community specifically benefits from independently downloadable large modules.

This design document describes **future architecture**. It is not authorization to merge, release, turn on external uploads, or start broad feature implementation before the maintenance foundation is accepted.
