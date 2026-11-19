# ERIDION G2M-C — experimental, unsigned x64 preview

Not a release or installer. This preview corrects verified login snapshots, keyboard navigation, owner Local Activity, provider status wording, missing route targets and the Trade landing page. Upload permissions remain unchanged and disabled by default.

The archive contains only `eridion.exe` (Windows) or `eridion` (Linux), this README and `SHA256SUMS`. Windows: compare `Get-FileHash .\eridion.exe -Algorithm SHA256` and `Get-FileHash .\README.md -Algorithm SHA256` with the manifest. Linux: `sha256sum -c SHA256SUMS`, then `chmod +x eridion` and `./eridion`.

Windows x64 requires Edge WebView2 Runtime and an unlocked desktop. Linux x64 requires glibc, GTK4, WebKitGTK 6.0 and a desktop session; saved provider credentials require Secret Service. Ubuntu 24.04 packages: `libgtk-4-1 libwebkitgtk-6.0-4`. No Node, Python or EDMC runtime is needed. Keep the WebView sandbox enabled.

Keep a private profile backup and quit the older native preview normally before starting this one. Exclusive profile locking is retained. The original legacy application remains separate and unchanged. This preview reuses existing history and consent. On first startup, an older derived cache is rebuilt from existing SQLite records, without changing original journals, checkpoints, credentials or publication policies. Subsequent starts use the upgraded cache. Ambiguous/incomplete startup evidence remains unknown; no ranks, materials or engineers are invented.

Secondary devices continue to use existing HTTPS pairing. Never bypass TLS certificate warnings. Owner Local Activity is collapsed until deliberately opened and is unavailable to paired displays. No new Frontier login is necessary to verify status wording. EDSM settings poll local cache metadata only; enabling or refreshing providers remains an explicit existing control.

## Acceptance checklist

1. Compare Commander ranks/progress/reputation, Engineering unlock progress and material stocks against the game; restart ERIDION without new game events and compare again.
2. Switch commanders; verify each account's data remains separate. Keep uploads disabled.
3. Focus Inventory and Spansh selectors and use arrow keys; edit/search normally without unwanted navigation. Plain page shortcuts should still work outside controls.
4. Open Missions → Local Activity deliberately; check Overview/Session retain their layouts. On a paired laptop, confirm private history remains blocked.
5. Check existing Frontier success/cache times, EDSM metadata after an already-authorized map lookup, explicit unknown Route target, Trade default and remembered trade subpage. No reauthentication or upload is required.

AUD-03/05/06/07/09/10/11 are deferred to G2M-D. Synthetic CI/browser tests do not establish physical gameplay acceptance.
