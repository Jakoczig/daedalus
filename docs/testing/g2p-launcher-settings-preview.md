# ERIDION G2P experimental unsigned preview

Windows x64 requires WebView2; Linux x64 requires GTK4, WebKitGTK 6.0 and a graphical
session. This is a local-first experimental executable, not an installer or release.
Verify the executable and README against SHA256SUMS before running. Preserve your
existing profile and legacy installation. Uploads remain separately consented and OFF
by default; opening Settings never performs a provider health request.

Settings contains General, Journal & Commander, External Services, Galaxy Atlas and
Credits. History, backups, recovery, built-in modules and provider diagnostics remain
in the dashboard's owner Settings → Advanced local tools. Pairing remains a separate
launcher action; use the existing HTTPS trust and pairing instructions. Viewers cannot
change profiles, portraits, credentials, provider consent or atlas installation.

## One owner / laptop acceptance checklist

- Check that all four launcher actions fit and Settings scrolls at normal and 125%
  Windows scaling; keyboard focus must remain visible in long sections.
- Check the actual journal directory and current verified profile with multiple
  accounts OFF; enable multiple accounts and explicitly select the intended profile.
- Change a portrait in the native launcher, restart and switch accounts; confirm it
  stays with that commander. The dashboard displays portraits but has no editor.
- Expand providers; decline Read consent and confirm it remains off. Accept only
  intended Read access. Leave Write and every publication category OFF for acceptance.
- Frontier: select a verified profile, including cached history with the game closed;
  save Frontier Read consent. If already connected, do not delete credentials or
  reconnect unnecessarily. Otherwise click Connect Frontier. Complete sign-in yourself
  in the system browser. The registered callback is
  `https://eridion.fitoserver.org/auth/frontier/callback`; it returns to the existing
  loopback receiver. If blocked, paste its local return into Manual browser return.
  Verify stored credential status without exposing the return URL, codes or tokens.
  Switch profiles during an unfinished flow and confirm the old return is rejected.
  A successful sign-in does not itself prove that every CAPI endpoint is available.
- Enable Atlas: accept its overview download, verify offline reuse after restart and
  disable without deletion. Check for updates without automatic installation. Clear
  map data only if intentionally testing reinstallation; history/credentials must stay.
- On the paired laptop confirm dashboard navigation/reconnect and cached maps work,
  while all owner settings and portrait mutation remain inaccessible.

Native inspection and CI use disposable synthetic profiles. Live Frontier sign-in,
physical gameplay and physical laptop acceptance are not inferred from those tests.
Full project and upstream license notices remain in LICENSE, docs/licenses/g2k,
provider schemas and bundled frontend notices in the source distribution. Credits
links identify the actual references; they do not imply endorsement.
