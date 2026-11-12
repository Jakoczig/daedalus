# G1B secure remote displays

Implements the approved PR #3 Go-first architecture and the maintainer's G1B
execution request, using PR #2's owner/viewer security guarantees. Base is accepted
G1A `9dd67eb9b0ba0359661bce60d6a76068694a3f4d`. The existing legacy app is untouched.

## Boundary and approach

One Wails/Go process owns one service/state instance. Its ephemeral IPv4 loopback
HTTP listener remains owner-only. A second listener exists **only** after the
owner explicitly enables remote displays; it serves HTTPS, the same embedded
React UI and the same offline RPC state. No proxy, extra backend, game processing,
provider calls or journal modules. No owner bootstrap or owner RPC on HTTPS.

The approved optional direct-TLS listener is retained. A reverse proxy/external
certificate service adds a runtime dependency and proxy trust; plaintext LAN or
certificate-error bypasses violate the requested boundary. Neither is implemented.
Engineering defaults below implement the already selected architecture, rather
than adding a new product/platform decision requiring another approval cycle.

## Certificates and storage

Owner selects a concrete local LAN IPv4 address and port (default 3443). Never bind
a wildcard, multicast or public address. Loopback is allowed for isolated tests.
IPv6, Internet access, UPnP and automatic firewall changes are outside G1B.
The certificate SAN is that exact IP; changing DHCP addresses requires owner
reconfiguration and using the new URL (reserve the host address when possible).

Generate ECDSA P-256 host-specific private CA and server certificate with crypto/rand
and crypto/x509. Root lifetime 10 years; leaf 90 days, renewed on enable/startup
when near expiry. TLS >=1.2, standard verification, no client bypass flags. Export
only the public root certificate and its SHA256 fingerprint through local owner
settings. The owner securely transfers it to each client and verifies its fingerprint
before explicitly trusting it. No automatic trust-store modification.

Separate `daedalus-go` per-user configuration directory; POSIX 0700 directory/0600
files, Windows protected DACL for current user and SYSTEM. No symlinked storage.
Atomic private file writes. Root/leaf private keys never leave the host or appear
in responses/logs. Save remote settings and device records only after successful
validation/writes; storage failures fail closed. Smoke tests use temporary storage.
No migration/read of old application credentials or preferences.

## Protocol and identity

Local POST `/auth/session` retains exact loopback peer/Host/Origin authentication.
Local POST `/owner/remote` additionally requires its owner cookie and exact Origin;
actions: status, enable, disable, pair, rename, revoke. Limit JSON bodies to 4 KiB
and reject unknown fields. All owner operations are rejected on HTTPS regardless
of supplied cookies/roles. Never derive authority from a browser-provided role.

HTTPS requires exact configured Host, real TLS, and rejects all forwarded headers.
POST `/auth/session` **recovers only an existing viewer session**, otherwise 401.
POST `/auth/pair` requires exact Origin and one valid pairing code. Codes contain
128 random bits (hex), expire after five minutes, are memory-only and single-use;
new issuance invalidates the previous code. Global pairing budget: ten attempts
per minute (including correct guesses), per-process; no attacker-controlled IP map.
Device names are length/control-character bounded, never interpreted as markup.

Viewer cookies: distinct `__Host-daedalus_viewer`, Secure, HttpOnly, SameSite=Strict,
Path=/, 12-hour absolute lifetime. Persist only SHA256 token hashes and device
metadata (random ID/name/expiry); random 256-bit bearer tokens are never returned
in JSON or device lists. Session recovery survives socket interruption/process
restart until expiry, without silently becoming owner. Expiry requires re-pairing.
Maximum 32 devices, 64 remote sockets total, four per device; prune expired records.
Device listing/rename/revocation are local-owner-only. Revoke/disable invalidate
active sockets immediately; disabling revokes all viewers and pending codes.

