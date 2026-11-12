# G1B secure remote displays implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans task by task, with TDD and a fresh whole-branch security/protocol review at the end.

**Goal:** Opt-in, verified HTTPS viewer devices on one Windows/Linux Go backend.
**Architecture:** Separate local owner/remote TLS transports sharing one service;
private certificates, hashed durable viewer sessions and explicit read-only RPCs.
**Tech Stack:** Existing Go 1.26.10, Wails v3.0.0, coder/websocket v1.8.14, React.
**Spec:** `docs/superpowers/specs/2026-11-12-g1b-secure-remote-displays-design.md`.

## Global constraints

Remote disabled by default; exact-address TLS only; no insecure verification;
no changes to legacy src/app or src/service; no game/provider/journal work.
Keep existing pins. Never modify existing PR branches, merge, release or deploy.

## Review focus

1. Local owner cookie copied to TLS cannot mint owner or manage devices.
2. Revoke/disable races with pairing/WS upgrade cannot resurrect access.
3. Corrupt/missing private state cannot reset trust or silently enable a listener.
4. Idle/sleep/reconnect and process restart retain viewer-only identity until expiry.
5. Mobile reconnects and several devices never create duplicate service instances.

## Task 1 — private files and verified host certificates

Files: `go/internal/privatefs/{files.go,permissions_windows.go,permissions_unix.go,files_test.go}`;
`go/internal/remotetls/{certificate.go,certificate_test.go}`.
Interfaces: privatefs.Prepare(dir), Write(dir,name,bytes), Read(dir,name);
remotetls.Load(dir,ip,now) returns TLS certificate, public CA PEM, fingerprint.

- [ ] Write/run failing tests: restrictive permissions/DACL, symlink rejection,
  atomic replacement, X509 verified IP/expiry/reload, corrupt key fails closed.
- [ ] Implement minimum storage and certificate generation; run package tests.
- [ ] Commit tested private storage/TLS implementation.

## Task 2 — opt-in listener, durable viewer identity and owner control

Files: webapp `remote.go`, `devices.go`, corresponding tests; modify server/session/socket.
Interfaces: StartLocalWithStorage(ctx,assets,dir); owner remote actions POST;
remote TLS session recovery/pair; internal device store transactions under mutex.

- [ ] Write/run failing real HTTPS tests using roots from public CA: disabled
  default, owner-only enable/status/pair/rename/revoke/disable, listener lifecycle,
  TLS-only/Host/Origin/proxy/forged-role rejection, cookies, expiry, rate/reuse,
  durable token hashes/restart recovery, bounds and in-flight revocation.
- [ ] Implement validated listener/settings/device store and transport-specific
  authentication; reuse only explicit offline read dispatch.
- [ ] Run full Go suite; commit passing boundary implementation.

## Task 3 — socket reliability and idle/concurrent acceptance

Files: socket.go/socket tests, remote integration tests.

- [ ] Write/run failing heartbeat/idle-expiry/concurrency/reconnect tests, including
  real 65-second idle followed by reads, several devices and session recovery.
- [ ] Implement bounded heartbeat/socket limits, immediate per-device invalidation
  and shutdown synchronization. Confirm one shared service and no leaked goroutines.
- [ ] Run normal tests and Linux race CI; commit reliability implementation.

## Task 4 — owner settings, viewer pairing and React recovery

Files: client socket.js; `components/remote-displays.js`; service JSX regression
tests; main.go/main tests for per-user config and isolated smoke storage.

- [ ] Write/run failing render/transport tests: owner controls only, viewer form
  and name, public CA only, safe mutation/errors, disconnect/ready/reconnect,
  failed bootstrap and stale callback rejection; legacy pages unaffected.
- [ ] Implement UI with explicit offline labels and local-only owner actions;
  add normal runtime private state without changing CLI smoke isolation.
- [ ] Run Node suites/lint, full React export, Go suite/vet and real native smoke;
  commit passing implementation.

## Task 5 — CI, independent review and handoff

Files: both CI workflow triggers, security lint paths/build acceptance tests,
`docs/testing/g1b-secure-remote-displays.md`, requested G1B handoff.

- [ ] Add failing CI contract for stacked target and long-idle coverage; implement
  blocking Windows/Linux build/security/normal/race/native-smoke gates.
- [ ] Independently review final patch; reproduce/fix confirmed issues with TDD.
- [ ] Publish new branch and stacked draft PR against go/g1a-desktop-foundation;
  inspect all Actions results and fix failures without suppression.
- [ ] Record exact commits/results/PR URL and manual certificate/firewall/device
  gaps; final docs-only rerun green. Stop at G1B.
