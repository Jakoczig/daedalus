# G2D — original DAEDALUS UI and journal feature parity

Draft stacked PR: https://github.com/Jakoczig/daedalus/pull/9  
Base: `go/g2c-exploration-parity` at `fe6e6e1ef88d5c01ec361f67ebd93c0f42f35bd8`.

## Delivered

- Recognizable original orange/cyan/Jura terminal, primary navigation and icon sidebar, branded header, connection status and owner settings drawer. Reuses the actual legacy Layout, Panel, PrimaryNavigation, SystemMapObject, fonts, textures and navigation definitions; scoped styling leaves the original application intact.
- Selectable, zoomable schematic system map; observed-body list/search/filter/pagination; physical body inspector with explicit units, parents, rings, compositions, discovery/mapping and biological evidence. Full context preserves known parents when filtering. Unknown facts remain unknown.
- Route origin/destination, current/next waypoint, matched progress, independent game-reported jump count, search/copy and bounded waypoint rendering. Existing conservative provenance remains authoritative.
- Biology work cards, confirmed signals/genuses/species and witnessed Log/Sample/Analyse progress; historical analysis is explicitly not unsold inventory. No speculative predictions or rewards.
- Read-only ship hull/fuel gauges and explicit journal capacity/mass/range/docked/landed facts. Exact private ShipID distinguishes same-type ships internally; no custom name/ident, modules, credits or private positions reach browsers. Source gaps, commander changes and ambiguous identities cannot carry old fuel into a different ship. These are last-known journal facts, not live flight instruments or fuel-safety predictions.
- Normal executable defaults to G2D. Existing eight smoke modes preserve earlier stages; two additional G2D modes require explicit synthetic fixtures. One Go journal reader and existing authenticated HTTP/WebSocket service serve all clients; no runtime Node/Python/backend duplication.

Implementation commits: `56b781722abc5a7a39a0501cb675f8ebaf1c6389` (batch), `7003e7e7c9fb77922d1f900c567e2c27ac3853bd` (final review correction).

## Review and security

One independent final review found no Critical issues and one Important issue: sibling moons shared map coordinates. A deterministic test first failed (five unique placements for seven bodies), then passed after allocating separate rows for complete subtrees. Nested branches, exact IDs, bounds and filtered parent provenance are covered. Downloaded Linux portrait screenshots subsequently exposed a legacy inspector flex rule leaking into G2D: content became narrow horizontal columns. A real-browser regression failed with flex layout, 1,460 px internal scroll width and a 78 px heading; scoped stacked-flow CSS fixes it for the body inspector and settings drawer. The browser gate now checks readable heading width, vertical facts and internal overflow at all three viewports. The corrected portrait capture was visually inspected. No disputed findings remain. Physical gameplay, Linux graphics and exhaustive re-audit of unchanged security code were explicitly outside that review's certification.

Owner/viewer/TLS/pairing/profile-locking/recovery behavior remains under existing regression tests. `getGoShip` is an explicit G2D-only typed allowlist. Every ship notification uses the original per-write authorization guard. New revocation tests observe unique post-revoke navigation, biology and hull data on the owner's live socket and reject it on the revoked viewer, including deliberately delayed socket closure. Viewers cannot acknowledge owner native readiness or invoke host controls. Original Node service and Windows launcher are unchanged; dependencies remain pinned.

## Verification

Source head: `7003e7e7c9fb77922d1f900c567e2c27ac3853bd`.

