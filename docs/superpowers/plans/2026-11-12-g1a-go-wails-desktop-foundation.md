# G1A: Go/Wails Windows and Linux Desktop Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Build a testable native Go/Wails v3 desktop preview on Windows and Linux which displays the current React interface without launching a Node service at runtime.

**Architecture:** One Go process embeds the Next.js static export, serves it over a loopback-only authenticated HTTP/WebSocket endpoint, and hosts a Wails window pointing to that local URL. This stage is an OFFLINE, SYNTHETIC foundation; it is not a replacement for the current working game companion or its integrations.

**Tech Stack:** Go 1.26+, pinned Wails v3.0.0, Windows WebView2, Linux GTK4/WebKitGTK 6.0 (Ubuntu 24.04+ / Debian 13+), Node 24 used only to build the existing React 17 / Next 12 frontend, Go HTTP and embedded filesystem, minimal pinned Go WebSocket library.

**Spec:** `docs/superpowers/specs/2026-11-12-go-first-windows-linux-rebuild-design.md` in branch `docs/performance-first-native-modules-2026-11-12`.

## Global Constraints

- Start Go migration before adding new Node backend features. Do not remove, disable or rewrite the existing legacy runtime yet.
- Do not rewrite React pages in this PR. Continue to export the existing 62-page frontend and preserve its layout and route behavior.
- No Node, Python, EDMC or independent browser engine processes are launched by the new Go executable at RUNTIME. npm is acceptable for build-time frontend compilation.
- No live journal processing, CMDR state, provider uploads, game controls, plugin installation or new application features in G1A.
- No remote LAN listener or secondary-device pairing in G1A. Phase G1B implements authenticated HTTPS remote access after the Go local boundary is proven.
- Local owner authentication, Origin checks, loopback binding and redacted diagnostics are mandatory. Follow security requirements and tests from Phase 1A PR #2.
- Do not use Wails nightly or @latest. Pin v3.0.0 and a Go 1.26+ toolchain; verify current stable docs and dependency hashes.
- No merge, installer replacement, release, deployment, external API traffic or real credentials in fixtures without separate approval.

## Git dependency and delivery

