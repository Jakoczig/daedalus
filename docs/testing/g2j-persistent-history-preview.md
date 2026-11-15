# DAEDALUS G2J — EXPERIMENTAL, UNSIGNED, LOCAL-FIRST PREVIEW

Portable Windows/Linux x64 executable; no installer or GitHub Release. Keep the
original application intact. Close other Go previews first: profile ownership is
exclusive. The native executable never launches the legacy Node backend.

Extract the executable, this README and `SHA256SUMS` together. Windows 10/11 x64:
compare `Get-FileHash .\daedalus.exe -Algorithm SHA256` with the manifest, then run
`daedalus.exe`. WebView2 Evergreen is required. Linux x64: `sha256sum -c SHA256SUMS`,
`chmod +x daedalus`, then `./daedalus`; GTK4, WebKitGTK 6.0, a graphical session and
session D-Bus are required (Ubuntu 24.04: `libgtk-4-1 libwebkitgtk-6.0-4`). Saved
Linux credentials additionally require `secret-tool` and unlocked Secret Service;
no plaintext fallback. The native SQLite library is compiled into the application.

## Local history and recovery

The private Go preview profile contains `daedalus.sqlite` and SQLite WAL/SHM files.
Do not copy a running database's individual files. Launcher → Settings → Data /
History provides a verified consistent Backup/Export, staged Restore, derived-view
rebuild and explicit quarantined-source reimport. Exports contain private commander
and location history; select a private local destination. No periodic backups run.
Verified backups precede schema migrations and healthy-database replacement. A
corrupt original is preserved for diagnosis; it is never silently reset.

Initial import retains relevant journal history. Subsequent starts restore cached
views and validate source identities/checkpoints before streaming only missing
records. Cached/waiting/catch-up labels are distinct from live game evidence.
Copied journals require matching full-file hashes before aliasing; changed sources
are quarantined for owner recovery. Missing original files do not delete history.

Restore requires application restart and clears publication exceptions, sets Never
and pauses uploads. Previously public data cannot be withdrawn by restoring a DB.

## Capability and approval matrix

| Capability | Implemented scope / remaining gate |
|---|---|
| SQLite / journal | Local WAL, serialized transactions, normalized relevant history, exact decimal IDs, atomic source checkpoints and cached reducers, incremental catch-up, migration/backup/export/recovery. Indexed commander-scoped history and construction depot resource display. |
| Publication | Persistent Never (default), Live, After sale; independent destination/category consent. Proven prior system-scan sale links; organic sales conservatively ambiguous with explicit owner confirmation. Bounded historical preview/reconfirmation, durable references/receipts; no automatic historical send. |
| Frontier | Existing PKCE/vault/refresh adapter remains developer-approval gated. Exact profile identity, ranks and typed owned/current ship references persist separately with provenance. Incomplete stored loadouts stay unknown. Station snapshots remain supplemental and are not rebroadcast as current docking evidence. `/communitygoals` response contract and optional `/journal` recovery are not enabled; registration/live authorization and complete response fixtures remain required. |
| Inara | Approval/key/read gates, exact-name profile matching, conditional missing/stale facts, independent public-goals read consent with documented `eventData: []`. Supported uploads and explicit <=30-day historical review are fixture-tested; whitelisting/live acceptance absent. |
| EDSM | Existing consented current/selected-system bodies/stations and account flight-log read/upload adapters preserved. Native offline nearest searches saved station references. A separately consented public reference adapter supports radius <=100 LY sphere lookup (at most 32 locked-coordinate candidates), selected-system stations and verified MarketID market/shipyard/outfitting previews. No automatic fan-out; unknown pad size stays unknown. Public Sol API response shapes were verified read-only on 2026-11-15 despite HTTP 403 on documentation pages. No account or upload acceptance is implied. |
| Spansh | Four documented exact-ID GET lookups, parent/coordinate validation, bounded response/cache and persistent cooldown. Station references feed offline nearest-service search. Contract fixtures; live-read acceptance pending. Route generation remains an external planner link. |
| Elite BGS | Opt-in named V5 systems/factions/stations pages (20/page, max 10 pages), latest tick, bounded cached allowlisted facts. Community provenance; no causal claims. Contract fixtures; live-read acceptance pending. |
| Raven Colonial | Offline journal depot/resources and contribution history first. Optional documented system/project/last/stats GETs, no contributor identities returned. Contract fixtures; canonical production write/authentication authorization remains unavailable; no create/update/contribute/supply action enabled. |
| Canonn | Opt-in exact `system.id64` filtered Guardian ruins, Thargoid structures, bark-mound and generation-ship collections; max 32/page, response system association checked, contributor identities removed. Published Strapi routes/model contracts inspected; fixture-tested, live availability/filter acceptance pending. Other collections not assumed supported. |
| EDAstro | Existing bounded rare-GEC `coordinates` array parser and cached reads preserved; no full-feed fallback. Selected live biology/carrier publication requires consent and timing policy. Status 401 remains no-new-observations, not a successful upload count. Historical release contract unverified and held. |
| EDDN | Existing journal/1 plus local commodity/3, shipyard/2 and outfitting/2 schema validation. Fresh sidecars require exact committed marker timestamp and current verified dock/MarketID/system/station. Separate categories and After sale live exceptions; conservative ship/module subset excludes unverified purchase/rank restrictions. No CAPI market rebroadcast or delayed historical relay. Fixture-tested; production publication not performed. |
| Engineered statistics | Native final observed modifiers per fitted slot, unchanged original values, rated enabled-module power sum and versioned bounded cache. No double engineering multiplication. Unsupported range/thruster/shield/resistance/weapon models remain unavailable; permission-cleared curves and real gold-reference builds are required. No restricted new Coriolis/EDSY data copied. |

