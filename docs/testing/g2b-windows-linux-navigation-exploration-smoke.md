# G2B — EXPERIMENTAL, UNSIGNED, LOCAL ROUTE + SURVEY PREVIEW (NO PROVIDER UPLOADS)

This is a limited Go/Wails test executable, not a release, installer or replacement
for the original Daedalus. It reads local journals, never writes game files, calls
providers, targets the game or uploads data. No Node, Python or EDMC runtime is
needed on the host or a paired secondary device.

## Download, prerequisites and checksums

Open [draft PR #7](https://github.com/Jakoczig/daedalus/pull/7), select a successful
Go desktop foundation run, and download the Windows/Linux x64 artifact named
`daedalus-g2b-…experimental-unsigned-local-route-survey-no-uploads`. Artifacts expire
after 14 days; GitHub sign-in may be required. Each ZIP must contain exactly its
executable, README.md and SHA256SUMS. Never reuse hashes from another commit or OS.

Windows requires an x64 graphical Windows desktop and Microsoft WebView2 Runtime.
Extract into a new folder; compare `Get-FileHash -Algorithm SHA256 .\daedalus.exe`
to `Get-Content .\SHA256SUMS`. Follow your device's unsigned-software policy;
do not disable security protections. Start `.\daedalus.exe` normally.

Linux requires an x64 graphical desktop with GTK 4 / WebKitGTK 6.0 runtime
libraries. Ubuntu 24.04 CI uses Xvfb/DBus and software rendering, not a physical
Steam/Proton game. After extraction run `sha256sum -c SHA256SUMS`, then
`chmod +x daedalus` and `./daedalus`. Packaging preserves bytes, not executable
mode. Distro, GPU, Wayland and AppArmor behavior still needs physical testing.

## Native PC and secondary laptop

Keep the old application. G2B reuses the separately protected Go profile and its
G1B pairing; do not move profiles into the extracted binary folder or distribute
profiles, remote.json, private CA keys or credentials. Normal startup reads the
Windows Saved Games Known Folder or bounded Linux native/Flatpak Steam-Proton
locations; the local owner's Journal directory control supports explicit Wine or
Heroic installs. It never creates/writes the game journal directory.

Remote displays stay disabled by default. On the gaming PC, open owner Remote
displays, choose its current private LAN IPv4 address/port and explicitly enable
HTTPS. Follow the existing G1B certificate trust instructions: install **only the
public CA certificate** on your own laptop/tablet and verify its identity locally.
Use the exact HTTPS IP included in the server certificate, not HTTP or an
unlisted hostname. Do not bypass browser certificate errors. Allow only this
preview/selected port on your trusted private firewall network if needed. IP
changes require a matching owner-approved configuration/certificate. Never port
forward to the Internet. See the preserved G1B remote-display runbook for recovery.

On the PC generate the six-character five-minute one-time code. Open that HTTPS
address in the laptop browser and enter the code. The browser is a read-only
viewer; it cannot select directories, target the game or invoke host commands.
Revocation invalidates session recovery and active streams. No second backend,
journal reader or game install is required on the laptop.

## Physical Windows gameplay acceptance (still required for G2B)

1. With the game stopped, confirm service connection/readiness is distinct from
   waiting/stale journal status. Inspect `/nav/route` and `/exploration/system` on
   PC and laptop; unported pages including `/nav/map` must say unfinished.
2. Enter the game, plot a multi-jump route. Check current system, target, final
   plotted destination and star classes on both screens. Route evidence requires
   the NavRoute.json timestamp to exactly match an in-session NavRoute marker,
   strictly after LoadGame and the latest clear. A missing/mismatched marker or
   file stays unverified. Replay is labeled last-known, not a fresh active target.
3. Complete an actual jump. StartJump must not advance arrival; FSDJump must.
   Remaining plotted hops are unknown if exact current address is not on the
   route. A selected target outside the expected route is shown separately with
   a disagreement label. LY distances are geometry only, not fuel/safety advice.
4. Clear the in-game route. Confirm both screens remove old waypoints and an
   unchanged stale NavRoute file cannot resurrect them. Rapid same-second events
   can conservatively stay unverified. Not every game cancellation is journaled;
   an attested plotted route is evidence, not a game control/current-target claim.
5. Honk/FSS, scan a body and perform DSS mapping where possible. Check scanner
   count/progress versus observed Scan rows. Missing fields must remain Unknown.
   WasMapped is a previous-source fact; your DSS completion requires the journal's
   SAAScanComplete. Signals/genus do not imply species, footfall or value.
6. Jump to another system and switch commanders/restart. Old bodies must not
   appear in a different system or commander. Addressless FSS aggregate events
   require continuous ordered source context after a verified session/arrival,
   with no pending hyperspace change. Body-specific events require exact address.
7. Disconnect/rejoin the laptop network and revoke a test device after updates.
   The disconnected page must hide stale live data; recovery must not replay old
   RPC results. Copy system name must write only this device's clipboard after a
   click. If clipboard permission is denied, manually select/copy the visible text.

Do not upload personal journals, commander/FID, paths, precise scan coordinates,
profiles, screenshots with identifying data, or certificate private keys.

## Synthetic developer/CI smoke

All six smoke modes use disposable protected profiles. The two offline foundation
modes never discover/read journals, even with --journal-dir. The four journal
modes require an explicit fabricated directory and never auto-discover it.

- `--smoke-test`, `--desktop-smoke-test`: unchanged G1A foundation checks.
- `--journal-smoke-test --journal-dir <G2A fixture>` and
  `--desktop-journal-smoke-test --journal-dir <G2A fixture>`: unchanged G2A checks.
- `--navigation-smoke-test --journal-dir <G2B fixture>` verifies both authenticated
  allowlisted projections and exact fabricated route/FSS/Scan/DSS facts.
- `--desktop-navigation-smoke-test --journal-dir <G2B fixture>` opens the real
  `/nav/route` React view with an owner-only fixture test layout containing both
  panels. Readiness requires that authenticated owner connection to request and
  receive both nonempty projections. Viewer calls cannot satisfy readiness.

The fixture helpers copy only committed fabricated files to fresh temporary
folders. CI retains mandatory Windows/Linux tests, security/build/vet, full Linux
race plus 25 focused race/stress repetitions, all six executable/native modes and
preview hashes. No production API calls or game actions are part of these checks.

Bounds: 2,048 route hops; 256 KiB per public projection; 512 retained bodies/system;
8 cached systems total, 64 identity contexts; 128 one-slot projection subscriptions;
32 signals/genus entries/event, 128-rune text. Overflow is explicit partial and an
oversized route never invents a destination from a truncated list. Surface gravity
uses 9.80665 m/s² per Earth g. No persistence, historical database, predictive
exobiology, neutron optimization, provider enrichment or other gameplay parity.
