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
