# G3C hybrid spatial Galaxy Map

Stacked on G2Q 905ef388579c9bacbffd149dd68fc9791861f61d. The draft PR description is the final exact-head ledger for implementation commits, Windows/Linux CI links, artifacts and independently verified SHA-256 values. No prior PR is modified.

## Capability matrix

| Capability | Status |
| --- | --- |
| Existing signed-v2 L0-L7 density Overview, three projections, route/search/current/destination overlays | Preserved |
| Local Volume, exact reported XYZ, synchronized XZ/XY/YZ, equal LY scale and clipped 3D route | Implemented |
| Follow current system, explicit destination/waypoint/XYZ inspection, drag, width/Y thickness, Fit Local Region, return to Overview | Implemented |
| Pinned-key signed G3B pointer/descriptor, immutable source manifest and hashed indexes/ESP1 blocks | Implemented and live HTTPS verified |
| Permanent 350 LY Bubble, lazy random-access ZIP, content reuse, offline restart, explicit owner removal | Implemented; restart proven with frozen synthetic fixture, public offline view verified |
| Cancellable two-worker current/adjacent/next-five-hop/target prefetch, offline/metered/consent controls | Implemented; deterministic transport regressions |
| Read-only paired cached viewing and cache-change notifications | Implemented; server permission tests; physical laptop acceptance pending |
| Names/spectral classes from spatial records | Not supplied; no inferred names/classes |
| Real gameplay, physical device/network-interruption acceptance, total Wails resource profiling | Manual acceptance pending |

## Architecture and trust

New atlas/spatial_*.go files consume frozen G3A v1 and G3B distribution; server spatial.go exposes authenticated cached views plus owner-only management. go-spatial.js uses event-driven Canvas 2D, the existing projection/route components and one shared Go service. No new runtime dependencies or game-data reader. Existing DB, credentials, Frontier authentication and provider permissions are untouched.

Same pinned raw Ed25519 key as density, key ID d679ccea38ac5d02. Both canonical payload signatures, pointer freshness/expiry and anti-replay watermark, exact descriptor/manifest byte hashes, schemas, child lengths/hashes, integer vectors, nonoverlap and tight ESP1 bounds are verified. JSON duplicate/unknown fields, unsupported binary/ZIP structures, unsafe paths/origins and all redirects are rejected. Zlib inflation is bounded with exact framing and no trailing stream. Bubble download streams to private staging, verifies the full ZIP digest, validates its classic stored directory and self-contained references, then activates atomically. Individual blocks are decoded only as needed. A changed Bubble digest/size requires renewed owner confirmation.

Startup uses prior authenticated files offline, even after pointer expiration. Failed or interrupted updates preserve the installed state. Explicit rollback preserves the anti-replay watermark. SHA-addressed indexes/blocks are shared across releases and Bubble reads. Missing/corrupt/over-budget data is a labeled density fallback, never a successful empty volume. New queries never show old coordinates while awaiting their matching result.

Downloads are off by default. The UI discloses approximate-region privacy and sends no commander identity, tokens, journals or credentials. Automatic index updates are optional, daily at startup with ETag/throttling; no new external polling loop. Existing local WebSocket notification ticks carry a cheap cache revision for subscribed displays; viewers only reread cached data and never authorize or reprioritize downloads.

## Bounds

Per query: 256 indexes, 4 MiB encoded index bytes, 128 blocks, 16 MiB cumulative decoded data, 80,000 retained positions. Results are all-or-fallback, without arbitrary point sampling. All projections use the same 3D-filtered list. Canvas aggregates coincident screen pixels, with DPR <=2 and backing dimensions <=2048; no animation loop.

Regional cache target: 64 MiB and 4096 files, with current owner assets protected from eviction; the frozen query bounds keep that pinned set below the byte target. The byte budget rounds up two conservative query encoded working sets (4 MiB indexes plus 128 x 200,000-byte blocks each); the secondary file cap permits at most sixteen saturated 256-request plans and bounds scans when sparse assets are tiny. An actual plan is further limited to 256 requests and 32 MiB encoded bytes across current volume, 20% adjacent margin, next five known hops and target. Two spatial downloads globally, at most 20 starts/second, bounded failure tracking/backoff and cancellation. Only the owner can initiate plans. Bubble ZIP is exempt from regional eviction; at most active plus one prior ZIP are retained, each under the 1 GiB wire cap with exact size consent. Current/previous/Bubble-authenticating manifests are retained; other known metadata is pruned. Density keeps its independent normal 128 MiB policy and visible fallback protection.

## References and attribution

