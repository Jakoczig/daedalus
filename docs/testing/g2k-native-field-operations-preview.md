# G2K — experimental, unsigned, local-first field operations preview

Windows x64: extract to a separate directory, verify SHA256SUMS with
`Get-FileHash -Algorithm SHA256`, then run `daedalus.exe`. Windows 10/11 x64 and
Microsoft Edge WebView2 runtime are required. This is unsigned experimental
software, not an installer or replacement for the original DAEDALUS.

Linux x64: verify `sha256sum -c SHA256SUMS`, `chmod +x daedalus`, run from a graphical
session with GTK4, WebKitGTK 6.0 and the distribution's C runtime installed
(Ubuntu 24.04: `libgtk-4-1 libwebkitgtk-6.0-4`). Secret Service is required only for
optional credential storage. No EDMC, Python or Node runtime is required.

Use a separate preview profile via `--data-dir <private-directory>` if testing
recovery. Existing G2J profiles migrate additively to schema 5 after a verified
pre-migration backup. Do not downgrade that profile with an older binary; keep
the verified backup. Never share the profile or private CA key.

| Capability | Working / boundary |
|---|---|
| Itinerary | Owner-selected UTF-8 CSV, 512 KiB/2,048 unique waypoints, persisted per commander, current/next/previous, manual preview/corrections, export, native owner clipboard. Target events never advance; address match preferred, names labeled weaker. Detours pause. Imported plan remains separate from GAME ROUTE. |
| Neutron/fuel | Actual historical BoostValue (including witnessed 6x), JetConeDamage warning and FuelScoop amount/total/timestamps. Planner neutron/refuel flags labeled. No inferred active boost, range/fuel-cost/scoop safety or game targeting. Auto-copy deliberately not enabled. |
| BGS | Indexed typed rows from the single committed journal stream; mission FactionEffects use impacted SystemAddress and ordinal influence/reputation labels. Trade, awards/redemption, colonisation and reported Powerplay amounts remain separate facts. Owner filters and local page export; exact event/ordinal paging; no uploads. |
| Tick grouping | Opt-in existing Elite BGS reads can retain verified boundaries. Closed intervals re-bucket at query time without duplicate activity. Missing/stale/open intervals show UTC day, explicitly not a tick. No automatic polling or provider consent change. |
| BioScan | Existing confirmed observations/progress preserved. 116 pinned candidate rules, 23 genus distance references, unknown/excluded/possible distinctions. Unsupported predicates remain unknown; no variant/nebula prediction or probability. |
| Sampling assistance | Live correlated Status snapshots only: matching current commander/system/body/name, HasLatLong, finite coordinates/radius, <=2s from organic event and <=5s from current clock. Approximate snapshot, never an exact pin. Haversine distances within the matching Log/Sample/Analyse cycle and current walking-to-sample spacing where that same active source session is verified; absent evidence stays unknown. Replay cannot manufacture old locations. |
| Ship calculations | One canonical Go v2 engine; React only presents results. Final modifiers never multiplied again; unengineered bundled reference draws explicitly identified. Known rated draw, complete stowed/deployed scenarios, priorities and observed-deployment scenario. Damage/emergency output and broader flight/shield/weapon/range models remain unverified/unavailable. |
| Built-in modules | Owner toggles itinerary/BGS/BioScan without deleting history or stopping the journal reader. Re-enabled BGS backfills retained events once; resuming itinerary may need manual correction. Credits and pinned licenses included. No arbitrary plugin install/runtime, overlay or update execution. |
| Paired display | Allowlisted current/next itinerary and BioScan facts; no commander identity, private history/tally, exact sample coordinates, file import/export, clipboard, module controls or provider authorization. Existing TLS pairing/revocation preserved. |

All provider/upload permissions are unchanged and off by default. G2J After sale
remains hold + manual review only. No production uploads or undocumented Spansh
route-generation calls occur. Legacy Node service and Windows launcher remain intact.

## One gameplay / paired-laptop checklist

1. Import a real Spansh CSV; compare IMPORTED ITINERARY with GAME ROUTE. Check
   current/next, target-without-arrival, one verified arrival, detour pause,
   manual correction, saved plan switch, native copy and CSV save dialogs.
2. Restart after missed jumps; confirm cached progress catches up once, then switch
   commander. Verify a witnessed boost and timestamps without assuming current 4x/6x.
3. Complete a multi-faction mission; check impacted system IDs, separate reward/INF
   labels, UTC-day fallback and consented verified closed-tick grouping. Inspect a
   local export warning and file. Do not infer actual influence gain from activity.
4. Scan a biological body: compare confirmed species with possible/unknown criteria;
   Log/Sample/Analyse and approximate correlated positions. Move/change body/restart:
   stale or missing positions must be unknown, with no fabricated eligibility.
5. Compare native final modifiers and rated scenarios on an engineered, near-stock
   and unknown/new-module ship. Real gold-reference performance models remain manual;
   unsupported flight/shield/weapon/range results must remain unavailable.
6. Pair a laptop using the existing HTTPS/CA trust and one-time-code workflow. Verify
   read-only itinerary/BioScan updates, reconnect and revoke. Check owner-only actions
   remain unavailable remotely. Exercise native Windows/Linux clipboard/save dialogs,
   Proton paths, Wayland/tray/keyring separately where available.

G2J physical gameplay acceptance was reported passed by the maintainer and is not
repeated/claimed as G2K acceptance. The final draft PR records exact-head CI,
artifact hashes, 100k/1k lifecycle comparison against exact G2J, indexed field-query
latencies and idle fixture CPU/RSS. These synthetic figures are not whole-native
GUI/WebView RAM or a real-game benchmark. Preview archives contain only executable,
this README and SHA256SUMS; no journal, database, profile, token or private key.

The mounted BioScan inspector refreshes its bounded indexed snapshot at 1.5s; it stops on disconnect/unmount. It uses the existing Status reader and never replays history or creates a background map watcher. Exact body identifiers are stored as TEXT, including values above signed 64-bit and JavaScript-safe integer ranges.
