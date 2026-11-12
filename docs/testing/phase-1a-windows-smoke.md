# Phase 1A Windows and paired-display acceptance

Keep all provider uploads disabled for these checks. Use synthetic journals first;
never attach Preferences.json, tokens, API keys or real journals to an issue.

## Automated checks

Use Node 24, `npm ci`, `npm test`, `npm run lint:security`, and
`npm run build:client`. Tests use temporary profiles and synthetic secrets;
the test preload rejects non-loopback Node HTTP/HTTPS/fetch traffic.
The CI Windows job compiles Go and runs `test-app.exe --smoke-test`.
That smoke test initializes WebView2; it does not exercise an Elite session.

## Local Windows owner

1. Build the client/app/service using BUILD.md. Start DAEDALUS Terminal normally.
   The existing launcher and new terminal windows continue to navigate localhost.
2. Verify loading finishes, open a second native terminal, then open a browser at
   `http://localhost:3300` (substitute the launcher's configured port).
3. Check navigation, route, mining, engineering/materials, exploration, controls
   and journal pages. Append synthetic events or run a game session with uploads
   disabled; verify live updates arrive in all owner windows.
4. In Settings, toggle sounds, exploration filters and Spansh. Restart and verify
   they persist. Set an EDSM/Inara key locally: the field clears after Save;
   unrelated settings changes preserve the credential. Clear explicitly to remove.
   Inspect traffic locally without recording the request containing the key.
5. Start Frontier authentication only with an approved registered client/test
   identity. Wrong-state callbacks must not cancel it. Disconnect while a token
   exchange/refresh is pending; the token must stay removed. This authenticated
   provider check is **not** part of offline automated verification.

## Enable secure tablet access explicitly

Obtain a TLS certificate/key for the PC's actual DNS name or IP. The certificate
must be trusted by the tablet and include that name/IP in its subject alternatives.
Keep its private key outside the repository. Do not use the disposable test key
under tests/fixtures/tls, and do not turn off browser certificate verification.

Set these environment variables before launching the service or native app:

```powershell
$env:DAEDALUS_REMOTE_PORT = '3443'
$env:DAEDALUS_TLS_CERT = 'C:\private\daedalus-cert.pem'
$env:DAEDALUS_TLS_KEY = 'C:\private\daedalus-key.pem'
# Optional: bind a particular LAN address instead of all remote interfaces.
$env:DAEDALUS_REMOTE_HOST = '192.168.1.10'
```

Only allow the HTTPS port through the Windows firewall on the intended private
network. The HTTP port remains on 127.0.0.1. TLS terminates directly in the service;
forwarded/proxy headers do not confer owner trust. With no remote port configured,
remote access is disabled. A missing certificate/key fails startup rather than
falling back to plaintext remote access.

1. Visit `https://<certificate-name>:3443` on the tablet. Before pairing, it must
   show the pairing screen and receive no journal/commander data.
2. In a local terminal's Settings, Create pairing code. Enter it on the tablet
   within five minutes. It is valid once; do not share screenshots of the code.
3. Check two tablets/tabs, journal updates, navigation, engineering and mining.
   Settings/authentication, host media controls/downloads, debug injection and
   arbitrary sync relay must be denied to paired displays. Theme remains local.
   Remote control of those host functions is intentionally outside this role.
4. Reuse the code and try an expired code: both must fail. A client-supplied
   `role: owner` must not grant privileges. Plain HTTP pairing must fail.
5. Revoke all displays on the PC. Existing remote sockets must lose access;
   they cannot reconnect using the old cookie. Restart also clears all sessions.
   Sessions expire after 12 hours; remote devices then need pairing again.

## Security limits to retain in release notes

The local user account/processes and same-origin localhost pages are trusted as
owner. This is not isolation from malware or another process under that account.
Owner sessions cannot be used on the remote TLS listener. Paired devices may read
personal game state, so revoke lost/untrusted devices. Credentials remain in the
existing per-user Preferences.json: POSIX writes use mode 0600; Windows protection
depends on the user profile's ACL. OS-vault migration and full CAPI identity/client
registration review remain separate work. Dependency, updater, release-asset and
third-party media licensing findings still block a public release.