- [Windows/Linux Go desktop CI](https://github.com/Jakoczig/daedalus/actions/runs/43566636784): **passed** on both OSes, including complete Go tests/vet, all ten native/headless modes, G2B/G2C/G2D repeated stress and Linux full/rerun race detection. Browser interaction and visual checks pass on both OSes.
- [Windows/Linux legacy/security CI](https://github.com/Jakoczig/daedalus/actions/runs/43566636899): passed, including both regression jobs, React build, original Windows launcher build/smoke and advisory broad lint. Mandatory scoped security lint remains blocking.
- Local Windows: JavaScript **79/79**, build/packaging/workflow tests **11/11**, security lint and asset lint pass; complete Go suite and vet pass; G2D ship/revocation stress **25 repetitions** passes. React exports **278 assets**. All **10** executable/native smoke modes pass, including real Wails/WebView2 owner React authentication, navigation/survey/ship delivery and shutdown on the clean source head.
- Real sandboxed Edge against the actual loopback Go fixture passes desktop **1920×1080**, laptop **1366×768** and tablet **820×1180**: one header, no document horizontal overflow, selected inspector, live search, next-hop display, biology progress and ship values; zero runtime errors. The reference screenshots render actual original DAEDALUS components using fabricated data, without launching its Node service. This is visual/browser evidence, not real gameplay acceptance.

Downloaded and independently verified source-head artifacts (14-day CI retention; GitHub sign-in may be required):

| Platform | Preview | Executable SHA-256 |
| --- | --- | --- |
| Windows x64 | [Download](https://github.com/Jakoczig/daedalus/actions/runs/43566636784/artifacts/14102931525) | `0fb70c33ebc229cbf2abbeec230d4ce9ef8b408f6a0b3656699ef9c5195e441d` |
| Linux x64 | [Download](https://github.com/Jakoczig/daedalus/actions/runs/43566636784/artifacts/14102628058) | `c115e0f4e93dacb8f5defdd5ad955a31dce39c21d36a5fa695bb764f10e82b82` |

README SHA-256: Windows `49d7ade0fccbbff5185eca05bb1b98d303098369ace0010f5f41acb22725a350`; Linux `4436f6419088fad04b32f7086ddd5f9d40468723285bd984e0cd052265b9583c` (checkout line endings differ). ZIP SHA-256: Windows `3502c831da57b786a3b8a805638c74e9ab0e399df86753e60204e5c4c5daf930`; Linux `857188bad3b43955e625ac2620bafbacb031cf72cff245ac726327fefb1eee32`. Each downloaded ZIP matches GitHub's artifact digest and both internal SHA256SUMS entries. [Windows visuals](https://github.com/Jakoczig/daedalus/actions/runs/43566636784/artifacts/14102971415) and [Linux visuals](https://github.com/Jakoczig/daedalus/actions/runs/43566636784/artifacts/14103121544) were also downloaded/digest-verified; their verification JSON reports success and no runtime errors. The final responsive-inspector correction and its exact-head CI/artifacts are recorded in the PR body after completion. The links in this document identify the earlier verified source build; use the final PR-body downloads for practical testing. Preview archives contain exactly executable, README and SHA256SUMS; synthetic screenshots/measurement JSON are separate artifacts, never generated user profiles, credentials or private CA keys.

## Measured responsiveness and cost

Local clean-source Edge, same synthetic replay, `/exploration/system`, 1366×768, explicit garbage collection before each measurement, 1.5-second idle sample:

| Metric | G2C plain workspace | G2D restored terminal |
| --- | ---: | ---: |
| Retained renderer JavaScript heap | 2,858,364 B | 4,919,140 B |
| Renderer main-thread busy fraction | 4.76% | 1.88% |

Map selection to second animation frame: desktop **12.8 ms**, laptop **11.4 ms**, tablet **15.1 ms**. This small synthetic sample is not a statistical gameplay benchmark or total native CPU/RSS measurement. G2D uses more retained JavaScript memory; no memory-reduction claim is made. No equivalent G2C map-selection timing is claimed. Machine/browser scheduling affects these numbers; CI measurement JSON supplies separate environments.

CI renderer samples corroborate the increased heap, not a universal CPU improvement: Windows G2C/G2D **2.954/4.946 MB**, busy **1.89/0.64%**; Linux **2.882/5.017 MB**, busy **1.65/1.99%**. CI selection was **20.7–62.9 ms** on Windows and **30.6–31.6 ms** on Linux. These short samples are sensitive to scheduling.

CI Windows replay: **549,277 journal events/s**, **203,574 projected events/s**; Linux: **467,223** and **166,398 events/s** respectively (three-iteration synthetic benchmarks, runner-dependent, no baseline improvement claim). Snapshot-copy benchmark: Windows **33,454 ns/op**, Linux **45,067 ns/op**, approximately **113 KB/op**, **508 allocations/op**. This includes detached data copies and is not a claim of zero-allocation end-to-end delivery.

Rendering is bounded to 24 visible bodies and 60 waypoints per page, with memoized filtering/layout and an isolated clock update. It reuses existing coalesced projections, detached snapshots and session bounds. No persistent survey database or additional reader/cache process was introduced.

## Remaining limits and one real-game acceptance checklist

The maintainer's prior G2C Windows-to-laptop gameplay acceptance remains recorded on PR #8; G2D's changed presentation and new ship projection still need practical acceptance. Physical Linux/Proton, mobile certificate stores, GPU/Wayland, long sleep/idle and LAN interruptions are not certified by synthetic CI. Unported workspaces say so. Shields/modules/engineering, surface-position radar, external providers/uploads and game commands are not implemented.

1. On native Windows and a paired laptop, plot/advance/clear a route and jump systems; compare current/next and stale/unverified labels with the game. Inspect a planet with multiple moons, filter/page, zoom and select each by mouse/keyboard. Compare units, rings and known/unknown facts with FSS/DSS.
2. Log → Sample → Analyse a biological species; switch species/body and restart. Check progress, interruptions and historical-completion wording, then change commander/system and verify no foreign bodies, routes or ship facts remain.
3. Compare hull/fuel after LoadGame, Loadout, jump, scoop and damage; swap between two ships of the same type. Missing identities/history must show unknown rather than inherited fuel. No inferred shields, switches or fuel-safety promises.
4. Pair/trust HTTPS normally, test live laptop updates, disconnect/reconnect, sleep/idle and revoke. Viewers stay read-only and lose active feeds. Repeat with a physical Linux/Steam Proton host and a mobile browser; preserve CA trust/profile and never bypass TLS or upload personal journals as evidence.

Experimental, unsigned, local journal preview with no provider uploads. Nothing merged, released, deployed or substituted for the legacy application. Stop at G2D.
