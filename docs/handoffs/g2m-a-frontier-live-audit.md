# G2M-A — owner-authorized Frontier live audit

Continues draft PR #17 from `a36d4d4868da3ef4c312faeb1b5a6b046bd3c910`. Audit date: 2026-11-18. Final correction head, CI and verified previews are recorded in the PR description. G2M-B is a separate stacked milestone.

## Actual evidence and limits

The maintainer reports working OAuth and a deployed registered HTTPS callback. Computer Use found the actual Windows ERIDION launcher/dashboard and running Elite Dangerous. Owner-triggered profile and Community Goals reads succeeded through the existing terminal. The original game/application were not stopped or replaced; settings were closed again. No consent/upload/security setting was changed.

Explicit local diagnostics used the same OS user's existing consent, selected credential namespace and verified commander association for one bounded GET at each approved endpoint. Profile, market, shipyard and communitygoals returned HTTP 200; fleetcarrier returned HTTP 204 (no owned carrier). Calls observed the conservative minute cooldown. Diagnostics performed no refresh, credential write, account modification, journal reading, listening service or upload. The original native profile was read-only; only value-anonymized structures were retained outside it. No Inara request was made.

This establishes actual response contracts and working existing authorization, not corrected native UI acceptance against real gameplay, a native restart, live refresh-token rotation or disconnect/revocation. Those remain manual. Token rotation and authorization/revocation fixtures remain required; the working login was not revoked.

## Endpoint / field matrix

| Endpoint / fields | Live structure and correction | Source limits |
| --- | --- | --- |
| Profile name | Exact verified journal name and FID namespace association retained | Numerical CAPI commander ID is never a journal FID |
| Credits, debt, ranks | Exact-decimal SQLite facts; additional soldier/exobiologist/builder/learner ranks; debt and missing rank references in Commander | Profile progress percentages and squadron/Powerplay account structure not established; no guessed fallback |
| Owned ships | ID-keyed dictionary; documented array form tested too. Exact IDs, key agreement, values, location and optional name/ident persist; Fleet includes CAPI-only gaps and Commander counts the same union | Journal names/model/state win; sale/transfer never becomes ownership/arrival. API current ship remains a reference |
| Current modules | Module/engineer/WorkInProgress_modifications dictionaries; typed slot/symbol/value/power-state/priority/blueprint and bounded modifier reference in Fleet details | Stored loadouts may be absent. API modifier units/absolute semantics remain unverified and do not feed calculations; final journal modifiers remain authoritative |
| Other profile fields | On-foot suits/loadouts, capabilities, mercCoins and undocumented rank semantics remain unsupported in these account pages | CAPI specialModifications/experimental-effect normalization is also deferred; journal engineering remains intact |
| Shipyard | Last station shipyard_list and module sale stock, persisted separately | Never owned ships or fitted/stored modules; no global inventory or observation time |
| Market | Last station identity and commodity quotes persisted as separate Frontier reference | Not a global/live feed, docking proof or guaranteed profit; no synthetic observation timestamp |
| Fleet carrier | HTTP 204 handled as normal no-owned-carrier reference | Full HTTP 200 owner-carrier normalization remains limited pending owner fixture; journal ownership remains separate |
| Community Goals | activeCommunityGoals global records normalized, stored and shown beside journal history | Actual response supplied no personal participation/progress fields. Global presence never subscribes a commander, invents contribution or deletes journal membership/rewards |

Fetch time, observation time and journal evidence remain distinct. These CAPI responses have no validated source observation timestamp, so a new fetch cannot overwrite newer witnessed gameplay. Rendering never fetches providers; consent, caches and conservative limits remain in use.

## Private local diagnostic

The desktop's command-only mode starts no desktop/service and cannot become a second owner. Run with the same Windows OS account and the adopted native storage directory from identity-v1.json, not the legacy Node profile:

```powershell
Start-Process -FilePath .\eridion.exe -WindowStyle Hidden -Wait -PassThru -ArgumentList @(
  '--frontier-diagnostic', 'profile', '--owner-read-only',
  '--data-dir', "$env:APPDATA\daedalus-go",
  '--diagnostic-output', "$env:LOCALAPPDATA\ERIDION-private-diagnostics"
)
```

Choose one endpoint: profile, market, shipyard, fleetcarrier or communitygoals. Wait at least a minute between diagnostic/desktop requests. Use a dedicated private output directory outside the original profile. Existing read consent, verified namespace/name and an unexpired token are required. Multiple recorded commanders require an explicit matching selected account; first discovery/last-reader guesses are refused. TLS verification and fixed endpoints remain mandatory; redirects are refused. The command never refreshes/revokes/replaces credentials. For expiration use the existing running desktop normally. The optional diagnostic maintains its own private cooldown and does not write the original history. Avoid parallel manual reads during an audit.

Only `*-structural.json` reports are written. Original strings, balances and numerical identities are replaced; ID/key relationships are synthetic, unknown property keys are redacted and depth/sizes are bounded. Never share the original profile, vault, raw responses or identifying screenshots. Repository fixtures were reduced further to representative entries/slots and public synthetic Asp/FSD symbols; they cannot reconstruct the real account/loadout. Diagnostic output is not packaged in artifacts.

## Verification and remaining acceptance

Red-to-green tests reproduced the keyed-fleet gap, Fleet-query omission and HTTP 204 failure. Added contracts cover reduced actual shapes, exact IDs, conflicting keys/current IDs, global versus personal CG data, malformed/empty arrays, unknown engineering semantics, CLI intent/profile preservation and diagnostic scrubbing. Existing migration, legacy pairing quarantine, failed-switch rollback, PKCE, vault/account and viewer/server regressions remain mandatory.

Final CI covers Windows/Linux builds/native smokes, complete Go/JS/security checks, Linux race detection, retained Chromium suites and independently verified unsigned preview manifests. Exact-head links/results are recorded in PR #17 after completion.

Manual: compare Fleet/Commander/station/CG after installing the preview when ready; verify a sale, transfer or new Loadout wins over cached CAPI; restart offline and inspect provenance; verify paired displays receive no credentials/private account operations. Do not revoke working authorization merely for testing. Live refresh/revocation needs a separately planned owner test.

Reference: [EDCD/FDevIDs contract at c7c29f7](https://github.com/EDCD/FDevIDs/blob/c7c29f7a13df543d4bc8b947eab29f88e06100fa/Frontier%20API/FrontierDevelopments-CAPI-endpoints.md). No upstream runtime code or proprietary artwork was copied.
