# G2B native navigation and current-system exploration — external review handoff

Draft stacked PR: https://github.com/Jakoczig/daedalus/pull/7. Branch
`go/g2b-local-nav-exploration`, target `go/g2a-journal-ingestion` at accepted
`07c2084a170ffe99d1ebe5d1c0e218127fb7923e`. PRs #2–#6 remain untouched/unmerged.
Approved spec and six-task plan read from docs branch
`69b50f633584735fc85fcfda84ab16adf575b079`.

## Implementation and evidence boundaries

The existing single Go journal/sidecar service now owns bounded navigation and
exploration reducers. No page/browser starts another reader. Exact SystemAddress
stays internal as a decimal string; BodyID is a decimal string in the body DTO.
NavRoute correlation requires exact UTC marker/file timestamp equality, strictly
after a timestamped verified LoadGame and the latest clear. File-first/marker-first
orders are supported; each candidate is claimed by one session epoch. Historical
markers are last-known, not fresh active game evidence. Invalid/missing/oversized
rewrites mark prior evidence stale; clear/session/source changes cannot silently
resurrect prior commander data. Progress uses actual arrivals and exact address,
not names or StartJump. Journal target and reported jump count stay separate from
plotted progression; disagreement is explicit. Distances use validated finite
coordinates internally only; no coordinates leave the service.

Exploration requires a verified identity plus confirmed exact arrived system and
ordered source context. Body events require matching SystemAddress. Addressless
FSSDiscoveryScan aggregate count/progress is accepted only in continuous context
after arrival with no pending ambiguous system change; supercruise is not a system
change. Scan rows, scanner-reported count and all-bodies evidence stay distinct.
Missing optional values remain unknown; WasMapped remains a source fact and your
DSS evidence requires SAAScanComplete. Confirmed signal counts/genus observations
never become inferred species, footfall, credits or predictions. Gravity uses
9.80665 m/s² per Earth g.

Public read-only RPCs/events: getGoNavigation/goNavigationChanged and
getGoExploration/goExplorationChanged. DTOs are explicitly typed and allowlisted.
No raw JSON, FID/commander, paths, StarPos/lat/lon, chat, materials, inventory,
credits or credentials are sent. Existing six-field journal summary stays exact.
New subscriptions are capped at 128 one-slot coalesced channels, separately deep
copied per subscriber; replay emits a settled projection instead of every record.
Per-send authentication remains inside the serialized writer. Tests independently
delay transport close and prove post-revocation route/survey notifications cannot
be newly authorized; old queued valid frames are distinguished from new data.

G2B is explicitly enabled for the normal new Go executable; old G1A/offline and
G2A journal smoke modes preserve their prior behavior. Only /nav/route and
/exploration/system mount the new real React panels. Other 62-route navigation
links remain, with unfinished pages explicitly labeled. G2B content/source card
share the real scrollable panel, avoiding inherited clipping/overlap. Native URL
validation permits only the loopback root and the two known canonical read-only
page paths, with no user-info/query/fragment/encoded paths or LAN hosts. Reconnect
clears stale views and ignores old RPC completions; live data wins over delayed
bootstrap. Copy acts only on the clicked viewer device clipboard, with a manual
fallback when browser permission is denied; no host/game action is exposed.

## Commits and TDD/reviews

| Published implementation commit | Change |
| --- | --- |
| 68fd2b44b7d07de661fc247475eeb936e54a8471 | Bounded route evidence parser and session correlation |
| 8d7cd19dc2265ef746b46105dd6c0671a8857ce5 | Actual arrival progression, next target/conflict and replay freshness |
| 8bd077759186c196c6ee56f09833fead3ead2c28 | Independent review regressions: reject null coordinates and combined invalid star classes |
| ddca7a55352a939d596017baafa92d040828f5f2 | Verified current-system scan/signal/DSS reducer, exact IDs and source continuity |
| c175980a31a1d958631a0ff4abbcef183d086b33 | Shared projections, 256 KiB payload caps, deep snapshot isolation, secure opt-in API/revocation |
| 23b2a7d577369791411600e739a14bb1e11e0867 | Two actual React pages, safe reconnect/copy behavior and scrollable layout |
| 7b0b4ca9700d059fe569df632c3c43c58652f6da | Six real smoke modes, native dual-owner-view readiness, Windows/Linux gates, fixtures, preview/runbook and benchmarks |
| 8c359e26aa723b655d7dd6f4c7011d07ff382650 | Final-review regressions: skipped-record trust, undated clear, incomplete arrival and strict coordinate tokens |

