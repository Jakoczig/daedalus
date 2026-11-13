# G2A — EXPERIMENTAL, UNSIGNED, LOCAL JOURNAL PREVIEW (NO PROVIDER UPLOADS)

This limited Go/Wails preview reads local Elite Dangerous journals and four JSON
snapshots. It displays current/last-known system and ingestion health. Exploration,
ship, engineering, routing and provider integrations remain unimplemented. Keep
the original DAEDALUS application. This is not a release, installer or replacement.
No Node, Python or EDMC is needed at runtime on the host or secondary device.

## Download and verify

Open [draft PR #6](https://github.com/Jakoczig/daedalus/pull/6) and its successful
**Go desktop foundation** Actions run. Download the Windows or Linux artifact
named `g2a-…experimental-unsigned-local-journal-no-uploads`. GitHub sign-in may be
required; CI artifacts expire after 14 days. Each ZIP contains only the executable,
this README and SHA256SUMS, never a generated profile or private certificate key.

Extract to a new directory. Windows PowerShell:

```powershell
Get-FileHash -Algorithm SHA256 .\daedalus.exe
Get-Content .\SHA256SUMS
```

Linux:

```sh
sha256sum -c SHA256SUMS
chmod +x daedalus
```

Compare the executable hash to its own manifest. Different builds embed different
commit metadata; do not reuse a prior build's hash. SHA-256 checks bytes, not a
publisher signature. These builds are unsigned: follow your device's software
policy without disabling security protection or requesting administrator rights.

## Windows desktop and real-game acceptance

Prerequisites: x64 Windows with a graphical desktop and Microsoft WebView2 Runtime.
Go and Node are build-time dependencies only. From the extracted directory:

```powershell
.\daedalus.exe --smoke-test
.\daedalus.exe --desktop-smoke-test
.\daedalus.exe
```

The first two modes use disposable, isolated profiles and do not discover/read
personal journals, even if `--journal-dir` is supplied. The second opens a real
native window and closes automatically. Their inherited G1A labels describe that
offline foundation check. Normal launch enables G2A journal ingestion.

1. Launch normally with the game stopped. Confirm the native window remains usable
   and shows journal availability separately from **service connected/ready**.
2. Windows uses the actual **Saved Games Known Folder** API, including moved folders,
   then `Frontier Developments\Elite Dangerous`. It does not assume `C:\Users`.
   Open **Journal directory (local owner)** to inspect the selected source.
3. If automatic discovery does not find your installation, copy the full existing
   journal-directory path from Explorer into the owner-only input and save it.
   An empty selected directory is allowed while waiting for the game. The app
   never creates or writes game files. **Use automatic discovery** clears the override.
4. Start Elite Dangerous and load a commander. Confirm a real Location/FSDJump
   arrival changes the displayed system without refreshing. StartJump alone must
   leave the previous arrival system displayed until the jump completes.
5. Test leaving/rejoining the game, switching accounts, a new journal part, and
   stopping/restarting DAEDALUS. Unknown identity must not inherit a known
   commander's system. Startup replay reconstructs state; it is not new live
   activity and does not trigger provider side effects.
6. Close the native window and confirm the process exits. Only one process may own
   a profile. A second instance must fail to become another owner.

Normal private configuration: `%AppData%\daedalus-go`, separate from the legacy
application. `journal.json` stores only the owner-selected path in protected storage.
Do not share this profile: it also contains private TLS material and viewer hashes.
Use `--data-dir PATH` for an explicitly separate private test profile, or
`--journal-dir PATH` as a local process source override. Invalid optional source
settings show a recovery/error state and do not prevent the local desktop starting.
Damaged journal preferences do not silently select a different source; repair by
an explicit local-owner selection or clearing the override.

“Local journal available” describes file availability, not a game-process detector.
Shutdown events show session ended; after a crash with no Shutdown record the last
known system can remain visible. No invented commander/system is substituted.

## Secondary laptop/tablet

The accepted G1B HTTPS boundary is retained. Fresh profiles have remote access off.
An existing profile retains only its previously explicit enablement and grants.

1. Open **Remote display settings (local owner)**. Select this host's concrete
   private IPv4 address and HTTPS port (normally 3443), then explicitly enable.
2. Manually allow inbound TCP only on that private LAN/subnet if the host firewall
   requires it. No port forwarding, Internet exposure, wildcard listener or HTTP
   LAN listener is needed. The app does not alter firewall rules.
3. Transfer only the public CA certificate through a trusted channel, compare its
   SHA-256 certificate fingerprint with the owner display, then explicitly import
   and trust it on the secondary browser/device. Never transfer `tls.json`, cookies,
   keys or the whole profile. Never bypass a TLS warning or verification.
4. Browse the exact HTTPS address shown by the owner. Enter its six-character
   uppercase code (example `7K4M9P`) and a device name. Lowercase is accepted;
   I/L/O/U, spaces and separators are invalid. Codes expire in five minutes, work
   once and are replaced by a new code. The global ten-attempt/minute budget is
   shared across devices and is not reset by issuing a new code.
5. Confirm **read-only**, the correct live system, no journal path/commander/raw
   chat/inventory, and no owner settings. Test reload, Wi-Fi interruption, two
   viewers simultaneously, owner rename, and revoke with an open connection.

Viewer sessions remain 12 hours, Secure/HttpOnly/SameSite Strict. Revocation and
disable close active sockets. The same service reads the game files for every
window/browser. No paired device may select host directories or run host actions.
Trust installation steps and recovery precautions remain in the
[G1B runbook](https://github.com/Jakoczig/daedalus/blob/go/g1b-secure-remote-displays/docs/testing/g1b-secure-remote-displays.md).

## Linux / Steam Proton

Initial binary target: x86-64 Ubuntu 24.04+ with GTK4 and WebKitGTK 6.0 runtime
libraries (`libgtk-4-1`, `libwebkitgtk-6.0-4`) and a graphical session. No universal
distro or Wayland compatibility is claimed. Use `ldd ./daedalus` to identify missing
libraries. Start `./daedalus --smoke-test`, then `--desktop-smoke-test`, then normal
`./daedalus`. Xvfb/D-Bus are needed only for headless native testing.

Ubuntu's AppArmor user-namespace policy may require an administrator-reviewed
profile for the exact executable path so WebKit can create its sandbox. The app
does not install one. Do not disable WebKit's sandbox or system-wide restrictions.
The executable-specific CI example is in `.github/workflows/go-desktop.yml`.

Discovery checks common native Steam and Flatpak Steam roots, bounded modern/old
`libraryfolders.vdf` entries, and app `359320` prefixes under
`steamapps/compatdata/359320/pfx/drive_c/users/*/Saved Games/Frontier Developments/Elite Dangerous`.
Custom library locations and non-`steamuser` names are supported. For Wine/Heroic
or an undiscovered prefix, select the final journal directory locally. No drives
are recursively scanned. Usual private profile: `~/.config/daedalus-go` (or XDG).
Physical Linux/Proton gameplay, distro permissions and Wayland remain manual tests.

## Synthetic developer verification

Source checkout/build tools only:

```sh
npm ci
npm test
npm run lint:security
npm run test:go-assets
npm run lint:go-assets
npm run build:go-assets
cd go
go test -count=1 -timeout=4m ./...
go vet ./...
go test -race -count=1 -timeout=4m ./...  # Linux / supported cgo toolchain
go test ./internal/journal -run '^$' -bench '^BenchmarkJournalReplay$' -benchmem -benchtime=3x
go build -o bin/daedalus ./cmd/daedalus
```

CI prepares committed fabricated records with
`node scripts/build/journal-smoke-fixture.cjs`; its `directory` step output is passed
to `--journal-smoke-test --journal-dir PATH` and
`--desktop-journal-smoke-test --journal-dir PATH`. Both require an explicit fixture
and use disposable profiles. The native journal mode requires an authenticated
owner React card request and delivery of a nonempty sanitized system before exit;
a viewer cannot satisfy that readiness signal. These tests do not read a personal
installation, launch Node/EDMC/Python, or contact a provider. Native renderer smoke
is not a physical gameplay or visual/pixel acceptance test.

## Bounds and remaining acceptance

- Poll cadence defaults to about 500 ms. Journals: 1 MiB/record, at most 4096 source
  directory entries; malformed complete lines are skipped with sanitized counts.
  Partial final lines await newline. Source positions, not timestamps, identify
  records. Replacement/truncation reset a cursor; restart reconstructs by replay.
- Four sidecars only: Status, Cargo, Materials, NavRoute; 4 MiB each. Complete valid
  UTF-8 JSON with stable observed metadata replaces the last-good cache; older
  dated snapshots are stale. Missing/partial/invalid files never erase valid data.
  Snapshots are identity-unverified internal source data; inventory gameplay and
  commander attribution are deliberately not implemented or exposed to viewers.
- Known commander caches are bounded to 64 plus an isolated unknown state. Public
  subscribers are bounded to 128 with a one-summary latest-value buffer; intermediate
  UI summaries may coalesce while source records are still processed sequentially.
- Exact 64-bit game IDs remain internal decimal strings. The G2A browser schema
  contains only sourceState/currentSystem/lastEventType/lastEventAt/importInProgress/
  eventsRead. It contains no raw event, FID, path, coordinates or historical list.
- Arbitrary in-place journal edits preserving identity, size and modification time
  are not detected. Slow/broken/network filesystems can stall OS reads; cancellation
  is checked between records/fragments, and the profile lock is retained until the
  old service exits. Power loss, unusual filesystem faults, large real archives,
  real game sessions, sleep/roaming and normal Linux desktop packaging need manual
  acceptance. No exactly-once external-delivery or resource-savings claim is made.

Report OS/build/run, steps, synthetic counts and sanitized errors. Never attach
personal journals, commander identifiers, credentials, private keys or profiles.
No release, deployment, merge or G2B work is authorized by these artifacts.
