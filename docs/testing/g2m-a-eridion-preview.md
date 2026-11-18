# ERIDION G2M-A — experimental unsigned preview

Windows x64: extract the archive into a new directory; verify `Get-FileHash .\eridion.exe -Algorithm SHA256` against SHA256SUMS. Requires Windows 10/11 and Microsoft WebView2 Evergreen. Run eridion.exe. Do not replace the legacy DAEDALUS installation.

Linux x64: verify `sha256sum -c SHA256SUMS`, `chmod +x eridion`, run from a graphical session. Requires GTK4, WebKitGTK 6.0, and an unlocked Secret Service with secret-tool for stored credentials. No plaintext credential fallback.

Both platforms: one Go service, local-first. New external access is disabled until consent. No release, installer or production uploads were performed. Existing Go profile data is adopted in place through a versioned ERIDION identity manifest; legacy Node data is untouched. See docs/handoffs/g2m-a-eridion-identity-frontier.md for migration/recovery details.

Frontier login needs the separately deployed HTTPS website described in docs/protocols/eridion-frontier-website-return.md. A public client ID is not a shared key or evidence of live authorization. Read consent alone never enables uploads. After website deployment, the owner can connect in Settings → Data Providers → Frontier. If the browser blocks the return link, copy the local return from that website into the owner-only Manual browser return field.

Paired laptop: keep existing trusted CA configuration; no bypass is required. Existing Go pairing grants are retained for their original commander. A different account needs its own owner-authorized pairing. No native credential or account identity/portrait is sent to viewers. Physical login and gameplay acceptance remain manual.


The later [Frontier live-audit handoff](../handoffs/g2m-a-frontier-live-audit.md) documents owner-only command-mode structural diagnostics and the Fleet, station-reference, carrier-204 and global-CG corrections. No raw account responses or diagnostics are included in the preview archive.
