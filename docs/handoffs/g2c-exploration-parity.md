# G2C exploration batch — external-review handoff

[Draft PR #8](https://github.com/Jakoczig/daedalus/pull/8), branch
`go/g2c-exploration-parity`, stacked against accepted G2B
`2a17d40c2dd5d2d44447556d62cfce6104350bae`. Previous PRs and legacy Node/Windows
application remain unchanged. No merge, release, deployment or replacement.

## Working features

- Six existing navigation/exploration routes now expose a selectable schematic
  system map, a searchable body list, physical/orbital details, rings/compositions,
  confirmed-biological worklists and enhanced plotted-route browsing. The map
  reuses the original SVG body/ring renderer and styles. Parent IDs remain exact;
  missing/barycentric/cyclic relationships stay unresolved rather than guessed.
- Journal-only Log → Sample → Analyse tracking, confirmed genus/species/variant,
  duplicate-safe stages, interruption by another organism and unknown missing
  starts. Biological evidence can appear before a body Scan. Analysis is historical
  completion, not unsold inventory or repeated-sampling tracking.
- Local filters, memoized hierarchy work and bounded rendering: 25 body rows/map
  entries and 80 waypoints per page; current/next focus and device-local copy.
  KGB FOAM annotations describe witnessed star class, not fuel safety.
- One existing Go reader, commander/system/source isolation, 256 KiB allowlisted
  DTOs, immutable nested copies and cached survey snapshots. Existing TLS/pairing,
  owner/viewer permissions and serialized per-send revocation remain. Tests now
  independently observe a post-revoke organic producer on the owner connection
  and prove its new notifications cannot reach the revoked viewer.

## Commits, review and verification

Implementation: `973f942e7792d83b2fe4fd002d98311d69c0ba87`.
Consolidated-review correction: `df301338d69c58456fcb93be2e0db98824717e8b`.
One fresh whole-batch review found no Critical/Important blocker. Its three UI
findings were reproduced with failing tests and corrected together: star-code
adaptation to legacy styles, parent provenance through filtering and genus-only
biology counts. No security assertion, timeout or mandatory gate was weakened.

Verified exact source head `df301338d69c58456fcb93be2e0db98824717e8b`:

- [Windows/Linux desktop push](https://github.com/Jakoczig/daedalus/actions/runs/43522455578)
  and [desktop PR checks](https://github.com/Jakoczig/daedalus/actions/runs/43522464206):
  all jobs completed/success. Full 11-package Go tests, vet, pinned modules,
  **72 JS tests, 10 build tests**, security/build lint, React export (62 routes,
  278 assets), Windows/Linux native builds and **all eight actual headless/native
  smoke modes** passed. Existing six modes retain their G1A/G2A/G2B behavior;
  two explicit-fixture G2C modes verify map/details/organic data and native React.
- Linux full race detection plus 25-repeat G2C, startup, revocation, navigation
  provenance and other G2B regression selections passed; both OSes passed repeat
  stress. CI logs identify clean exact-head executables, not a nearby commit.
- [Legacy/security CI](https://github.com/Jakoczig/daedalus/actions/runs/43522464207):
  all five Windows/Linux regression/lint/client/original-app build jobs passed.
- Local full Go/vet/module verification, JS/security/React checks and Windows build
  passed; all eight real Windows smoke modes passed. Local Linux or Windows race
  is not claimed; Linux native/race evidence is from Actions.

The final handoff-only commit changes this document alone. Its own fresh CI and
artifact verification are recorded in PR #8, avoiding a self-referencing doc loop.

## Experimental previews

[Windows ZIP](https://github.com/Jakoczig/daedalus/actions/runs/43522455578/artifacts/14084486053)
· [Linux ZIP](https://github.com/Jakoczig/daedalus/actions/runs/43522455578/artifacts/14082994912).
Both were downloaded/opened: exactly executable, README.md and SHA256SUMS; file
hashes and GitHub archive digests match. PE/ELF signatures, experimental/unsigned
labels and runtime prerequisites were checked. No profiles, private keys, tokens,
credentials or journals are packaged. Retention is 14 days; no Release/installer.

| Source-head file | SHA-256 |
| --- | --- |
| Windows executable | `b6e827a456e09fa86ad269365455a9de0e9d0e5b1c58b1c4e627c8bcce22d466` |
| Windows README | `1debfc54a302f68e115eba65961cde3be3b7f7c24060b3281fb29c5c84016ecd` |
| Windows ZIP | `68c16a776b847c15d5288843ba69ca7c0cc665b4539af4291332b81fb244f1a7` |
| Linux executable | `2cfc89438291148211fe60b18e9e8edd522ff324a948697fc8e6196846ebdef7` |
| Linux README | `f165254486c5a40fe2915104d29402462abf37a31a14f8ff187993ce2cf4864c` |
| Linux ZIP | `6759f3860096e3da04f7036f39061191ff4e8b0f023a5d6be10dd66b01dc51c3` |

## Performance and limits

Same Windows Ryzen 6800U, fixed 500-body snapshot workload, 200 iterations:
accepted-G2B reducer **535270 → 106089 ns/read**, **467505 → 113116 allocated
B/read**, **1523 → 508 allocations/read** with G2C caching. About 5.0× faster
repeated snapshot reads and 76% fewer allocated bytes; each returned view remains
independent. Source-head CI measured 105228 ns/read on Windows and 41813 on Ubuntu;
those different runners are not a platform comparison. No total-app, legacy-Node,
idle CPU/RSS, retained-memory or end-to-end throughput improvement is claimed.

No unresolved Critical/Important source-review findings. Deliberate limits:
schematic map, not an orrery; no provider data/uploads, predicted species/payouts,
first-footfall assumptions, surface-position radar, sale inventory, fuel modelling
or ship/engineering parity. Missing physical fields remain unknown; malformed
updates can retain prior valid observations with a partial label. Caches are
bounded/in-memory and reconstructed by replay. Sampler evidence after restart is
explicitly last-known; historical Analyse does not start a new sampling cycle.
Map/table order can differ; selected details may remain on another page. Route
pages refocus on current/next during updates. Signal-only bodies can use ID labels.
Physical gameplay, Linux/Proton/distro/GPU/Wayland/AppArmor, mobile certificate trust,
clipboard, sleep/roaming and large real archives remain manual. The inherited
advanced-owner-source-details refresh limitation also remains. Keep the legacy app.

## One real-game acceptance pass

1. Plot/jump/clear a route; verify current/next, search, paging, local copy and
   stale/unverified evidence on both the desktop and paired laptop.
2. Honk/FSS/DSS a star, planet and moon; compare hierarchy, selection, class,
   radius/temperature/pressure/orbital units, rings and mapping evidence to the game.
3. Check biological signals/genus, Log → Sample → Analyse, interruption, duplicate
   safety, restart/replay and commander/system changes. No foreign or invented
   progress should appear. Confirm historical completion is distinguished from sales.
4. Repeat on a physical Linux/Proton host and mobile browser; test trusted HTTPS,
   disconnect/reconnect, idle/sleep, readonly controls and immediate revocation.

[Installation, certificate/pairing and detailed smoke instructions](../testing/g2c-exploration-preview.md).
ScanOrganic semantics follow the [official journal manual](https://hosting.zaonce.net/community/journal/v32/Journal_Manual-v32.pdf)
and existing legacy handling; fixtures are entirely fabricated. Stop after G2C.