These are canonical published source commits; local intermediate metadata hashes
are not the published identifiers. Every connector-published tree was checked
against its tested local tree. The final handoff-only commit changes this document
only; its fresh current-head results/artifacts are recorded in PR #7 after checking
them, avoiding a self-referencing documentation-commit loop.

All six tasks use failing tests before implementation. Independent reviews
between model, service/security and UI boundaries confirmed/fixed null-as-zero
coordinates, inexact star enum matching, subscriber memory aliasing and inherited
non-scrollable/overlapping layout. Each received a reproducing RED then GREEN.
The service guard mutation (temporarily removing principal.valid in send) leaks
the unique post-revoke target and is rejected by the new regression; the original
guard is restored unchanged. Unknown/missing source values, invalid rewrites,
exact large IDs, two FIDs, out-of-system scans, DSS semantics, route clears,
replay coalescing, source selection and cancelled subscriptions are covered.

Bounds: 2,048 route entries; 256 KiB per public DTO; 512 retained bodies/system,
eight systems total and 64 identity contexts. Oversized route DTOs hide all hops
and destination with an explicit partial warning; body view truncation is marked
partial. Signals/genus arrays are bounded at 32 per event and strings at 128 runes.
No user journals or identifying screenshots were committed; fixtures are fabricated.

## Final source review and corrections

The fresh whole-branch review of `07c2084..7b0b4ca` completed after the credit-limit
interruption. It found no Critical issues, three Important correctness defects
and one strict-coordinate-token Minor. All four were independently reproduced
with failing regressions, then corrected in one implementation pass:

- Completed malformed/oversized records now consume an internal per-file source
  sequence number even though they are not emitted as game events. A gap breaks
  G2B identity/system trust in both reducers. A complete LoadGame and confirmed
  arrival restore exploration context. Valid unknown future events preserve
  continuity. G2A valid-event counts/schema remain unchanged; malformed data is
  never forwarded. Actual reader/service tests cover bad arrival, bad identity
  and oversized records followed by an addressless FSS aggregate.
- An untimestamped NavRouteClear keeps a conservative unknown-clear barrier. Old
  markers/files cannot restore the route. A dated clear or verified new session
  can establish a fresh barrier before a newly correlated route is accepted.
- Any confirmed-arrival category clears previous target/count evidence before
  validating the new system identity; incomplete arrivals cannot retain their
  pre-arrival target as the next target.
- Coordinates require actual JSON numeric tokens; quoted numeric strings and
  null values cannot become measured geometry. This strict-token correction was
  independently started before the reviewer returned its Minor grade.

The new cases are also included in mandatory 25-repeat Windows/Linux stress and
25-repeat Linux race gates. No per-send auth guard, assertion or timeout was
weakened. The correction-head verification below supersedes the initial implementation's
already-green CI.

### Review rulings and remaining manual boundaries

- Physical gameplay/Proton/CI/artifacts were not inferred from source review;
  automated evidence is recorded separately, physical acceptance stays manual.
- Unobserved crashes have no trustworthy shutdown signal: replay is explicitly
  last-known, not a wall-clock guarantee. Cost: no claim of current game activity.
- Off-route departures without FSDTarget retain only labeled plotted-waypoint
  evidence and never advance arrival. Cost: next target can remain unknown.
- Same-timestamp sidecar correlation is the approved deterministic evidence rule;
  a journal marker does not attest a content hash. Cost: same-second plot changes
  can be ambiguous; the provenance label must not imply game control/certainty.
- Same-commander/exact-system caches are intentional bounded historical source
  observations. Cost: revisits can contain previously witnessed scans, not a claim
  that every row was newly scanned in the current visit.
- Missing/invalid nested values stay unknown/rejected; exact identifiers retain
  decimal strings. Cost: unsupported values can be omitted rather than guessed.
- Pre-authorized bytes already in transport buffers cannot be recalled by revoke;
  delayed-close tests instead reject newly generated post-revoke game data.
- Snapshot copies, one-slot coalescing, subscriber caps and byte budgets bound the
  inspected paths. No end-to-end resource/performance guarantee was inferred.
