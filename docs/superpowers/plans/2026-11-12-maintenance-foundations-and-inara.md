# Maintenance Foundations and Inara Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a secure, regression-tested Daedalus baseline and deliver correct, durable, opt-in Inara integration without breaking local or paired second-screen clients.

**Architecture:** Preserve the existing CommonJS Node service, Next.js client and Go launcher. Extract Inara payload translation, transport, response checking and durable outbound delivery behind testable modules. Implement a least-privilege socket boundary before exposing sensitive settings to browser clients. Do not change unrelated provider payloads in this PR.

**Tech Stack:** Node.js >=20 (project CI Node 24), built-in `node:test` and `node:assert/strict`, axios, nedb-promises, existing WebSocket protocol, React 17/Next 12, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-11-11-maintenance-first-integrations-and-exobiology-design.md`

**Related audit:** `docs/audits/2026-11-11-existing-functionality-review.md`

## Global Constraints

- First release is **maintenance**, not new route planning or new placeholder screens.
- No real network uploads from CI or tests. No real API keys or live CMDR identities in fixtures.
- EDSM, EDDN and CAPI are explicitly enabled only in later independent plans/PRs.
- Always honor explicit consent, connected-account identity, Legacy/beta restrictions, Inara limits and legal obligations.
- All external host messages are untrusted; never leak API keys or OAuth credentials via WebSocket.
- Preserve local Windows functionality and deliberately paired remote browser/tablet operation.
- Do not merge if access-control or test-coverage regressions are outstanding.

## Review Focus

The highest-impact user conditions to pin in the tasks below:

1. Two clients (PC and tablet), one unpaired: unpaired client cannot receive credentials or mutate settings (Task 1).
2. Service crashes after Inara HTTP submission but before acknowledging it: resend semantics avoid silent loss and limit replay/duplicates (Task 4).
3. Login starts mid-session with incomplete Location or Materials.json: no fake position, no empty materials overwrite (Tasks 3 and 5).
4. User disables Inara or changes CMDR/API key while an event is pending: do not upload under the wrong account, keep or quarantine safely (Task 4).
5. Inara returns HTTP 200 but mixed event-status errors or rate limits: persist/report each event's result without marking all as delivered (Task 5).

---

## File responsibility map

- **Create** `src/service/lib/integrations/inara-payload.js`: pure journal/snapshot-to-Inara event mapping, header and identity construction.
- **Create** `src/service/lib/integrations/inara-response.js`: pure response classification for header and per-event status.
- **Create** `src/service/lib/integrations/outbox.js`: bounded durable per-provider/commander queued records and acknowledgement ledger, with injected clock/store.
- **Create** `src/service/lib/integrations/inara-client.js`: injected HTTP client plus batch / error handling.
- **Create** `src/service/lib/socket-auth.js`: local trust/paired client access and scoped RPC policy.
- **Modify** `src/service/main.js`: authenticate sockets; restrict sensitive callback paths; keep non-secret streaming stable.
- **Modify** `src/service/lib/event-handlers.js`: sanitize preferences response, gate updates, forward consent-safe settings.
- **Modify** `src/service/lib/event-handlers/integrations.js`: adapter wiring, queue scheduling and integration status.
- **Modify** `src/client/components/settings.js`: separate write-only credentials UI and honest sync/status copy.
- **Modify** `src/client/lib/socket.js`: receive auth/pairing status, select `wss:` on HTTPS without breaking HTTP.
- **Modify** `package.json`, `.github/workflows/ci.yml`: deterministic tests required in CI.
- **Create** `tests/integrations/inara-payload.test.js`, `tests/integrations/inara-response.test.js`, `tests/integrations/outbox.test.js`, `tests/service/socket-auth.test.js`, `tests/integrations/inara-client.test.js`, with anonymized JSON fixture files in `tests/fixtures/`.

## Task 1: Secure RPC boundary with paired second-screen clients

**Files:** Create `src/service/lib/socket-auth.js`, `tests/service/socket-auth.test.js`; modify `src/service/main.js`, `src/service/lib/event-handlers.js`, `src/client/lib/socket.js`, `src/client/components/settings.js`.

**Interfaces:**
- `authorizeRpc({principal,handlerName}): boolean` (default deny for unpaired, explicitly allow only public display methods for trusted read-only clients).
- `publicPreferences(preferences): object` (must omit API keys, OAuth refresh/access tokens, and any user-identifying credentials).
- `setIntegrationSecret({principal, provider, secret}): Promise<{ configured:boolean }>` accessible only to authenticated local-admin context.
- Device registration and local owner login use OS-appropriate secrets, origin validation and short-lived sessions; do not use default anonymous LAN access.

- [ ] **Step 1: Write failing tests.** Check anonymous `getPreferences` and `setPreferences` are denied, remote read-only cannot call `beginCapiAuth`, admin can update a key without returning it, a second tab shares only permitted data, and `publicPreferences` removes `edsmApiKey`, `inaraApiKey`, `capiRefreshToken`; unpaired device must not see personal status.
- [ ] **Step 2: Run** `node --test tests/service/socket-auth.test.js` — expect FAIL before implementation.
- [ ] **Step 3: Implement** the interfaces and pair/trust flow; always check the RPC name and role server-side and never trust browser-reported roles. Restrict OAuth callback to loopback, validate state in the CAPI workflow. Keep remote pairing disabled until tested.
- [ ] **Step 4: Run** `node --test tests/service/socket-auth.test.js` — expect PASS; manually verify Windows WebView and one paired browser device before public release.
- [ ] **Step 5: Commit** using `git add src/service src/client tests && git commit -m "security: protect service RPC and third-party credentials"`.

## Task 2: Test-gated provider fixtures and isolated HTTP transport

**Files:** Create `tests/fixtures/inara-fsdjump.json`, `tests/fixtures/inara-materials.json`, `tests/integrations/inara-payload.test.js`; modify `package.json`, `.github/workflows/ci.yml`.

**Interfaces:** `npm run test:unit` runs `node --test tests/**/*.test.js` (avoid shell-glob reliance by using `node --test tests` if necessary); CI runs tests on PRs.

- [ ] **Step 1: Write failing fixture tests** for supported journal events and malformed input; assert no outbound requests/production endpoints are contacted; include empty missing field and Windows filename fixtures.
- [ ] **Step 2: Run** `node --test tests/integrations/inara-payload.test.js` — expect FAIL for not-yet-created mapping.
- [ ] **Step 3: Implement test script and CI gate**; retain advisory legacy global lint until separately fixed, but no `|| true` for new tests; enforce generated payload lint and type checks in touched files.
- [ ] **Step 4: Run** `npm run test:unit` — tests passing for completed fixture scope.
- [ ] **Step 5: Commit** `git add tests package.json .github/workflows/ci.yml && git commit -m "test: introduce deterministic provider contract gate"`.

## Task 3: Correct Inara event translation and header

**Files:** Create `src/service/lib/integrations/inara-payload.js`, `tests/integrations/inara-payload.test.js`; modify `src/service/lib/event-handlers/integrations.js`.

**Interfaces:**
- `makeInaraHeader({appVersion,apiKey,commanderName,isBeingDeveloped}): object`.
- `mapJournalEvent(entry,gameState): object[]` returns an array of documented Inara event objects or `[]`; never add undocumented fields.
- `mapMaterialsSnapshot(snapshot,gameState): object[]` returns `[]` unless complete snapshot is verified.

- [ ] **Step 1: Write failing tests**: `FSDJump` maps to documented `addCommanderTravelFSDJump`; `Location` maps to `setCommanderTravelLocation` only when valid; `StarPos` => `[x,y,z]`; fields use exact Inara API names; beta/Legacy sessions produce no uploads; invalid position produces no fabricated coordinates; incomplete Materials.json produces no inventory reset; supported form of credits sent once when appropriate.
- [ ] **Step 2: Run** `node --test tests/integrations/inara-payload.test.js` — expect FAIL.
- [ ] **Step 3: Implement pure mapping** using the actual Inara documentation as the source of truth and EDMC as a behavior cross-check. Check `eventStatus` documented values, `searchName` syntax, and travel field casing from latest official API immediately before implementation; store a pinned contract revision in tests. Do not assume copied comments are specifications.
- [ ] **Step 4: Run** `node --test tests/integrations/inara-payload.test.js` — expect PASS.
- [ ] **Step 5: Commit** `git add src/service/lib/integrations src/service/lib/event-handlers/integrations.js tests && git commit -m "fix(inara): map supported travel and inventory events"`.

## Task 4: Durable, bounded delivery queue

**Files:** Create `src/service/lib/integrations/outbox.js`, `tests/integrations/outbox.test.js`; modify `src/service/lib/event-handlers/integrations.js`.

**Interfaces:**
- `createOutbox({store,clock,maxQueued,maxAgeMs}): {enqueue,claim,ack,nack,listStatus}`.
- `enqueue({provider,commanderId,sourceKey,payload,timestamp})`: idempotent by stable source fingerprint; returns durable ID.
- `claim({provider,commanderId,limit})`: leases bounded unsent events, returns only same-account entries.
- `ack(ids)` and `nack(ids,{retryAt,reason})`: transactionally update result, without removing before delivery acknowledgement.

- [ ] **Step 1: Write failing tests** for queue survival after a simulated process restart, multiple commanders, missing key, provider disable, repeated identical journal events, network timeout, 429/5xx, status-required retry, time-based expiry and queue bounds.
- [ ] **Step 2: Run** `node --test tests/integrations/outbox.test.js` — expect FAIL.
- [ ] **Step 3: Implement** per-provider/CMDR storage and bounded retries with exponential backoff and jitter; write-before-send, acknowledge only positive application-level success, quarantine expired and permanent failures. Where ack is ambiguous after a crash, use destination dedup semantics and idempotent source keys to bound duplicate risk; do not promise exactly-once delivery without server support.
- [ ] **Step 4: Run** `node --test tests/integrations/outbox.test.js` — expect PASS.
- [ ] **Step 5: Commit** `git add src/service/lib/integrations/outbox.js src/service/lib/event-handlers/integrations.js tests/integrations/outbox.test.js && git commit -m "fix(sync): persist provider outbox and retry safely"`.

## Task 5: Inara transport, response classification and integration health

**Files:** Create `src/service/lib/integrations/inara-client.js`, `src/service/lib/integrations/inara-response.js`, `tests/integrations/inara-client.test.js`, `tests/integrations/inara-response.test.js`; modify `src/service/lib/event-handlers/integrations.js`, `src/client/components/settings.js`.

**Interfaces:**
- `classifyInaraResponse({httpStatus,body}): {ok,acks:[{index,status,kind,message}],retryAfterMs?}`.
- `postInaraBatch({http,endpoint,header,events,timeoutMs}): Promise<Classification>` (injected HTTP).
- `getIntegrationStatus` returns enabled/configured/lastSuccess/lastError/queueCount per provider; no keys, tokens, personal FID.

- [ ] **Step 1: Write failing tests** for Inara documented 200 success and mixed `events[].eventStatus` results, invalid/missing response structure, HTTP 429 and 500, 200 with API error, bad credentials, absent CMDR and lack of persistent network connectivity; test test-connection must not depend on fictitious 'unknown' commander.
- [ ] **Step 2: Run** `node --test tests/integrations/inara-response.test.js tests/integrations/inara-client.test.js` — expect FAIL.
- [ ] **Step 3: Implement** request pacing and batch limits from current official API, separate test-connection from syncing, appropriate status sanitization, and per-event ack/nack; never log secret headers or bodies.
- [ ] **Step 4: Run** `npm run test:unit` — expect PASS. Validate manually against a registered test application and an opted-in real test commander **only** outside CI; record sanitized result.
- [ ] **Step 5: Commit** `git add src/service src/client tests && git commit -m "fix(inara): validate responses and expose trustworthy sync status"`.

## Task 6: End-to-end offline/restart and second-screen release gate

**Files:** Create `tests/integrations/inara-replay.test.js`, `tests/fixtures/journal-session.jsonl`, `docs/testing/manual-windows-smoke.md`; modify `.github/workflows/ci.yml`, `docs/README.md` if it exists, otherwise `README.md`.

**Interfaces:** Real network disabled in test. Replay anonymized journal events through existing handler chain with injected clock, preferences and HTTP and verify payloads + queue + status + paired client permission.

- [ ] **Step 1: Write failing tests** for process restart after unacknowledged send, partial lines, startup before game LoadGame, wrong commander/CMDR-switch, eventual success after timeout and duplicate injection.
- [ ] **Step 2: Run** `node --test tests/integrations/inara-replay.test.js` — expect FAIL.
- [ ] **Step 3: Wire minimal replay harness** and document Windows test: install, run Elite, connect native window, use tablet pairing, enable Inara, jump, dock, update materials, restart while offline, reconnect, disable upload. Ensure connected local display still works with upload disabled.
- [ ] **Step 4: Run** `npm run test:unit && npm run build:client` — expect PASS, followed by Windows executable smoke test on Actions. Do not mark manually untested workflows as verified.
- [ ] **Step 5: Commit** `git add tests docs .github/workflows/ci.yml README.md && git commit -m "test: cover live-session Inara delivery and Windows acceptance"`.

## Follow-on independent implementation plans (not in this branch)

- EDSM's full contract, retry and rate-limited journal batching.
- EDDN's per-event live schema validation, privacy and retry.
- CAPI OAuth identity/token storage and registered-client verification.
- SrvSurvey rule diff, strict bio parser and evidence-based confidence/value labeling.
- Ship and Powerplay data sync, remote wishlist sync, mining correctness, GalNet/media, package/updater supply chain.
- Neutron planner and other missing features only after these maintenance releases.

## Self-review

The steps intentionally pin source paths, interfaces, fixture assertions and commands, avoiding product-code changes in this documentation branch. Task 1 establishes the security gate; Task 2 establishes testing; Tasks 3–5 repair Inara; Task 6 validates the integrated behavior. EDSM/EDDN/CAPI and Bio remain separate implementation plans by design to keep PRs independently reviewable. CI cannot run automatically until the branch adds a workflow and tests; verify execution results rather than inferring success.
