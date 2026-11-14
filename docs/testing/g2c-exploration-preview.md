# DAEDALUS G2C — EXPERIMENTAL, UNSIGNED, LOCAL EXPLORATION + BIOLOGY PREVIEW (NO PROVIDER UPLOADS)

Portable test executable, not an installer or Release. Keep the original Daedalus
and its launcher. Do not copy generated profiles, credentials or CA private keys
into the preview folder or share them. This build does not start Node or Python.

## Verify and run

Download the Windows or Linux artifact from the linked GitHub Actions run in the
PR. Extract all three files: executable, README.md and SHA256SUMS. Hashes protect
against download damage; they are not publisher signatures.

Windows x64: Windows 10/11 with Microsoft Edge WebView2 Evergreen Runtime.
Use PowerShell `Get-FileHash .\daedalus.exe -Algorithm SHA256` and
`Get-FileHash .\README.md -Algorithm SHA256`; compare both to SHA256SUMS, then
run `.\daedalus.exe`. It is unsigned: only run a preview you built or verified.

Linux x64: GTK4, WebKitGTK 6.0, graphics/display session and WebKit sandbox support.
Ubuntu 24.04 packages: `libgtk-4-1 libwebkitgtk-6.0-4`. Run
`sha256sum -c SHA256SUMS`, `chmod +x daedalus`, then `./daedalus`.
Distro, Wayland/GPU and AppArmor behavior outside CI still needs manual testing.
Do not disable TLS verification, the WebKit sandbox or profile locking.

The Go profile is separate: `%APPDATA%\daedalus-go` on Windows and the platform's
user configuration directory (`daedalus-go`) on Linux. Configure a readable journal
directory in the owner card if automatic discovery needs an override (including
Steam/Proton Saved Games/Frontier Developments/Elite Dangerous). Waiting for a
journal is normal; no fake game data appears in normal use.

## Secondary display

Remote access stays disabled until the local owner enables it. In owner settings,
choose the host's LAN IP and an unused HTTPS port; allow only the needed executable/
port through the private-network firewall. Connect the laptop to that HTTPS address,
not localhost. Install/trust the public CA certificate supplied by the owner on
that laptop/browser and verify its fingerprint through the owner interface; never
copy the CA private key or bypass the browser's TLS warnings. Address changes may
require a new server certificate. Preserve the CA/profile; do not regenerate trust
as a troubleshooting shortcut. Use a fresh six-character pairing code (five-minute,
one-time use); the laptop is a read-only viewer. Owner settings can list/rename/revoke
it. Revoke closes sessions/streams; reconnect needs new owner authorization.
See the G1B/G2B runbooks in the repository for platform certificate-store details.

## Consolidated real-game acceptance

1. Start the desktop, confirm G2C/local journal state, plot a route, jump/clear and
   verify current/next, search, copy and page controls. Changed same-timestamp route
   evidence must remain unverified; no game targeting or commands are provided.
2. Honk/FSS/DSS: check System map, body selection by mouse/keyboard, parent grouping,
   radius/temperature/pressure/orbit units, rings/composition and mapping indicators
   against the game. Unknown parents are marked; the map is schematic, not an orrery.
3. Open Biologicals: check confirmed signal/genus counts, then Log → Sample → Analyse
   for one organism. Duplicate observations must not inflate progress. Switch to a
   different species/body and confirm interruption. Restart/replay and change
   commander/system; no foreign data or invented sample start should appear.
4. Repeat with a paired laptop and mobile browser. Verify live updates, scrolling,
   filters, disconnect/reconnect, idle/sleep and revocation. Viewers must never see
   owner controls, raw journal data, credentials or private positions.
5. Repeat on a physical Linux/Steam Proton installation; test distro/GPU/Wayland,
   certificate trust and filesystem behavior. CI's synthetic native smoke is not
   physical gameplay acceptance. Do not upload personal journals to demonstrate it.

Limits: journal-only data; bounded in-memory replay cache, no persistent survey DB.
Missing history is unknown. Biological analysis is historical completion, not
unsold inventory. No speculative species, payout/first-footfall assumptions,
providers/uploads, surface-position radar, fuel predictions or ship/engineering
parity. Other unported pages remain explicitly unfinished.