- Native URLs permit only canonical root/two pages on explicit 127.0.0.1 ports;
  dual-view readiness requires the actual owner connection. No broader native
  origin or role is trusted.
- UI async cleanup, device-local copy and unsupported-page boundaries were checked;
  physical mobile layout/clipboard policy remains a manual compatibility gate.
- Rotation lacking a new verifiable session becomes unverified. Cost: conservative
  withholding until explicit session/arrival evidence, rather than guessing FID.
- Existing dedicated checkout/new branch and portable ledger helpers were reused;
  previous branches are untouched. Cost: manual task bookkeeping, checked against
  published trees and commit history.
- Both public DTOs use a 256 KiB budget and independently copied deliveries.
  Cost: oversized route evidence is hidden and survey rows can be explicitly
  partial; per-subscriber copying is bounded work.
- Native smoke needed the two known page URLs; the guard stayed narrow and its
  negative-origin/encoded/query/owner-path tests remain mandatory.
- Implementation commits precede CI because GitHub needs a published SHA to run;
  final evidence is a separate handoff-only commit verified afterward.

## Passing verification — correction head 8c359e2

All workflow conclusions were independently checked as completed/success for exact
`8c359e26aa723b655d7dd6f4c7011d07ff382650` on 2026-11-13:

| Immutable CI evidence | Verified result |
| --- | --- |
| [Desktop branch push 43507990581](https://github.com/Jakoczig/daedalus/actions/runs/43507990581) | Windows + Ubuntu 24.04 PASS, exact branch commit build |
| [Desktop PR 43508263938](https://github.com/Jakoczig/daedalus/actions/runs/43508263938) | Windows + Ubuntu PASS against unchanged G2A base |
| [Legacy/security 43508264067](https://github.com/Jakoczig/daedalus/actions/runs/43508264067) | All five jobs PASS: Windows/Linux regressions and security lint, legacy launcher build/smoke, React build and existing advisory style job |
| [Reopened desktop PR 43508191473](https://github.com/Jakoczig/daedalus/actions/runs/43508191473) | Windows + Ubuntu PASS on the same corrected head |
| [Reopened legacy/security 43508191497](https://github.com/Jakoczig/daedalus/actions/runs/43508191497) | All five jobs PASS on the same corrected head |

Inspected exact-commit desktop logs:
[Windows job 132170535818](https://github.com/Jakoczig/daedalus/actions/runs/43507990581/job/132170535818),
[Linux job 132170535418](https://github.com/Jakoczig/daedalus/actions/runs/43507990581/job/132170535418).
Both confirm Go 1.26.10 / Wails 3.0.0, vcs.modified=false and the exact correction
revision. The required gates all passed:

- 63 JavaScript security/legacy/UI regressions; nine build/fixture/package tests;
  mandatory security/build lint; React build/export preserving 62 routes and
  278 embedded assets; pinned module verification and go vet.
- Complete uncached Go tests across all 11 packages on Windows and Ubuntu,
  preserving G1A/G1B/G2A security, profile-lock, TLS/pairing and long-idle tests.
- Full Linux `go test -race` and 25-repeat Linux race checks of prior startup/
  revocation cases plus G2B clear/arrival/FID/source-gap/broadcast/revocation cases.
  The same G2B cases also ran 25 times without race on both OSes.
- Native Windows/Linux builds and **all six** smoke modes per OS: G1A headless and
  real native React; G2A explicit-fixture headless and native journal card; G2B
  explicit-fixture headless route/survey and real native dual-owner-view React.
  Linux uses Xvfb/DBus/software rendering and the existing executable-specific
  AppArmor userns permission, retaining WebKit sandbox/global policy.
- Preview packaging/upload only after the above gates, with no soft security or
  native checks. No release, installer, deployment or game/provider action.

Local correction re-verification also passed: full 11-package Go suite (webapp
71.048 s), vet/modules, 63 JS tests, nine build tests, both lints and focused
25-repeat regressions. Clean Windows correction binary passed all six real
modes. WebView2 157.0.4341.46 logged the inherited nonfatal cleanup warning 1412
following successful shutdown; no smoke failure was hidden. Linux/race execution
is verified in CI; no local Linux installation or Windows cgo race result is
claimed.

### Downloaded experimental previews and independently checked SHA-256

Exact correction-commit branch-push artifacts, retention 14 days:

- [Windows x64 artifact 14078110797](https://github.com/Jakoczig/daedalus/actions/runs/43507990581/artifacts/14078110797)
- [Linux x64 artifact 14078095804](https://github.com/Jakoczig/daedalus/actions/runs/43507990581/artifacts/14078095804)

Both ZIPs were actually downloaded and opened. Each contains exactly its platform
executable, README.md and SHA256SUMS. Both manifest entries match independently
computed uncompressed file hashes; complete ZIP digests match GitHub's artifact
metadata. PE/ELF signatures, experimental/unsigned/no-provider-uploads labels,
Windows WebView2/Linux GTK-WebKit prerequisites and verification instructions were
checked. No user profile, private CA key, token, credentials or journal files are
packaged. README hashes differ because each runner uses its own checkout line
endings; each matches its own manifest. These are not publisher signatures.

| Verified correction artifact file | SHA-256 |
| --- | --- |
| Windows daedalus.exe | `d8efdd2dfd75f2209086edd5c5ed2447ad989d8d5036d6757f2db545eb73454a` |
| Windows README.md | `54100d89a128c23f0a6d9c412011e343c0bf900d6681ea20cbb766a66cd12ac3` |
| Windows ZIP | `e1f0bec9782b976f258555b0790f2b2ddd398712be35f1ab8f070960742985d5` |
| Linux daedalus | `15c637942bdcc78ce0832037038c9998e649fa83c564bd8a2aec29f5d142c978` |
| Linux README.md | `4cdb4292cf6a198cc57a616f28d0ad49418a06c1c7a183f72ed7e2573a3fc20c` |
| Linux ZIP | `21566a1cbd7b891bc63ef7d9e19d6c8bc44e64ce903517be85ced16a74c55b35` |

### Measurements, with limits

Correction-head CI projection-only replay, 50,000 fabricated updates, three
iterations; parser/source IO, network, Wails and UI excluded:

| Runner | ns/op | events/s | allocated B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| Windows amd64 | 273586967 | 182757 | 68421989 | 1150217 |
| Ubuntu amd64 | 175754545 | 284488 | 68421712 | 1150215 |

Earlier pre-correction local projection-only measurement (50,000 synthetic updates, three
iterations, Windows amd64 Ryzen 7 6800U): 194,909,500 ns/op, 256,530 events/s,
68,425,234 allocated B/op and 1,150,214 allocs/op. This excludes source IO/parser,
network, Wails and UI; allocated bytes are not retained RSS. No idle CPU/RAM or
overall legacy-versus-Go efficiency claim.

## Remaining physical acceptance and limitations

G2A real Windows gameplay and laptop updates were confirmed by the maintainer.
G2B route plot/jump/clear, honk/FSS/DSS and paired-laptop updates still need physical
acceptance; CI uses synthetic journals and native React only. Physical Linux
Steam/Proton, distro/GPU/Wayland/AppArmor, mobile certificate trust, sleep/roaming,
large real archives and filesystem/power-loss behavior remain manual. See the
[bundled runbook](../testing/g2b-windows-linux-navigation-exploration-smoke.md).

Not every game route cancellation is journaled; a verified marker is an evidence
claim, not a control channel or guarantee of unreported current game state.
Conservative timestamp matching can leave rapid same-second or incomplete route
evidence unverified. Untimestamped clears require a dated clear or new verified
session before route recovery. Skipped-record gaps require a complete explicit
session/arrival before G2B attribution resumes. Bounded caches are in-memory and are rebuilt by replay;
there is no historical database. Same-identity in-place file edits preserving
size/mtime can evade the inherited reader checks. OS/network filesystem stalls
cannot be forcibly interrupted; the profile lock is retained until readers stop.

The prior G2A minor remains: advanced owner directory details may stay stale after
automatic source recovery until reconnect/owner action; the streamed status is
correct. No native folder picker or legacy preference migration is included.

Keep the original Daedalus. No legacy Node/launcher or dependency pins changed.
No providers/uploads, EDMC/Python/Node runtime, game commands/targeting, speculative
exobiology, neutron optimization, merge, deployment, release or later-phase work.
Stop after G2B and request external review; physical acceptance is not claimed.
