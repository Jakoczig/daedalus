# DAEDALUS G1B — EXPERIMENTAL, UNSIGNED, OFFLINE PREVIEW

Developer test executable only. This is NOT a release, installer, security
certification or working game-companion replacement. Keep the original app.
Game journals, Inara/EDSM/EDDN/CAPI and gameplay modules are unimplemented.
No Node, Python or EDMC installation is needed to run this Go/Wails preview.

The CI artifact ZIP contains exactly one executable, this README and SHA256SUMS.
It contains no user profile, private certificate key, credential or session token.
Never copy an existing profile into an artifact you share with someone else.

## Download and verify

Open the successful **Go desktop foundation** Actions run linked from PR #5:
https://github.com/Jakoczig/daedalus/pull/5
Sign into GitHub if required, open its Artifacts section and select your OS's
`experimental-unsigned-offline-preview` artifact. Extract the ZIP to a new folder.
Artifacts expire after 14 days; use a current successful run if a link has expired.

Verify the executable hash against SHA256SUMS before running it:

Windows PowerShell, from the extracted folder:

```powershell
Get-FileHash -Algorithm SHA256 .\daedalus.exe
Get-Content .\SHA256SUMS
```

Linux:

```sh
sha256sum -c SHA256SUMS
chmod +x daedalus
```

SHA-256 detects changed bytes; it is not a publisher signature. These builds are
unsigned. Follow your device's software/security policy; do not disable security
protection to run them. No installer, updater or privilege elevation is included.

## Windows desktop test

Prerequisites: x64 Windows with Microsoft WebView2 Runtime installed, a graphical
desktop, and a writable per-user configuration directory. CI tests Windows Server
2025; development testing used Windows 11. No game installation is required.

From the extracted folder:

```powershell
.\daedalus.exe --smoke-test
.\daedalus.exe --desktop-smoke-test
.\daedalus.exe
```

The first command checks local HTTP/WS without a window. The second opens a real
native window and automatically closes after React authentication/readiness.
Both use fresh temporary profiles with remote access disabled. Their inherited
G1A console labels describe the smoke contract; the normal interface says G1B.
The final command opens the desktop normally. Confirm the offline label and
navigation, then close the last window and verify that the process exits.

The normal profile is `%AppData%\daedalus-go`; it is separate from the legacy app.
Only one instance may own it. Use `--data-dir PATH` for an explicitly separate test
profile. Remote access is disabled on a fresh profile. Do not upload that profile.

## Connect a secondary laptop/tablet through HTTPS

1. In the host's native window, open **Remote display settings (local owner)**.
2. Select the gaming PC's private IPv4 address and an unused HTTPS port (3443 by
   default). Prefer a DHCP reservation. Explicitly enable HTTPS remote displays.
3. Manually allow inbound TCP on that port for the private LAN/subnet only if your
   firewall requires it. Never use router port forwarding or a public network.
4. Download the public CA certificate from the owner settings. Securely transfer
   only that public `.crt` to the secondary device, compare its SHA256 certificate
   fingerprint with the local owner display, and explicitly trust it there. Never
   transfer `tls.json`, private keys, cookies, tokens or the entire Go profile.
5. Open the exact **HTTPS** address/port shown by the owner. A secondary laptop
   needs only its browser. Never bypass a certificate warning or disable TLS
   verification; stop and fix trust/address/time if the certificate is rejected.
6. Create a pairing code on the host. Enter its six characters (e.g. `7K4M9P`)
   and a device name in the secondary browser, then **Refresh devices** on the owner.
   The visible field uppercases letters automatically; lowercase also works. I, L,
   O and U are excluded; use exactly six characters, no spaces or dashes. The code
   works once and expires after five minutes, at the deadline shown by the owner.
   A new code replaces the previous one but never resets the shared ten-attempt/
   minute budget. After too many failed attempts, wait a minute.
7. Confirm read-only/offline status. Test browser reload, network loss/recovery,
   two devices, rename and revoke an open viewer. Viewers cannot run host actions.

Session lifetime is 12 hours. Clean host restart can recover viewer sessions;
revocation, expiry or an unclean exit requires pairing again. Disable remote
displays to stop HTTPS and revoke all devices. A changed host IP needs local-owner
reconfiguration and typically pairing again at the new address.

## Linux prerequisites

x86-64 Ubuntu 24.04+ with a graphical session, GTK4/WebKitGTK 6.0 runtime libraries
(`libgtk-4-1`, `libwebkitgtk-6.0-4`). No universal distro compatibility is claimed.
Use `ldd ./daedalus` to find missing libraries; install them through your OS package
manager. Start with `./daedalus --smoke-test`, then native smoke and normal launch.
Headless native testing requires Xvfb/D-Bus. Ubuntu's AppArmor user-namespace policy
may require an administrator-reviewed profile for the exact executable path so
WebKit can create its sandbox. Do not disable WebKit sandboxing or system-wide
security restrictions. This artifact does not install an AppArmor profile.
The normal profile is usually `~/.config/daedalus-go` (or XDG_CONFIG_HOME).

## Recovery and manual acceptance

Corrupt optional remote settings keep the local owner desktop available, disable
remote access and show recovery instructions. Damaged files are retained; do not
enable/repair by deleting individual CA files. Close the app, back up the dedicated
Go profile and restore a trusted consistent backup, or deliberately reset that
profile and remove the old public CA from clients before importing a new one.
A locked profile or an inaccessible/unsafe profile root still prevents a competing
owner from starting. No automatic trust reset or viewer restoration occurs.

Physical certificate import, mobile browser policies, firewalls, sleep/Wi-Fi
roaming, Wayland/AppArmor packaging and power-loss durability need manual testing.
Full instructions and limitations:
https://github.com/Jakoczig/daedalus/blob/go/g1b-secure-remote-displays/docs/testing/g1b-secure-remote-displays.md

Report the PR head/run, OS, steps and sanitized errors. Never attach profiles,
private certificates, credentials, tokens or real game data. No release, deployment
or G2 functionality is authorized by this test artifact.
