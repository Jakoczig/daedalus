# G2L — experimental, unsigned, local-first fleet/economy preview

Windows x64: extract separately, check `Get-FileHash -Algorithm SHA256` against
`SHA256SUMS`, run `daedalus.exe`. Requires Windows 10/11 and Edge WebView2.
Linux x64: `sha256sum -c SHA256SUMS`, `chmod +x daedalus`, run in a graphical
session with GTK4/WebKitGTK 6.0 (Ubuntu 24.04: `libgtk-4-1 libwebkitgtk-6.0-4`).
Secret Service is needed only for optional credentials. No Node/Python/EDMC runtime.
This is a portable preview, not an installer or legacy replacement.

Use `--data-dir <private-preview-directory>` for an isolated profile. Existing v5
profiles migrate additively to v6 after a verified pre-migration backup. Stop the
preview before recovery; select a verified backup in Data/History and acknowledge
staging. Restore applies after restart, preserves the replaced database and holds
publication pending renewed consent. A pre-v6 backup can be used with G2K; do not
open a v6 database with an older executable. Keep private exports/backups private.

Secondary displays retain G1B HTTPS and pairing: enable remote access explicitly,
use the host IP, install/trust the generated public CA certificate on the laptop,
allow the configured HTTPS port through the host firewall, pair with the owner’s
one-time six-character code. Never copy the CA private key. No TLS bypass. New
fleet, carrier, economic and crew history is owner-only; a laptop receives a
privacy explanation, not those records. Existing read-only public navigation,
exploration and ship projections continue unchanged.

| Capability | State / evidence |
|---|---|
| Fleet, storage, transfer requests, sold/active markers | Local working; timestamp-aware journal evidence, never a complete/live fleet claim |
| Historical fitted modules / final engineering | Local working for witnessed Loadout; stored module grade never becomes inferred modifiers |
| Owned carrier fields, services, fuel, finance, orders, requests/movements | Local working; explicit CarrierBuy/CarrierStats proof, matching owned arrival required; transfers are not income |
| Purchases, sale receipts and known-basis gross spreads | Local working; exact decimal strings; bounded filtered pages and selected-page totals, no FIFO/net-profit invention |
| Market history / two-market quoted spread | Local working with dock/marker/sidecar identity proof; source timestamps shown, not a global/live route planner |
| Historical mining tonnes / attributable sale receipts | Local working; empty ship-cargo baseline, same session and exclusively refined input required; mixed sources unknown |
| CG participation/progress/reward, Powerplay, squadron, wing/NPC crew | Local working; last witnessed state/history, sparse identity stays unknown; no fighter deployment or inferred Powerplay 2.0 mechanics |
| Frontier profile fleet reference | Existing typed cached reference; preferred external source; developer approval/OAuth gated, live G2L not accepted |
| Frontier `/communitygoals` | Documented GET exposed behind existing approval gate; body normalization awaits authorized verified fixture, not claimed complete |
| Inara public goals | Existing documented separate read consent, durable bounded cache added; approval/key gated, no profile polling from pages |
| EDSM selected-station market | Existing fixed-host, owner-requested read, separate consent; verified MarketID/system/station; cache quotes only with known source observation time |
| PTN / FCOC | Manual community website/profile references only; no scraping, feed or background loading |
| Export / recovery | Owner-only selected-page CSV with preview-hash + acknowledgment, formula-safe cells; exact-hash original journal recovery, no new publication |
| G2L physical gameplay / Linux-Proton / live providers | Manual acceptance pending. G2K gameplay accepted by maintainer; visual rough edges explicitly deferred |

Opening a page never authorizes providers or uploads. Uploads remain disabled by
default and existing publication policies/exceptions are unchanged. Never perform
live uploads as part of preview acceptance without separate explicit consent.

One consolidated acceptance checklist:
1. Compare two same-model ships with distinct IDs, request a transfer, switch/sell
   one ship and reopen shipyard. Check timestamps, pending/last-known states,
   historical final modifiers and stored modules, including an empty snapshot.
2. If you own a carrier, open management, change observed finances/services and
   request/cancel/complete a jump in the game. Verify an unrelated carrier cannot
   establish ownership or arrival; requests/orders/transfers are not transactions.
3. Buy/sell a commodity with known basis; sell mined/mixed cargo. Compare exact
   expenditure/receipts and withheld unknown profit. Open station markets at two
   stations, compare dated quotes and confirm no uploads/provider requests occur.
4. Observe two CGs, a received reward, Powerplay actions, squadron/crew/wing events.
   Restart after playing with the app closed. Check retained history, pagination,
   commander isolation and explicit unknown fields. Test verified original-source
   recovery only on a private backup; changed/missing originals must fail safely.
5. On the paired laptop verify G2L private pages deny access while existing public
   screens reconnect normally; revoke that device. Preview/export selected owner
   records explicitly, including formula-prefixed names. Keep exports private.
