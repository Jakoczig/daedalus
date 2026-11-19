# G2N: real-coordinate galaxy atlas

## Outcome and integrity gate

The Galaxy view now supports an offline atlas generated from measured Elite Dangerous system coordinates, not illustrative spiral graphics. This branch deliberately includes **no purported real atlas tiles**: a genuine downloadable input and its redistribution rights must be verified before distributing the derived atlas. Until then, the interface displays an explicit unavailable label, with actual journal/system coordinate markers remaining visible. No new live lookup or upload is introduced.

## Primary data source

EDSM publishes nightly [system coordinate dumps](https://www.edsm.net/en/nightly-dumps) and documents its [system API](https://www.edsm.net/en/api-v1). Its systems-with-coordinates JSON records contain a coords object with numerical x/y/z coordinates in light years. The published dump is large and changes over time; no request is made by the running desktop application. This choice avoids undocumented live provider requests. Spansh bulk data is another possible source but is not necessary for this initial atlas pipeline.

**Critical semantics:** the EDSM dump is a nonuniform sample of *reported systems*, not the procedural distribution of all Elite Dangerous star systems. Density therefore represents the distribution of known community observations. The rendered map must not claim that unreported regions are devoid of stars. No generated arms, random star positions, or artistic fills are permitted.

**License/rights:** public access is not proof of permission to redistribute derived raster tiles. Review EDSM's current rights/attribution terms and obtain any necessary permissions before committing tiles or shipping them inside ERIDION. The manifest records the input hash, source URL/date provided by operator and a redistribution warning. No license assertion is invented.

## Coordinate reference and projection

Coordinates are X/Y/Z in LY with Y vertical and Sol at (0,0,0). Fixed visualization bounds are X [-65000,+65000], Y [-15000,+15000], Z [-35000,+95000] LY. Out-of-bounds records are counted, not silently painted elsewhere. These bounds are UI framing choices, not declared galaxy boundaries.

| Panel | Axes | Screen right | Screen up | Natural aspect |
| --- | --- | --- | --- | --- |
| Center | X/Z | +X | +Z | 1:1 |
| Below | X/Y | +X | +Y | 130:30 |
| Right | Y/Z | +Y | +Z | 30:130 |

The right-hand edge-on projection is rotated for a tall strip. Both side panels use the same source record in the same binning pass and the same overlay coordinate transform.

## Rendering

One streaming scan of the supplied compressed EDSM JSON or strict CSV creates three count grids. Each finer level is aggregated into the lower resolutions by exact 2x2 count sums. Counts are mapped to log-density RGB and transparency, encoded as lossless 256px RGBA PNG tiles. The manifest is published atomically after all output tiles, with a content-checksum-labelled run directory. A failure leaves the last manifest intact.

At runtime the view computes intersecting tiles and selects the smallest adequate level, with an upper bound of 48 images per projection. The view needs neither a giant full-resolution bitmap nor provider traffic. Zoom beyond the maximum source resolution cannot synthesize absent individual stars. Default level 4 is a compromise: X/Z 4096x4096, X/Y 4096x945, Y/Z 4096x17749; approximately 67 + 16 + 291 MiB for finest uncompressed uint32 count arrays, plus overhead. Level 5 requires a larger preprocessing machine. Output disk size depends on actual sampling and PNG compressibility; inspect the produced directory before release.

## Offline refresh procedure

1. Obtain a current official systems-with-coordinates EDSM JSON dump. Document the actual URL, time of release, terms and checksum. Do not put the multi-gigabyte source archive in the repository.
2. Execute from repository root (once rights/terms are checked):

       node scripts/build/galaxy-atlas.cjs --input /absolute/path/systemsWithCoordinates.json.gz --format edsm-json-gz --out src/client/public/images/galaxy-atlas --source-url ACTUAL_DOWNLOAD_URL --source-date YYYY-MM-DD --max-level 4

   For an authorized pre-extracted CSV with header x,y,z, select --format csv. The generator never fetches the URL.
3. Inspect manifest.json fields status, inputSha256, generatedAt, stats, views, maxLevel and run; inspect the three real density maps and compare several independently known coordinate positions.
4. Only when redistribution is authorized, include generated tiles intentionally in a build, then run npm run test:go-assets, npm run build:go-assets, Go tests and native visual smoke tests. Legacy preservation and paired display restrictions remain unchanged.
5. Remove superseded tiles only after verifying the replacement manifest and native build. No always-on updater is provided.

## Acceptance

- Source format and license verified on the actual dataset, not only fixtures.
- Sol and multiple off-plane known systems align across all views; no cross-axis inversion.
- Top view central square, bottom 4.33:1 width strip, right 4.33:1 height strip at 1366x768 and on larger displays.
- Fit and repeated zoom/pan progressively fetch the correct local tiles, without external provider requests or unbounded DOM/image growth.
- Sparse/missing data never appear as invented arm geometry or artificial stars.
- Missing tiles show an explicit warning; current/search/destination/route markers remain usable.
- Windows/Linux desktop, private owner and paired read-only screen acceptance; legacy and native test suites pass.

## Limitations

This PR establishes the real-data processing, tiled renderer, fixture-backed tests, and source/refresh handoff, **not a prepackaged real dataset**. It must not be approved as visually complete without an actual licensed data run. Generated physical stellar density cannot be inferred from reported-system counts. No in-game target selection is introduced. Full Windows/Linux CI and manual visual acceptance are pending until checks are observed.
