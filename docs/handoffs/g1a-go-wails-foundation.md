# G1A Go/Wails foundation — ChatGPT handoff

Branch: `go/g1a-desktop-foundation`.
Draft PR: https://github.com/Jakoczig/daedalus/pull/4.
Base: `security/phase-1a-foundations`, commit
`1acb9c554f7578a6877b92e640aa71650afa38c5` (security PR #2 remains unmerged).
The approved design and five-task plan were read from
`docs/performance-first-native-modules-2026-11-12`; G1A only is implemented.

## Implementation commits

| Commit | Task |
| --- | --- |
| `b6e25740c7e8e809c251ad1e081e1ba1d5d85e52` | Deterministic, validated React asset exporter and fixtures |
| `d26710bb73f561a640471211caacc4b3cfdd29cd` | Embedded frontend, deep-link routing, single ephemeral loopback listener |
| `86c1bdadf5384f4431c44242b9489a2302aea029` | Owner authentication, bounded offline WebSocket protocol, honest React gate |
| `49a8d6585b3473af452324d9dc6b00fbb0343572` | Native Wails launcher, startup/shutdown ownership, executable smoke modes |
| `557943b6b64b677362bf40a481ce9fe7791a8993` | Blocking Windows/Linux desktop CI, stacked security CI trigger, runbook |
| `5256b454e8e33931e300162042d157558cc84e33` | Ubuntu native WebKit namespace prerequisite, retaining the renderer sandbox |

Go module: `github.com/Jakoczig/daedalus/go`. Go 1.26.10 toolchain,
minimum language version 1.26.0, Wails **v3.0.0**, coder/websocket v1.8.14.
All transitive Go versions/hashes are pinned in go.mod/go.sum. The existing npm
lockfile and Node dependencies are unchanged; desktop CI pins Node 24.22.1.
Wails module hash verified:
`h1:lBe89kFu23bVz1dc1H9v0srkHQgWsy7BAKpH4sLQuDw=`.

The executable embeds generated `go/internal/frontend/dist` and hosts the existing
React navigation/theme through one `127.0.0.1:0` HTTP/WebSocket service. No Node,
Python, EDMC, shell command or provider client is launched by the new application.
Native OS WebView2/GTK4-WebKitGTK supply the renderer. Native Wails custom-scheme
asset handling does not create another TCP listener; no `server` build tag is used.

Static UI is locally public to enable bootstrap. Private RPC requires a random,
12-hour, HttpOnly SameSite=Strict memory session. Bootstrap and WS enforce real
loopback peer, exact Host/Origin, and reject forwarded headers. Session count is
bounded, frames are capped at 1 MiB, and shutdown revokes sessions/closes sockets.
Only synthetic `getLoadingStatus` and local-only `hostInfo` are implemented.
Unknown/inherited/privileged methods return sanitized not-implemented errors.
No privileged Go services are bound through Wails.

The G1A bootstrap marker selects an explicit offline shell using existing React
layout/navigation. Unported gameplay children are not mounted, preventing false
game cards and unsupported RPC requests. The old service does not return this
marker and continues rendering its existing pages. `src/app` and `src/service`
have **zero changes** relative to the security PR base. Legacy owner/viewer tests
remain intact. There is no LAN listener, remote pairing or tablet authority in G1A.

## Verification evidence

TDD red phases were observed before implementations: missing exporter module;
undefined Go assets/server interfaces; rejected auth/WS bootstrap before handlers;
offline React child mounted before gate; undefined desktop/CLI lifecycle functions;
missing native-bootstrap readiness signal; missing desktop workflow before its
acceptance test. Shutdown-session resurrection received its own regression test.

Local Windows 11 Pro, amd64; Go 1.26.10; Node 24.22.1; npm 11.6.2:

| Command/check | Result |
| --- | --- |
| `npm test` | PASS: 29 tests, 0 failures/skips; external network blocked by test helper |
| `npm run test:go-assets` | PASS: 4 tests (export safety/bytes/freshness + blocking CI contract) |
| `npm run lint:security` | PASS |
| `npm run lint:go-assets` | PASS |
| `npm run build:go-assets` | PASS: 62 static routes, 278 exported files plus generated SHA256 manifest |
| `go test -count=1 ./...` in `go/` | PASS: all 4 packages; JSON run records 25 passing test/subtest events |
| `go vet ./...` | PASS |
| `go mod verify` | PASS: all modules verified |
| `go build -o bin/daedalus.exe ./cmd/daedalus` | PASS |
| `go/bin/daedalus.exe --smoke-test` | PASS: `G1A offline smoke passed` |
| `go/bin/daedalus.exe --desktop-smoke-test` | PASS: real WebView2 157.0.4341.46 window authenticated React/RPC, quit and stopped service |
| Local `go test -race ./internal/webapp` | NOT RUN: `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`; no local C compiler. Linux CI owns the race gate. |

The actual executable smoke reads root, `/exploration/system`, and embedded
JS/CSS assets over HTTP, then authenticates and requests synthetic status over
WebSocket. Native smoke requires a real renderer handshake before it can pass;
it is not a timer-only window launch. Unit tests cover occupied legacy port 3300,
concurrent deep-link requests, malformed paths, forged Host/Origin/proxy/roles,
anonymous WS, inherited/mutation RPC denial, expiry, oversized frames, socket
termination, context cancellation, startup errors and idempotent shutdown.

Windows UI inspection through the computer-use skill also confirmed the rendered
theme/navigation, `/exploration` then `/exploration/system` navigation, and a
Ctrl+R deep-link reload. Clicking the last native close button exited with code 0;
the process and port were gone. During this inspection `Get-NetTCPConnection`
showed exactly one listener owned by the app, `127.0.0.1:54395`, and there were no
Node/Python/EDMC children. The random port is an observation, not a configured port.

Initial [desktop CI run](https://github.com/Jakoczig/daedalus/actions/runs/43208146391)
passed Windows native smoke and both OSes' build/tests/offline smoke; Linux race
checks passed. The Linux native-window step failed with
`bwrap: setting up uid map: Permission denied`, then
`Failed to fully launch dbus-proxy` / SIGTRAP (exit 2). Investigation matched
[Ubuntu's documented unprivileged namespace restriction](https://documentation.ubuntu.com/release-notes/24.04/#unprivileged-user-namespace-restrictions).
The corrective CI commit installs an AppArmor `userns,` profile attached only to
the exact CI executable path. It leaves WebKit sandboxing and global namespace
policy enabled; the application changes no system settings. This is a deployment
prerequisite for later packaging, not an authorization bypass or suppressed test.

Verified GitHub Actions evidence (see PR checks for subsequent documentation-only
reruns):

| Run / source head | Job | Result |
| --- | --- | --- |
| [43208146391](https://github.com/Jakoczig/daedalus/actions/runs/43208146391), `557943b6` | Windows desktop | PASS: 29 security tests, 4 build fixtures, both lints, 62-route export, all four Go packages, vet, build, real executable offline and native React smoke |
| [43208885374](https://github.com/Jakoczig/daedalus/actions/runs/43208885374), `5256b454` | Ubuntu 24.04 desktop | PASS: same gates plus `go test -race ./...`; actual GTK 4.14.5 / WebKitGTK 2.52.6 native React bootstrap and shutdown |
| [43208146400](https://github.com/Jakoczig/daedalus/actions/runs/43208146400), `557943b6` | Legacy/security CI | PASS: Windows/Linux security regressions and lint, client build, untouched original Windows app build and binary smoke |

Linux native success was logged at 2026-11-12 11:27:52 UTC:
`G1A native React bootstrap and shutdown passed`. All four race-tested packages
passed. Linux was verified natively on GitHub's Ubuntu 24.04.5 runner under Xvfb
and a D-Bus session, not by Windows cross-compilation. The corrective profile
changes only Linux CI setup; Windows application code is unchanged from its
passing run. Security/desktop gates do not use failure suppression.

## Review, limitations and decisions

A fresh independent whole-branch review found **no Critical or Important issue**.
Reviewer independently reran Go tests/vet, all 29 service tests and all four build
fixtures. One **deferred minor**: if a same-machine browser keeps the preview open
after the native app exits, the offline shell retains a healthy connection label
because it hardcodes Layout connected/ready. Reconnect state is tracked by the
socket layer, but this preview label does not yet reflect it. No gameplay or
privileged operation is exposed by this display defect.

**Update:** this connection-status defect is now fixed by
`348b9f992bcf7792136bef680bac218d89747b51`; see the follow-up verification below.

Verification limits:

- Native smoke proves renderer bootstrap and programmatic quit; Windows also had
  the limited UI inspection above. Full keyboard/touch, real-game and manual
  Wayland desktop acceptance remain unverified.
- Two simultaneous native windows are not exercised. The launcher reuses a
  supplied service URL and exposes no new-window UI in G1A; no second server is
  started by window creation.
- Windows WebView2 emitted nonfatal cleanup warning `Failed to unregister class
  Chrome_WidgetWin_0. Error = 1412` after successful native smoke and exit 0.
- Missing Linux linked GTK/WebKit libraries can fail in the OS dynamic loader
  before Go can print a diagnostic. The runbook provides package/ldd instructions;
  missing graphical session receives an explicit diagnostic. No automatic install
  or Node fallback exists.
- Existing Next build reports an outdated caniuse-lite database. No unsolicited
  dependency upgrade was performed. Existing repository-wide JS lint is advisory;
  security lint and all new desktop gates are blocking.
- No performance baseline, release packaging or feature parity has been measured.

Implementation decisions: reused the clean dedicated checkout on the requested
stacked branch; pinned the exact Wails-declared Go 1.26.10 toolchain from the
official historical distribution list; added an explicit native shutdown callback
to close the service during platform teardown; added native renderer smoke beyond
the plan's headless mode. These choices preserve the intended boundary but do not
replace later packaging/platform acceptance. The offline gate is selected only by
the new server's marker, preserving legacy behavior.

## Next milestone — stop here

Do not merge, release, deploy or replace the legacy Windows installer based on
G1A. No production API calls, credentials or game uploads were used.
G1B is the recommended next separately authorized milestone: secure remote HTTPS
and pairing for the secondary laptop, with full Phase 1A authorization parity.
Keep tablets read-only. Perform manual
Windows/Linux navigation/close/multiple-window checks before distribution.
Journal migration, Steam/Proton paths, commander state, Inara/CAPI, game controls,
plugins, media and new modules remain unimplemented and outside G1A.

Reproduction: [Windows/Linux runbook](../testing/g1a-windows-linux-smoke.md).

## External review follow-up — connection status (2026-11-12)

Review: https://github.com/Jakoczig/daedalus/pull/4#issuecomment-6824382348.
Confirmed against head `1ce33c0a15f091285bb8497aff2b24f94e047cd1`: the socket's
close handler already cleared connected/ready, but PrototypeShell hardcoded both
Layout props and always claimed a healthy service. No authentication defect was
identified. Fix commit: `348b9f992bcf7792136bef680bac218d89747b51`.

PrototypeShell now consumes connected/ready from useSocket, passes those actual
values to the existing Layout, and displays disconnected/reconnecting,
connected/waiting-for-ready, or connected-and-ready text. Its status paragraph is
outside Layout so the existing loading fade cannot hide the disconnect message.
Every status labels G1A as an offline preview; gameplay remains unimplemented.
The legacy children path, shared Layout implementation, transport/retry logic,
owner/viewer model, Go server and loopback security boundary are unchanged.

TDD: before the fix, the three display-state cases and connection lifecycle test
failed against the old healthy message. After the fix, all six prototype tests
pass. The regression harness executes the actual provider/socket close handler,
scheduled five-second reconnect callback, new socket open handler and loading RPC
response with deterministic fake browser transport/timers. It renders the real
Layout/Loader to verify visibility and readiness, without adding dependencies or
making network/provider calls. Legacy children still render unchanged for all
three connection states when no G1A marker is supplied.

Local Windows verification after correction:

- `npm test`: **33 passed**, 0 failed/skipped, including six G1A/legacy tests.
- `npm run lint:security`: **PASS** (test-fixture curly-brace lint error corrected).
- `npm run test:go-assets`: **4 passed**, 0 failed/skipped.
- `npm run build:go-assets`: **PASS**, all 62 routes / 278 assets rebuilt/exported.
- `go test -count=1 ./...`: **PASS**, all four packages.
- `go vet ./...`: **PASS**.
- `go build -o bin/daedalus.exe ./cmd/daedalus`: **PASS**.
- Rebuilt executable `--smoke-test`: **PASS**.
- Rebuilt executable `--desktop-smoke-test`: **PASS**, actual WebView2 React
  bootstrap/shutdown. The previously documented nonfatal cleanup warning remains.

Correction-commit CI evidence (head `348b9f992bcf7792136bef680bac218d89747b51`):

| Run | Result |
| --- | --- |
| [Desktop CI 43226575483](https://github.com/Jakoczig/daedalus/actions/runs/43226575483) | **PASS on Windows and Ubuntu 24.04**: 33 service tests, 4 build tests, security/build lint, React asset build, Go tests/vet, binary build, headless HTTP/WS smoke and actual native React bootstrap/shutdown; Linux `go test -race ./...` also PASS |
| [Legacy/security CI 43226575864](https://github.com/Jakoczig/daedalus/actions/runs/43226575864) | **PASS**: Windows/Linux regressions and security lint, React build, original Windows launcher build/smoke |

The subsequent handoff-only commit reruns the same checks; see PR #4 for its
latest check links. No application changes follow the verified correction.

The review's static-allocation observation is not a measured defect; no unmeasured
optimization was made. Linux distribution/AppArmor packaging and real Wayland
acceptance remain limitations. Long-idle/heartbeat policy remains future G1B work;
this follow-up tests client reconnect/status only. PR #4 remains draft and stacked
on PR #2. No merge, release, deployment, provider calls or G1B work occurred.
