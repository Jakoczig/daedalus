# G1B secure remote displays — developer setup and acceptance

This is an offline Go/Wails preview. It shares one Go service among the native
desktop and paired browsers; it reads no journals and calls no game/provider APIs.
The old application is separate and untouched. Remote access starts disabled.

Experimental unsigned executables can be downloaded from successful desktop
GitHub Actions runs on [PR #5](https://github.com/Jakoczig/daedalus/pull/5), without
compiling locally. Each OS artifact includes SHA256SUMS and the
[standalone Windows/Linux test instructions](g1b-preview-artifact-readme.md).
These are expiring CI test artifacts, never a Release or installer.

## Build and start

Use the [G1A prerequisites](g1a-windows-linux-smoke.md): Go 1.26.10, Node 24.22.1
for build time, Wails v3.0.0, Windows WebView2 or Linux GTK4/WebKitGTK 6.0.

```sh
npm ci
npm test
npm run lint:security
npm run test:go-assets
npm run build:go-assets
cd go
go test -count=1 -timeout=4m ./...
go vet ./...
go build -o bin/daedalus ./cmd/daedalus
./bin/daedalus --smoke-test
./bin/daedalus --desktop-smoke-test
./bin/daedalus
```

Windows: build `bin/daedalus.exe` and invoke `.\bin\daedalus.exe`.
Linux: also run `go test -race -count=1 -timeout=4m ./...` and use
`xvfb-run -a dbus-run-session -- ./bin/daedalus --desktop-smoke-test` headlessly.
Both smoke modes create/discard their own private temporary profile and cannot
enable an existing saved LAN listener. Their inherited G1A console labels describe
the offline smoke contract; normal runtime now reports G1B in its bootstrap/UI.
Linux AppArmor/WebKit namespace prerequisites from G1A still apply.

Normal settings live in `os.UserConfigDir()/daedalus-go`: usually
`%AppData%\daedalus-go` on Windows or `$XDG_CONFIG_HOME/daedalus-go` (normally
`~/.config/daedalus-go`) on Linux. Advanced runs can set `--data-dir PATH`.
Only one process can own a profile; close it before launching another instance.
This never reads/migrates the old app's preferences or credentials.

## Owner setup

1. Open **Remote display settings (local owner)** in the native window or its
   exact local loopback browser URL. Settings are not exposed to paired viewers.
2. Select this host's private LAN **IPv4 address**, not the tablet's address.
   The field suggests active private addresses. Find/check the address in Windows
   network properties or Linux network settings. Prefer a router DHCP reservation.
   IPv6, public addresses, proxies, UPnP and Internet access are not supported here.
3. Choose an unused TCP port, default 3443. Click **Enable HTTPS remote displays**.
   Only that concrete interface/port is bound; no wildcard listener or HTTP LAN
   listener is created. If the port/IP is unavailable, correct it locally.
4. If needed, manually allow inbound TCP on this selected port in the host firewall
   for the **private LAN/subnet only**. The app does not alter firewall rules or
   accept OS permission dialogs. Avoid public networks and router port forwarding.
5. Download the **public CA certificate** in owner settings. Transfer only that
   `.crt` to the client by a trusted channel. Compare its SHA256 certificate
   fingerprint with the local owner display before importing it. Never transfer
   the profile, `tls.json`, private keys, session tokens or pairing codes publicly.
6. Explicitly trust this CA for websites on the client. The app never changes
   trust stores, uses an insecure verification option, or bypasses an interstitial.
   If you cannot install the CA under device policy, remote mode is unavailable.
7. Open the **exact HTTPS URL shown by the owner**, including port. Once trusted,
   the browser should show the pairing form without a certificate warning.
8. Click **Create pairing code** on the owner. Enter its one-use code and a device
   name in the trusted remote form. Codes expire after five minutes; making a new
   one invalidates the previous code. Pairing has a shared ten-attempt/minute
   budget, so wait one minute after excessive attempts.
9. Click **Refresh devices** on the owner to list the newly paired device. Confirm
   the remote shell says **G1B offline** and **read-only**. No gameplay
   parity is claimed. Owner controls and secrets must not appear remotely.

## Client trust references

These are manual, security-sensitive user steps. CI verifies X509 trust with an
explicit root pool; it does not certify the physical device/browser trust UI.

- Windows browsers: import the verified public CA into the appropriate current-user
  Trusted Root Certification Authorities store, using the certificate UI or your
  administrator's policy. See [Microsoft certificate store documentation](https://learn.microsoft.com/en-us/windows/win32/seccrypto/system-store-locations)
  and [certutil reference](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/certutil).
- Firefox/Linux: import the verified CA as a website authority in the browser's
  certificate manager or use the OS trust integration appropriate to that build.
  See [Mozilla CA setup](https://support.mozilla.org/en-US/kb/setting-certificate-authorities-firefox)
  and [manual authority import](https://wiki.mozilla.org/CA/Changing_Trust_Settings).
- Android: device/managed-profile policy and browser determine whether user CAs
  are accepted. Follow [Google's certificate installation instructions](https://support.google.com/pixelphone/answer/2844832).
- iOS/iPadOS: installing the certificate profile alone does not automatically
  grant SSL trust. Follow [Apple's manual certificate trust instructions](https://support.apple.com/en-us/102390)
  after checking the fingerprint. Managed devices may forbid user trust changes.

Trusting a CA grants certificate authority to its private key. Protect the host
profile and remove this CA from clients when retiring or resetting the preview.
No private CA is downloaded automatically from an untrusted remote endpoint.

## Device management and recovery

- Owner settings list IDs, names and absolute expiry times. Rename is cosmetic;
  revoke invalidates that device token and closes its active sockets immediately.
  Disable stops HTTPS and revokes every device and pending pairing code.
- Sessions are read-only, expire after 12 hours, and use Secure/HttpOnly/
  SameSite=Strict cookies. Reload/reconnect uses the existing cookie and must
  recover **viewer**, never owner. At most 32 devices, four sockets/device and
  64 total remote TCP connections are accepted. The local owner listener remains
  independent under remote saturation.
- Heartbeat: ping every 20 seconds with a 10-second response deadline. Browser
  engines handle control frames; dropped or sleeping connections retry after
  connectivity returns. Transient bootstrap errors retry; expired/revoked identity
  shows pairing. Pending RPCs are canceled at disconnect and are not replayed as
  stale actions. The queue is bounded to 128 requests.
- Clean host restart retains devices, HTTPS address and trust until expiry. An
  unclean exit or persistence failure requires owner re-enable/re-pair: an active
  run latch prevents old tokens from silently reappearing. A failed operation is
  never acknowledged as a successful durable revocation. Restart after repairing
  storage permissions/space, then enable and pair again.
- If DHCP changes the host IP, reconfigure from the local owner window and use the
  new exact HTTPS URL. The CA remains the same; the IP-specific leaf is renewed.
  Cookies are host-scoped, so pair again at a new IP. Reserve the address to avoid
  this. If the saved remote address cannot bind, local owner access stays available.
- Private CA lasts ten years; server leaf lasts 90 days and renews near expiry on
  startup/enable and during TLS handshakes. A failed renewal rejects admission.
  Corrupt/missing initialized CA state never silently changes the trust identity.
  Damaged optional remote settings now quarantine the remote module while keeping
  the local owner desktop/RPC and recovery warning available. No remote listener
  starts, saved viewers are discarded in memory, and damaged files are not changed
  on startup or shutdown. Remote mutations remain blocked until offline repair.
  A second process holding the profile lock, an unsafe profile root or a profile
  that cannot be securely locked still prevents a competing local owner.
  Close the app and back up the dedicated Go profile first; restore a consistent
  trusted backup and correct its permissions, or deliberately reset the entire Go
  profile as below. Do not delete isolated TLS files to force regeneration.
  To reset deliberately: stop the app, revoke/remove the old CA from clients,
  back up/remove the dedicated Go profile yourself, restart and pair/import the
  new verified CA. This resets every device; never delete the legacy profile.

## Automated and manual acceptance

Automated tests exercise real TLS/WS with ordinary root verification, owner-cookie
separation, exact Host/Origin and no proxy trust, anonymous/role/RPC rejection,
pair expiry/reuse/rate limits, private hashed durable state, process locking,
revocation/storage-failure races, restart recovery, socket expiry, multiple clients,
pre-auth saturation and a real **65-second idle** then reconnect/read. React tests
cover owner controls, viewer restrictions, legacy rendering, connection/readiness,
temporary network failure and stale callbacks. Both CI OSes must build and run
actual native renderer smoke; Linux also runs fresh race tests.

Before any distribution, record physical acceptance on Windows and Linux hosts:
trust import on a second laptop and Android/iOS tablet; private firewall prompts;
Wi-Fi loss/recovery and device sleep; two devices together; revoke an open tab;
clean restart and abrupt kill; DHCP change; old app on port 3300; last native-window
shutdown leaves no listeners. Check with `Get-NetTCPConnection`/`ss -ltnp`: one
ephemeral loopback owner listener, plus exactly one selected IPv4 TLS listener
only while enabled. No Node/Python/EDMC child processes or duplicate game watchers.
These physical/mobile/Wayland checks are not proven by Xvfb/native smoke in CI.

Stop after G1B. Do not merge, release, deploy, replace the old app or add gameplay
modules based on this preview. See the [G1B handoff](../handoffs/g1b-secure-remote-displays.md)
for exact implementation/check evidence and remaining limitations.
