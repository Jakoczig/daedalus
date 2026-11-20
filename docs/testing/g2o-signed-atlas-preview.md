# G2O — experimental, unsigned atlas preview

This is a local-first preview, not an installer or release. The original application
is unchanged. Windows x64 needs the Microsoft Edge WebView2 runtime. Linux x64
needs GTK4, WebKitGTK 6.0 and a graphical session (Ubuntu 24.04 packages
`libgtk-4-1 libwebkitgtk-6.0-4`). Check `SHA256SUMS` before running the executable.
Keep the executable and README in a separate folder; do not replace a legacy install.

The new atlas is **not bundled**. Startup is offline. Existing journals, Frontier
authorization, commander selection and paired-display permissions are preserved.
No new uploads are enabled.

## Install and use

1. Launch ERIDION, select your commander if requested, then open Settings → Galaxy Atlas.
2. Click **Check for atlas updates**, then **Install verified overview**. This downloads
   about 13.87 MiB over HTTPS and verifies the pinned publisher signature and all assets.
3. Open Navigation → Galaxy Map. Fit Galaxy shows the real EDSM-derived overview;
   Fit Route, zoom and drag retain independent game-position/route overlays.
4. Optionally allow **missing visible high-resolution tiles**. This permission is
   independent of automatic version checks. Only occupied visible tiles are fetched,
   with two concurrent downloads and a 128 MiB persistent detail target.
5. Leave automatic checks off for fully manual network use. If enabled, only a signed
   pointer check runs on startup, at most once per 24 hours, including failure cooldown.
   Updates still require the Install button. The metered pause blocks automatic checks
   and detail downloads; explicit manual check/install remain available.

Automatic and manual cleanup preserve the current owner viewport's verified
sources and temporary fallbacks. If that working set itself exceeds 128 MiB,
Settings reports the actual retained bytes and effective budget; nonvisible files
are evicted. Only the current working set gets this exception (at most 576 files,
2.25 GiB worst-case disk use at the protocol's 4 MiB/file bound). Refinement and
panning release old pins and immediately restore the normal 128 MiB target.

The atlas host receives ordinary asset requests and tile addresses (which reveal the
viewport region), never account tokens, commander/system names, raw journals or uploads.
Detailed pixels are shown only when verified; unavailable detail is labeled as lower
resolution. The publisher provides source-download time and generation-method provenance,
not a generation timestamp; ERIDION does not invent one.

Paired displays can read cached atlas tiles and adjust their own viewport. They cannot
install, change settings, initiate remote downloads or access private history/credentials.
Open/pan their map again after the owner has cached additional detail. Density represents
community-reported systems, **not the complete procedurally generated galaxy**.
[EDSM source](https://www.edsm.net/).

## Recovery and acceptance

Failed downloads leave the installed version and detailed cache intact. A successful
new release retains the previous version; **Restore previous verified atlas** switches
back locally. **Clear unused detailed tiles** preserves the overview and current owner
viewport. Corrupt settings are preserved and disable automatic access; do not delete
your game or application profile to repair the atlas.

One gameplay / laptop checklist:

- Confirm commander selection and existing Frontier connection remain unchanged.
- Install the overview; compare X/Z, X/Y and Y/Z orientation and player/destination positions.
- Pan/zoom and Fit Route; allow detail, then revisit the same area offline after restart.
- After extended high-zoom browsing, confirm visible detail/fallback does not disappear; pan away and confirm the reported budget returns to normal.
- Disconnect the network during an update; confirm the old map remains usable.
- On a paired laptop, confirm cached map/overlays/reconnect work and owner controls remain unavailable.
- Check FSD compact mode and return-to-page behavior during real travel. Keep uploads disabled.

CI uses synthetic journals and no external atlas traffic. Explicit live acceptance uses
only the public pinned atlas and disposable synthetic profiles. This does not establish
physical gameplay, physical Linux/Proton or paired-laptop acceptance.
