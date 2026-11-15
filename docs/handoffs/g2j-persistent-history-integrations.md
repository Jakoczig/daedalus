# G2J — persistent history and controlled integrations

Stacked on G2H `d7c5cd93e16ee0480fa11ef53d2ea9d9d0d4cb58`. PR #13 remains unchanged.
The new draft is on `go/g2j-persistent-history-integrations`, targeting
`go/g2h-unified-navigation-integrations`; its final head, exact CI run links and
artifact digests are recorded in that PR's consolidated delivery report.

Implementation commits:

- [`eea3608`](https://github.com/Jakoczig/daedalus/commit/eea360867e7a1b5967911c2da0f88682a9011084): native SQLite WAL, normalized retained history, atomic checkpoints/reducers, migrations and verified recovery.
- [`72815f2`](https://github.com/Jakoczig/daedalus/commit/72815f23d8e969ee51014e2e5256e839ee95eb2a): canonical consent/provenance, publication review, reference adapters and schema-validated station sharing.
- [`a79e05b`](https://github.com/Jakoczig/daedalus/commit/a79e05bebb05e505e6118290ef2f05410bb4f4cf): verified nearby services, construction history, copy/fingerprint safeguards and exact identifiers. Final delivery additionally covers accepted-sidecar persistence, safe restoration of exact route ownership, offline cached routes, provenance JSON precision and mandatory G2J artifact/visual/performance gates.

Working local functionality: cached startup, first import and incremental catch-up;
per-commander indexed history; whole-file verified copy aliases; explicit quarantine
reimport; backup/export/staged restore; source conflicts/freshness; Never/Live/After
sale and exact recipient/category live exceptions; sale evidence and bounded
historical preview/reconfirmation with durable outcomes; observed final module
modifiers and native versioned power sums. Paired viewers cannot access private
history, credentials, provider actions, recovery or publication authorization.

New read adapters implement published Spansh lookups, bounded Elite BGS pages,
Raven project references, filtered Canonn sites and EDSM nearby/station/inventory
references. EDDN station messages require current verified docking and an exact
committed marker/sidecar timestamp. All new consent starts off. See the concise
[provider capability/approval matrix and gameplay checklist](../testing/g2j-persistent-history-preview.md).

Local Windows verification (followed by affected regression/visual reruns after final corrections): full Go suite (20 packages), Go vet/module verification,
120 JavaScript regressions, security lint, React export (287 embedded assets),
19 asset tests and all 13 executable/native smoke modes. G2J browser checks cover
1366×768 and 680×580, zero horizontal overflow, cached readiness, disabled historical
publication and final engineering evidence; no runtime errors or external requests.
The Windows PE imports only Windows/UCRT DLLs. Linux/race/native verification and
final-head preview hashes must be taken from the final PR's CI evidence, not this
earlier local run or any G2H artifact.

## Measured lifecycle evidence

One local development Windows Ryzen 7 6800U run before the final cache corrections; final-head CI repeats the measurements. Fixture: 100,000 synthetic lines/two FIDs plus a 1,000-record
seven-day catch-up. Baseline is an isolated archive of the exact G2H base with the
same fixture and only its existing reader byte counter exposed for measurement.

| Metric | G2H baseline | G2J |
|---|---:|---:|
| Initial import | 771.5 ms | 5,760 ms |
| Import throughput | 129,614 events/s | 17,363 events/s |
| Unchanged restart | 761.2 ms | 190.0 ms |
| Restart journal payload bytes | 10,599,956 | 0 |
| Missed-app catch-up | 769.3 ms | 251.0 ms |
| Catch-up journal payload bytes | 10,705,956 | 106,000 |
| Cached view during catch-up | unavailable | 184.1 ms |
| Core benchmark process peak RSS | 52,879,360 bytes | 61,534,208 bytes |

Durable normalization costs first-import time and about 8.3 MiB additional peak RSS
in this fixture, while avoiding full restart replay. Byte metrics count parsed
payload, excluding bounded source fingerprint probes. RSS is the core benchmark
process, not the complete native GUI/WebView process tree. Renderer heap in the
G2J visual fixture was 4,722,004 bytes after GC; this is not total application RAM.
CI repeats the lifecycle comparison independently on Windows and Linux.

Remaining gates: Frontier developer/callback registration, Inara whitelisting and
live account/publication acceptance; Raven production write authorization; optional
CAPI journal recovery/community-goal response fixtures; broader licensed/gold-tested
ship performance models. Unknowns are explicit, not base-value approximations.
Physical gameplay, Linux/Proton/Wayland/tray/keyring and certificate/laptop tests
remain the consolidated manual checklist. No production upload, merge, release,
deployment or legacy replacement was performed.
