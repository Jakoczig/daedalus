# G2K — Native Field Operations

Base: G2J `2adbc86fcfc68d6a2d7822b219640a386bed738b`.
Branch: `go/g2k-native-field-operations`, stacked draft targeting
`go/g2j-persistent-history-integrations`. PRs #13/#14 stay unchanged/unmerged.
The final PR delivery report is authoritative for implementation commit, exact-head
Windows/Linux CI, preview ZIP/executable SHA-256 hashes and measured comparisons.

Implemented: native bounded CSV parsing, persisted itinerary/progress/manual
corrections, owner clipboard/save dialogs; typed journal-derived BGS rows with
multi-system mission effects and UTC/verified-closed-tick grouping; licensed pinned
BioScan candidates and correlated surface snapshots; one canonical Go power-scenario
engine and explicit unknowns; built-in module controls and credits. Existing UI,
SQLite checkpoint transactions/recovery, single reader, HTTPS pairing and upload
permissions are preserved. Schema 5 is additive with verified pre-migration backup;
old migration checksums remain unchanged. BGS backfill runs once for migration or
explicit module re-enable, not during rendering or on a recurring timer.

Focused tests cover CSV/arrival bounds, multi-system influence, cursor paging,
module re-enable deduplication, migration backup, status correlation/haversine,
licensed criteria states, native scenario arithmetic and owner/viewer privileges.
The old frontend arithmetic regression was ported to Go; React tests now verify
presentation without independent calculations. Final CI retains all earlier gates
and adds G2K browser/import/tally/BioScan screenshots, exact-G2J lifecycle baseline,
100k-row field queries and idle fixture resource evidence before packaging.

[Capability/permission matrix and consolidated gameplay checklist](../testing/g2k-native-field-operations-preview.md)
· [Pinned source credits and licenses](../licenses/g2k/README.md).

Limitations: no auto-copy, direct route generation, game commands, species/variant
guarantees, exact sample pins, causal BGS gain or unvalidated ship performance.
Captured synthetic engineering inputs are regression fixtures, not three accepted
real-game gold builds. Those builds, native file-dialog/clipboard gameplay,
physical Linux/Proton/Wayland and paired-device acceptance remain manual. Live
provider approval and production upload acceptance are not implied by G2J gameplay
acceptance. No merge, release, deployment, legacy replacement or production upload.


Review correction batch (review comment 6907617913, baseline `451cfd9`):
- Historical BioScan spacing now requires a nonempty, verified exact session and
  a witnessed Log→Sample prefix or Log→Sample→Analyse cycle. Missing points,
  restarted/interrupted cycles, source/sequence gaps and body/session boundaries
  withhold distance and remaining spacing. Proof is bounded to 4,096 journal
  records and the existing 24-sample window; evidence outside either is unknown.
- Static native power scenarios use FID, exact ship ID, fitted module evidence
  and formula version, excluding Status and module health/ammunition. One encoded
  in-memory entry avoids repeated SQLite reads; existing bounded persistent cache
  survives restarts. Current observed deployment is overlaid after every cache
  lookup. React refetches on fitted/deployment evidence, not Status timestamps.
- New deterministic regressions failed against the reviewed implementation:
  cross-session/incomplete BioScan cycles produced spacing, and 100 Status-only
  changes created 100 extra cache writes. Corrected tests require zero extra
  writes, valid within-cycle distances, On/priority/modifier invalidation and
  fresh deployment on persistent-cache reuse. No migration or formula change.
- Final CI adds the 1,000-update native statistics benchmark and an open Ship
  Statistics browser workload: exact-timestamp waits, 12 live sidecar updates,
  zero volatile refetches and exactly one deployment refetch. Windows local
  synthetic workload: 6.24 s, 28.2 MiB Go fixture RSS, 0.75% one-core CPU,
  10.8 MiB browser heap, 2.35% browser main-thread time. These short synthetic
  figures exclude Wails/WebView overhead and are not gameplay guarantees.

Final correction commit, exact-head Windows/Linux CI, downloadable corrected
previews and independently verified SHA-256 manifests are recorded in the
[PR #15 final verification report](https://github.com/Jakoczig/daedalus/pull/15).
Manual acceptance remains the single checklist linked above, especially restart
on the same biological body and live hardpoint changes with a paired laptop.
