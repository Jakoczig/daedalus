# G2A: Go Elite Dangerous Journal and JSON Ingestion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the new Go/Wails Windows/Linux Daedalus app discover and consume **real local** Elite Dangerous journal/JSON game data and display a minimal, sanitized live current-system status on already-paired secondary devices.

**Architecture:** Create one cancellable source pipeline in Go: OS-specific discovery -> streaming JSONL tail/sidecar snapshots -> per-commander state projection -> bounded read-only WebSocket subscriptions. Keep the existing owner/viewer authorization, Go local server and React preview navigation intact. Do not port large Node handlers or provider integrations yet.

**Tech Stack:** Existing Go 1.26.10, Wails v3.0.0, github.com/coder/websocket v1.8.14, Go standard library (encoding/json with UseNumber; os/fs; context) and current Next/React client. Prefer stdlib polling at a bounded ~500 ms cadence before introducing another watcher dependency; test/benchmark across both OSes.

**Spec:** `docs/superpowers/specs/2026-11-12-g2a-go-journal-ingestion-design.md` on branch `docs/performance-first-native-modules-2026-11-12`; also read parent Go-first spec in the same directory.

## Global Constraints

- G1B [PR #5](https://github.com/Jakoczig/daedalus/pull/5) and its 6-character secure pairing are accepted; user reports real laptop pairing passed. Maintain security, short code, recovery/quarantine, owner-only control, and private state unchanged. The implementation branch `go/g2a-journal-ingestion` MUST start from `go/g1b-secure-remote-displays` and open a **draft stacked PR against G1B**, not main.
- No Node/Python/EDMC process at runtime. Do not modify/delete legacy `src/service` and `src/app` or old credentials. Do not introduce live provider uploads, HTTP lookups, shell/game control or arbitrary remote filesystem reads.
- Existing React 62-route UI remains; only G2A's limited live preview is new. No falsely functional unported game pages.
- All files in `tests/testdata/g2a` must be fabricated, anonymous and small; never commit personal Journal, FID, CMDR/chat, coordinates or credentials. No real data or network use in CI.
- Read ONLY recognized journal filename pattern `Journal.*.*.log` and selected sidecars: `Status.json`, `Cargo.json`, `Materials.json`, `NavRoute.json`. Ignore unknown files and never WRITE to the game folder.
- No silent loss: partial lines remain pending, complete malformed lines produce sanitized parse diagnostics, resets/rotation are observed and replay status explicitly reports failures/incomplete files. Bound I/O per record; do not create unbounded retained journal history.
- Preserve numbers over JavaScript 2^53 safely. `SystemAddress` and `MarketID` cross browser as decimal **strings** (not rounded JS numbers).
- Viewer RPCs return ONLY allowlisted source availability/current system/last-safe-event category/time; owner-only path details and owner-only change operations have existing exact Origin + owner session checks. Never broadcast full journal, ReceiveText, names, FID, raw private JSON, file paths or keys.
- Keep default disabled remote TLS rule; G2A journal ingestion may run locally with zero viewers. One active source ingestion pipeline per Go server, not per web socket. Closing server or changing directory must stop old timers/readers.
- CI Windows + Ubuntu, Linux race, JS/security tests, builds and native WebView smoke must remain required. Artifact label becomes **experimental, unsigned, local-journal preview (no provider uploads)**; do not release, merge, deploy or replace the old Windows app.

## File responsibility map

**Create:**
- `go/internal/journal/discovery.go`, `discovery_windows.go`, `discovery_linux.go`, `discovery_test.go` and OS-specific tests: explicit directory validation, bounded native Windows known-folder and Linux Steam/Proton library discovery.
- `go/internal/journal/parser.go`, `parser_test.go`: line-level JSON parse, source offsets, Fileheader metadata and exact 64-bit values; retain raw internal only.
- `go/internal/journal/reader.go`, `reader_test.go`: chronological historical backfill plus incremental tailing, file identity/offset, rotation, truncation, partial lines, no repeats.
- `go/internal/sidecar/snapshots.go`, `snapshots_test.go`: only four allowlisted JSON sidecars, atomic valid-snapshot updates.
- `go/internal/gamestate/service.go`, `service_test.go`: one cancellable aggregator, per-FID separation, immutable sanitized status, subscription/broadcast.
- `go/internal/webapp/gamefeed.go`, `gamefeed_test.go`: owner-only journal directory configuration, slim RPC and live fan-out with existing roles; optionally extend `go/internal/webapp/socket_test.go` and `server_test.go`.
- `src/client/components/go-journal-status.js` and `tests/service/go-journal-status.test.js`: actual live preview card and folder status/override local owner controls.
- `tests/testdata/g2a/`: synthetic Fileheader, LoadGame, Location, FSDJump, partial/rotation and four sidecar files; if fixture paths differ on Windows, use fs-based fixtures.

**Modify minimally:**
- `go/internal/webapp/server.go`, `socket.go`: initialize ONE gamestate service, read-only status calls/events, stop it on shutdown; preserve existing public StartLocal/WithStorage APIs and G1B authentication.
- `go/cmd/daedalus/main.go`: `--journal-dir` developer/owner override and fixture smoke handling; no game reads in standard disabled-profile smoke mode.
- `src/client/lib/socket.js`: maintain G1A/G1B and legacy client behavior; add G2A preview marker and narrow status event/read.
- `.github/workflows/go-desktop.yml` and `scripts/build/package-go-preview.cjs`: run G2A tests on stacked PR; create correctly labeled experimental artifacts.
- `docs/testing/g2a-windows-linux-journal-smoke.md`, `docs/handoffs/g2a-go-journal-ingestion.md`: OS setup, test evidence and user-safe handoff.

## Review Focus — five user-visible failure classes

1. **No game launched, or journal folder appears later:** desktop must start and show `Waiting for journal`; recover automatically without restart. Task 1 + Task 3/5 tests.
2. **Journal line is being written or journal rotates:** never publish half an event, never lose first event of new part, never publish the same completed line twice on repeated polls. Task 3 tests.
3. **Two accounts share a journal folder:** switching `LoadGame` FID must not mix current-system states and must not expose identities to viewer. Task 2/5 tests.
4. **A paired viewer requests raw private messages or owner path changes:** denied by Go server regardless of UI; viewer gets only sanitized source/current-system summary. Task 5 tests.
5. **Proton on a second Steam library / moved Windows Saved Games:** discovery must find correct directory or show owner override without changing OS settings. Task 1 tests.

---

### Task 1: Native Windows/Linux directory discovery and protected override

**Files:** Create `go/internal/journal/discovery.go`, `discovery_windows.go`, `discovery_linux.go`, `discovery_test.go`; use build-tagged tests and minimal separate Go private profile source preference helper.

**Interfaces:**
- `journal.Discover(ctx context.Context, options DiscoveryOptions) (DiscoveryResult, error)` with `DiscoveryOptions{Override string, Roots []string, SavedGames string}` (Roots/SavedGames are **test-injection**, not browser inputs).
- `DiscoveryResult{Directory string, Found bool, Source string, Candidates []string}` internal-only; no full paths for remote viewers.
- `journal.ValidateDir(path string) error` checks existing readable directory with source filename/snapshot files OR an explicitly selected directory that may be initially empty; never create the game's folder.
- Native Windows default obtains `FOLDERID_SavedGames` via Windows Known Folder API. Linux checks documented default Steam, Flatpak Steam and bounded extra `libraryfolders.vdf` roots for app 359320, Wine/custom owner override.

- [ ] **Step 1: Write failing tests** `TestDiscoverWindowsMovedSavedGames`, `TestDiscoverLinuxSteamFlatpakExtraLibraryAndWineOverride`, `TestDiscoverAbsentAndNewlyCreated`, `TestBadOwnerOverrideDoesNotCrashApp`; fixtures include renamed Saved Games and multiple Steam libraries, and inject known-folder path for CI reproducibility.
- [ ] **Step 2: Run** `cd go && go test ./internal/journal -run 'TestDiscover' -count=1`; expected RED because package/API does not yet exist.
- [ ] **Step 3: Implement** only the discovery interfaces; return explicit `Found:false` for missing optional game sources rather than fatal startup; ensure custom Windows SavedGames location does not rely on hardcoded `%USERPROFILE%`.
- [ ] **Step 4: Run** the targeted tests on both OSes, plus `go vet ./internal/journal`; expected PASS.
- [ ] **Step 5: Commit** `feat(go): discover Elite journal directories on Windows and Proton Linux`.

### Task 2: Strict JSONL journal parser and exact numeric IDs

**Files:** Create `go/internal/journal/parser.go`, `parser_test.go` and synthetic fixtures in `tests/testdata/g2a`.

**Interfaces:**
- `journal.ParseLine(line []byte, source SourcePosition) (Event, error)`.
- `SourcePosition{Path string, FileID string, Offset int64}` is internal; `Event{Type string, Timestamp time.Time, GameVersion string, GameBuild string, FID string, SystemName string, SystemAddress string, MarketID string, Raw json.RawMessage, Position SourcePosition}` keeps internal raw only. Parsing accepts both `Fileheader` and `fileheader`, CRLF/trailing CR, unknown future events and timestamp precision; returns error on invalid complete line; path stays internal.
- Use `json.Decoder.UseNumber` and schema-aware ID conversion to decimal string; never feed huge IDs through float64 or frontend Number.

- [ ] **Step 1: Write failing tests** `TestParseJournalUTF8CRLFAndFileheader`, `TestParseExactSystemAndMarketIDs` with `SystemAddress=9223372036854775807` and `MarketID=9007199254740993`, `TestParseDuplicateTimestampNotDedupKey`, `TestParseMalformedCompleteLine`, `TestParseUnknownFutureEvent` and `TestSyntheticFixturesContainNoPrivateData`.
- [ ] **Step 2: Run** `cd go && go test ./internal/journal -run 'TestParse|TestSynthetic' -count=1`; expected RED.
- [ ] **Step 3: Implement** parsing/normalization with a bounded 1MiB record; limit externally visible keys in the Task 5 projection, not by deleting unknown internal event fields. Do not persist raw events.
- [ ] **Step 4: Re-run** parser tests on Windows and Linux; expected PASS. Verify large IDs exactly match string literals, not exponential notation.
- [ ] **Step 5: Commit** `feat(go): parse Elite JSONL with exact IDs and source provenance`.

### Task 3: Single-reader replay, append, rotation and truncation

**Files:** Create `go/internal/journal/reader.go`, `reader_test.go`; use Task 2 parser and Task 1 directory.

**Interfaces:**
- `journal.NewReader(dir string, opts ReaderOptions) (*Reader, error)`; `ReaderOptions{PollInterval time.Duration, MaxRecordBytes int}`.
- `(*Reader).Run(ctx context.Context, emit func(Event, Phase) error) error`, where `Phase` is `Replay` or `Live`; stream completed JSON lines chronologically, reconstruct state from replay and emit Live only for new completed source offsets. Expose `Reader.Stats()` with counts/cursor/source status; thread-safe.
- Detect active file via matching `Journal.*.*.log` filenames and metadata. Track `os.SameFile` identity where available plus file size; on same-name file replacement/truncation reset the specific cursor safely. A new part begins with the first Fileheader line; don't rely on timestamps for uniqueness.

- [ ] **Step 1: Write failing integration tests** `TestReaderStartsBeforeGameDirectoryExists`, `TestReaderWaitsForNewline`, `TestReaderAppendNoDuplicatesAndNoRescan`, `TestReaderRotationAndSamePathReplacement`, `TestReaderTruncateRecovery`, `TestReaderMalformedLineContinues`, `TestReaderRestartReconstructsWithoutLiveDuplicate` and `TestReaderCancellationStopsPolling`. Use a controllable poll driver or short test cadence; assert no leaked goroutines or unbounded buffering.
- [ ] **Step 2: Run** `cd go && go test ./internal/journal -run 'TestReader' -count=1`; expected RED.
- [ ] **Step 3: Implement** bounded streaming with pending incomplete line, stable identity+byte-offset and cancellation; ensure initial replay does not broadcast as new live activity. Use one polling goroutine per feed, not per connected client.
- [ ] **Step 4: Re-run** journal tests and `go test -race ./internal/journal` where supported; expected PASS.
- [ ] **Step 5: Commit** `feat(go): replay and tail Elite journals once with rotation safety`.

### Task 4: Complete-only sidecar JSON snapshots

**Files:** Create `go/internal/sidecar/snapshots.go`, `snapshots_test.go` and synthetic fixture sidecars.

**Interfaces:**
- `sidecar.NewReader(dir string, opts Options) (*Reader, error)`; `(*Reader).Poll(ctx context.Context) ([]Change, error)`; `Change{Kind string, Timestamp time.Time, Valid bool, Raw json.RawMessage}` is internal and scoped to recognized files only. `sidecar.Options{MaxBytes int}` defaults to 4MiB. Keep last valid read for each selected file; suppress re-emission of unchanged content based on hash, and report incomplete/missing/permission status without corrupting state.

- [ ] **Step 1: Write failing tests** `TestSidecarStatusCargoMaterialsNavRoute`, `TestSidecarAtomicReplacementAndPartialWrite`, `TestSidecarNoDuplicateWhenUnchanged`, `TestSidecarRecoverAfterMalformedSnapshot`, `TestSidecarRejectHugeUnknownFile`; no actual journal/user data.
- [ ] **Step 2: Run** `cd go && go test ./internal/sidecar -count=1`; expected RED.
- [ ] **Step 3: Implement** complete JSON decoding, allowlist, size cap and immutable last-good state; don't broadcast entire sidecar documents to viewers.
- [ ] **Step 4: Re-run** sidecar tests and races where supported; expected PASS.
- [ ] **Step 5: Commit** `feat(go): read bounded valid game sidecar snapshots`.

### Task 5: Per-commander Go state, secure read-only API and single shared service

**Files:** Create `go/internal/gamestate/service.go` and tests; `go/internal/webapp/gamefeed.go` and tests; modify `go/internal/webapp/server.go`, `socket.go` and `go/cmd/daedalus/main.go`.

**Interfaces:**
- `gamestate.NewService(opts Options) (*Service,error)` owns ONE journal reader and sidecar reader and provides `Run(ctx)`, `Snapshot() Summary` and `Subscribe(ctx) (<-chan Summary, func())`. `Summary` has only sanitized public fields (`sourceState`, `currentSystem`, `lastEventType`, `lastEventAt`, `importInProgress`, `eventsRead`) and no raw event, FID or path. Internal per-FID state is separate and unknown FID never inherits a previous FID. Only `Location`/`FSDJump`/confirmed `CarrierJump` update arrival system.
- Add Go WebSocket read-only RPC `getJournalSummary` and broadcast `journalSummaryChanged` for BOTH authenticated owner/viewer, with bounded coalesced changes and no extra filesystem readers per socket; owner-only action `POST /owner/journal` for `status`, `selectDir` and `clearOverride` with exact existing same-origin/cookie checks. Viewer must never see owner source paths. Persist owner-selected override with private Go profile storage; invalid override shows error and leaves local desktop working. Add internal version flag `stage:"G2A"` while preserving G1A/G1B and legacy client behavior.
- Extend `--journal-dir PATH` CLI as a **local process argument** for testing; the original `--smoke-test` and `--desktop-smoke-test` MUST stay synthetic/isolated and must not inadvertently read user game folders, even if `--journal-dir` was passed. Introduce `StartLocalWithOptions(ctx, assets, options)` while keeping old exported constructors as compatible wrappers.

- [ ] **Step 1: Write failing tests** `TestGameStateTwoFIDSessionsDoNotMix`, `TestCurrentSystemOnlyUpdatesAfterArrival`, `TestMissingSourceGivesHonestStatus`, `TestGameFeedOwnerViewerSanitizedRPC`, `TestViewerCannotChangeJournalDirectory`, `TestRawReceiveTextAndFIDNeverBroadcast`, `TestViewerReconnectDoesNotStartSecondReader`, `TestRevocationStopsGameFeed` and `TestSmokeUsesNoPersonalJournals`. Include all five Review Focus cases that own these boundaries.
- [ ] **Step 2: Run** `cd go && go test ./internal/gamestate ./internal/webapp -run 'TestGame|TestCurrentSystem|TestMissingSource|TestViewer|TestRaw|TestRevocation|TestSmoke' -count=1`; expected RED.
- [ ] **Step 3: Implement** one context-owned service, exact-ID safe state, owner-only folder override and read-only status projection. Keep WebSocket write concurrency serialized and event subscribers bounded; latest-state coalescing acceptable, raw journal broadcast prohibited. Changing directories cancels the previous reader before a new one begins.
- [ ] **Step 4: Re-run** affected Go tests and Linux race test; expected PASS. Security regression suite must continue passing on both OSes. Inspect the actual response schema for FID/private path leakage.
- [ ] **Step 5: Commit** `feat(go): project safe shared journal state to paired displays`.

### Task 6: Honest live React status card and owner setup

**Files:** Create `src/client/components/go-journal-status.js`, `tests/service/go-journal-status.test.js`; modify `src/client/lib/socket.js` and targeted existing prototype fixtures.

**Interfaces:** A G2A preview banner + card renders current system only from `getJournalSummary` and `journalSummaryChanged`; on absent game shows `Waiting for Elite Dangerous journal`, on permission fault shows status without exposing host file path to viewer, and on live arrival displays the correct system. Only owner sees journal-directory status and editable path override, using the existing authenticated local-owner HTTP endpoint; readers never acquire owner controls. Unported pages remain explicitly disabled.

- [ ] **Step 1: Write failing React tests** `TestG2AOwnerAndViewerCurrentSystemCard`, `TestG2AMissingSourceAndDisconnectedStates`, `TestG2AOwnerOnlyDirectoryControl`, `TestG2AReplayStatusNotFakeGameConnection` and `TestLegacyAndG1BClientStillWorks`.
- [ ] **Step 2: Run** `node --test tests/service/go-journal-status.test.js`; expected RED.
- [ ] **Step 3: Implement** minimal UI and subscribe/unsubscribe to the G2A sanitized event stream; no interval per browser needed. Distinguish service-ready vs game-active. Keep G1B six-digit code display and fallback for old Node 32-character pairing.
- [ ] **Step 4: Run** `npm test && npm run lint:security && npm run build:go-assets` and new React tests; expected PASS.
- [ ] **Step 5: Commit** `feat(ui): show real Go journal status without exposing private data`.

### Task 7: CI, synthetic end-to-end smoke, bounded performance and handoff

**Files:** Modify `.github/workflows/go-desktop.yml` and preview packaging/docs; create `docs/testing/g2a-windows-linux-journal-smoke.md`, `docs/handoffs/g2a-go-journal-ingestion.md`. Add source-benchmark tests in existing Go test packages.

**Interfaces:** CI must trigger on `go/g2a-journal-ingestion` (as stacked PR target and push branch), run full Windows/Linux builds, Go/React tests, Linux race, required native smoke and a second synthetic journal-driven smoke that uses only the new `--journal-dir` test option. Upload preview artifacts clearly marked `g2a-experimental-unsigned-local-journal-no-uploads`. All fixtures synthetic; no actual game install in CI.

- [ ] **Step 1: Write failing build/workflow tests** verifying Windows/Linux gates, correct G2A branch trigger, fixture-based journal smoke, source-read cancel tests and absence of provider network/credentials in test binaries. Add `BenchmarkJournalReplay` measuring events/sec and allocations (no guessed threshold).
- [ ] **Step 2: Run** `npm run test:go-assets && cd go && go test ./...` plus GitHub workflow validation tests; expected RED until fixtures/workflow are wired.
- [ ] **Step 3: Implement** CI/script/runbook/handoff. Document optional Windows real-game current-system verification and secondary laptop connection and Linux Steam/Proton/manual-folder acceptance. Label new preview truthfully as limited live-journal support, while unported game pages and integrations remain disabled.
- [ ] **Step 4: Verify current-head workflows** on Windows and Ubuntu (full Go, JavaScript, native renderer, Linux race), including independently checked artifact name and checksum manifest. Record actual limits: replay throughput, idle CPU/memory where measured, missing real Linux gameplay tests, unknown data sizes and file-system faults.
- [ ] **Step 5: Commit** `ci(go): gate cross-platform G2A journal preview and publish handoff`.

## Final acceptance / stop conditions

- Normal native Windows and Linux programs start with or without the game installed. A missing journal does not crash the UI and can be detected later; manual override is owner-only.
- Startup historical replay and live tailing are bounded, accurate for relevant events and file replacement/rotation, and have deterministic tests; source IDs remain exact.
- One ingestion reader and shared cache serve all authenticated windows/devices; unpaired clients see no journal or viewer credentials; all owner-only file controls are enforced by Go.
- Paired laptop sees only sanitized current system and source availability, never raw game chat or commander private IDs. Live updates do not require a browser refresh, and revocation closes subscription.
- All current CI jobs pass on Windows and Linux; older G1B, G1A and legacy regression tests remain valid and green.
- A native desktop preview displays real local journal data, but still does not claim functional exploration/mining/engineering/game-provider modules. **No provider uploads, release, merge, deployment or replacement.**

**Handoff:** Place exact PR URL, commit list, verification commands/results, CI links, synthetic fixture cases, accepted/manual Windows and Linux gameplay evidence, data migration limitations and next G2B module-parity slice in `docs/handoffs/g2a-go-journal-ingestion.md`. Stop at G2A and request external review. Next independent slice should migrate *current system/navigation display* using the new event service, not reimplement everything in the same PR.

## Plan self-review

- Parent architecture and G2A spec requirements map to Tasks 1–7. The intentional missing items (full feature parity, provider uploads, native picker, database, headless daemon) are explicitly deferred rather than silently omitted.
- Review Focus #1 maps Tasks 1/3/5/6, #2 Task 3, #3 Tasks 2/5, #4 Task 5, #5 Task 1. Every focus includes failing-first assertions.
- Go event/sidecar boundaries do not reuse raw private browser messages. `Summary` (sanitized) is the only server-owned public status type. Proposed source names are consistent across tasks.
- All steps are one testable action with exact paths/commands and expected red/green; each task ends in an independently reviewable commit.
- One deliberate engineering choice left open: Wails native folder picker API. A validated owner-only path input is sufficient for G2A; native picker can follow once Windows and Linux UI trust boundary is independently tested.
