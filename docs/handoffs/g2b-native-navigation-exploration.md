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

Later canonical implementation commits are added after publication; local
metadata hashes are not final published identifiers. Every connector-published
tree is checked against its tested local tree.

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

## Verification status

Implementation verification is still in progress. Do not infer Windows/Linux CI,
race/native success or artifact hashes from workflow wiring. Exact current-head
CI results, published commit hashes and independently inspected preview ZIPs will
replace this section after completion. Earlier G2A acceptance is baseline only.

Local projection-only replay measurement (50,000 synthetic updates, three
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
evidence unverified. Bounded caches are in-memory and are rebuilt by replay;
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
