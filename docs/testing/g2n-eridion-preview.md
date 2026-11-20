# ERIDION G2N — experimental, unsigned x64 preview

Local-first Commander and Fleet completion, stacked on accepted G2M-C. This is not a release or installer. Upload permissions remain unchanged and off by default. Preserve the original legacy application and back up the private native profile before testing.

## Verify and run

The archive contains only `eridion.exe` (Windows) or `eridion` (Linux), this README and `SHA256SUMS`. Windows: compare `Get-FileHash .\eridion.exe -Algorithm SHA256` and `Get-FileHash .\README.md -Algorithm SHA256` with both manifest entries. Linux: `sha256sum -c SHA256SUMS`, then `chmod +x eridion` and `./eridion`.

Windows x64 requires Edge WebView2 Runtime and a desktop session. Linux x64 requires glibc, GTK4, WebKitGTK 6.0 and a desktop session; saved credentials require Secret Service. Ubuntu 24.04 runtime packages: `libgtk-4-1 libwebkitgtk-6.0-4`. Keep the WebView sandbox enabled. No Node, Python or EDMC runtime is required.

Quit an older native preview normally before running this one. Exclusive profile locking, SQLite history/checkpoints, stored credentials, HTTPS pairing and publication policies are retained. No new SQLite schema or automatic provider grant is introduced.

## Capability and source policy

- Multi-account mode is controlled in Settings. Each new service process requires an explicit launcher choice before opening the terminal. Settings portraits remain local to the selected commander. Single-account offline operation remains available.
- Frontier is the first external account source. Connected, read-consented Frontier suppresses overlapping Inara profile/goals requests. Witnessed live game facts, exact fitted engineering and journal participation remain distinct from CAPI snapshots with unknown observation times.
- Community Goals combines exact-ID Frontier global metadata with separately witnessed subscription/contribution/rewards. Use **Refresh Frontier goals** for an explicit read through the existing authenticated `/communitygoals` adapter. No automatic external fetch, inferred subscription, export or new authorization occurs. Expired caches remain labeled.
- Fleet supports owned/sold/in-transit views and bounded journal/CAPI paging; witnessed sales/transfers are never overwritten by older provider references. Expand a ship for its saved build summaries. Stored modules are grouped by system. Some newer module/hull references remain unsupported; no class or performance is guessed.
- Activity summaries are integrated into Missions, Powerplay, Colonisation and Trade. They summarize the visible bounded page by system/faction/period/category/unit; influence ratings are never added as percentage changes, and merit balances are never summed as earned merits. Individual records and the existing deliberate private export remain available. No reporting is enabled.
- The existing eleven official Spansh workflows, separate game/imported routes, manual clipboard copy, durable history/capacity management and three aligned Galaxy panels remain. Real galaxy-density assets/research are deferred; current artwork remains explicitly illustrative. No game targeting or undocumented route-generation API exists.

## Gameplay / paired-laptop checklist

1. Enable multi-account mode in Settings. On launch choose a verified commander, open the terminal, then restart and confirm the launcher asks again. Check that switching accounts does not move credentials, history or paired grants between commanders.
2. Set a local portrait in Settings; check the Overview name, fleet count, Powerplay facts and portrait. Rename the current ship in-game and check Statistics and Fleet without changing another ship's name.
3. With existing Frontier read consent/authentication, explicitly refresh Community Goals. Compare global totals and local contribution; check offline/expired state. Do not reauthenticate or enable uploads merely for testing.
4. Browse Fleet, including multiple pages and sold/in-transit entries. Expand a historical build and inspect stored modules. CAPI station stock must never appear as owned ships.
5. Complete a mission or earn Powerplay merits, inspect the relevant activity page and compare ratings, rewards/redemptions and earned merits versus balances. Trade volumes are not attributed faction profit.
6. Reuse a Spansh CSV, restart and verify route history/progress; use Inventory/Spansh arrows normally. Check Galaxy framing at the laptop size. On a paired laptop confirm account management, Fleet finances, provider reads and private activity remain blocked. Leave uploads disabled.

Holo-Me retrieval, unlicensed new ship imagery, broad Inara integration, guessed CZ victories/INF effects, automatic Discord/API reporting and real-data Galaxy generation are not implemented. General audit polish remains a separate batch. Fixture and CI passes do not establish real gameplay/provider approval acceptance.
