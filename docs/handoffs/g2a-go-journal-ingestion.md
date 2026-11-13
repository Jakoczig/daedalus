# G2A native Go journal ingestion — external review handoff

Date: 2026-11-13. Draft stacked PR: https://github.com/Jakoczig/daedalus/pull/6.
Branch `go/g2a-journal-ingestion`, based on accepted G1B
`2a79991adb10ca4e6c5cadfdb450c5d3fea92032`; target
`go/g1b-secure-remote-displays`. PRs #2/#3/#4/#5 remain unchanged and unmerged.
Approved architecture/spec/seven-task plan were read from documentation branch
`9183736` under `docs/superpowers/{specs,plans}/2026-11-12-*`.

## Implementation

| Published commit | Result |
| --- | --- |
| `2e2b6cca2d674ab8542e9253e65e8ed90e895a7b` | Native Saved Games and bounded Steam/Flatpak/Proton discovery; private override |
| `9509248fdcbed988d5189ab7dfee8f44bf40a12a` | Reviewed discovery corrections: require automatic source evidence; deduplicate queued libraries |
| `79363045629889f4c96dc1c101f4823deadae5db` | Strict UTF-8 JSONL, exact uint64 decimal strings, source provenance, fabricated replay |
| `c2ccbd48b1b3989fa95c9bb5f979c780ad1d3dc1` | Bounded replay/tail, offsets, partials, replacement/truncation, cancellation |
| `aad3780ddad970e5eab1c9dcaaafcb0a06bc70c2` | Windows read-only handles share writes/deletes so game rotation is not blocked |
| `5460c47987610af92f370ba9a1df3f74b88daa39` | Complete-only four-sidecar snapshots, limits, hash suppression and last-good state |
| `8c152d5e845901642298bcf2412f8efdf65079a4` | Reviewed correction: reject invalid UTF-8 sidecars without destroying last-good data |
| `6346fa922fc608d944b7967485655491878fd620` | One shared per-commander service, narrow authenticated RPC/events, protected owner selection, G2A runtime |
| `a7d3d7de0ed613ca248143ae5e4600efdd1ddcb3` | Minimal React journal card, real connection/source states, owner-only setup and async recovery guards |
| `20fddfa87ae0fd6a4557ca7aa2558c38e2c78d36` | Cross-platform CI gates, four smoke modes, replay benchmark, synthetic fixture helper and experimental artifacts/runbook |
| `8feb0ee7f8d987676b4422dea037bcabb2e155bd` | Independent whole-branch review notes; clean source used for the local Windows preview |

CI/smoke/handoff commits and final checked head are recorded in the PR description
after publication. Publishing uses GitHub connector trees/commits because local
Git push has no credentials. Every published tree is compared to its tested local
tree, then the local branch is synchronized; intermediate local metadata hashes
are not the published commit identifiers.

## Architecture and privacy

Normal native execution starts one cancellable Go source service after acquiring
the existing exclusive protected-profile lock. No browser/window starts another
reader. Old exported constructors stay synthetic/offline; new options opt into
ingestion. Windows uses the Known Folder API. Linux checks bounded native/Flatpak
Steam roots, old/new VDF libraries and non-hardcoded Proton user names; explicit
Wine/Heroic override is supported. Missing/invalid source settings never brick the
local owner desktop. Owner override is stored in protected `journal.json`.

Historical records reconstruct state without a stream of new live events. Live
completed source offsets are delivered once per reader run; timestamps are not
deduplication keys. File identity/size/prefix/anchor changes reset relevant cursors.
Partial final lines await newline, oversize/malformed complete lines have sanitized
diagnostics, and startup replay is repeated safely after restart. There are no
provider side effects or exactly-once external-delivery claims.

Known commander states are partitioned by FID internally, bounded to 64 plus an
isolated unknown state. Fileheader/unknown LoadGame do not inherit known identity.
Only Location/FSDJump/CarrierJump arrival records update current system, not
StartJump. Exact SystemAddress/MarketID values remain internal decimal strings;
no float64/JavaScript Number round-trip is used for game IDs.

Public `Summary` contains exactly sourceState, currentSystem, lastEventType,
lastEventAt, importInProgress and eventsRead. Unknown/chat event data never becomes
a last-safe category. No raw record, FID, commander, path, coordinates, inventories
or visited history crosses either WebSocket. Owner path/status and mutations use
only authenticated exact-origin localhost `POST /owner/journal`; remote listeners
deny all owner paths. Streams are bounded/coalesced and payload writes serialized.
G1B HTTPS, six-character pairing, viewer role, revocation, cookies, CA quarantine,
exclusive locking and default-disabled remote access are retained.

Private sidecar caches accept only complete valid UTF-8 JSON from Status/Cargo/
Materials/NavRoute; old timestamps, incomplete writes and missing files retain
last-good data and report state. Sidecars have no trustworthy FID and are not
silently assigned to a new commander or turned into inventory/ship UI in G2A.

