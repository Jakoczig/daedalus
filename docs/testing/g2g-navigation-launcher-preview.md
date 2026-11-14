# DAEDALUS G2G — EXPERIMENTAL, UNSIGNED COMPANION / NAVIGATION PREVIEW

Portable preview, no installer or Release. Preserve the original Node application.
Close another Go preview before starting: the profile remains exclusively locked.
Extract executable, README.md and SHA256SUMS. Compare both listed SHA-256 hashes
before running. Windows: Get-FileHash .\daedalus.exe -Algorithm SHA256.
Linux: sha256sum -c SHA256SUMS, chmod +x daedalus, then ./daedalus.

Windows 10/11 x64 needs WebView2 Evergreen. Linux x64 needs GTK4, WebKitGTK 6.0,
a graphical session and a session D-Bus (Ubuntu 24.04: libgtk-4-1 libwebkitgtk-6.0-4).
Tray behavior needs a StatusNotifier/AppIndicator host. If the Linux host cannot
be detected through gdbus, close-to-tray minimizes the launcher window instead,
so a hidden service cannot become inaccessible. Explicit Quit remains available.
Physical Linux/GPU/Wayland/tray compatibility needs manual verification.

## Launcher and secondary display
Startup opens the small status launcher only. Settings and Pairing remain there;
Open Dashboard creates a separate native window. Closing that dashboard preserves
the one Go service/journal reader and paired browser connections. Launcher close
minimizes to tray by default; Settings can change it to quit the service. Tray
menu offers Show Launcher, Open Dashboard and Quit. No desktop dashboard opens
because of FSD events. No Node.js, Python or second server is started.

The original Go profile location and remote HTTPS pairing remain unchanged.
Remote access is disabled by default; explicitly enable it in Pairing settings.
Use the host's LAN IP, allow only the selected HTTPS port through the firewall,
install only this host's public CA certificate on the secondary laptop, and verify
its fingerprint locally. Never copy CA private keys or bypass TLS verification.
The owner issues a one-time six-character pairing code, valid for five minutes.
Paired devices stay read-only; owner can rename/revoke them. No owner bootstrap
or native-window commands are available through the remote listener.

## Navigation, providers and automatic displays
Cartography fits all retained known bodies, independently of catalogue paging.
Zoom/pan uses the map scroll area; Fit restores the full schematic. Large systems
reduce labels while keeping every retained body selectable. Body inspector keeps
local scan/sampling facts separate from community references. Route badges show
star scoopability; a fuel-scoop module must be witnessed before claiming equipment.
Security/Powerplay fields are omitted unless a journal explicitly supplied them.

Provider controls/cache timestamps are under Settings → Data Providers. EDSM is
opt-in and read-only: current or explicitly requested system name and network IP
are disclosed; no credentials, journal records or coordinates are uploaded. Only
an owner can initiate a request. Viewer search/view uses local/shared cached data.
Lookups have an eight-second deadline, per-response 2 MiB limit, one request group
per minute, six-hour cache, and 16 systems / 256 bodies / 64 stations per system.
Stations without id64 bind to the verified body's matching provider ID and name;
provider IDs are never treated as game SystemAddress. Live provider acceptance
is pending. Existing Spansh search/route interfaces were evaluated; journal/EDSM cover the
required lookups. Broader Powerplay search and routing remain deferred.

Galaxy uses an original illustrative four-arm 2D texture, not EDAstro assets or
measured total stellar density. Green marks the last witnessed player position;
cyan marks search and amber destination. ED Y is vertical: X/Z, X/Y and Z/Y are
linked projections. Only known galactic coordinates are drawn. Navigate is an
explicitly disabled future action. No Bubble/Colonia/community overlay is added.
Offline utility: node scripts/build/galaxy-density.cjs --out <directory>.
For licensed coordinate input use --csv <x,y,z CSV> --source <provenance>
--license <license>. It streams input, bins and smooths log-density, emits three
small PNGs plus metadata, and never downloads data. Dump format conversion and
actual licensed density datasets remain future work. No runtime star-dump download.

Automatic FSD settings have separate charge/jump toggles, owner/all-viewer/individual
paired-device selection and optional restoration. Defaults are off. Charge needs
explicit hyperdrive Status evidence; generic supercruise charge is not enough.
Only fresh live evidence with a verified route triggers a compact route/mini-galaxy
screen. Arrival/cancel restores where enabled; manual navigation overrides the
whole episode. Stale/replayed evidence cannot start automatic transitions.

## One gameplay acceptance session
1. Launch without dashboard; confirm journal/import and paired laptop live updates.
2. Open/close dashboard, close launcher to tray, restore it, then verify explicit Quit
   disconnects clients and a second instance cannot acquire the same profile.
3. Check Ship fit/readability, full large-system Fit/zoom/catalogue scrolling, route
   scoop/equipment/security facts, waypoint View System and galaxy projections.
4. Opt into EDSM in a populated system, verify bodies AND stations and owner search,
   then disable it. Local scan/sampling facts must remain. Viewers cannot query online.
5. Target only the laptop for charge/jump, cancel/arrive, manually override, reconnect
   and revoke. Repeat physical Linux/Proton and long-idle/resource checks.

CI uses fabricated journals/provider transports. It is not real gameplay, live
provider, physical-device or whole-native CPU/RAM certification. Artifacts never
include generated profiles, private CA keys, tokens or personal journals.