Reads/uploads are OFF unless explicitly consented. A configured adapter is not
proof of live provider acceptance. New network traffic is owner-triggered, fixed-
host HTTPS, bounded, cached and throttled. Credentials remain in DPAPI/Secret
Service, never SQLite. Provider sees IP and requested/submitted location fields.
Historical preview returns sanitized outgoing event fields, never request keys.
Timeout delivery stays uncertain and is not silently retried as a new submission.

After sale is **hold + manual review only**, with no automatic sale-triggered
uploads. Manual review cannot authorize unsupported delayed recipients (EDDN,
EDSM or EDAstro). Inara's existing approved-category one-off historical review
still requires account/application approval, sale proof where applicable and
explicit owner reconfirmation. No automatic delayed-upload contract is assumed.
After sale holds location-sensitive categories across destinations. A live-only
exception names one exact recipient/category and requires a separate disclosure
acknowledgment. Sale evidence alone never enables a provider or enqueues history.

## Secondary laptop

On the owner PC, explicitly enable Remote Displays with its actual LAN address.
Install/trust only the exported public CA certificate on the second laptop; never
copy the private CA key or entire profile. Allow only the selected HTTPS port in
Windows Firewall. Open the displayed `https://<PC-LAN-address>:<port>` URL; verify
the certificate and enter the six-character one-time code generated by the owner.
No certificate-verification bypass. Remote access is disabled by default. A paired
device is read-only: no private DB/history, backup, credentials, provider requests
or upload authorizations. Revoke a test device and confirm it stops receiving data.

## One gameplay acceptance checklist

1. Import real journals offline; inspect two commanders, ships/engineering and
   scans/sales/resources. Restart unchanged, then after a missed game session;
   cached views must appear promptly and catch up without false live status.
2. Launch/restart Elite; rotate journals and jump/scan/sample while both owner and
   laptop are open. Confirm exact system/body/route continuity and no cross-FID data.
3. Create/verify a private export. Test restore only against a disposable preview
   profile; verify original preservation and Never/paused publication afterward.
4. Inspect final modifier values against the game's modules UI. Treat unsupported
   ship performance as unavailable; check rated draw separately from combat draw.
5. Keep uploads off first. Inspect sale ambiguity and historical preview; test each
   approved provider separately only with explicit live-production authorization.
6. Test long idle, reconnect, revoke and shutdown. Physical Linux/Wayland/Proton,
   certificates/firewall, GPU/tray/keyring and real provider accounts remain manual.

CI artifacts contain only executable, README and SHA-256 manifest; never a profile,
database, journal, CA key or credential. See the new G2J draft PR for exact final-
head Windows/Linux CI, performance measurements and verified artifact digests.