Anonymous HTTPS may fetch static bootstrap UI only; no private asset/data endpoints
exist in this offline milestone. Unknown auth/owner endpoints fail closed. Both
roles use explicit RPC dispatch: implemented reads are getLoadingStatus and
hostInfo only. Viewers receive their HTTPS URL, never the loopback owner URL,
certificate paths, cookies, pairing codes or owner metadata. All other RPC names,
including inherited names and host/device/settings operations, are rejected.

## Reliability and UI

Share the existing embedded frontend and state. G1B prototype marker keeps
gameplay children unmounted and visibly offline. Only local owners see remote
settings, pairing-code/public-certificate export and device management. Viewers
get a pairing form with a device name, then the same read-only navigation shell.
The legacy frontend path remains unchanged without a Go prototype marker.

Authenticated sockets retain the 1 MiB frame cap and bounded writes. Add a 20-second
ping/10-second pong deadline while the reader remains active, and close expired,
revoked, dead or over-capacity connections. No duplicate per-device backend work.
Client reconnect reuses the HTTPS cookie, resets readiness and pending callbacks,
handles failed bootstrap/network loss and does not let stale socket callbacks
mark a newer connection ready. No payload logging; sanitized errors only.

## Proof and delivery

TDD for private storage/certificate verification, enable/default-disabled state,
owner/viewer separation, TLS/Host/Origin/proxy rejection, pairing rate/expiry/reuse,
hashed durable sessions, device limits/rename/revoke/disable, idle expiry/recovery,
concurrent clients and rendered owner/viewer UI. Real verified TLS (custom test
root pool, no skip verification) and WS integration fixtures; 65-second idle test
on both CI OSes, Linux race detector. Test native smoke with temporary state and
remote disabled, preserving G1A and Phase 1A suites. Add CI triggers for this stacked
branch target. Independently review protocol before implementation and the final
patch after testing. Keep PR draft, target G1A, and stop at G1B.

Manual limits: physical iOS/Android/desktop browser trust installation, firewall
prompts, changing Wi-Fi/DHCP, Linux distro/Wayland packaging and real-device sleep
are documented acceptance work, never reported as CI-proven. No release/deploy.

## Independent protocol review decisions

- Exclusive OS file lock covers each configured profile through shutdown. A second
  process fails before loading remote identity/settings. Windows rejects reparse
  points, validates current-user ownership, and protects directory/temp-file DACL
  before any secret bytes are written; existing secret files must already be private.
- Persist an active-run latch before remote admission. Successful clean shutdown
  clears it. An unclean run or write failure prevents automatic restoration of
  viewers: close all remote admission/sockets, retain the latch, require owner
  re-enable/re-pair after durable clearing. Normal clean restarts recover sessions;
  a crash deliberately requires pairing again. A failed revoke never reports success.
- Pair consumption/store commit/token publication occur under the manager lock.
  Every listener gets an epoch; WS admission revalidates epoch/device and reserves
  capacity under the same lock used by revoke/disable. Old upgrades cannot survive
  disable/re-enable. Acknowledge revocation only after durable commit; on storage
  failure stop all remote traffic and leave the crash latch set.
- Limit all remote TCP connections to 64 before TLS work, with handshake/header
  deadlines, 16 KiB headers, 4 KiB auth bodies and bounded writes. Administrative
  loopback requests use a separate listener and remain responsive at remote capacity.
- Refresh the cached leaf certificate during running TLS handshakes near expiry;
  renewal failure rejects admission rather than bypassing verification. Missing or
  corrupt initialized CA state never silently creates a replacement trust root.
  Manual explicit profile/trust reset invalidates all viewers and requires importing
  a new root; release packaging/automatic key rotation are not implemented here.
- Exactly one matching Origin is required independently on remote recovery,
  pairing and WebSocket upgrade, including with valid cookies and matching Host.
- Transient bootstrap failures retry; only 401/403 require pairing. One bounded
  reconnect timer, at most 128 pending/queued RPCs, reject pending work on close,
  cancel timers and ignore superseded fetch/socket callbacks. Legacy traffic keeps
  its current wire protocol; no client role can grant server authority.
