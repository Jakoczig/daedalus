# G2L — Fleet, Economy & Commander Operations

Base: accepted G2K `fd721266d93bf86976de029d5ce3e626e57a6786`.
Branch: `go/g2l-fleet-economy-commander-operations`, stacked draft targeting G2K.
The final draft PR report records the implementation commit, exact-head CI runs,
Windows/Linux artifact links, independently verified SHA-256 hashes and measurements.
PRs #13/#14/#15 remain unchanged/unmerged. Maintainer-reported G2K gameplay PASS
is preserved. The maintainer’s unspecified visual rough edges are deferred; no
unrelated redesign or new physical/live-service acceptance is claimed.

Implemented: additive schema v6 with verified pre-migration backup; atomic indexed
fleet/module/carrier/trade/market/mining/CG/Powerplay/crew/squadron projections in
the existing journal transaction. Durable migration backfill streams bounded batches
from retained records. Missing old payloads remain unavailable until the owner
selects an original source: exact recorded offsets/line hashes are verified before
an atomic recovery. Failure rolls back; source cursors and upload policies do not
change. Copied/reimported physical records use header+offset+line-hash provenance
for accounting deduplication. Genuine repeated records at different offsets remain.

Fleet handles distinct exact IDs, old snapshot precedence, explicit sale/storage,
in-transit requests, latest empty stored-module snapshots and exact historical
Loadout modifiers. Carrier ownership requires purchase/owner management evidence;
matching owned arrivals can close requests, unrelated carriers cannot. Finances,
services, orders and location each retain independent observation timestamps.
Trade uses exact decimal arithmetic, only positive game-reported cost basis for
gross spreads; page totals distinguish expenditure from receipts. Mining sales
require a witnessed empty cargo baseline and unmixed same-session refined input.
Market persistence now works without EDDN consent while preserving the existing
upload gates; dock, marker, source identity and timestamp checks precede storage.

Private pages are server-owner-only. No new fields enter paired-display feeds.
All queries use fixed SQL and bounded cursor pages; IDs/money survive JSON as
strings. CSV export binds acknowledgment to a hash of the previewed page and
neutralizes spreadsheet formulas. Built-in toggles preserve history and backfill
on explicit enable; no arbitrary plugin execution or additional runtime backend.

New retained families: CarrierBuy/Finance/BankTransfer/DepositFuel/TradeOrder/
CrewServices/ShipPack/ModulePack/JumpCancelled/Decommission/CancelDecommission/
DockingPermission/NameChanged; ShipyardNew/SellShipOnRebuy, module storage/transfer
variants, MiningRefined/CollectCargo/EjectCargo/CargoTransfer/CargoDepot,
CommunityGoalDiscard, CrewHire/Assign/Fire, WingJoin/Add/Leave/Invite, JoinACrew/
QuitACrew/ChangeCrewRole and documented squadron variants. Previously retained
Loadout/StoredShips/StoredModules/MarketBuy/Sell/Powerplay/CG records are reused.
Only allowlisted operation fields are projected; raw unrelated text stays excluded.

Focused regressions cover exact >JS-safe IDs, known/unknown cost basis, copied
sources, FID isolation, original recovery/rollback/idempotence, empty snapshots,
old active-ship/carrier finance evidence, owned jump confirmation, module toggles,
viewer RPC denials and acknowledged exports. Final CI retains all G1A–G2K gates,
adds G2L two-viewport real-browser checks and local-market ingestion with uploads
OFF, plus same-run exact-G2K 100k/1k lifecycle and 1k fleet/10k trade/50k quote/
1k goal query benchmarks. Resource evidence separates Go fixture RSS/CPU and
Chromium heap/main-thread work; full Wails/WebView overhead is excluded. Single
short runs are not statistically significant comparisons.

[Capabilities, prerequisites and single gameplay/laptop checklist](../testing/g2l-fleet-economy-preview.md).

Provider limits: Frontier profile remains preferred cached external fleet evidence,
not newer journal replacement without trustworthy observation time. Documented
Frontier `/communitygoals` GET remains approval-gated and its body normalization
awaits a verified authorized fixture. Inara public goals retain separate consent;
selected EDSM market cache requires verified identity/source age. No new guessed
APIs, global best prices, carrier/game commands, production uploads or credentials
in SQLite. External authorization and real gameplay remain manual requirements.

