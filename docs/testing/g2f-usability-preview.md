# DAEDALUS G2F — EXPERIMENTAL, UNSIGNED, LOCAL FEATURE-PARITY PREVIEW (NO PROVIDER UPLOADS)

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


## Optional EDSM community lookup
Off by default. In local Owner settings, select Enable EDSM lookups. Opening
the owner map then sends the current system name and network IP to EDSM over
verified HTTPS. No keys, account identity, raw journals, surface positions or
uploads. Paired viewers can only read the shared cache; they cannot initiate
provider requests. Disable to cancel requests and remove cached community data.

Responses are cached in memory for six hours (up to 16 systems, 256 bodies and
64 stations each); two fixed GET endpoints, an eight-second deadline, 2 MiB per
response and one lookup attempt per minute. No persistent provider database.
The current journal system must have an exact verified system ID; body joins
use exact game body IDs. Community data never changes your scans, FSS, biological
sampling or discovery flags. Station timestamps and lookup times are independent.
Missing community results do not imply undiscovered/unmapped planets.

Tests use a fabricated provider transport. Live EDSM availability and current
response coverage still require owner acceptance; no live gameplay lookup was
performed during implementation. Offline functionality remains available.

Check: readable Ship switches (ON/OFF/UNKNOWN, observed only); Fighter Hangar name
and class-0 placeholder; sidebar pointer labels disappear after two seconds but
keyboard focus remains labeled; map with EDSM disabled/enabled; station freshness;
restart/change commander; paired viewer reconnect and revocation.