## TDD and independent review

Each task began with failing tests before implementation. Independent reviews
covered discovery, parsing, reader lifecycle, sidecars, server/state boundaries
and React. Confirmed findings received explicit failing-then-passing regressions:

- Empty default directories masking active extra libraries and duplicate library
  entries exhausting a bounded queue.
- Windows default read handles preventing game-owned rename during an open read.
- Invalid UTF-8 sidecars being accepted by encoding/json.
- A slow pre-shutdown owner request writing after profile lock release. Shutdown
  now serializes with source configuration writes; handlers revalidate the owner
  session after acquiring the lock. A replacement owner/profile regression proves
  the stale request cannot change its settings.
- An early subscriber missing configuration errors and recovered sources retaining
  stale diagnostics.
- React reconnect error messages persisting after success, and delayed initial
  owner status replacing newer directory edits/actions.

Fresh independent whole-branch review at `20fddfa` found **no Critical or Important
findings**. Journal/sidecar/gamestate tests, focused transport/security tests and
25 UI/build checks passed independently. One confirmed Minor was deferred: the
owner advanced source settings can retain an old private error/path after automatic
recovery/discovery until reconnect or an owner action; the streamed live card is
correct. Refreshing owner details on health transitions must preserve dirty edits
and operation generations in a later polish fix. Linux/race/physical gameplay and
artifact checks were not inferred from that source review. Current-head CI evidence
is recorded below after completion. No actual player journals/identifiers or provider credentials were used
as fixtures. All committed fixture records, IDs and systems are fabricated.

## Local verification before CI

- **PASS 52 JavaScript tests**, **7 build fixtures**, security and build-script lints.
- **PASS React export: 62 routes / 278 embedded assets**.
- **PASS all nine Go packages**, uncached full suite with webapp 70.812 s, including
  the existing real 65-second two-viewer idle/reconnect acceptance; vet and module
  verification pass. A final added native-isolation test also passes separately.
- **PASS Windows executable build and all four actual executable modes**: isolated
  headless/native foundation smoke and explicit-fixture headless/native G2A journal
  smoke. Native G2A requires the owner React card request and sanitized system
  delivery; a viewer cannot satisfy native readiness. WebView2 157.0.4341.46;
  the inherited nonfatal cleanup warning 1412 remains.
- Local Windows race execution is unavailable without a cgo compiler. Linux CI
  runs the fresh full race suite. No successful Linux result is inferred locally.

Local Windows amd64 / Ryzen 7 6800U synthetic reader+parser benchmark, 50,000
records, three iterations: **128,128,067 ns/op, 35.12 MB/s, 390,235 events/s,
94,608,338 B/op, 1,300,126 allocations/op**. B/op is total allocation per replay,
not retained heap/RSS; this benchmark excludes Wails, UI, TLS and state fan-out.
No idle CPU/RSS, end-to-end latency or legacy-versus-Go savings claim is made.

## Windows/Linux CI and experimental artifacts

