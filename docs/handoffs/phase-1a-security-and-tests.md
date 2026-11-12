# Phase 1A: security and regression tests

**Branch:** `security/phase-1a-foundations`  
**Draft PR:** https://github.com/Jakoczig/daedalus/pull/2  
**Base:** `06a69635623c5bfacff51785a33fbb5f09948c9d` (fork main)  
**Implementation commits:**
- `c5908d5151d511de326e290fd0c3c44cfec28ac5` — authenticated service/clients, credential isolation, tests.
- `90732724ec43711ca4982726db4bd834f67d0d1f` — npm commands and blocking CI jobs.

Documentation commits follow those implementation commits; use the PR's commit
list for their hashes (this document cannot contain its own commit hash).
The GitHub connector published trees verified byte-for-byte against the locally
tested Git trees because local Git had no push credentials. The local branch
was synchronized with the published commits. Main, upstream Varzeki and PR #1
were not modified. No merge, release or deployment was performed.

## Fixed and architectural decisions

- Anonymous/cross-site WebSocket upgrades fail before receiving game data.
  The shared boundary authorizes each RPC and broadcast; inherited handler names
  are rejected. Viewers have an explicit display-method allowlist, not a naming
  convention. Settings, secrets, authentication, host controls/downloads and
  debug/broadcast injection remain owner-only.
- Existing Go windows use HTTP localhost unchanged. The service binds that port
  only to 127.0.0.1. Same-origin POST plus loopback address and hostname checks
  bootstrap an owner session; local processes/user account are explicitly trusted.
- Remote access is disabled by default. Optional direct TLS plus a one-use,
  five-minute owner-issued pairing code grants read-only display access. Cookies
  are HttpOnly/SameSite Strict, and remote cookies are Secure with a __Host name.
  Sessions expire after 12 hours, revoke for existing sockets, and die on restart.
  Owner cookies cannot authorize the TLS listener. Proxy headers confer no trust.
- Ordinary preference responses contain allowlisted settings, never stored API
  keys, OAuth tokens or commander credential fields. API keys have separate
  write/clear operations returning only configured state. Full-view preference
  saves preserve omitted credentials, including rotated tokens.
- Preferences and OAuth updates share serialized atomic file writes. Removed
  the stale response cache. CAPI callbacks validate state before consuming the
  pending request; disconnect cancels pending login and invalidates delayed
  refresh/exchange responses so they cannot restore credentials.
- Provider exceptions/status text and socket debug payloads are not reflected
  as credential-bearing diagnostics. Existing Inara mappings and queue behavior
  were deliberately left for Phase 1B.

## Files changed

- Service: `src/service/main.js`; `src/service/lib/{socket-auth,socket-server,preferences-store,event-handlers,events}.js`; `src/service/lib/event-handlers/integrations.js`.
- Client: `src/client/lib/socket.js`; `src/client/components/settings.js`.
- Test/CI: `package.json`; `.github/workflows/ci.yml`; seven `tests/service/*.test.js` files; `tests/helpers/no-external-network.cjs`; disposable `tests/fixtures/tls/{README.md,cert.pem,key.pem}`.
- Documentation: `README.md`; this handoff; `docs/handoffs/phase-1b-inara-scope.md`; `docs/testing/phase-1a-windows-smoke.md`; `docs/superpowers/plans/2026-11-12-phase-1a.md`.

## Verification (2026-11-12)

| Command | Result |
| --- | --- |
| `npm ci --loglevel=error` | Passed, lockfile unchanged; 896 packages installed. |
| `npm test` / `npm run test:unit` | Passed: 15 tests, 0 failures/skips. |
| `npm run lint:security` | Passed on all changed JavaScript and test helpers; blocking in CI. |
| `npm run build:client` | Passed: 62 pages generated and exported. Existing Browserslist freshness warning remains. |
| `node node_modules/standard/bin/cmd.cjs "src/**/*.js"` | Failed with 1430 legacy diagnostic lines outside the clean changed-file gate; global CI lint remains explicitly advisory. |
| `git diff --check` | Passed. |

Local host: Windows, Node v24.22.1. npm was missing from PATH, so a disposable
untracked npm 11.6.2 wrapper under ignored temp/tools supplied the normal npm
commands. No global tool installation or dependency upgrades were made.
Go is unavailable locally; Windows compile/WebView2 smoke verification is delegated
to the existing CI build-app job. CI results are recorded below when available.

Tests cover real HTTP/HTTPS/WS connections, cross-origin and DNS-rebinding-style
bootstrap rejection, anonymous denial, forged roles/inherited methods, pairing
reuse/expiry/rate limits, revocation, broadcasts, sanitized exceptions, real
preference RPCs, token preservation and delayed CAPI responses. An actual child
service starts with a temporary profile, loads a synthetic journal, streams an
appended event and answers mining/materials/navigation requests. That fixture
normalizes the journal path to forward slashes; it does not certify every Windows
save-directory/watch configuration. Final test commands preload a loopback-only
HTTP/HTTPS/fetch guard. An early lifecycle-test attempt before injection was wired
unintentionally reached Frontier with synthetic credentials and received HTTP 500;
no real credentials or game data were used. The guard was added before final runs.

A fresh read-only security-boundary investigation and a separate candidate review
were performed. The reviewer found the Spansh-setting allowlist omission and the
CAPI disconnect race; both were reproduced and fixed with regression assertions.
No concrete surviving anonymous RPC bypass was reported. The author also inspected
the complete source/test/config diff. This is not comprehensive security certification.

## Remaining verification, concerns and approvals

- Run the [manual Windows/tablet procedure](../testing/phase-1a-windows-smoke.md)
  before release: native windows, tablet certificate trust and pairing UX, game
  journal/JSON rotation, real Elite session and approved provider login. Automated
  protocol tests and Next build do not prove those device interactions.
- Credentials remain in the existing per-user Preferences.json, not an OS vault.
  Windows protection relies on the profile ACL. Local malicious processes and
  compromised same-origin code remain inside the owner trust boundary.
- Read-only tablets cannot change shared settings, control host media/downloads
  or issue arbitrary sync relays. Sessions require pairing again after restart
  or 12 hours. Certificate provisioning is required to enable remote access.
- `npm ci` reported 45 dependency vulnerabilities (3 low, 9 moderate, 26 high,
  7 critical); dependency triage, updater integrity, release assets and audio
  distribution rights remain separate release blockers from the original audit.
- CI jobs fail on regression/lint failures without suppression. Making their
  status checks mandatory in branch-protection rules is a maintainer repository
  policy decision; no branch protection was changed.
- No further approval is needed to review this draft. Merging/releasing/deploying,
  enabling real uploads and provider registration/live credential testing still
  require the maintainer's explicit approval and applicable provider consent.

**Next task:** Phase 1B Inara only, using the
[independently verified scope](phase-1b-inara-scope.md). Do not treat Inara sync as
repaired by this PR and do not automatically start it from this handoff.
