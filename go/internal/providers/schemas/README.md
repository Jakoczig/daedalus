EDDN journal/1 schema, fetched 2026-11-14 from https://raw.githubusercontent.com/EDCD/EDDN/live/schemas/journal-v1.0.json. BSD-3-Clause, see LICENSE. Embedded locally; validation never fetches a schema over the network. Only the supported travel/exploration subset is constructed. Schema updates require contract tests.

G2J additionally embeds commodity-v3.0.json, shipyard-v2.0.json and outfitting-v2.0.json from the same EDDN live schema repository (retrieved 2026-11-15). BSD-3-Clause LICENSE applies. Station publication validates locally, preserves source timestamps/MarketID and strips unrelated fields. Outfitting and shipyard use a conservative public subset; no CAPI-derived version metadata is invented.

Pinned G2J station-schema source revision: `4ad669bb7bbe1eae080e4c354e786dca4db91f35` (all three bundled files verified byte-for-byte). SHA-256:

- `commodity-v3.0.json`: `315a3a8ba7a6c4d6f3e7cc272db5bee8fcfb3458049a04467774e39fae74209b`
- `shipyard-v2.0.json`: `5234674fd56b7ee65c898d8a2c1bb10b25e4a06d414bbba2cdd07538f86e2f0e`
- `outfitting-v2.0.json`: `6590d8ff496f3773de303d28bcf99cd8f553d560f919b01d5d1be72f726a7e43`
