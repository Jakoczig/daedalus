# ERIDION G2M-B — experimental, unsigned x64 preview

Local-first native navigation and contextual field operations. Not a release or installer. Keep the original DAEDALUS application and a private profile backup. No uploads are enabled by installing this preview.

## Verify and run

The archive contains only `eridion.exe` (Windows) or `eridion` (Linux), this README and `SHA256SUMS`. Verify both manifest entries before running. On Windows use `Get-FileHash .\eridion.exe -Algorithm SHA256` and `Get-FileHash .\README.md -Algorithm SHA256`; compare with `SHA256SUMS`. On Linux run `sha256sum -c SHA256SUMS`, then `chmod +x eridion` and `./eridion`.

Windows x64 needs Microsoft Edge WebView2 Runtime and a desktop session. Linux x64 needs glibc, GTK4, WebKitGTK 6.0, a desktop session and Secret Service for saved provider credentials. Ubuntu 24.04 packages: `libgtk-4-1 libwebkitgtk-6.0-4`. Keep the WebView sandbox enabled. The executable requires no Node, Python or EDMC runtime.

Quit an older native preview normally before starting this one: profiles are exclusively locked. Do not run this alongside another native owner using the same profile. The legacy application remains separate. Existing private history, account selection and consent are reused; a profile backup remains advisable.

## Navigation and Spansh

Native tabs are System, System List, Route, Spansh Plotter and Galaxy. Controls is removed from native navigation; legacy navigation is unchanged. Existing map/list/plotter URLs remain compatible. The old Field/BGS URL opens Commander activity.

Spansh Plotter offers the current public modes and Route History. Planning notes stay local and are copied only when requested. Open the fixed official web page yourself, configure its advanced options and export CSV. No automatic route-generation API, embedded browser, query-prefill contract or result retrieval is claimed.

Import supports UTF-8 CSV up to 512 KiB, 2,048 unique systems and 16 named plans per commander. A `System Name` or `system` column is required. Body/trade/loop exports may need editing to a unique-system itinerary. Imported routes never change the game's NavRoute or target. Successful import opens Route. Manual import/progress/export controls are below the game route. History and manual activation use the existing transactional SQLite store and survive restart/crash. Copy-to-owner-PC remains local-owner-only.

Galaxy has a large X/Z view, a thin X/Y band below and a thin Y/Z band on the right. X and Z markers align with the main view under the shared pan/zoom frame. Artwork is the original illustrative procedural model, not measured galaxy density or copied EDAstro imagery.

## Contextual activity

Mission/faction effects are in Missions; Powerplay activity is in Powerplay; contributions and witnessed depot progress are in Commander → Colonisation; the complete private activity ledger is in Commander. Carrier activities remain in the existing owned-carrier ledger, never attributed from unrelated personal trades. Built-in module controls and creator credits are in Owner Settings. Tick grouping requires already-cached verified closed boundaries; otherwise labels show UTC day. INF/REP ratings are ordinal, awards are separate from redemption, and no BGS percentage or combined profit is invented. CSV export requires explicit owner acknowledgment. No reporting is activated.

## Secondary laptop

Existing pairing is preserved. Enable remote access only in Owner Settings, install/trust the owner's generated CA on the laptop, use the displayed HTTPS address, allow the chosen firewall port and pair with the short-lived six-character code. Never bypass TLS warnings. Private plan history, provider settings, activity exports and construction history remain owner-only; paired displays receive the existing allowlisted route progress. Revoke the device in owner settings to disconnect it.

## Gameplay checklist

1. Import a synthetic or known Spansh CSV; confirm Route opens, GAME ROUTE stays separate, copy is explicit, arrival advances only on journal evidence, and history/progress survive restart.
2. Check System/System List, Spansh modes, Route History and thin galaxy projections at your Windows scaling and laptop size. Pan/zoom and compare aligned current/destination markers.
3. Complete a mission and inspect faction effects, rewards and UTC/tick labels; check witnessed Powerplay and construction/carrier activity where available. Unknown evidence must stay unknown.
4. Check Commander, Fleet, Ship and Frontier references still match witnessed/local versus cached sources. No new live Frontier authentication is needed for this test.
5. Pair/reconnect/revoke a laptop and switch commanders. Confirm private owner pages stay blocked and another commander's history never appears. Leave uploads disabled.
