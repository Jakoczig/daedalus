# ERIDION project summary

ERIDION is the maintainer's renamed native successor to DAEDALUS Terminal, an Elite Dangerous companion. The original Windows/Node application remains intact. This summary covers implementation history; maintainer gameplay acceptance and automated/source review are different evidence categories.

| Batch | Work completed |
| --- | --- |
| Phase 1A / PR2 | Owner/viewer authorization, browser/socket boundaries and security regression foundation |
| Architecture / PR3 | Approved performance-first Windows/Linux Go and Wails migration plan |
| G1A / PR4 | Embedded React assets, one loopback Go service, authentic offline bootstrap, no Node backend runtime; connection-state correction |
| G1B / PR5 | Optional HTTPS secondary displays, private CA/trust, device pairing/revocation, reconnect, recoverable optional-remote configuration; six-character typo-resistant pairing and unsigned preview artifacts |
| G2A / PR6 | Single native journal reader, discovery/replay/live rotation/truncation/partial lines, sidecars, exact identifiers and commander isolation; late discovery and revocation synchronization fixes |
| G2B / PR7 | Actual NavRoute and current-system exploration; conservative marker/file provenance including equal-timestamp content changes |
| G2C / PR8 | Useful local exploration and biology workflow, map/detail reuse and responsiveness |
| G2D / PR9 | Original interface parity and native companion improvements |
| G2E / PR10 | Ship Status instrumentation, modules/engineering inspector, cargo/inventory and broader native feature parity |
| G2F / PR11 | Readability/layout corrections and optional cached read-only EDSM enrichment |
| G2G / PR12 | Native navigation/launcher and companion workflow refinements |
| Combined G2H/I / PR13 | Orbital map, ship statistics, Galaxy UI and unified provider/security architecture; EDAstro GEC contract corrections; provider/live gameplay limitations recorded |
| G2J / PR14 | SQLite history, migrations/backups/recovery, cached startup and incremental catch-up, source reconciliation, publication policies/wizard and verified provider contracts |
| G2K / PR15 | Offline neutron CSV itinerary, native BGS activity accounting, BioScan candidates/sample distances, authoritative Go engineering calculations and built-in modules; session/cycle and live-Status caching corrections |
| G2L / PR16 | Fleet/carrier, persistent trade/mining/account operations, Community Goals/Powerplay/Squadron/Wing/Crew, additive v6 history; complete CG snapshot handling and controlled Windows import investigation |
| G2M-A / current draft | ERIDION identity and safe native-profile adoption, per-account selection/vault/consent/viewer isolation, registered HTTPS Frontier return, Commander/CG/Fleet/Modules/Statistics refinements |

Core invariants: one journal pipeline and SQLite writer; Windows/Linux native desktops; explicitly allowlisted read-only paired data; no viewer credentials or native host actions; offline functionality; new reads/uploads require explicit consent; exact game identifiers, commander/source provenance and bounded state. Undocumented APIs, guessed performance and unported gameplay are not presented as working.

The maintainer reported real gameplay acceptance through G2L, including paired-laptop updates in earlier phases. That does not establish G2M-A Frontier website deployment, live authorization, multiple-account migration or current physical-device acceptance. Current technical status and checklist: [G2M-A handoff](handoffs/g2m-a-eridion-identity-frontier.md). Separate website contract: [Frontier return](protocols/eridion-frontier-website-return.md). Historical handoffs remain available in `docs/handoffs/`.

G2M-B navigation/Galaxy presentation requirements are recorded in the current handoff and intentionally deferred. No merge, release, deployment, replacement of the original application or unauthorized provider upload is part of these draft batches.


G2M-A follow-up on 2026-11-18: maintainer OAuth acceptance is followed by an owner-authorized live read audit of the five approved Frontier endpoints, scalar-anonymized structural fixtures, owned-fleet gap filling, separate last-station/global-CG references and normal carrier-204 handling. No uploads, credential changes, application replacement or live revocation were performed. See the [live-audit matrix](handoffs/g2m-a-frontier-live-audit.md).

## G2M-B — navigation and contextual field operations

Continues from the live-audited G2M-A corrections. Native System/System List/Spansh naming, unified game/imported Route with transactional history, official Spansh manual web workflow, true aligned three-plane galaxy geometry, contextual mission/Powerplay/colonisation activity and existing ownership-qualified carrier history. Review corrections restore clean Commander Overview/Session layouts, keep complete activity behind an explicit owner-settings view and provide confirmed inactive-route deletion to recover saved-route capacity. Module controls/credits move to owner settings. Existing single SQLite/journal pipeline, secure displays and upload permissions remain intact. See [G2M-B handoff](handoffs/g2m-b-navigation-field-context.md) and its preview acceptance checklist; final PR metadata records exact-head CI and verified artifacts.
