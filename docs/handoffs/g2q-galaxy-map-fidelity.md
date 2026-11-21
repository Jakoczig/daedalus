# G2Q Galaxy Map fidelity

Stacked on G2P 9b5b328af61118946230c2e324946e79802fbb72. Final commit, CI and artifact evidence are recorded in the draft PR description to avoid rebuilding solely for evidence links.

Confirmed causes: outer SVG sizing CSS also stretched nested tile SVGs; LOD ignored actual per-projection pixels and DPR; close zoom lacked Painter occupied-bin rendering; the amber route competed with the density layer. Fixes scope sizing to the outer projection, measure actual dimensions with bounded DPR, report requested versus displayed resolution and missing tiles, render genuine L7 bins on demand, and use thin non-scaling route strokes with a visibility switch. Thin side projections retain raster when either axis is below native resolution. Frame transforms and navigation data are preserved.

Canvas bounds: 12,000 symbols per projection, 2,048 pixels per backing dimension, DPR capped at 2, 48 decoded source images, no animation loop. Dense views fall back to genuine raster. Existing two download workers, signed verification, owner-only download controls, visible fallback pins and normal 128 MiB cache policy remain unchanged. No publisher, generator or published image changes.

Read-only reference: [Painter atlas.py](https://github.com/Jakoczig/Eridion-Galaxy-Painter/blob/0acb1a6350cc4ade5d211de3b336194151be3e63/atlas.py), blob 75551b7ef96f3d8d5b09950688206c6def35d3b5, GPL-3.0. Original alpha occupancy, bin centres, RGBA, bounded radius and dense fallback are retained. No individual-star or spectral-color claims.

Local verification: 162 JavaScript regressions and 26 build/asset tests passed; build-script lint passed. Client export produced 65 pages / 290 assets. Focused Go LOD/refinement tests passed; Atlas race tests passed. Synthetic Chromium checks cover layout, DPR, route visibility, exact canvas renderer, bounded symbols and decoded-source reuse. Final CI is the authoritative cross-platform verification.

Real signed public release 8207cdfe6be22603c1f18b06 was installed in a disposable synthetic profile. Matching X/Z frame is x[-47,53], z[-47.5,52.5], 349x349 CSS pixels. Intentional L4 fallback was captured, then verified L7 detail reached zero unavailable tiles. Eight visible L7 sources, 676 primary occupied symbols, active cache 30,624,702 bytes. Navigation overlays are synthetic; density PNGs are real verified published data. No private profile or game acceptance is implied.

[Fallback with route](g2q-galaxy-evidence/real-L4-fallback-route.png), [fallback without route](g2q-galaxy-evidence/real-L4-fallback-no-route.png), [L7 result](g2q-galaxy-evidence/real-L7-occupied-bins-no-route.png), [client density only](g2q-galaxy-evidence/eridion-density-only.png), [unmodified Painter matching frame](g2q-galaxy-evidence/painter-grade7-matching.png).

Painter comparison uses identical verified PNGs and its unmodified renderer. Mean RGB difference 9.367/255; bright-mask IoU 0.860 (threshold 50). These are descriptive, not pixel-equivalence gates: Canvas antialiasing differs from Pillow ellipses. Remaining warm yellow near Sol is present in genuine density pixels, not fabricated star color.

Single 1.5-second idle Chromium sample: 8,074,668-byte JS heap, 1.593% renderer main-thread busy. This is not total native RAM/CPU or a statistical speedup. Synthetic pan reused one source fetch. No external frontend requests or browser errors. No comparative native resource claim is made.

Manual acceptance: test real route at matching zoom on Windows and paired laptop; compare all three projection alignment; pan/resize, hide route, verify destination remains; enable owner detail consent and observe honest fallback/refinement; restart offline; confirm viewers cannot change Atlas settings. Native real-game gameplay and physical laptop acceptance remain owner tests.
