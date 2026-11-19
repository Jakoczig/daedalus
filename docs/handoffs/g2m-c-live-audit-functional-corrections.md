# ERIDION G2M-C — live-audit functional corrections

Stacked on G2M-B `0cb90ec73b28fc0c279d88ad12379810e0892eb3`, targeting `go/g2m-b-navigation-field-context`. Scope: AUD-01/02/04/08/12/13 only. The new draft PR description is the final-head CI, artifact and independently verified checksum ledger; this avoids a documentation-only rebuild after CI. PR #18 is unchanged.

## Correction and before/after evidence

The Windows audit observed valid startup snapshots stored before LoadGame but absent from Commander/Engineering. The synthetic fixture reproduces Fileheader → Music → Commander → Materials → Rank → Progress → Reputation → EngineerProgress → LoadGame → Location. Before the correction its regressions failed with empty ranks/engineers and unknown materials. Afterward the same records produce three ranks, three reputation values, one unlocked engineer and 91 iron. Commander shows Combat rank 3 / 27%, Explore rank 5 / 42%, Trade rank 4; Engineering shows Felicity Farseer unlocked, rank 4 / 0.75 progress and the same material stock. These values are fabricated, not the owner's private counts.

The new private startup collector retains only five snapshot types, with a 64-record / 512-KiB / two-minute bound. A valid Commander must precede them, followed by matching same-file/path LoadGame identity. Exact byte/sequence continuity, monotonic times, strict JSON, duplicate-key/category detection and existing model validation reject ambiguous, missing, malformed, truncated and cross-commander evidence. No provisional snapshot is published. The restoration capability is sealed to its exact LoadGame and cannot overwrite a newer model cursor. No counters, synthetic history entries or provider/upload callbacks are replayed.

The collector persists in the existing atomic checkpoint cache, including a new-file prefix split before LoadGame and across restart. Old caches automatically rebuild existing derived projections once from SQLite, with no new journal event required; future starts use the corrected cache. Original events/checkpoints/schema are retained. A verified Shutdown retains account-scoped last-known material stock while clearing live telemetry and authorization to apply later deltas; the next header/session/gap retains existing invalidation. Ranks/engineers already retain last-known state at Shutdown.

Account-isolation regressions cover valid A → B with no snapshots and valid distinct B values, foreign FID/file/path, duplicate/out-of-order/gapped records, old evidence, malformed material/engineer rows, pre-binding withholding, cache upgrade/restart/idempotency and newer-live-state protection. A paired-viewer RPC test confirms the existing workspace allowlist contains no FID, commander name or source path and blocks private Commander/ledger/export RPCs. No permission or publication-policy changes.

## UI corrections

- Global shortcuts ignore editing/interactive controls, modified arrow chords and IME; no arrow-key input blur. Existing intentional Alt+1–7 section shortcuts remain available outside controls.
- Missions → Local Activity opens the existing collapsed, deliberately accessed owner-only ledger. Overview/Session layouts and server permissions are unchanged; viewers receive an owner-only notice.
- Frontier status distinguishes a recorded successful authorized read from adapter contract evidence and cache freshness. EDSM settings refresh shared **local metadata** every 15 seconds, without new provider calls.
- Route displays “Not reported” for an empty target. Trade root opens the implemented comparison; remembered explicit trade pages remain intact.

## Verification and limitations

Focused startup tests were observed failing before implementation and passing afterward. Local JavaScript suite: 142/142; asset/workflow tests: 24/24; security/assets lint passed; React static export compiled all 65 pages. A local overlapping asset-export/Go-build attempt encountered a generated-directory file lock; exporting again sequentially and rerunning the three affected Go packages passed. All other Go packages, including history/security, passed; native Go build/vet and Windows offline/native React bootstrap/shutdown passed. The final comprehensive Windows/Linux CI runs all Go/JS tests, security gates, Linux race detection, database regressions, native smoke tests, all existing browser suites and the new G2M-C browser checks before packaging.

New real Chromium evidence is in the `eridion-g2mc-synthetic-evidence-Windows/Linux` artifacts: Commander/engineer/material startup projections, actual focused dropdown arrow inputs, deliberate Local Activity, unknown target, Trade landing and remembered navigation. External requests are intercepted and prohibited. This is rendered synthetic verification, not live gameplay or physical laptop acceptance.

The Windows control inventory during this correction showed no running ERIDION or game window. The original private profile/application was not opened, replaced, reauthenticated or modified for testing. Physical private-profile acceptance remains pending. Incomplete sources (including journals missing their identity/header prefix) conservatively remain unknown. Deferred AUD-03/05/06/07/09/10/11 remain for G2M-D; no claim that their appearance/reference gaps are fixed.

## Manual acceptance

1. Back up the private profile and quit the older native preview; verify both manifest entries before starting the unsigned preview.
2. Compare actual ranks/progress/reputation, engineers and material stocks, then restart without new gameplay events (also after closing the game). Last-known snapshots must remain distinct from live Status.
3. Switch commanders and compare their independent snapshots; on a paired laptop verify private Local Activity/history and owner actions stay blocked. Leave uploads disabled.
4. Use arrows in Inventory/Spansh selectors and ordinary editing; check plain navigation outside controls. Check Local Activity stays collapsed until opened, Frontier/cache wording, unknown Route target and remembered Trade subpages. No new Frontier login is required.