The corrected React authentication flow is on `security/phase-1a-foundations` (PR #2). If it is not merged, branch `go/g1a-desktop-foundation` FROM THAT BRANCH and open a DRAFT stacked PR against `security/phase-1a-foundations`, not against main. If PR #2 has been merged, branch from updated main and open a draft PR against main. Never copy the old unauthenticated service back into the new design. Read the approved spec on PR #3 and the full functionality audit in PR #1 as reference.

## File responsibility map

New module: `go/go.mod` and `go/go.sum` (module name `github.com/Jakoczig/daedalus/go`).

- `scripts/build/export-go-assets.cjs`: deterministic export/copy from existing `build/client` into generated `go/internal/frontend/dist`; reject missing/stale source.
- `tests/build/export-go-assets.test.cjs`: build script fixture tests.
- `go/internal/frontend/assets.go`: `embed.FS` containing generated frontend; expose root filesystem for server.
- `go/internal/webapp/static.go` + `static_test.go`: nested Next-exported HTML routing, JS/CSS/assets, traversal rejection.
- `go/internal/webapp/server.go` + `server_test.go`: one 127.0.0.1:0 listener, readiness and shutdown.
- `go/internal/webapp/session.go` + `session_test.go`: authenticated local owner bootstrap; reject remote and cross-origin.
- `go/internal/webapp/socket.go` + `socket_test.go`: sanitized existing WebSocket envelope, synthetic offline results, no arbitrary RPC dispatch.
- `go/internal/desktop/app.go` + tests: Wails window lifecycle, real native desktop app.
- `go/cmd/daedalus/main.go` + tests: CLI entrypoint and headless `--smoke-test`.
- `.github/workflows/go-desktop.yml`: Windows + Linux required build/test and offline smoke.
- `docs/testing/g1a-windows-linux-smoke.md`: manual desktop and deep-link checklist.
- `docs/handoffs/g1a-go-wails-foundation.md`: implementation commits, evidence and next phase.
- Modify `package.json` and `.gitignore` minimally for build script and ignored embedded output. Modify `src/client/lib/socket.js` ONLY if needed to display honestly the incomplete/offline service; preserve its existing behavior when running against the old service.

## Review Focus: five realistic failures and their owning tests

1. Port 3300 is already occupied by the legacy Node service: Go starts independently on a kernel-selected loopback port. (Task 2)
2. A malicious browser tab, forged Host/Origin or fake forwarded LAN request tries to claim owner: fail closed; never trust roles/proxy headers. (Task 3)
3. A user reloads `/exploration/system` or a nested Next route: serve the matching exported HTML and `_next` assets, not a different page or 404. (Task 2)
4. Closing the Wails window or failing during startup leaves an orphan server: always stop the listener. Two Wails windows must share the same server. (Task 4)
5. Linux launch without game journals or GUI dependencies: show explicit prototype/no-game status when supported; give clear diagnostics when GTK/WebKit is missing; no Node fallback. (Tasks 4-5)

## Task 1: Build-time export of existing React assets

**Files:** Create `scripts/build/export-go-assets.cjs`, `tests/build/export-go-assets.test.cjs`; modify `package.json`, `.gitignore`.

**Interfaces:** `exportAssets({sourceDir,destinationDir}): Promise<{files:number}>`; source `build/client` generated by `npm run build:client`; destination `go/internal/frontend/dist` ignored by Git.

- [ ] Step 1 — Write failing Node fixture tests verifying missing index.html produces a clear error; fake `index.html`, `exploration/system.html` and `_next/static/chunk.js` copy byte-for-byte; stale destination files are deleted; copy does not escape destination.
- [ ] Step 2 — Run `node --test tests/build/export-go-assets.test.cjs`. Expected: FAIL because build script is absent.
- [ ] Step 3 — Implement the tested exporter and a `npm run build:go-assets` command that builds the current Next client then copies all exported assets; leave the old app build scripts intact.
- [ ] Step 4 — Run `node --test tests/build/export-go-assets.test.cjs && npm run build:go-assets`. Expected: PASS; exported index, nested route and _next assets present, and `git status --short` does not show generated assets.
- [ ] Step 5 — Commit with message `build(go): export existing React assets for embedding`.

## Task 2: Embedding, nested routes and a single loopback server

**Files:** Create `go/go.mod`, `go/go.sum`, `go/internal/frontend/assets.go`, `go/internal/webapp/static.go`, `static_test.go`, `server.go`, `server_test.go`.

**Interfaces:**
- `frontend.FS() (fs.FS,error)` exposes the generated static assets rooted at `dist` using a Go embed directive in its own package.
- `webapp.NewStaticHandler(assets fs.FS) http.Handler` maps `/` to index, `/exploration/system` to `exploration/system.html`, serves existing _next assets with correct Content-Type, and rejects traversal/malformed paths.
- `webapp.StartLocal(ctx context.Context, assets fs.FS) (*Server,error)` binds exactly `127.0.0.1:0`; `(*Server).URL() string` exposes local URL; `(*Server).Close(ctx context.Context) error` shuts down listener and sessions.

- [ ] Step 1 — Write failing Go tests using `fstest.MapFS`: index/deep link/CSS/JS served correctly; unknown paths return 404, traversal denied, two concurrent requests share one server, port 3300 occupied does not block start, and close prevents future requests.
- [ ] Step 2 — Run `cd go && go test ./internal/webapp ./internal/frontend`. Expected: FAIL for missing functions.
- [ ] Step 3 — Implement file routing and server lifecycle, pin Go 1.26/Wails v3.0.0 in go.mod. Follow actual Wails v3 docs (not v2/beta/nightly).
- [ ] Step 4 — Re-run Go tests after the frontend export. Expected: PASS.
- [ ] Step 5 — Commit `feat(go): serve embedded React UI on a single loopback listener`.

## Task 3: Phase 1A-compatible authentication and safe WebSocket bootstrap

**Files:** Create `go/internal/webapp/session.go`, `session_test.go`, `socket.go`, `socket_test.go`; modify `server.go`.

**Interfaces:**
- `POST /auth/session` only from a real loopback peer and same Origin/Host issues a 12-hour, random, server-side owner session with HttpOnly SameSite=Strict cookie and response `{"role":"owner"}`.
- WebSocket `GET /` validates session and Origin, caps frames at 1 MiB, and speaks the existing JSON RPC shape `{"requestId":"id","name":"getLoadingStatus","message":{}}`.
- Only `getLoadingStatus` (offline/no actual files, `loadingComplete:true`, zero events) and `hostInfo` (loopback URL only) are available. Unknown names, sensitive mutations and journal lookups must return a sanitized forbidden/not-implemented response. Provide a clearly visible prototype/offline cue; do not fake gameplay data.

- [ ] Step 1 — Write failing authenticated HTTP/WS tests: anonymous connection denied, remote address rejected, cross-origin session and WS rejected, forged role/forwarded headers ineffective, inherited method names denied, valid owner read-only bootstrap works, sensitive methods forbidden, cookies protected, sockets stop on shutdown.
- [ ] Step 2 — Run `cd go && go test ./internal/webapp`. Expected: FAIL for missing handler.
- [ ] Step 3 — Implement a minimal session store and one pinned maintained WebSocket dependency. Match Phase 1A client `src/client/lib/socket.js` and ensure a loadingProgress status broadcast or RPC handshake reaches the React client.
- [ ] Step 4 — Run `cd go && go test -race ./internal/webapp` on supported platform (normal tests on both Windows and Linux). Expected: PASS, no Internet calls.
- [ ] Step 5 — Commit `security(go): protect bootstrap RPC and report synthetic offline state`.

## Task 4: Wails native application and headless smoke test

**Files:** Create `go/internal/desktop/app.go`, tests and `go/cmd/daedalus/main.go`, tests.

**Interfaces:**
- `desktop.Run(ctx context.Context, localURL string) error` creates Wails v3 application and native window with `WebviewWindowOptions.URL` set to the same-origin Go HTTP server, tracks shutdown, allows future windows to share the listener.
- `daedalus --smoke-test` starts the same Go listener, verifies HTTP+WS handshake and asset availability with synthetic data, closes without launching GUI and exits zero. Any failure exits nonzero with redacted diagnostics.

- [ ] Step 1 — Write failing tests for `--smoke-test` success/failure, no orphan listener, and desktop lifecycle with injected window factory instead of requiring a real GUI in unit tests.
- [ ] Step 2 — Run `cd go && go test ./internal/desktop ./cmd/daedalus`. Expected: FAIL before implementation.
- [ ] Step 3 — Implement Wails v3.0.0 launcher with one process and no Node subprocess; use existing React page, and ensure native Windows + Linux builds with platform libraries.
- [ ] Step 4 — Run `cd go && go test ./... && go run ./cmd/daedalus --smoke-test`. Expected: PASS. Manually launch a real Wails window on both platforms where GUI is available, and record gaps honestly.
- [ ] Step 5 — Commit `feat(go): start Wails Windows/Linux desktop shell`.

## Task 5: Gate both OSes in CI and document a reproducible handoff

**Files:** Create `.github/workflows/go-desktop.yml`, `docs/testing/g1a-windows-linux-smoke.md`, `docs/handoffs/g1a-go-wails-foundation.md`; change `README.md` only to label preview developer build.

**Interfaces:** CI on `windows-latest` and `ubuntu-24.04`: Node 24, Go >=1.26, Wails v3.0.0; Linux deps `build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`; build frontend first, run `cd go && go test ./...`, `go build`, binary `--smoke-test`; retain Phase 1A `npm test`. Required jobs cannot suppress failures.

- [ ] Step 1 — Define failing CI acceptance assertions: both OSes must run Go tests, compile desktop executable and run actual binary smoke, without `continue-on-error` or `|| true`.
- [ ] Step 2 — Run commands locally and collect exact failing prerequisite/error output if the current host cannot launch Wails.
- [ ] Step 3 — Add workflow and manual test runbook (deep links, React rendering, HTTP+WS security, shutdown, missing journal, old service on 3300, Linux WebKit requirements). Explicitly state remote second-screen access is **not yet enabled in G1A**.
- [ ] Step 4 — Verify GitHub Actions on implementation PR: both Windows and Linux jobs PASS and old Phase 1A CI remains green. If real GUI cannot run headlessly, do not claim it was tested.
- [ ] Step 5 — Commit `ci(go): verify native desktop prototype on Windows and Linux` and include Codex handoff document.

## Completion and handoff

G1A is complete only when a real native Wails app builds on both systems, embeds the existing frontend, uses a single Go loopback service, refuses unauthorized HTTP/WebSocket access, runs no Node/EDMC/Python runtime, and reports its gameplay functionality as **unimplemented/offline** rather than working. The legacy Windows app must remain intact. Do not publish a replacement release.

Codex's handoff must include: branch, commit hashes, PR URL, exact tests/CI links and outcomes, WebView/GTK manual results, known unported functions, performance baseline if measured, and **G1B next milestone** (secure remote HTTPS/pairing for the secondary laptop; complete Phase 1A authorization parity).

Later independent milestones: G2 real Windows/Linux Steam/Proton journal reading and feature-by-feature parity; G3 corrected APIs/exobiology in Go; G4 release packaging and performance; G5 neutron/BGS/native modules.

**Plan self-review:** The design covers many projects. This plan implements only first independent deliverable G1A. Its five review-focus failure modes are pinned in Tasks 2-5. Interface names, paths and test commands match task outputs. All production API calls remain disabled. The Go/Wails backend is not approved for shipping until later parity gates.
