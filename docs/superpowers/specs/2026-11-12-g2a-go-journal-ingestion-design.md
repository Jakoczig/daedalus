# DAEDALUS G2A — Cross-Platform Go Journal Ingestion

**Date:** 2026-11-12  
**Status:** G2A-specific design derived from the maintainer-approved Go-first architecture and 2026-11-12 authorization to continue after G1B acceptance  
**Parent design:** [Go-first Windows/Linux rebuild](./2026-11-12-go-first-windows-linux-rebuild-design.md)  
**Dependency:** G1B [PR #5](https://github.com/Jakoczig/daedalus/pull/5) (secure remote displays), G1A PR #4 and Phase 1A PR #2  
**Implementation plan:** [G2A incremental plan](../plans/2026-11-12-g2a-go-journal-ingestion.md)

## Decision / intent

G1B's six-character pairing and real Windows-to-secondary-laptop connectivity passed maintainer acceptance. Build the first *real Elite Dangerous capability* in the Go/Wails Windows/Linux application: find the game's journal directory, ingest historical and live journal lines plus selected JSON snapshots, reconstruct a minimal real-game state, and show that state safely on the owner desktop and authenticated second-screen browser.

The user wants one efficient local processor that later feeds exploration, navigation, BGS, exobiology, ships, mining and external integration modules. Preserve the old Node game companion unchanged. **Do not turn on Inara, EDSM, EDDN, CAPI, Spansh, game controls or file uploads in G2A.**

## Scope and alternatives considered

**A. Port all Node handler logic into Go immediately:** rejected. It would mix ingestion, UI feature migration, provider correctness and persistent historical queries in one unreviewable change.

**B. Build one Go journal/JSON ingestion engine and minimal live status before migrating handlers:** selected. Separates correctness of source tracking, file identity, replay and privacy from game feature behavior; supports performance tests and keeps G1B pairing intact.

**C. Use a Node/EDMC subprocess as a data bridge:** rejected for duplication of runtime, watchers, memory and operational risk.

### G2A delivers
- Discover journals on Windows and Linux/Proton, including custom Steam libraries and explicit user override.
- Stream journal JSONL without unbounded RAM, process completed newline-delimited records, survive journal growth, part rotation, same-path replacement, truncation, partial final lines and restart.
- Track safe source provenance and full 64-bit values without JavaScript rounding; read and reconcile selected complete JSON snapshot files atomically.
- One service reading source files for all authorized browser clients, bounded state/subscribers, non-blocking/cancelable shutdown.
- Small actual-data dashboard: available/unavailable source, last event type/time, current star system and import status (show clear "not found" / "waiting for game" states).
- Deterministic synthetic replay and full Windows/Linux CI with no provider calls.

### Deferred to G2B and later
- Complete navigation/system/ship/exobiology/mining/engineering UI parity and historical catalog/database.
- Upload provider adapters, network refresh, material traders, route plotter or native overlays.
- Automatic setting migration from old Windows installation; full scanner for arbitrary user directories; Cloud sync.
- Broadcasting raw journal records or chat messages to tablet; enabling unsafe host actions.
- New unrestricted remote filesystem RPCs or browsing; native graphical picker if platform constraints prevent secure use in G2A (provide a local-only text field/CLI and document deferred picker).

## Real source contracts

Frontier's journal consists of `Journal.<datestamp>.<part>.log` with one JSON event per completed line and an initial `Fileheader`/`fileheader` record containing game/build context. Files can rotate by increasing part number; timestamps may repeat, be missing or be out-of-order. Parse by source location/offset, **not timestamp uniqueness**.

Selected JSON snapshot files initially: `Status.json`, `Cargo.json`, `Materials.json` and `NavRoute.json`. Files are whole-file replaced or rewritten; accept only complete, valid JSON. Unknown `*.json` and market/provider files are not automatically incorporated. Sidecar freshness/staleness must not be silently confused with current state.

Windows discovery: call the Windows Saved Games Known Folder (`FOLDERID_SavedGames`, not a fixed `C:\Users` literal), then append `Frontier Developments/Elite Dangerous`; detect folder absence clearly and allow explicit override. Linux discovery: bounded check of common Steam native and Flatpak roots and library folders in `libraryfolders.vdf` for app ID `359320`, their `compatdata/359320/pfx/drive_c/users/*/Saved Games/Frontier Developments/Elite Dangerous`; support custom Steam libraries, Wine/Heroic prefixes via owner override. Do not walk all drives or assume `steamuser` in every environment. Provide owner-configurable directory in Go's separate private profile; an invalid override is a visible error, not a reason for the desktop to refuse launch.

Sources: Frontier journal [format](https://elite-journal.readthedocs.io/en/latest/File%20Format.html), [Status](https://elite-journal.readthedocs.io/en/latest/Status%20File.html), EDMC [journal paths](https://github.com/EDCD/EDMarketConnector/wiki/Installation-%26-Setup), Microsoft `FOLDERID_SavedGames` [documentation](https://learn.microsoft.com/en-us/windows/win32/shell/knownfolderid). Use Node's existing `src/service/lib/elite-log.js` and `elite-json.js` as behavioral references only, not unvalidated sources of truth.

## Backend boundaries and replay semantics

Add focused packages under existing `go/` module:
- `internal/journal`: discover the directory; parse/validate event envelopes; incremental tail from source file identity + byte offset; extract needed normalized fields with `json.Decoder.UseNumber`.
- `internal/sidecar`: read complete snapshot and detect content changes from a whitelist; atomic swap or incomplete write never replaces last known valid state.
- `internal/gamestate`: minimal aggregate of current system and source health with per-commander identity partitioning; bounded immutable read snapshots.
- `internal/webapp`: attach **one** service and safe WebSocket read-only RPCs, separate owner-only source-folder change with existing Origin/owner-session checks.

Start with deterministic historical backfill and then live tail. Historical ingestion reconstructs current state but does not get re-broadcast to clients as a stream of *new* live events. Live completed records should be delivered once per source identity/offset even on repeated polling. Restart may replay historical events to reconstruct state; it must not trigger any external side effects. **Exactly-once external delivery is deliberately out of scope** and must not be claimed.

Use bounded streaming, record length cap (recommended 1 MiB, document any exceptions), bounded per-file progress, backpressure and cancellation. Do not trust timestamp as dedup key. Skip/log only anonymized diagnostics for malformed complete lines; partial trailing JSON is buffered until newline or file change. Watch active file rotation even if game is not running at startup. A source unavailable at launch should be detected when created; reconnect UI after game starts without restarting the app. On stop, close watchers/timers and do not leave goroutines or duplicate readers.

Game ID fields such as `SystemAddress` / `MarketID` may exceed 2^53; parse exactly (Go `json.Number` or 64-bit int) and **serialize as decimal strings** when crossing the JavaScript browser boundary. Internally preserve exact values. Current system changes only upon arrival/known location events (`Location`, `FSDJump`, supported carrier jump) rather than `StartJump` (departure intent).

Commander identity: derive from `LoadGame` FID where present, keep per-FID aggregation separate; when FID is absent, mark identity unknown and avoid merging unknown and known commander data. Do not put FID or commander name in remote viewer payloads or logs.

## Security and UI

G1A/G1B's owner and viewer listeners, origins, pairing, revocation, cookie security, secret redaction, private Go profile and fail-closed recovery remain unchanged. A paired viewer may ask only for a narrow **sanitized** live system/status projection (current star system name and last non-sensitive event category/time) after normal session validation. No raw journal blobs, chat events, logs, full paths, FID, tokens, material inventories or visited-system history in G2A network responses. Return owner-only folder status/path only to the authenticated localhost owner; the backend alone reads the files.

Keep existing React routes and navigation but visibly label all unported pages as unfinished; add a small Go-powered current-system/ingestion card to the limited preview shell, using proper loading, game-not-running, source-missing and permission-error states. Do not confuse "Go server ready" with "journal source present": they are independent status fields. Avoid displaying an invented commander or system when no data has been received.

Owner-only folder override can be a validated local text input at G2A, together with CLI `--journal-dir` for developer smoke fixtures. Use native picker only when a secure, tested Wails cross-platform API is proven; a remote viewer **must never** provide arbitrary file paths to be read by the host. Persist override in dedicated protected storage, not legacy preferences. Never write to the game's journals or sidecars.

## Verification / acceptance

1. **Windows and Linux discovery:** simulated Windows moved Saved Games path; default Steam, Flatpak Steam, extra Steam library root and custom Wine prefix; absent folder; invalid custom override.
2. **Journal integrity:** complete lines, CRLF, UTF-8 names, partial trailing JSON, malformed complete JSON, duplicate timestamps, >2^53 IDs, Fileheader, fresh append, rotation with new part, same-name replacement/truncation and restart. Exactly one live event per completed source-position; no real player records in repo.
3. **Sidecar integrity:** Status/Cargo/Materials/NavRoute valid replacements, temporarily malformed/truncated file, empty/missing file, stable content not re-emitted, stale file recovery and file-size caps.
4. **Commander separation:** multiple LoadGame/FID fixtures, unknown identity, no cross-commander status contamination, no premature jump-arrival state.
5. **Transport/security:** owner and viewer receive only approved current-system/status fields. Unknown/privileged RPCs rejected, no raw ReceiveText/chat/FID/path leaks, revocation closes active viewers, no extra journal reader when second device connects.
6. **Operational:** deterministic fixture-driven integration with startup + late journal creation, stop/restart, timing/journal counts and a realistic larger replay; no unbounded heap or network, no spawn of Node service. Repeat Windows/Linux CI, Linux race detector, React/native window smoke, old legacy/security CI.

Physical acceptance after source/CI: Windows gaming PC discovers actual folder and updates current system on jump; secondary laptop immediately follows through the secure existing paired connection; Linux Steam/Proton game/distro paths when available. Never record or publish a personal journal or FID in a report without sanitizing.

G2A's execution branch must be **stacked on G1B PR #5**, itself stacked on G1A PR #4, itself on security PR #2; all remain draft/unmerged until explicit integration/merge sequencing. G2A does not authorize public release. New downloadable builds are experimental, unsigned, and clearly labeled **live local journal preview (no provider uploads)** instead of "offline" when real journal ingestion is enabled.
