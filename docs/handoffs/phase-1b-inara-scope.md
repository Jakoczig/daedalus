# Phase 1B: Inara repair scope (investigation only)

Checked 2026-11-12 against the implementation on main at `06a6963` and the
Phase 1A branch. No Inara payload, status-success rule or delivery semantics
were repaired in Phase 1A. Only security-related persistence/diagnostics changed.

## Independently confirmed defects and contract

| Current source: src/service/lib/event-handlers/integrations.js | Phase 1B acceptance |
| --- | --- |
| `_queueForInara` uses `addCommanderFlightLog` for FSDJump and Location. | Use `addCommanderTravelFSDJump` and `setCommanderTravelLocation` respectively; do not fabricate a flight-log jump at login. |
| Both mappings turn StarPos into `{x,y,z}`. | Send numeric `[X,Y,Z]`; omit invalid/unknown positions. Remove undocumented fields. |
| Both request headers use `isDeveloped`. | Use `isBeingDeveloped`; derive commander identity from the journal. |
| `testInara` checks header status 100; flush checks only header 200. | Classify HTTP, header and each event result separately: 200 success, 202 warning, 204 soft error, 400 error. Never acknowledge all items from the header alone. |
| Mapper covers only FSDJump, Location and LoadGame credits; no materials payload exists. | Emit `setCommanderInventoryMaterials` only from complete Raw/Manufactured/Encoded state; this method replaces the entire remote inventory. |
| `_flushInara` splices the entire in-memory queue before checking the key or sending. | Persist before send; acknowledge individual accepted items. Missing credentials, timeout, rejection or restart must not silently lose work. |

These protocol requirements come from the [official API documentation](https://inara.cz/elite/inara-api-docs/).
Its example for setCommanderTravelLocation has a duplicated coordinate property;
follow the property table and journal facts rather than copying that example.

The [official developer guide](https://inara.cz/elite/inara-api-devguide/) requires
application whitelisting, Live data, batching, restrained request frequency and
no repeated identical events. Confirm the community fork's approved application
identity before public activation; do not assume the upstream name is registered
for this fork. No provider contact or authenticated upload was performed here.

## Behavioral reference and limits

[EDMC Inara plugin at 056cab9](https://github.com/EDCD/EDMarketConnector/blob/056cab9b3bfcbb3e02e89b8e232b3df7dd17f8d2/plugins/inara.py)
was inspected for travel mapping, combined material state, commander/FID
partitioning, fake-system/multicrew/Legacy handling, change suppression and
per-event response processing. Its timing constants are implementation choices,
not documented Inara rate limits. Its in-memory delivery/retry code is **not**
proof of crash durability or exactly-once delivery. No Python code was copied.

## Proposed independently testable sequence

1. Pure header/travel/material translators with deterministic journal fixtures.
   Missing position/identity, partial materials, beta/Legacy and fake locations
   must produce no fabricated submission. Include credits only from reliable
   journal facts; inspect the existing forced zero loan rather than retaining it.
2. Response classifier and injected HTTP transport: mixed statuses, absent/malformed
   replies, timeout, 429/5xx and authentication failure. Persist only sanitized
   diagnostics. Connection test must not fall back to an invented commander.
3. Existing NeDB-based bounded outbox partitioned by provider, commander/FID and
   credential generation. Test disable/re-enable, account/key switch, missing key,
   duplicate source events, restart before/after send and ambiguous acknowledgement.
   Bound replay risk; do not promise exactly-once without provider support.
4. Replay complete startup/session/material updates through the coordinator with
   network disabled. Then, only with provider registration and user-owned test
   identity, run an explicit opt-in manual validation outside CI.

Keep EDSM, EDDN, full CAPI registration/FID/OS-vault work, biological data,
ship catalogs and neutron plotting in their own phases. Add no new provider
production traffic while the documented defects remain.
