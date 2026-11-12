# G1A offline Windows/Linux developer preview

This preview embeds the existing 62-route React static export in a native Wails
v3.0.0 executable. Node 24.22.1/npm are build tools only. The executable never
starts the Node service, Python, EDMC or a separate browser engine. Wails uses
the OS WebView2 (Windows) or GTK4/WebKitGTK 6.0 (Linux).

## Build prerequisites

- Go 1.26.10 (go.mod also requires Go 1.26.0 and pins the toolchain).
- Node 24.22.1; install exactly the dependencies in package-lock.json with npm ci.
- Windows: installed Microsoft WebView2 Runtime. The preview does not download it.
- Ubuntu 24.04+ / Debian 13+: `sudo apt-get install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev`. A working X11 or Wayland session is needed for a window.
- Headless Linux native-window verification also needs `xvfb dbus-x11`.

At the repository root:

```sh
npm ci
npm test
npm run lint:security
npm run lint:go-assets
npm run test:go-assets
npm run build:go-assets
cd go
go mod verify
go test ./...
go vet ./...
go build -o bin/daedalus ./cmd/daedalus
./bin/daedalus --smoke-test
./bin/daedalus --desktop-smoke-test
./bin/daedalus
```

On Windows use `go build -o bin/daedalus.exe ./cmd/daedalus` and
`.\bin\daedalus.exe`. Generated embedded assets are intentionally ignored;
build them before Go tests/build. Export refuses missing/stale/symlinked inputs.
Rebuild assets after frontend changes; rebuild the executable after exporting.

Linux race checks: `go test -race ./...`. Headless native window:
`xvfb-run -a dbus-run-session -- ./bin/daedalus --desktop-smoke-test`.
No `server` build tag: it would select a different Wails runtime.

Ubuntu 24.04 restricts unprofiled user namespaces. If WebKit fails with
`bwrap: setting up uid map: Permission denied`, it needs an administrator-reviewed
AppArmor profile granting `userns,` to the exact built executable, as described
in the [Ubuntu release notes](https://documentation.ubuntu.com/release-notes/24.04/#unprivileged-user-namespace-restrictions).
The desktop CI installs that narrowly attached profile in its disposable runner.
The executable never installs a profile or changes system settings itself.
Do not disable WebKit's sandbox or turn off system-wide namespace restrictions.
Release packaging must resolve this prerequisite for a fixed installed path.

## What the two smoke modes prove

`--smoke-test` starts the same kernel-selected `127.0.0.1:0` listener used by
the desktop, reads root/deep-link and JS/CSS assets over HTTP, obtains an owner
cookie, requests offline status through a real authenticated WebSocket, closes
connections and stops the listener. It opens no GUI and contacts no provider.

`--desktop-smoke-test` opens a real native window and waits up to 30 seconds for
the embedded React client to authenticate and request loading status. It then
quits the native app and closes the service. Missing handshake is a failure.
This is rendering/bootstrap/lifecycle evidence, not a visual or gameplay audit.

## Manual checklist

1. Start the legacy app on port 3300, then launch the Go executable. Both must
   coexist. Confirm the new listener with `Get-NetTCPConnection` on Windows or
   `ss -ltnp` on Linux: one random TCP port bound to 127.0.0.1, never 0.0.0.0,
   :: or a LAN interface. The Go process must not start Node/Python/EDMC.
2. Verify the existing theme/navigation loads and the main panel explicitly
   says G1A offline, gameplay unimplemented, no commander/journal data loaded.
   No real credentials, game install or journals should be needed.
3. Visit `/exploration/system`, `/nav/map`, and other navigation links. Reload
   each address: matching exported HTML and `_next` JS/CSS must load, with the
   same honest offline panel. Actual gameplay pages are gated in G1A so they
   cannot request unported RPCs or display fabricated zero-value game cards.
4. In a same-machine browser at the exact local URL, root/deep links can load
   public static UI. Data transport requires the local HttpOnly SameSite=Strict
   owner session and exact Origin/Host. Anonymous WS, forged hosts, cross-origin
   bootstrap/WS, proxy headers and frames above 1 MiB must fail (automated tests).
5. Close the last native window. Confirm its process, listener and WebSocket
   are gone. Relaunch, interrupt startup and repeat; no orphan service should
   remain. Startup-failure/context-cancellation paths are unit tested.
6. A second native window must use the same service URL, never StartLocal again.
   G1A exposes no new-window UI; window creation is isolated behind the launcher.
7. Linux without DISPLAY/WAYLAND_DISPLAY: launch reports a graphical-session
   diagnostic and exits nonzero; headless smoke still works if runtime libraries
   exist. Missing linked GTK/WebKit libraries may fail in the OS loader before
   Go can print a diagnostic. Use `ldd bin/daedalus` to identify missing libraries;
   install the prerequisites above. There is no Node fallback or auto-installer.

## Security and scope

Only real loopback peers can bootstrap owner, with exact Host/Origin and no
forwarded headers. Roles supplied by clients are ignored. Sessions are random,
memory-only, expire after 12 hours, are bounded to 128, and are revoked at shutdown.
Only `getLoadingStatus` and loopback-only `hostInfo` exist. All other RPCs return
a generic not-implemented error. No Wails Go services are bound to privileged
operations. The native WebView handler is not an additional network listener.

Remote second-screen access is **not enabled in G1A**. Tablets receive no
privileges. The legacy owner/viewer model and existing pages remain unchanged
when used with the legacy service. The G1A marker alone selects the offline shell.
G1B must implement authenticated HTTPS pairing and complete authorization parity
before any remote listener is introduced.

Journals, Steam/Proton paths, commander state, game controls, provider APIs,
Inara/CAPI, uploads, audio/video and plugins are unported. No performance or
feature-parity claim is made. Do not replace the legacy installer or publish this
preview as a working game companion. See the [handoff](../handoffs/g1a-go-wails-foundation.md)
for measured checks and remaining manual gaps.
