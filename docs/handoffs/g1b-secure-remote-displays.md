# G1B secure remote displays — ChatGPT/external review handoff

## Six-character pairing acceptance follow-up

Maintainer feedback: https://github.com/Jakoczig/daedalus/pull/5#issuecomment-6833011040.
The maintainer reports successful real secondary-device pairing/connectivity on the
preceding preview. This is user-reported physical acceptance, not independently
observed device/browser coverage. The six-character entry change still needs its
own physical usability acceptance; certificate/mobile/firewall/roaming and normal
Linux packaging gaps below are not all closed by the connectivity report.

Implementation: `e8a18952856bd5f17052fdbfbe4235f7de7e2c96`.
Final ASCII validation correction: `c47e1713a4eb14991709d52b79f369620aff6bf7`.
Initial verification handoff: `9e1c8a117e8f208e65fa26816dfbef8bcaf2ae7b`.

- Codes use `0123456789ABCDEFGHJKMNPQRSTVWXYZ`, six characters / 30 bits.
  Each character uses `crypto/rand.Int` uniform sampling; no token truncation or
  math/rand. A random collision with the pending code is redrawn so replacement
  cannot preserve that old code. Only its SHA-256 hash and expiry stay in memory.
- Server input uppercases lowercase, then strictly validates six characters and
  the alphabet. No I/L/O/U aliasing, Unicode lookalikes, spaces or separators.
  Expected hash comparison is constant-time. Five-minute expiry, one-use and
  global ten-attempt/minute admission remain; failures consume the same budget
  across peers and repeated owner issuance never resets it. Codes are not logged.
- Viewer cookies still use independent 256-bit random tokens and all existing
  expiry/revocation/owner-viewer/TLS/Host/Origin/profile boundaries remain intact.
  The shorter code intentionally has lower entropy than the prior 128-bit code;
  its approved online security depends on short lifetime and the global rate cap.
- Owner output is large, spaced, selectable and shows a five-minute deadline.
  Viewer entry is visible, mobile-sized, max/min six, alphabet-validated and
  uppercases on change. Anonymous G1B session bootstrap returns only a public
  code-length hint with HTTP 401, never a role/session. This selects the short
  form; the shared legacy form retains its original long-code entry/submission.
  Updating the Node backend's pairing protocol is outside this G1B change.

TDD evidence: new code-shape/length, lowercase admission and public-hint tests
failed on the old server; owner prominence and viewer-entry tests failed on the
old UI. A deterministic entropy fixture reproduced a repeated-code collision
before its redraw guard. All pass after implementation. Added regressions cover
exact expiry, replacement, one-use, invalid alphabets/lengths, two-peer global
attempt exhaustion despite repeated issuance, budget recovery, hash-only pending
state, unchanged 32-byte cookie, UI submission and legacy 32-character entry.
Independent read-only review found no actionable issue; focused Go/JS checks pass.
A later self-check found Unicode long-s (`ſ`) could case-fold to ASCII S. A new
matching-hash regression first admitted that invalid alphabet (HTTP 200), then
passed after validating exactly six ASCII alphabet bytes before normalization.
Rejected Unicode input still consumes the shared rate budget. Full CI is rerun
on that final correction. Independent re-review of ASCII validation and rate-counter
ordering found no actionable issue and passed focused Go checks.

Local verification: **44 JS tests**, **6 build fixtures**, both security/build
lints, **62 routes / 278 embedded assets**, fresh Go tests in all six packages
(final corrected webapp 74.626 s), vet/module verification, Windows executable and HTTP/WS/native
React bootstrap/shutdown smoke **PASS**. Known nonfatal WebView2 cleanup 1412
remains. Windows local race checking is unavailable (no cgo compiler); Linux CI
runs the mandatory fresh full race suite. No dependencies or legacy backend/
launcher files were changed.

Verified final implementation run results on `c47e171`:

