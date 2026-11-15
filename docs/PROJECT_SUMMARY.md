# DAEDALUS project summary

Updated 2026-11-15. This records the community fork and native migration through combined G2H/G2I, including the PR #13 EDAstro correction. The original Node/React application and Windows launcher remain available and intact; the native application is experimental. No release, deployment or merge is authorized by this work.

## Development history

| Stage / PR | Work delivered |
|---|---|
| [Phase 1A #2](https://github.com/Jakoczig/daedalus/pull/2) | Owner/viewer authorization, authenticated secondary screens, server-side restrictions and security regressions; corrections preserved previously working viewer pages without granting host operations. |
| [Architecture #3](https://github.com/Jakoczig/daedalus/pull/3) | Approved performance-first Go migration design and staged implementation specifications. Phase 1B documentation was scope work, not a claim that its integrations shipped. |
| [G1A #4](https://github.com/Jakoczig/daedalus/pull/4) | Go/Wails Windows and Linux foundation, embedded React assets, authenticated loopback HTTP/WebSocket service, honest offline bootstrap without a Node backend. Corrected connection/ready indicators and reconnect regressions. |
| [G1B #5](https://github.com/Jakoczig/daedalus/pull/5) | Optional remote HTTPS, private certificate authority and trust instructions, five-minute one-use six-character pairing, global rate limiting, device management and immediate revocation. Invalid remote state disables remote access while preserving the local desktop, damaged files and exclusive profile locking. Experimental CI artifacts/checksums. |
| [G2A #6](https://github.com/Jakoczig/daedalus/pull/6) | One native journal pipeline: Windows/Steam Proton discovery, historical replay, live updates, rotation, truncation, partial lines and sidecars. Exact 64-bit identifiers and commander separation; typed public status/current-system data. Deterministic late-discovery and viewer-revocation tests. |
| [G2B #7](https://github.com/Jakoczig/daedalus/pull/7) | Real navigation routes, next jump and current-system exploration. Conservative NavRoute provenance, including changed content with unchanged timestamps; stale commander/system data is not silently promoted to verified. |
| [G2C #8](https://github.com/Jakoczig/daedalus/pull/8) | Legacy exploration map/body information, rings/composition, confirmed biological signals/genus/species and sampling workflow; expanded route/exploration UI and bounded caches. |
| [G2D #9](https://github.com/Jakoczig/daedalus/pull/9) | Restored original visual language and layouts, ship information and reusable original assets; expanded native parity while labeling unported features. |
| [G2E #10](https://github.com/Jakoczig/daedalus/pull/10) | Live ship instruments/status flags/pips, modules and engineering inspector, cargo/material inventory, local engineering planner, commander/mission/mining journal information. |
| [G2F #11](https://github.com/Jakoczig/daedalus/pull/11) | Readability/layout corrections and optional cached read-only EDSM system-map enrichment. Unknown module values remain unknown. |
| [G2G #12](https://github.com/Jakoczig/daedalus/pull/12) | Native companion/tray and independent dashboard lifecycle, compact ship UI, map Fit/catalogue, route/scoopability facts, illustrative multi-projection galaxy map, owner EDSM lookups and per-device jump-follow/manual display controls. |
| [Combined G2H/G2I #13](https://github.com/Jakoczig/daedalus/pull/13) | Orbital tree layout and compact body inspector, comprehensive journal-backed Ship Statistics with explicit unsupported calculations, improved galaxy framing/layers, unified bounded provider architecture, protected credentials and supported optional Inara/EDSM/EDAstro/EDDN/CAPI adapters. |

## Current behavior and boundaries

The Go application owns one journal reader regardless of browser count. Historical replay reconstructs local state; it does not silently upload historical journals. Public secondary-display projections explicitly allowlist fields. Owner bootstrap, credentials, integration configuration, upload authorization and native actions remain unavailable to viewers. Revocation invalidates sessions/connections. Remote access is disabled by default and requires HTTPS/trust setup; a second process cannot become another owner.

Exploration, navigation, ship, cargo/inventory and related journal views are useful local capabilities. Missing or stale measurements remain marked as such. The galaxy background is illustrative, not a measured density dataset. External enrichment is supplemental and does not overwrite journal provenance. The legacy application is preserved rather than replaced.

| Integration | Supported scope / remaining acceptance |
|---|---|
| EDSM | Optional cached system/body/station enrichment; separately consented account flight-log readback and selected journal uploads. Live account acceptance remains manual. |
| Inara | Profile reads and selected official travel/loadout/material/rank/credit events. Application whitelisting is required; gated and fixture-tested until approved. |
| EDAstro | Explicit system and rare-GEC reads, selected biology/carrier submissions with independent consent. Actual public GEC format verified; live submissions not performed. |
| EDDN | Opt-in schema-validated journal/1 public observations. No general historical search, permanent subscription or market/outfitting/shipyard publication. |
| Frontier | Approval-gated read-only CAPI with OAuth PKCE, expiring one-use state, exact loopback callback and refresh. Developer credentials/callback registration and live authentication remain prerequisites. No game commands. |

Uploads start disabled and require provider/category/commander consent. Windows DPAPI and Linux Secret Service protect saved credentials without plaintext fallback. Requests, queues, retry behavior, accepted endpoints and retained data are bounded. No production upload or live provider authentication is implied by fixture tests.

## PR #13 review correction

The external review correctly identified a fabricated GEC fixture: published records have a numeric `coordinates` array, with string `type` and `type2` categories. The correction parses that contract, retains only allowlisted point fields, uses `galMapSearch` for system inspection, and labels nebulae found in either category.

Public read-only measurements on 2026-11-15 found `/gec/json/all` at **2,274,159 bytes** (660 records), exceeding the unchanged **2 MiB** response budget. The documented `/gec/json/rare` endpoint was **429,722 bytes** (134 records). The application now explicitly uses this smaller subset, retains at most 2,048 normalized points, and caches it in memory for 24 hours. Only an owner-authorized request fetches it; no startup/viewer fetch or unlimited full-feed fallback is introduced. Restart loses the cache. If the rare feed outgrows the budget it fails closed. Public documentation: https://edastro.com/api-details.html.

EDAstro JSON status 401 inside a successful HTTP response means no new observations. It is displayed/counted separately and does not increment confirmed successful uploads. HTTP authentication failures remain errors. Regression tests cover the published structural fixture, category attribution, invalid coordinates, expiry/consent, response budget and submission classification.

## Verification and remaining practical acceptance

Before the targeted correction, reviewed head `31fb7b6c398785ebf1accbb6114a9ec7ad5b67ff` passed [Go desktop CI](https://github.com/Jakoczig/daedalus/actions/runs/43633239467) and [legacy/security CI](https://github.com/Jakoczig/daedalus/actions/runs/43633239431): Windows/Linux native builds/smoke tests, 115 JavaScript and 18 asset tests per OS, Linux race detection and browser visual/interaction checks. These are prior-head results. The correction's exact final commit, new CI runs and downloadable artifact/checksum evidence are recorded in the updated PR #13 handoff; prior artifacts must not be presented as corrected builds.

The maintainer confirmed real Windows-to-laptop HTTPS pairing and G2A real-game automatic updates. This does not establish physical Linux behavior, every later calculation or live provider authentication/upload correctness. Those remain separate manual checks. Unsupported flight/shield/resistance calculations, neutron optimization, game commands and unspecified API capabilities are not claimed to work.

One consolidated gameplay checklist:

1. Start the native preview alongside the preserved legacy installation; check ship instruments/modules, cargo/inventory and journal freshness during actual play.
2. Jump/scan/sample and switch commander: verify route provenance, orbital bodies, biography/sample tracking and unknown/stale indicators on desktop and paired laptop.
3. Exercise manual map framing/body/system selection; explicitly enable EDAstro reads and inspect the labeled rare-GEC cache and selected system.
4. Test idle/reconnect/revocation and shutdown; verify physical Linux tray/keyring/TLS behavior where available.
5. Test each approved provider separately with explicit consent and disposable acceptance data; inspect no-new/error/confirmed outcomes without assuming all capabilities are live.

## Source handoffs

Earlier implementation evidence is in [Phase 1A](handoffs/phase-1a-security-and-tests.md), [G1A](handoffs/g1a-go-wails-foundation.md), [G1B](handoffs/g1b-secure-remote-displays.md), [G2A](handoffs/g2a-go-journal-ingestion.md), [G2B](handoffs/g2b-native-navigation-exploration.md), [G2C](handoffs/g2c-exploration-parity.md) and [G2D](handoffs/g2d-original-ui-parity.md). Later concise preview matrices and acceptance instructions are in [G2E](testing/g2e-feature-parity-preview.md), [G2F](testing/g2f-usability-preview.md), [G2G](testing/g2g-navigation-launcher-preview.md) and [G2H](testing/g2h-orbital-integrations-preview.md), with exact-head delivery evidence in the linked PRs.

## G2H gameplay feedback follow-up

The maintainer's `2bea5d8` UI correction was retained and finalized: intentional journal waiting state, separate known-empty/filter-empty states, stable selectable nonspatial EDSM station markers with cache provenance, and scrollable/focusable native launcher settings. Historical lookup results cannot leak from a previously requested system into the next request. Browser fixtures include live startup transitions, binary/local/community bodies, 64 stations and small/scaled launcher layouts. The actual Windows Wails window was inspected through Data Providers to its lower settings with all integrations disabled. See the [G2H preview notes](testing/g2h-orbital-integrations-preview.md) and PR #13 for final-commit verification/downloads; physical gameplay and additional DPI/Linux device acceptance remain manual.


## G2J — persistent history and controlled integrations

The new G2J branch starts at G2H `d7c5cd93e16ee0480fa11ef53d2ea9d9d0d4cb58` and leaves PR #13 and the legacy application unchanged. A private native SQLite WAL database now retains normalized relevant journal history, verified durable source checkpoints and cached reducers. It supports incremental restart catch-up, whole-file verified copy aliases, backups before migrations, manual export/restore and conservative recovery without deleting damaged originals.

Owner-only Data/History exposes indexed per-commander pages, provenance/conflicts, construction depot resources and explicit historical-publication review. Persistent Never/Live/After sale policies, independently consented recipient/category live exceptions and durable outcomes apply before provider queues. Paired displays retain bounded read-only public projections. New reference adapters cover published Spansh lookups, bounded Elite BGS, Raven read-only projects and filtered Canonn science collections; credentials remain outside SQLite. Native engineered statistics preserve final fitted modifiers and withhold unvalidated performance models.

The [G2J preview matrix and acceptance checklist](testing/g2j-persistent-history-preview.md) distinguishes fixture implementation from approval-dependent or unverified live capabilities. Exact final-head CI/artifact evidence belongs to the new G2J draft PR; earlier milestones' green checks do not verify G2J. No production upload, release, merge, deployment or legacy replacement is authorized by this work.
