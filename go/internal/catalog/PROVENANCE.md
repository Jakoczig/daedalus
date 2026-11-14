# Bundled reference data

These are static reference facts selected from data already committed in this
repository. They describe stock modules and item names, not measured player ship
statistics. The runtime never opens resource directories or personal game files.

- `outfitting.csv`, `material.csv`, `microresources.csv`, `commodity.csv` are exact
  copies of the corresponding `resources/data/edcd/fdevids/` source files.
- `modules.json` selects `symbol`, `ukName` (as `name`), `class`, `rating`, `mass`,
  `power`, and `ukDiscript` (as `description`) from every JSON file under
  `resources/data/edcd/coriolis/modules/`. Missing values remain null. Generation
  used the repository's existing bundled data; no network source was fetched.
- The public provenance label is `bundled_edcd_coriolis_reference`. The reference
  catalog's `massT` and `powerMW` describe stock modules. Observed engineering
  modifiers remain separate, and are not silently applied to these values.

The bundled snapshots may omit newer modules or labels. Unknown item symbols are
preserved; absent reference fields remain unknown.