Read-only GPL-3.0 G3A reference: [spatial.py and frozen contract](https://github.com/Jakoczig/Eridion-Galaxy-Painter/tree/5d1cbf1aee7a707259000e57ea963175f7cd4f3b). Imported invented-coordinate fixtures are unchanged from tests/fixtures/spatial-v1, including pinned Bubble SHA 0ec1cfb361adeb4b862cd1327be9634546308c0e157f32f5fc61aa1b021443ba. Go decoder/reader and JS projection/clipping semantics are attributed in source and Credits.

G3B distribution, tests and client handoff were read at e3c6e40 (PR #3). G3A spatial.py/local_view.py/fixtures remain byte-identical to the frozen commit. The handoff's old private-staging statement was superseded by the user's operator acceptance and independent live verification, not assumed from an URL. No publisher, generator, signing key, site, Proxmox or publication was modified.

## Verification and real data

Focused tests first failed for missing decoder/manager/UI implementations, then passed. Additional regressions cover malformed signatures/JSON/path/ZIP/zlib, exact negative ticks, all safety budgets, no truncated results, offline restart, cross-release reuse and rollback, interrupted/corrupt updates, current-first queueing, route cancellation, offline consent, two-worker bounds, Bubble preservation/removal, recovered error state, 3D clipping and side-panel aspect. Spatial race tests and Go vet passed locally. JS 165/165 and asset/build tests 27/27 passed; security/script lint and React build/export passed. The opt-in TestSpatialLiveSignedRelease passed; it is deliberately skipped in offline CI. Final complete native/security/race/browser checks are linked in the PR ledger.

Public release f5eca8346b2e637325449c1a verified via pinned-key HTTPS pointer and descriptor. Distribution SHA e18aa936370dcbb443b491ba7e60c0881321fb205d32818e2119fa521a7dec6a; spatial manifest SHA 288802a6f1396d5197d210572ec873388669fcc597f162e2d0a1a24575eec6f1. Bubble SHA c486fb3124df68417c3a9fd1d1b79a2d459d24d4793e9632024676e59049053a, 3,144,891 bytes, 248,264 positions. Only disposable fabricated-navigation profiles were used.

[Sol actual positions](g3c-spatial-evidence/real-Sol-local-volume.png), [same selected bounds in density Overview](g3c-spatial-evidence/real-Sol-density-same-volume.png), [Beagle Point actual positions](g3c-spatial-evidence/real-Beagle-Point-local-volume.png), [measurement record](g3c-spatial-evidence/live-verification.json).

Both real volumes are 200 x 100 x 200 LY. Sol: [-100,-50,-100] to [100,50,100], 9,238 positions / 8 blocks / 1,159,264 decoded bytes. Beagle Point center [-1111.5625,-134.21875,65269.75]: 43 positions / 1 block / 33,392 decoded bytes. The name and reference center come from the upstream benchmark, not from the anonymous spatial data. Density comparison uses identical bounds but remains full line of sight; local positions are depth-filtered. Overlays are synthetic. No physical gameplay acceptance is inferred from these images.

## Performance evidence

Public HTTPS direct Go test installed manifest and Bubble in 832 ms; first Sol decode/query took 14.93 ms (single samples). Browser/WebSocket round trips including 9,238-position JSON transfer measured 47.78 ms at Sol and 6.19 ms for 43 positions at Beagle Point. Regional cache after this route was 20,349 bytes; permanent Bubble 3,144,891 bytes. Total spatial received bytes 7,754,923 include manifest, Bubble, signatures and sparse assets. Re-reading offline Sol returned all 9,238 positions without increasing downloaded bytes.

Windows Chromium 1.5-second scene samples, single run and sequential order: density baseline renderer heap 6,395,324 bytes / main-thread busy 1.482% / synthetic Go host RSS 57,135,104 bytes; local Sol 8,059,596 / 1.506% / 50,946,048; local sparse 7,994,924 / 1.624% / 54,349,824. Synthetic backend one-core CPU samples were 5.09%, 5.97% and 7.66%, respectively. These short samples can include route-prefetch/notification settling and are not established steady-state idle or throughput measurements. This measures the fixture's real Go service and Chromium, not total Wails RAM/CPU. Cache history and GC affect RSS; no statistical speedup or causal resource reduction is claimed.

## Acceptance checklist and limitations

1. Owner installs the signed spatial index and confirms Bubble size. Check local status, then restart disconnected from network and inspect a volume fully inside Bubble coverage.
2. Follow real current position across jumps; replace/cancel a route, inspect destination and selected waypoint, then resume following. Verify only nearby/upcoming data is requested.
3. Compare Sol at 200 x 100 x 200 and 40 x 20 x 40 LY with Overview; inspect a sparse region, pan/resize all panels, adjust thickness and hide/show route. Current/destination markers must stay legible and correctly placed.
4. On a paired laptop repeat cached navigation, interruption/reconnect and owner-side cache completion; confirm viewers cannot enable downloads, install/remove/rollback or change settings. Revoke a test device only if explicitly authorized during physical acceptance.
5. Exercise offline/metered pause, a cache miss and an over-dense volume: expect an honest labeled fallback, never empty-space claims. Test rollback/removal using a disposable profile first.

Boundary-straddling Bubble volumes can need regional cache or a smaller selection. Provider catalogue incompleteness, absent names/classes and 80k-volume limits are intentional. Real network interruptions, actual gameplay and physical laptop checks remain manual. No live Frontier sign-in, production uploads, legacy replacement, merge, release or deployment was performed.

Transition regression: explicit Overview XYZ bounds now carry into Local Volume without silently clamping or squaring X/Z. Independent X width, Z depth and Y thickness retain rectangular scale; oversized selections produce the existing bounded fallback. Equal light-year geometry and round markers are tested for asymmetric volumes and the browser tests exercise a 400 x 50 x 100 LY round trip.

CI follow-up: a Windows retry exposed a queued ResizeObserver callback after Galaxy Map unmount. A deterministic test reproduced the null clientWidth access, then passed after guarding the cleared view reference. Existing G2G runtime-error assertions remain mandatory. An earlier Windows G2B owner-control setup timed out; its unchanged revocation stress test passed 25 local Windows repetitions and on the subsequent CI attempt. No timeout or security assertion was relaxed.
