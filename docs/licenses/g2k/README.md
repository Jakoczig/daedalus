# Native Field Operations source credits

DAEDALUS field transforms, SQLite integration, native UI and power-scenario
formulas are original Go/React implementation under the repository GPL-3.0
license. They do not execute or install upstream EDMC/Python plugins.

| Reference | Pinned revision | Use / license |
|---|---|---|
| [Neutron Dancer — dwomble](https://github.com/dwomble/EDMC-NeutronDancer/tree/3b0bfc762b565397b71dc03fa9db1518c2c2c4d8) | `3b0bfc762b565397b71dc03fa9db1518c2c2c4d8` | CSV header/refuel/neutron workflow inspiration. MIT notice retained in NeutronDancer-LICENSE.txt. No route-generation API or router copied. |
| [BGS-Tally — tezw21/aussig](https://github.com/aussig/BGS-Tally/tree/3a15d9738fee2cddcc2bb0ddc0823cf10615ae66) | `3a15d9738fee2cddcc2bb0ddc0823cf10615ae66` | Workflow inspiration; original typed journal transformations. MIT notice retained in BGS-Tally-LICENSE.txt. |
| [EDMC-BioScan — Jeremy Rimpo](https://github.com/Silarn/EDMC-BioScan/tree/fc6b85c3de48c8fb79fa9e4f79ad780e48f56629) | `fc6b85c3de48c8fb79fa9e4f79ad780e48f56629` | 116 species' rules from `src/bio_scan/bio_data/rulesets/*.py`, converted literal data to bio-rules.json, with each source file hash retained there. README and load.py grant GPL-2.0-or-later; GPL-3.0 selected for this integration. BioScan-LICENSE.txt retained. |
| [ExploData — Silarn](https://github.com/Silarn/EDMC-ExploData/tree/fd4e188defd6405e46398fb0a54ac8669c9e2189) | `fd4e188defd6405e46398fb0a54ac8669c9e2189` | Genus names and 23 reference distances from `src/ExploData/explo_data/bio_data/genus.py`. GPL-2.0-or-later grant in load.py/const.py, GPL-3.0 selected; ExploData-LICENSE.txt retained. No bundled SQLAlchemy/binary libraries copied. |

Only the licensed rule literals are redistributed; nebula maps, region maps,
colors/variant predictions and separately sourced artwork/datasets are excluded.
Rules needing region, stellar, nebula, body-distance or parent evidence remain
unknown. Pinned criteria may be outdated and never establish confirmed species,
probability, first-discovery bonus or guaranteed sale value. JSON is editable
source data embedded in Go; no Python runtime or downloaded dependency is needed.

Haversine is a standard mathematical formula, implemented independently with
radians, finite-coordinate/radius checks and antimeridian/pole regression tests.
BioScan supplied the sampling-assistance workflow, not proof of game eligibility.

Journal field references: [station services](https://elite-journal.readthedocs.io/en/latest/Station%20Services.html),
[status](https://elite-journal.readthedocs.io/en/latest/Status%20File.html),
[other events](https://elite-journal.readthedocs.io/en/latest/Other%20Events.html).
No EDSY code/assets, new ship curves or restricted model data were copied. Existing
DAEDALUS module-reference licensing/provenance is preserved. Full game-performance
models remain unavailable without permission-cleared constants and gold builds.

EDMCModernOverlay and the EDMC registry are future references only: neither is
bundled, fetched automatically, installed or executed by G2K.