| Gate | Result |
| --- | --- |
| [Desktop CI 43288554692](https://github.com/Jakoczig/daedalus/actions/runs/43288554692) | **PASS Windows and Linux**: 44 JS tests, 6 build fixtures, both lints, 62 routes / 278 assets, six Go packages, vet/module verification, native builds and both executable/native React smoke modes |
| Linux normal / race webapp | **PASS**: 65.289 s / 66.685 s, including fresh long-idle/reconnect and pairing regressions |
| Windows normal webapp | **PASS**: 68.495 s, including ACL/lock/recovery regressions |
| [Legacy/security CI 43288554788](https://github.com/Jakoczig/daedalus/actions/runs/43288554788) | **PASS all five jobs**: both OS security regressions, lint, React build and original launcher build/smoke |

Updated **experimental, unsigned, offline preview** downloads from this exact
implementation head (GitHub sign-in may be required; expire 2026-11-26):

- [Windows x64](https://github.com/Jakoczig/daedalus/actions/runs/43288554692/artifacts/13976312272)
- [Linux x64](https://github.com/Jakoczig/daedalus/actions/runs/43288554692/artifacts/13976961245)

Both downloaded ZIPs were independently inspected: exactly executable, README.md
and SHA256SUMS; executable/README hashes matched each bundled manifest, and ZIP
hashes matched GitHub artifact digests. No keys/profiles/credentials are included.
These supersede the earlier 32-character-code previews linked in the historical
recovery record below. Use the new artifact's own manifest for any later run;
new builds may differ because Go embeds commit metadata.

| OS | Executable SHA-256 | ZIP SHA-256 |
| --- | --- | --- |
| Windows | `f685ecbad654cd29f136c2448a12584416b5f1b102f3d4f830036c01e5aee733` | `ded6396b143094363a1c0546abac68194777aa96ca6d7d8b064c968d4256b919` |
| Linux | `99bed51b3539ca68e3f9b725939197bd51595d88cc853badbc3fca7d2240e574` | `9ea9769c70e9c6fb59f5ce7fac796892c15987f1fc9e800bfe1fb6b864cfd1b7` |

The subsequent handoff-only commit is also subject to both full workflows; final
head/check links are recorded in the PR description to avoid a self-referential
commit hash in this document. Runtime sources do not change in that commit.

PR #5 remains draft, stacked and unmerged. Request incremental source/CI review
before closing this milestone. No release, deployment, provider traffic, journal
work or G2A planning/implementation is included.

## External review corrections and experimental downloads

Review: https://github.com/Jakoczig/daedalus/pull/5#issuecomment-6831759803.
The startup finding was independently reproduced before implementation: malformed
or missing initialized remote settings aborted StartLocalWithStorage and prevented
the native desktop from launching. Missing TLS files also exposed paths that could
recreate a CA when both identity files were absent. The regressions failed first.

Correction commits:

- `966ac730be4ac6837be4b3eb9a767e163760b357` — quarantine optional remote corruption
  after securing the exclusive profile lock; preserve local owner HTTP/WS, display
  recovery instructions, and retain damaged files without startup/shutdown writes.
- `b077c2d0da7d59111ea55d7521f002c10e36b1de` — gated Windows/Linux experimental
  artifact ZIPs with executable, standalone prerequisite/setup README and hashes.
- `185159494ea5b40599d58c84f14f5a6325dfb46d` — never regenerate a lost initialized
  CA during running-profile re-enable or certificate renewal.
- `1d373c6fccb8565e446f0dc8f30a8d0b06c87990` — isolate/retire the shutdown
  fixture HTTP connections; retain the original three-second assertion.

Recovery clears remote/device state in memory and blocks every remote mutation.
Only authenticated local-owner status remains available for diagnosis. No HTTPS
listener starts for invalid remote state. Initialized identity is validated without
creating/renewing keys or overwriting configuration. Invalid permissions on optional
files enter the same local-only mode; an unprotectable/unlockable profile root or
an already locked profile remains fatal to prevent another competing owner.
The actionable warning is outside collapsed settings. Repair/reset is manual,
with a backup and explicit trust/device re-pairing; no automatic reset was added.

Server regressions cover malformed remote.json, missing remote.json in an initialized
profile, missing TLS key/certificate file, missing initialization marker, both TLS
files missing, malformed TLS identity, unsafe file permissions, byte-for-byte file
preservation, blocked old viewer tokens, no listener, working local assets/RPC,
exclusive lock retention, unusable root, and running-profile trust loss. A React
regression verifies the warning stays visible and unsafe enable controls are absent.
An independent read-only review of recovery/packaging found no actionable defect
and freshly passed its targeted Go and Node checks. The later running-profile
guards were additionally verified by failing-then-passing tests.

Local verification on the correction tree: **41 Node tests**, **6 build tests**,
both security/build lints, **62-route / 278-asset React export**, six fresh Go package
tests (webapp ~71 s including 65-second idle/reconnect), Go vet/module verification,
Windows executable build, headless HTTP/WS smoke and actual WebView2 native React
bootstrap/shutdown all **PASS**. An initial local export/Go overlap caused a Windows
file-sharing failure; sequential re-export and fresh verification passed. No check
was suppressed. Existing nonfatal WebView2 cleanup warning 1412 remains.

A Linux race run on `1851594` failed the existing shutdown fixture deadline
(`TestSingleLoopbackLifecycle`, no data race reported). Concurrent global-client
requests could leave speculative idle sockets; Go gives unused StateNew sockets
a five-second graceful-shutdown grace. The fixture now owns its HTTP transport
and closes idle connections before cancellation, without changing runtime code
or extending its assertion deadline. Thirty repeated targeted runs passed locally.
The mandatory complete race suite is rerun on the corrected head below.

Artifact packaging reads only two explicit inputs (compiled executable and authored
README), creates a fresh directory and adds SHA256SUMS for those files. Tests prove
adjacent private keys/profiles are excluded and hashes match; directories/symlinks
are refused as inputs. Upload follows mandatory tests/build/native smoke using a
pinned upload-artifact commit, with 14-day retention and missing-files-as-error.
ZIPs are **EXPERIMENTAL, UNSIGNED, OFFLINE PREVIEW**, never a Release or installer.
Each includes [simple Windows/Linux and secondary-laptop HTTPS instructions](../testing/g1b-preview-artifact-readme.md).
Linux users restore the executable bit after extracting (`chmod +x daedalus`).

Verified implementation head: `1d373c6fccb8565e446f0dc8f30a8d0b06c87990`.
Documentation-only commits after this head do not change these executable bytes;
PR checks are rerun on the final documentation head as well.

| Verification | Result |
| --- | --- |
| [Desktop Windows/Linux run 43280351914](https://github.com/Jakoczig/daedalus/actions/runs/43280351914) | **PASS both OSes**: 41 JS tests, 6 build fixtures, both lints, 62 routes / 278 embedded assets, module verification, six Go packages, vet, native builds, executable HTTP/WS smoke and native React bootstrap/shutdown |
| Linux normal / race webapp package | **PASS**: 65.239 s / 66.387 s, fresh full suites including long idle/reconnect |
| Windows normal webapp package | **PASS**: 66.667 s; Windows ACL/lock/recovery regressions included |
| [Legacy/security run 43280351917](https://github.com/Jakoczig/daedalus/actions/runs/43280351917) | **PASS all five jobs**: both OS security regressions, lint, React build, original Windows launcher build/smoke |
| Local final fixture-head verification | **PASS**: fresh 41 + 6 Node tests, both lints, six Go packages (webapp 70.053 s), vet, Windows rebuild and both smoke modes |

Download **EXPERIMENTAL, UNSIGNED, OFFLINE PREVIEW** executables from the verified
implementation run (GitHub sign-in may be required; retention expires 2026-11-26):

- [Windows x64 ZIP](https://github.com/Jakoczig/daedalus/actions/runs/43280351914/artifacts/13973730841)
- [Linux x64 ZIP](https://github.com/Jakoczig/daedalus/actions/runs/43280351914/artifacts/13971424763)

Both actual downloadable ZIPs were inspected: exactly the executable, README.md
and SHA256SUMS; both inner hashes matched, with no profile/key/credential files.
README bytes differ only by checkout line endings across OSes; each local hash
matches its included manifest. Verified SHA-256 values:

| OS | Executable SHA-256 | ZIP SHA-256 |
| --- | --- | --- |
| Windows | `5e05dbe42d1368813aeae3d96ccbb8f57d1bbf19e3b0db9274252b463cf35cbf` | `cda82558eee964c9e734ac469b3ba74763463b4d03dce37182723d24803252b2` |
| Linux | `57ed0266cf42cc9943b085ab56b2a31309efb16ca1fc67282389cb78346ced7a` | `58b045ccca8079eb2bf90eaddf0f8500a6208bee3a50aecd6d4369c027ec4ccd` |

No confirmed finding is disputed. Review suggestions about the 12-hour session
lifetime and trust-installation burden remain documented usability/security
tradeoffs; this correction does not change either protocol decision.

Physical trust import, firewall permissions, second-laptop/mobile browser behavior,
sleep/roaming, Wayland/AppArmor packaging, multi-window and power-loss acceptance
remain manual. The original application/security boundary is preserved; no release,
merge, deployment, provider traffic or G2 work is included.

## Original G1B implementation record

Draft PR: https://github.com/Jakoczig/daedalus/pull/5.
Branch: `go/g1b-secure-remote-displays`.
Stacked base: `go/g1a-desktop-foundation` at accepted
`9dd67eb9b0ba0359661bce60d6a76068694a3f4d` (PR #4).
Existing PRs #2/#3/#4 were read and remain unchanged.

## Implementation commits

| Commit | Result |
| --- | --- |
| `485a109c59efa18a9834f3e49e555613d68dbf59` | Approved-architecture-derived G1B spec/plan with independent protocol review decisions |
| `6e4bb53193270032e5fd61fee720021458402e3c` | Protected profile, OS process lock, private CA and verified renewable IP certificate |
| `68dd20d3f49a96f59dea06d7244605ec39f065f7` | Opt-in TLS listener, durable hashed viewer identity, pairing/device management and failure latch |
| `6a45bb9c65e323f4e92ae600f19d6fbef338a47b` | Heartbeat, bounded pre-auth admission, real 65-second multi-viewer idle/recovery acceptance |
| `e92e524eb65db3527c219b2df0c9455df1f6d3d7` | Owner settings, viewer pairing/reconnect, isolated CLI smoke profiles and startup ordering regression |
| `ec95605020a8cf8ef0060c83e1b7aa50e6d1a657` | Stacked Windows/Linux CI gates, fresh idle/race runs, setup/trust/firewall runbook |
| `97bdd8350fe13adb486267e54bfef02324ae27f5` | Independent review corrections: slow-body owner starvation, expired queued RPC replay, device refresh |
| `555d0d345344f5345ea837675eba2193758596e7` | Windows CI correction: compare binary SID identities instead of SDDL account aliases; keep owner/SYSTEM-only ACLs |

Publishing used GitHub connector commit/tree operations because local Git push has
no credentials. Each published tree SHA was checked against the tested local Git
tree, then the local branch was synchronized to the published history. Commit
metadata differs from local intermediate hashes; the hashes above are canonical.

## Architecture and authority

One Windows/Linux Wails process owns one shared Go service and embedded React
export. Its existing ephemeral `127.0.0.1:0` HTTP owner listener remains separate.
Remote access is **disabled by default**, with no remote listener until an
authenticated local owner explicitly enables it. Normal clean restart restores an
explicitly enabled setting; smoke modes always use a fresh temporary disabled
profile. One exact private IPv4 address/port serves TLS only; no wildcard/public
bind, plaintext LAN listener, proxy trust, extra backend or duplicate game processing.

Local owner POST `/auth/session` retains exact loopback peer/Host/Origin checks.
POST `/owner/remote` additionally requires its owner session. The TLS transport
cannot bootstrap owner: `/auth/session` recovers **only an existing viewer** or
returns 401. It rejects every owner route even with a copied local owner cookie.
All remote auth/WS paths require the exact configured HTTPS Host and exactly one
matching Origin; missing/null/cross/duplicate Origins and forwarded headers confer
no trust. Server-side RPC dispatch implements only offline `getLoadingStatus` and
transport-appropriate `hostInfo`. No naming-based or arbitrary dispatch exists.
Viewer hostInfo never returns the loopback owner URL or settings/secrets.

Owner actions: enable/disable, issue code, list/refresh devices, rename and revoke.
Codes now contain 30 random bits as six Crockford-style Base32 characters,
memory-only, five-minute, one-use; making another
invalidates the previous code. Ten pairing attempts/minute globally bound guessing
without an attacker-controlled IP map. Names are bounded/validated and escaped in
React. Viewer cookies contain random 256-bit tokens, use a distinct `__Host-` name,
Secure/HttpOnly/SameSite Strict and expire absolutely after 12 hours. Disk stores
only SHA256 token hashes, random device ID/name and expiry. No token/key is included
in device lists, JSON responses or logs. Cookie recovery survives network loss and
a clean process restart until expiry, never upgrades a viewer's role.

Remote/device mutations are serialized and written before success/token publication.
Socket admission revalidates device and listener epoch under the same manager lock;
revoke/disable invalidate principals and detach/close their active sockets. Disable
also revokes all viewers and pairing codes. Before remote admission an active-run
latch is persisted; only clean shutdown clears it. An unclean exit or failed
mutation stops remote access and prevents automatic recovery of old tokens after
restart; owner re-enable and re-pair are required. A failed revoke reports failure.
Private-state corruption fails closed rather than resetting trust silently.

## Certificates and private state

`os.UserConfigDir()/daedalus-go`, optionally `--data-dir PATH`, is separate from
legacy preferences. An exclusive OS file lock rejects a second profile owner.
POSIX directories/files use 0700/0600. Windows validates ownership and applies
protected current-user/SYSTEM DACLs before writing secret bytes; broad existing
secret ACLs and reparse/symlink paths are rejected. Writes use temporary private
files, file Sync and atomic replacement. No secret migration occurs.

Go crypto/rand + crypto/x509 generates ECDSA P-256 CA/leaf keys. CA lifetime is ten
years, leaf 90 days, exact-IP SAN. TLS minimum 1.2 with standard verification.
Leaves renew near expiry on enable/startup and running TLS handshakes; the root
and client trust remain stable. Missing/corrupt initialized CA state fails closed.
Only the public CA PEM and SHA256 fingerprint are exported through local owner
settings. The user securely transfers/verifies/imports it on each client. No trust
store/firewall changes, insecure verification flag or warning bypass is automated.

## Reliability and frontend

Maximum 32 devices, four WS/device and 64 total remote TCP connections **before**
TLS/HTTP work. Bounded headers/auth bodies/writes, 1 MiB WS frames. Heartbeat pings
every 20 seconds with 10-second pong deadline; idle expired/revoked/dead sockets
close. Local owner administration remains available when anonymous remote capacity
is exhausted or a request body stalls. A failed saved address bind leaves the local
owner UI available to reconfigure it.

React preserves existing theme/navigation and legacy child rendering. A G1B marker
keeps gameplay children unmounted and labels the preview offline. Only local owner
UI shows TLS/device settings. Viewers see pairing/name entry then read-only navigation.
Status reflects actual connected/ready context and stays visible outside Layout's
loading fade. Transient session bootstrap errors retry with one five-second timer;
401/403 requests pairing. Bootstrap has a ten-second abort deadline. Disconnect
cancels pending promises/timers; queue/callbacks are capped at 128. Timed-out queued
work is removed and stale socket generations cannot restore old readiness.

No runtime Node/Python/EDMC is launched. `src/app`, `src/service` and npm lockfile
have **zero changes** from G1A. Go/Wails/websocket versions remain 1.26.10/v3.0.0/
v1.8.14. Existing x/sys v0.48.0 became a direct dependency for file locking/ACLs;
no dependency versions were upgraded. No journal/commander/provider/API module,
game control, upload or new gameplay data has been added.

## Verification and independent review

TDD observed missing package/functions, absent G1B UI/CLI, incorrect transient
pairing state, uncanceled promises, unbounded queue and missing stacked CI triggers
before implementation. A startup-order regression failed before remote activation
was moved after complete server initialization. Real HTTPS tests use the generated
public CA in a normal root pool; **no verification bypass** is used in tests.

Independent protocol review preceded implementation and supplied eight incorporated
decisions: durable failure latch, atomic epoch admission, profile process lock,
Windows permissions before secret creation, pre-auth bounds, runtime renewal,
explicit Origin on every auth path and bounded client retries/pending work.
Fresh whole-branch review found no Critical issue and two Important issues:

- Anonymous slow auth bodies held the manager mutex and delayed owner revocation.
  A gated reader test failed on a 200 ms local-owner deadline; parsing outside the
  lock plus epoch revalidation fixed it. Network response writes also occur outside
  the remote auth lock.
- Timeouts removed callbacks but retained queued work, bypassing the cap and
  replaying expired requests. A connecting-socket test failed on replay; removing
  expired entries and independently bounding the queue fixed it.

The reviewer also noted stale owner device listing. This was treated as necessary
to the requested owner-management flow: an explicit **Refresh devices** button
lets the owner discover/revoke a newly paired device without reload or periodic
polling. Its render assertion failed first, then passed. No second review was
substituted for the failing-then-passing tests and full suite after these fixes.

Local Windows 11 / Go 1.26.10 / Node 24.22.1 results on the correction tree:

| Check | Result |
| --- | --- |
| `npm test` | **40 passed**, 0 failed/skipped; external network guard enabled |
| `npm run test:go-assets` | **4 passed**, 0 failed/skipped |
| `npm run lint:security`, `npm run lint:go-assets` | **PASS**, including new JSX/tests |
| `npm run build:go-assets` | **PASS**: 62 routes / 278 files plus hash manifest |
| `go test -count=1 -timeout=4m ./...` | **PASS**: six packages; webapp ~68.6 s including real 65-second two-device idle/recovery |
| `go vet ./...`, `go mod verify` | **PASS**, all modules verified |
| Windows executable build | **PASS** |
| Rebuilt executable `--smoke-test` | **PASS**, isolated disabled profile, real local HTTP/WS |
| Rebuilt executable `--desktop-smoke-test` | **PASS**, real WebView2 React authentication/RPC/shutdown |

Tests include default-disabled lifecycle, owner/viewer cookie and role separation,
TLS/Host/Origin/proxy/plaintext rejection, pairing expiry/reuse/rate bounds,
device capacity/rename/revoke/disable, hash-only persistence, process locking,
clean restart recovery, real write-failure injection/unclean latch, private ACLs,
certificate identity/SAN/renewal/corruption, live-socket expiry, pre-auth saturation,
long idle/multiple viewers/reconnect and sanitized UI/security/legacy regressions.

The first [desktop CI run 43233679895](https://github.com/Jakoczig/daedalus/actions/runs/43233679895)
found a Windows-account-specific ACL validation failure: SDDL canonicalizes some
account SIDs to aliases such as `LA`, so string comparison rejected the private
ACL on the hosted runner. The correction validates native ACE trustee SIDs with
binary SID equality. A local canonical-alias regression and Everyone-access
rejection test pass, as do privatefs/TLS and remote integration regressions.
Owner/SYSTEM-only policy was preserved rather than bypassed. Initial
[legacy/security CI 43233679841](https://github.com/Jakoczig/daedalus/actions/runs/43233679841)
passed all jobs.

Corrected implementation head `555d0d345344f5345ea837675eba2193758596e7`:

| GitHub Actions run | Verified result |
| --- | --- |
| [Desktop 43234429962](https://github.com/Jakoczig/daedalus/actions/runs/43234429962) | **PASS on Windows and Ubuntu 24.04**: 40 Node tests, 4 build fixtures, security/build lint, 62-route export, six fresh Go package tests, vet/module verification, executable build, actual HTTP/WS and native React bootstrap/shutdown smoke |
| Same desktop run, Linux race gate | **PASS**: all six packages, including fresh 65-second idle/recovery; webapp 66.377 s under race instrumentation |
| [Legacy/security 43234430203](https://github.com/Jakoczig/daedalus/actions/runs/43234430203) | **PASS**, all five jobs: both OS security regressions/lint, React build, unchanged original Windows launcher build/native smoke, advisory global lint |

Windows native smoke ran WebView2 156.0.4314.58 on Windows Server 2025 and logged
success at 2026-11-12 14:14:20 UTC. Linux native smoke ran GTK 4.14.5 / WebKitGTK
2.52.6 on Ubuntu 24.04.5 under Xvfb/D-Bus and logged success at 14:14:19 UTC.
Windows webapp acceptance took 66.325 s; Linux normal acceptance took 65.162 s.
These are real platform executions, not cross-compilation-only results. No CI
security/desktop failure is suppressed. Subsequent handoff-only checks are linked
on PR #5; no executable changes follow this verified implementation head.

## Remaining limitations / rulings

- **Manual trust/device acceptance remains open:** no physical second laptop,
  Android/iOS trust import, user firewall prompt, Wi-Fi roaming/sleep or browser
  policy test was performed. CI browser/native smoke is not a physical-device claim.
  Follow [the setup and manual acceptance runbook](../testing/g1b-secure-remote-displays.md).
- Private LAN IPv4 only; no IPv6, Internet hosting, DNS names, proxies, automatic
  firewall/trust changes, installation/signing or replacement release. A changed
  host IP requires owner reconfiguration/new URL and usually pairing again.
- Normal Linux distro/Wayland/AppArmor packaging and two simultaneous native
  windows remain G1A manual gaps; CI uses Ubuntu/Xvfb and its narrow executable
  user-namespace profile. WebKit sandboxing remains enabled.
- Atomic file writes/latch tests establish application crash/failure behavior;
  storage-controller/power-loss durability was not physically tested. No formal
  filesystem crash-consistency or key-compromise certification is claimed.
- The reviewer did not independently execute Linux/race or physical tests, nor
  exhaustive barrier-controlled upgrade/revocation interleavings. Epoch/locked
  admission was inspected with no concrete resurrection found; normal revocation,
  failure and idle paths are tested. The CI results above provide separate OS evidence.
- POSIX UID checking was suggested as defense in depth; no exploitable issue was
  established for the ordinary-user/trusted-local-user boundary (foreign-owned
  directories cannot be chmod'd, private files cannot normally be read). No extra
  policy was added. Host user/process compromise is outside this pairing boundary.
- No CPU/RAM improvement was measured, and per-request static-file allocation was
  not optimized without evidence. Legacy global JS lint remains advisory; all
  security/new desktop gates are blocking. Existing Browserslist warning and
  nonfatal WebView2 cleanup warning 1412 remain.
- A short name/code UI is implemented with no QR/discovery service. Device-list
  refresh is explicit to avoid background polling. Session expiry or unclean host
  exit requires re-pairing by design; clean restart/network reconnect recovers.

Engineering choices implement the user-approved architecture: exact-IP direct TLS,
manual public-CA trust, fail-closed recovery after an unclean run, and an isolated
new Go profile. Costs: setup/trust steps and re-pairing after crashes; broader
network/packaging/platform acceptance is deliberately not claimed.

## Stop and next review

G1B only. PR #5 stays draft and stacked on G1A for external review. No existing PR
was modified, and no merge/release/deployment/provider traffic/upload occurred.
Do not replace the legacy app or proceed to journals/Inara/EDSM/EDDN/CAPI/neutron
or gameplay modules automatically. Next actions are external security review and
the documented physical trust/device acceptance, then a separately authorized
migration milestone.