Primary contracts: [journal Station Services](https://elite-journal.readthedocs.io/en/latest/Station%20Services.html),
[Fleet Carriers](https://elite-journal.readthedocs.io/en/latest/Fleet%20Carriers.html),
[Trade](https://elite-journal.readthedocs.io/en/latest/Trade.html),
[Powerplay](https://elite-journal.readthedocs.io/en/latest/Powerplay.html),
[Squadrons](https://elite-journal.readthedocs.io/en/latest/Squadrons.html),
[Frontier CAPI endpoints](https://github.com/EDCD/FDevIDs/blob/master/Frontier%20API/FrontierDevelopments-CAPI-endpoints.md).
No upstream implementation code copied or dependencies added.


## Review correction — complete CommunityGoal snapshots

Review: [6957005043](https://github.com/Jakoczig/daedalus/pull/16#issuecomment-6957005043).
`CurrentGoals` is now validated as a bounded complete snapshot. A per-FID durable
marker includes explicit empty lists; only strictly newer observation times replace
membership. Missing/null/malformed/duplicate-ID/oversized arrays cannot clear it.
All contribution, participation, discard and received-reward history remains.
Current rows distinguish subscribed-as-last-witnessed, historical and unknown;
expiry/bonus/completion never implies payment or automatic membership changes.
The label is explicitly membership at the complete snapshot time, not a live claim.

Existing reviewed v6 profiles receive a one-time, keyset-batched repair from
retained CommunityGoal events, including old empty snapshots. This changes only
derived current state and an operations-state marker: no migration/checksum,
immutable history, original-source reread, checkpoint or upload permission changes.
Subsequent warm opens check one marker rather than replaying journals. Explicit
backfill and restart preserve newer snapshot evidence. Regressions cover A→empty,
A+B→B, older replay, malformed/missing arrays, two FIDs, expiry, restart/old-profile
repair and injected transaction rollback. React and real browser checks verify the
known-empty display while historical reward records remain visible.

## Controlled Windows import investigation

[Full samples, environment, stage metrics and CPU summaries](../benchmarks/g2l-controlled-windows-review.json).
Exact G2K `fd72126` and reviewed G2L `54a90b3`; immutable source archives plus an
identical test-only mixed-workload overlay. Go 1.26.10 / GCC 16.2, same Windows 11
PC, fresh process/profile per run, order AB/BA/AB, no concurrent build/test workload.
Three paired runs for each workload (12 timing invocations), then one bounded
four-run diagnostic pass in disposable instrumented copies only.

| Cold import | G2K samples / median | G2L samples / median |
|---|---|---|
| Original 100k material workload | 5818, 6163, 6083 / **6083 ms** | 6449, 6496, 6727 / **6496 ms** |
| 10k mixed fleet/carrier/trade/CG/crew | 1011, 1022, 1461 / **1022 ms** | 2285, 2623, 3479 / **2623 ms** |

The original CI +87% material increase did not reproduce here. A smaller +6.8%
median difference remains; its full cause is not established. This is one host
and three pairs, not a statistical or cross-hardware conclusion. The mixed case
is intentionally not feature-equivalent: G2K has no G2L operation projections.
Diagnostic mixed normalization rose 483.5→1672 ms; 1102 ms was in the 8997 new
operation projections. Journal decoding was 65.32→66.26 ms and durable commit
710→400.4 ms. Material performed zero operation projections; durable commit was
1839→1859 ms, normalization 2670→2812 ms. These diagnostic intervals overlap and
material includes original warm/catch-up stages; do not add them or substitute
for the uninstrumented cold samples. CPU profiles point primarily to SQLite/CGo,
without evidence for a journal decoder or large transaction-durability regression.

No speculative performance fix or SQLite/schema/durability tuning was applied.
The CI host's original large variance is still not causally explained. Reproduce
with `python scripts/testing/g2l-controlled-windows.py --prepare`, then `--run`
with the printed isolated directory; `--profile` is a separate optional diagnostic
only after a reproduced difference. Script GO/CC paths describe this workspace;
adapt to equivalent pinned Windows tools on another machine. Generated profiles
stay under ignored temp directories and are never packaged in previews.

The final PR report records the correction commit, exact-head final CI and refreshed
Windows/Linux artifact/executable SHA-256 verification. Physical G2L/laptop and live
provider acceptance remain manual; no release/merge/deploy/production upload.