**Actions restored; correction verification in progress.** The maintainer
completed the fork-specific Enable workflows confirmation. The original retry
`1b47d96bac7882d17cdd26815441d959532a7bdc` ran both workflows:
[legacy/security PASS (five jobs)](https://github.com/Jakoczig/daedalus/actions/runs/43459886156),
[desktop PR: Windows PASS, Ubuntu Go-test FAIL](https://github.com/Jakoczig/daedalus/actions/runs/43459886159),
[desktop push: both Go-test FAIL](https://github.com/Jakoczig/daedalus/actions/runs/43459866594).
These are diagnostic baseline results, not acceptance of the corrected head.
Current-head passing CI and artifact verification must be recorded after the
focused corrections below run in both desktop triggers and legacy/security CI.

### CI failure investigation and deterministic regression corrections

The [external investigation](https://github.com/Jakoczig/daedalus/pull/6#issuecomment-6859071834)
was independently verified against failed Windows/Ubuntu logs and current code.
Both reported failures came from incorrect test ordering assumptions; no runtime
reader, authorization or revocation behavior was changed.

- **Late discovery:** constructor SourceState=waiting does not mean the reader
  has polled. A test-only first-poll gate reproduces the old failure on demand:
  the file is created while Run is blocked, then its first record is correctly
  Replay, violating the test's incorrect Live expectation. The corrected late
  test waits for waiting **and ImportInProgress=false**, published under the
  stats mutex after the first completed poll. A separate gated test creates the
  source before that first poll and requires Replay. Live and Replay assertions
  remain strict, with no longer timeout or sleep-based startup barrier.
- **Revocation:** the initial Subscribe snapshot (or another valid notification)
  can be queued before revoke and read afterward. A real pre-revocation FSDJump
  notification is deliberately buffered by a test receiver; the old any-readable-
  frame assertion deterministically fails. The corrected test establishes this
  stream barrier, synchronously revokes via the owner endpoint, then creates a
  unique new FSDJump. The owner must actually receive that new notification.
  The revoked stream may drain only old allowlisted summaries at or below the
  pre-revocation event count; any new event count/system fails. It must close
  before the original three-second deadline (timeout is failure), session
  recovery must return 401, and a new WebSocket handshake must return 403.
- **Independent authorization defense:** a second revocation subtest removes the
  socket from the close registry under its mutex, simulating delayed transport
  closure. Per-message authorization must still prevent all newly authorized
  notifications and close the socket. Temporarily deleting send's principal.valid
  guard produced deterministic RED with the unique post-revoke system delivered;
  restoring the unchanged production guard produced GREEN. Pre-authorized bytes
  cannot be recalled from TCP/client buffers; they are not new authorizations.
- Both OS jobs now repeat all three tests 25 times; Linux additionally repeats
  them 25 times under -race after the full race suite. All existing gates stay
  mandatory and timeouts are unchanged. Local targeted count10 and count25 passed;
  independent read-only review also ran count10 and found no actionable issue.

Historical local fallback (before Actions restoration):
the actual clean `8feb0ee` Windows executable was rebuilt and all
four smoke modes passed again. A local experimental ZIP was produced under the
ignored workspace-relative path
`temp/g2a-windows-x64-experimental-unsigned-local-journal-no-uploads.zip`.
It is delivered as a local file in the chat, not a GitHub Release or CI artifact.
Its README explicitly states CI is blocked. The ZIP was opened and independently
checked: exactly executable, README and SHA256SUMS, both internal hashes match.

| Local Windows file | SHA-256 |
| --- | --- |
| daedalus.exe | `fbf9a51a0eb8a8d3226193740a09a95b98534e7ee31e7559eddf3981901897c5` |
| README.md | `2418636af00f1ea81655d3184b958cfedbf38f235bfe1c3944c408695e17e884` |
| ZIP archive | `b6cb0531fbb36976fbe6fc5b5ff4eadcb20fcf2120fe57bc248601f15b23d32f` |

There is no installed local WSL distribution or Docker runtime; no substitute
Linux result or binary is claimed, and no OS distribution was installed. Local
Windows race execution still lacks a cgo compiler. G2A implementation is reviewable,
but its required cross-platform verification and CI artifact publication are
**pending verification of the corrected Windows/Linux workflows and artifacts**.

Artifacts are **experimental, unsigned, local journal preview (no provider uploads)**,
not Releases/installers. Packaging reads only the executable and authored runbook,
adds SHA256SUMS, and runs after mandatory tests/security/build/native gates. No
profile, private key, token or game data is packaged. Retention is 14 days.

## Decisions, bounds and limitations

- Work stayed in the existing dedicated checkout on the requested new branch;
  G1B's branch/PR was never written. Independent reviews were retained between
  major tasks as explicitly requested, alongside a final whole-branch review.
- Ruling: sidecars remain source-level identity-unverified private snapshots;
  associating old snapshots with a new commander would be unsafe. Future gameplay
  modules must establish identity/freshness before using them. G2A exposes no
  inventory feature and no source-level raw data.
- Ruling: binary string scans cannot prove the absence of provider behavior. Tests
  instead constrain ingestion packages from net/http/net/os/exec imports, exercise
  synthetic offline modes, retain no-external-network JS tests, and independent
  review checks the call paths. WebSocket/loopback networking is intentionally used.
- Bounds: 1 MiB journal record, 4096 directory entries/cursors, 32 discovery roots
  and Proton users, 4 MiB per sidecar, 64 known commanders, 128 one-value public
  subscribers, ~500 ms polling. Larger unusual archives report limits instead of
  retaining unbounded history. Intermediate UI summaries may coalesce.
- Arbitrary same-identity in-place edits preserving size/mtime may be undetected.
  OS/network filesystem stalls are not forcibly interruptible; cancellation is
  checked between bounded records/fragments and the private-profile lock remains
  held until the old service exits. Power-loss/host-fault durability is not certified.
- No native folder picker, source preference migration from legacy, headless daemon,
  historical database or complete gameplay page migration is included.
- The maintainer reports accepted G1B Windows-to-laptop HTTPS and six-character
  pairing. G2A real-game arrival updates on Windows, secondary-laptop follow-up,
  Linux Steam/Proton installations, distro/Wayland/AppArmor, mobile trust, firewall,
  sleep/roaming, account switching and large personal archives remain manual.

Follow [the bundled Windows/Linux runbook](../testing/g2a-windows-linux-journal-smoke.md)
without sharing journals, FIDs, profiles or secrets. No Node service/original
launcher code, dependency pins or provider modules were changed. No Inara, EDSM,
EDDN, CAPI, uploads, game controls, merge, deployment, release or replacement.

Stop after G2A and request ChatGPT external review. A later separately authorized
slice may build navigation/current-system UI on this service; G2B has not begun.

