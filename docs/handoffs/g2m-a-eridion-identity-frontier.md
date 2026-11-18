# ERIDION G2M-A — identity, Frontier return and commander/ship refinement

Base: G2L `5f0bee9fb5df8ea243c54dadcc7edcc02581efef`. Branch: `go/g2m-a-eridion-identity-frontier`, stacked draft targeting `go/g2l-fleet-economy-commander-operations`. [Draft PR #17](https://github.com/Jakoczig/eridion/pull/17). Initial implementation commit: `0c8712b1b181d7d98faf39ab2428fd0814ef130a`; the cache-owner correction follows in the same draft. The final draft PR description is the exact-head verification ledger (commit, both CI runs, artifacts and independently checked hashes).

## Implemented

| Capability | Status / boundary |
| --- | --- |
| ERIDION / ERID identity | Native Wails titles, terminal/launcher/document title, Go module/import paths, new certificates, provider software label and preview executable names; legacy app and cryptographic compatibility identifiers preserved |
| Native profile migration | Versioned atomic identity manifest adopts existing native storage **in place**; same SQLite, checkpoints, backups, vault path, TLS keys, pairing grants and exclusive profile lock |
| Global commander selection | Multiple accounts OFF by default; owner launcher/settings cards; explicit selection stays pinned while another journal account is ingested; shared pages/database queries use selected FID |
| Credential/consent separation | Per-FID namespaces inside the same OS vault and transactional SQLite consent state; generations cancel in-flight work on change; legacy credentials without verifiable association retained encrypted and quarantined |
| Viewer permissions | Grants bind to original profile; changing selection closes sockets and prevents recovery under another account. Remote state v2 makes older native versions fail closed rather than ignore account scoping |
| Frontier authorization | Public approved client UUID, private local UUID override, exact registered HTTPS redirect, PKCE S256, five-minute single-use complete state binding; website source + strict loopback return and owner-only manual fallback |
| Frontier account reads | Existing bounded `/profile`, `/market`, `/shipyard`, `/fleetcarrier`; profile-name association required before other account endpoints. Numeric CAPI IDs are not treated as journal FIDs |
| Commander | Identity/portrait, current ship/model/system, exact credits when witnessed, ranks/progress/reputation, owned-fleet count, squadron/Powerplay evidence; Session unchanged |
| Ship/Fleet/CG | Human ship labels from existing bundled FDevIDs; explicit unknown module class; stored modules under Modules; concise statistics with optional diagnostics; expandable fleet/history and dedicated subscribed-goal cards retaining historical rewards |
| Source priority / uploads | Existing journal-first, demonstrably-newer associated Frontier, validated calculation and necessary approved gap-fill rules retained. Fetch time never proves newer observations. No new upload permission or production request |

## Exact profile/account migration and recovery

Default new profile: OS user configuration directory / `eridion-go`. If `daedalus-go` already exists, `eridion-go/identity-v1.json` contains version 1 and the fixed storage name `daedalus-go`. This is logical adoption, **not a database copy or physical directory rename**. The original absolute path keeps Linux Secret Service key lookup and old/new native exclusive locking intact. No keys, databases or permissions are regenerated. Explicit `--data-dir` keeps using the explicitly supplied native directory.

Atomic manifest creation resumes after an interrupted temporary write; diagnostic remnants remain. Invalid/ambiguous manifests or a missing adopted profile stop startup with recovery instructions instead of creating an empty database. Restore the original storage directory and valid manifest from backup, then restart. An unmodified G2L native executable can still find the original directory, but remote v2 deliberately remains disabled there; use a full pre-upgrade backup for a true downgrade. Never hand-edit encrypted vault files or reset TLS merely to clear a warning.

Back up both the identity-manifest directory and the adopted native storage directory. SQLite export remains a database export, not a credential/certificate/portrait backup. Native `daedalus.sqlite`, cookie name and Linux vault cryptographic identifiers remain compatibility names. The Node/legacy profile is never adopted.

Existing personal API keys migrate only if their commander matches verified cached journal identity (or already has an explicit verified FID association). Legacy OAuth tokens without an established association remain encrypted but unavailable to an arbitrary selected account: reconnect Frontier. This avoids assigning account A's credentials to the last-seen account B. Empty offline profiles do not create an OS vault or require an unlocked Secret Service.

Cached projection ownership is explicit: catch-up/replay for B cannot authorize an older cached view from A. A selected profile can still recover its own cached view while another account replays. The deterministic regression failed before this correction and passes with the explicit FID check.

## Verification and practical limits

Focused failing-then-passing tests cover manifest adoption/interruption/recovery/locking, account pinning, durable namespaces, ambiguous legacy credentials, empty offline vault, PKCE/HTTPS redirect/expiry/replaced and cancelled state, in-flight token rejection, portraits and viewer isolation. Existing regression gates remain mandatory. Browser smoke covers Commander, CG, Fleet, Modules and Statistics at 1366×768 and 680×580 with loopback-only request interception and a second synthetic journal account.

Final Windows/Linux native, race, legacy/security, browser and artifact results are recorded in the draft PR verification block. Synthetic measurements are not live game or whole-process gameplay benchmarks. The CI evidence artifact contains screenshots, browser resource measurements and exact G2L/current lifecycle samples; single-pass timings are not a statistical speedup claim. Prior G2L browser screenshots in its accepted CI evidence provide the before reference; G2M-A artifacts provide the after views.

Pending owner acceptance: separately deploy the HTTPS website, real Frontier login/refresh/revoke and browser fallback on Windows/Linux, fresh/previous real profile migration, multiple real accounts, portraits and paired-laptop profile isolation. No live provider login or production upload was attempted. Frontier community-goal normalization awaits a verified authorized schema; Inara developer/CG authorization remains deferred. Stored ships without witnessed builds have unknown performance; no guessed shield/DPS/speed curves or new ship images are bundled. No owner-carrier physical test is claimed.

## Deferred G2M-B — recorded, not implemented

Rename Map→System, List→System List, Neutron Plotter→Spansh Plotter. Move small CSV access to the bottom of the unified Route screen; use supported official Spansh website/import, no guessed API. Integrate upstream BGS Tally contextually instead of a standalone main page. Remove Controls from primary navigation while preserving useful Galaxy information.

Galaxy layout: large top-down **X/Z**, immediately below it a **very thin, wide X/Y** matching the main width, immediately right a **tall, narrow Z/Y** matching the main height. Three independent projections with aligned shared zoom and markers, never rotated copies or equal-sized cards. EDAstro CC BY-NC-SA 3.0 reference is layout inspiration only; do not bundle without a licensing decision.

## Consolidated acceptance checklist

1. Back up the native profile; open ERIDION and confirm history/routes/consents and existing CA trust survive. Verify a second instance cannot become owner.
2. With Multiple accounts OFF, witness the normal commander. Enable it, choose A, then play B: confirm an explicit mismatch and no silent switch; switch deliberately and check Fleet/CG/ship data and own credentials.
3. Pair the laptop to A; switch to B and confirm A's session stops. Pair B independently; verify neither display can open owner/provider/portrait/account operations.
4. Check Commander, module inspector, stored modules, Fleet and Statistics with an actual engineered ship. Unknown/unwitnessed figures must remain unknown; Session/Cargo/Inventory/Ship Status should match prior behavior.
5. After separate website deployment, log into Frontier on the gaming PC, verify profile identity before other reads, test cancel/expiry/manual return and revocation. Keep uploads OFF. Review goal subscription removal/empty snapshots and historical reward retention.
