# Mining Section — Implementation Plan

## Research Summary

### Reference: ObservatoryProspectorBasic
The Prospector plugin was reviewed in full (`ProspectorBasic.cs`, `ProspectorSettings.cs`,
`TrackedData.cs`, `RingDetails.cs`, `MaterialData.cs`). It is the canonical reference for
the mining workflow. Key takeaways:

- Tracks **Scan** events to find rings of the right type containing target commodities
- Tracks **SAASignalsFound** events for DSS hotspot counts per commodity per ring body
- Tracks **ProspectedAsteroid** events — materials (by %), motherlode, content level (High/Medium/Low), remaining %
- Maintains a **cargo + limpet inventory** via `MiningRefined`, `LaunchDrone`, `BuyDrones`,
  `SellDrones`, `EjectCargo`, `Synthesis`, `Cargo`/`CargoFile`, `Loadout`
- Tracks **session stats**: good rocks / total prospected, limpets used / synthed / abandoned
- Emits an alert on `Undocked` if the ship has mining modules but no limpets

### Current State in Daedalus
- Four stub pages exist: `/mining/status`, `/mining/log`, `/mining/statistics` (all show
  `ComingSoonPanel`) and `/mining/index.js` (redirects to status)
- `MiningPanelNavItems` in `navigation-items.js` already has three nav tabs
- **No service event handler** for mining exists yet — the most significant gap
- Icons available in the daedalus-terminal font: `asteroid-base`, `cargo`, `barrel`,
  `fuel`, `planet-ringed`, `signal-even-bars`, `inventory`, `trending-up-chart`,
  `warning`, `cargo-buy`

---

## Scope

### Phase 1 — On-Grid Mining Dashboard (highest priority)
Replace `/mining/status` stub with a full scaled dashboard (same approach as
`ShipInstrumentation` — `applyScaling()` CSS transform to fit any screen).

### Phase 2 — Ring Scanner Page
New `/mining/rings` page for FSS/DSS workflow — visible before dropping into a field.

### Phase 3 — Asteroid Log
Replace `/mining/log` stub with a real scrollable table of prospector results.

### Phase 4 — Session Statistics
Replace `/mining/statistics` stub with session efficiency graphs and per-material totals.

---

## Architecture

### Service Layer: `src/service/lib/event-handlers/mining.js`

New class `Mining`, registered in `src/service/lib/event-handlers.js` alongside the
existing `ShipStatus`, `Exploration`, etc. handlers. Same pattern: constructor receives
`{ eliteLog, eliteJson }`, exposes `getHandlers()` for `sendEvent` dispatch, and its
methods are called directly from `event-handlers.js` `logEventHandler`.

#### In-Memory Session State (reset on `LoadGame`)

```js
{
  systemName: 'HIP 12345',
  sessionStart: null,          // ISO timestamp of first mining event
  hasMiningModules: false,     // from Loadout
  
  cargo: {
    max: 128,                  // from Loadout.CargoCapacity
    current: 40,               // rolling total
    items: { tritium: 30, limpets: 16 }   // { [key]: count }
  },

  limpets: {
    remaining: 16,             // derived from cargo.items.limpets
    used: 42,                  // incremented on LaunchDrone (prospector/collector)
    synthed: 4,                // incremented on Synthesis (limpet)
    abandoned: 2               // incremented on EjectCargo (drone)
  },

  rings: [                     // populated by Scan + SAASignalsFound
    {
      bodyName: 'A 1 Ring',
      shortName: '1 Ring',
      ringType: 'Icy',         // 'Icy' | 'Metallic' | 'MetalRich' | 'Rocky'
      distanceLs: 1240,
      hotspots: { lowtemperaturediamond: 3, opal: 1 }  // from SAASignalsFound
    }
  ],

  activeProspectors: [null, null],  // [slot0, slot1] — rolling 2-slot buffer
  // Each slot: { id, timestamp, materials:[{name,pct,isTarget}], motherlode, contentLevel, remaining, isGood }

  session: {
    goodRocks: 12,
    totalRocks: 47,
    recentResults: [],         // last 50 ProspectedAsteroid entries
  },

  refined: {                   // from MiningRefined events
    items: { lowtemperaturediamond: 38, opal: 2 },
    totalTons: 40
  }
}
```

#### Journal Events Handled

| Journal Event | Action |
|---|---|
| `Scan` | Extract ring bodies. For each ring, classify type (`Icy`/`Metallic`/`MetalRich`/`Rocky`), compute density (mass / ring area). Store in `rings[]` if ring type matches any target material's preferred ring type. |
| `SAASignalsFound` | Find matching ring in `rings[]` by body name. Add hotspot counts per commodity. Filter to target materials only (configurable). |
| `ProspectedAsteroid` | Parse materials (by %). Flag each material as `isTarget`. Detect motherlode. Classify `isGood` (any target material above `minPercent` threshold, or motherlode of a target material). Increment `session.totalRocks`, conditionally `session.goodRocks`. Push to `recentResults`. Update `activeProspectors` rolling buffer. Broadcast `miningStateChange`. |
| `MiningRefined` | Add 1 to `refined.items[type]`. Broadcast `miningStateChange`. |
| `LaunchDrone` | Subtract 1 from `cargo.items.limpets`. Increment `limpets.used`. |
| `BuyDrones` | Add `Count` to `cargo.items.limpets`. |
| `SellDrones` | Subtract `Count` from `cargo.items.limpets`. |
| `EjectCargo { Type: "Drones" }` | Subtract from limpets. Increment `limpets.abandoned`. |
| `Synthesis { Name: "Limpet" }` | Add estimated count (min(4, available space)) to limpets. Increment `limpets.synthed`. |
| `Cargo` / `CargoFile` | Sync full `cargo.items` from inventory list. |
| `Loadout` | Set `cargo.max`. Set `hasMiningModules` (count modules with `mining` category). |
| `FSDJump` | Set `systemName`. Reset `rings[]`, `activeProspectors`, `session.*`, `refined.*`. Keep `cargo`/`limpets` (resupply mid-session is common). |
| `Location` | Set `systemName` on game load. Do not reset session (covers carrier jumps and game restarts in the same session). |
| `LoadGame` | Full session reset including cargo/limpets. This is the login boundary — the start of a new session. |
| `SupercruiseEntry` | Reset `activeProspectors`. Broadcast `miningStateChange`. Client uses this for autoswitch → Rings page. |
| `SupercruiseExit` | Broadcast `miningStateChange`. Client uses this for autoswitch → Status dashboard. |
| `Undocked` | If `hasMiningModules && cargo.items.limpets === 0`, include `warnNoLimpets: true` in broadcast. |

#### `getMiningStatus()` Return Shape

```js
{
  active: bool,            // has at least one mining event fired this session
  systemName: 'HIP 12345',
  sessionStart: '2026-04-28T14:00:00Z',
  hasMiningModules: true,
  targetMaterials: ['tritium','painite','platinum','opal','lowtemperaturedianond'],
  minPercent: 10,          // from preferences

  cargo: {
    max: 128,
    current: 40,
    fillPercent: 31.25,
    items: [{ key:'lowtemperaturedianond', name:'Low Temp Diamonds', count:36, isTarget:true }],
    limpetCount: 16
  },

  limpets: {
    remaining: 16,
    used: 42,
    synthed: 4,
    abandoned: 2,
    warnNoLimpets: false,
    wastedPercent: 4.5      // abandoned / (used+abandoned) * 100
  },

  rings: [
    {
      bodyName: 'A 1 Ring',
      shortName: '1 Ring',
      ringType: 'Icy',
      distanceLs: 1240,
      hotspots: [{ name: 'Low Temp Diamonds', key: 'lowtemperaturedianond', count: 3 }],
      hasTargetHotspots: true
    }
  ],

  activeProspectors: [
    {
      id: 0,
      timestamp: '...',
      materials: [
        { key:'lowtemperaturedianond', name:'Low Temp Diamonds', pct:35.2, isTarget:true },
        { key:'opal', name:'Void Opals', pct:11.8, isTarget:true }
      ],
      motherlode: null,
      contentLevel: 'Medium',   // 'High' | 'Medium' | 'Low'
      remaining: 100,
      isGood: true
    },
    null   // slot 2 empty
  ],

  session: {
    goodRocks: 12,
    totalRocks: 47,
    goodRockPercent: 25.5,
    recentResults: [/* last 10 */]
  },

  refined: {
    items: [
      {
        key: 'lowtemperaturediamond',
        name: 'Low Temp Diamonds',
        count: 38,
        lastRefinedAt: '2026-04-28T14:02:00Z',
        secsSinceLastRefine: 47     // computed at serve-time for freshness indicator
      }
    ],
    totalTons: 40,
    ratePerHour: 12.5     // computed from sessionStart
  }
}
```

#### Registration in `event-handlers.js`
```js
const Mining = require('./event-handlers/mining')
// ...in constructor:
this.mining = this._register(new Mining({ eliteLog, eliteJson }))
// ...in logEventHandler, add to relevant event arrays:
const MINING_EVENTS = ['ProspectedAsteroid','MiningRefined','LaunchDrone','BuyDrones',
  'SellDrones','EjectCargo','Synthesis','Cargo','CargoFile','Loadout','Scan',
  'SAASignalsFound','FSDJump','Location','SupercruiseEntry','Undocked']
if (MINING_EVENTS.includes(logEvent.event)) {
  this.mining.logEventHandler(logEvent)
}
```

---

## Client Pages

### Page 1: `/mining/status` — On-Grid Mining Dashboard

**Design goal**: A fixed-size, non-scrolling dashboard. The entire panel auto-scales to
fit the viewport (same `applyScaling()` CSS transform technique as `ShipInstrumentation`).
Works on 720p RPi Chromium kiosk mode without scrolling.

#### Dashboard Layout (conceptual)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  [asteroid-base icon]  HIP 12345 · A 1 Ring · Icy · 1,240 Ls          │
├──────────────────────────────────────────────────────────────────────────┤
│  CARGO   [████████████████████░░░░░░░░░░░░░░░]  40 / 128t              │
│          LTDs: 36  ·  Void Opals: 4  ·  Limpets: 16                    │
│  LIMPETS [████████░░░░░░░░░░░░░░░░░░░░░░░░░░░]  16 / 58 launched       │
├─────────────────────────────┬───────────────────────────────────────────┤
│  PROSPECTOR 1               │  PROSPECTOR 2                             │
│  ┌─────────────────────┐   │  ┌─────────────────────┐                  │
│  │ LTDs     █████ 35% ✓│   │  │  —  awaiting —       │                  │
│  │ V.Opals  ██    12% ✓│   │  └─────────────────────┘                  │
│  │ [Medium content]    │   │                                            │
│  └─────────────────────┘   │                                            │
├─────────────────────────────┼───────────────────────────────────────────┤
│  RINGS IN SYSTEM            │  SESSION STATS                            │
│  ▸ A 1 Ring [Icy] 3×LTD ●  │  Good rocks:  12 / 47  (25.5%)           │
│  ▸ A 2 Ring [Icy] 1×Opal ● │  Refined:     40 t  ·  12.5 t/hr         │
│  ▸ A 3 Ring [Metallic]      │  Limpets wasted:  4.5%                   │
└─────────────────────────────┴───────────────────────────────────────────┘
```

#### Sub-components
All live under `src/client/components/panels/mining/mining-status/`:

- **`MiningCargoBar`** — horizontal bar like the fuel bar on ship status; segments
  colored per commodity (target materials in `text-secondary`, others in `text-info`,
  limpets in `text-muted`). Shows `current / max t` label.
- **`MiningLimpetBar`** — horizontal bar showing remaining/total limpets launched.
  Shows warning icon + red color when `remaining < 10` (configurable threshold).
- **`MiningProspectorCard`** — one card per prospector slot (2 total). When populated:
  horizontal bars for each material (target materials highlighted, bars sized by %);
  content level badge; motherlode crown icon; "Good Rock" highlight when `isGood`.
  When empty: dim placeholder.
- **`MiningRingList`** — list of rings with ring type badge (`Icy`/`Metallic`/etc.),
  hotspot commodity + count with target-material highlight.
- **`MiningSessionStats`** — good rock ratio, refined tonnage, rate/hr, limpet waste %.

#### Autoswitch
Only fires if the current route is already within `/mining/*`.

- **`SupercruiseExit`** → `router.push('/mining/status')` — player just dropped into
  the asteroid field; show the on-grid dashboard.
- **`SupercruiseEntry`** → `router.push('/mining/rings')` — player went back to
  supercruise; show the system ring overview.

The `newLogEntry` listener handles both triggers. The check `router.pathname.startsWith('/mining')` prevents the switch from firing while the user is elsewhere in the app.

#### State management
Same pattern as `ship/status.js`:
- On mount / `[connected, ready]` change: `sendEvent('getMiningStatus')`
- `eventListener('miningStateChange', handler)` for live updates (broadcast from service)
- `eventListener('newLogEntry', handler)` for autoswitch + lightweight optimistic updates

---

### Page 2: `/mining/rings` — Ring Scanner (new page)

Shows all ring bodies in the current system with their types and hotspot data.
Most useful during FSS/DSS scanning before dropping into the asteroid field.

**Layout**: Scrollable panel (same as log pages). Table or card list:

```
 Body         Ring Type   Hotspots (target materials only)   Distance
 A 1 Ring     Icy         LTD ×3  ·  Void Opals ×1           1,240 Ls
 A 2 Ring     Icy         Void Opals ×2                       1,540 Ls
 A 3 Ring     Metallic    (none mapped yet)                   2,100 Ls
 B 1 Ring     MetalRich   Tritium ×1                          8,200 Ls
```

- Rows with target hotspots highlighted in secondary color
- Ring type badge color-coded: Icy=blue, Metallic=silver, MetalRich=gold, Rocky=muted
- "not yet mapped" vs "mapped, no hotspots" vs "has target hotspots" states
- Autoswitch TO this page when `SAASignalsFound` fires for a target material and user
  is on any other mining page

---

### Page 3: `/mining/log` — Asteroid Log

Scrollable table of all `ProspectedAsteroid` results from the current session.
Equivalent to the Prospector Observatory grid.

**Columns**: Time | Location | Commodities | % | Content | Notes

```
 14:02   A 1 Ring   LTDs · V.Opals   35.2% / 11.8%   Medium   ✓ Good Rock
 14:01   A 1 Ring   (nothing of interest)              Low      —
 13:58   A 1 Ring   LTDs (Core!)      —                High     ★ Core
```

- Good rocks highlighted in `text-secondary`
- Cores highlighted with star icon
- Filter bar: All / Good only / Cores only
- Clears on `FSDJump`

---

### Page 4: `/mining/statistics` — Session Statistics

Scrollable page showing efficiency metrics:

- **Session**: start time, duration, system, current ring
- **Prospector efficiency**: good rocks/total, good rock %, cores found
- **Limpet efficiency**: launched, synthed, abandoned, % wasted
- **Refined totals**: table of commodities with counts, running total tonnage
- **Rate**: tons per hour (running average over session)

Text-based (no canvas required). Styled consistently with other statistics pages.

---

## Navigation Updates

Update `MiningPanelNavItems` in `src/client/lib/navigation-items.js`:

```js
function MiningPanelNavItems (activePanel) {
  const navigationItems = [
    { name: 'Status',     icon: 'asteroid-base',    url: '/mining/status' },
    { name: 'Rings',      icon: 'planet-ringed',     url: '/mining/rings' },   // NEW
    { name: 'Log',        icon: 'table-inspector',   url: '/mining/log' },
    { name: 'Statistics', icon: 'trending-up-chart', url: '/mining/statistics' }
  ]
  // ...
}
```

Also add `/mining/rings` stub page with `index.js` redirect logic.

---

## Preferences Integration

The mining target materials and sensitivity threshold should be stored alongside
existing preferences in `Preferences.json`:

```json
{
  "miningTargetMaterials": ["tritium","painite","platinum","opal","lowtemperaturediamond"],
  "miningMinimumPercent": 10
}
```

Exposed via `getPreferences` / `setPreferences` (already exists). A future settings UI
can surface these, but for now they can be edited manually or default as above.

---

## Icons Needed

### New SVG Icons (designed by you)

#### `mining-core.svg`
Source: `Mining.svg` already downloaded to `resources/icons/`. Use or adapt this for the
motherlode / core asteroid indicator on the prospector cards.

#### Ring type badges (`ring-icy.svg`, `ring-metallic.svg`, `ring-metal-rich.svg`, `ring-rocky.svg`)
Two concentric ring arcs close together (like a cross-section of a Saturn-style ring),
colored by type:
- **Icy**: cool blue stroke
- **Metallic**: silver/grey stroke  
- **MetalRich**: warm gold/amber stroke
- **Rocky**: muted brown/stone stroke

#### Hotspot overlay (`hotspot-overlay.svg`)
A yellow filled circle with a radial gradient fading outward to low-opacity orange at the
edges. Intended to be layered over a ring badge (or inline in a ring list row) to indicate
a confirmed DSS hotspot at that ring. Can be a standalone SVG composited in CSS/JSX rather
than a font glyph.

#### No limpet icon needed
Limpets will use `barrel` from the existing font for the limpet bar label.

### Existing icons reused (no new icon needed)
- Cargo hold → `cargo`
- Limpet bar label → `barrel`
- Ring body list header → `planet-ringed`
- Session stats → `trending-up-chart`
- Warning (no limpets) → `warning`
- Core/motherlode → `mining-core.svg` (new)

---

## Implementation Order

1. **`src/service/lib/event-handlers/mining.js`** — service handler class; no UI changes.
   Enables all other work. Includes `getMiningStatus()` + `miningStateChange` broadcast.
   Register in `event-handlers.js`.

2. **`src/client/pages/mining/status.js`** + sub-components in
   `src/client/components/panels/mining/mining-status/` — the primary deliverable.
   Start without the auto-scaling wrapper (normal layout first), add scaling once
   component is functionally correct.

3. **`src/client/pages/mining/rings.js`** + `src/client/pages/mining/rings/index.js`
   stub — the FSS/DSS workflow view.

4. **`src/client/pages/mining/log.js`** — replace ComingSoonPanel with real table
   driven by `session.recentResults` (served by `getMiningStatus()`).

5. **Navigation update** — add Rings tab to `MiningPanelNavItems`.

6. **`src/client/pages/mining/statistics.js`** — session efficiency stats page.

7. **CSS** — `src/client/css/panels/mining-panel.css` (new file).
   Import in `main.css`.

---

## Refinery Bin Estimation

The game API does not expose refinery bin fill level directly. The only refinery-related
journal event is `MiningRefined`, which fires when a full bin has been processed and one
tonne is added to cargo. There is no "fragment landed in bin" event.

**Approach — history + correction sync**:
1. Track every `MiningRefined` event: commodity type, timestamp, and session ton count.
2. Record `lastRefinedAt` per commodity. The dashboard shows "last refined Xs ago" as a
   freshness indicator so the player knows the refinery is active.
3. Estimate bin fill using ProspectedAsteroid data: after a prospector result arrives, we
   know the material % of the current asteroid. When the player fires a collector limpet
   (tracked by `LaunchDrone` — all drone types use the same event), we know a fragment
   collection attempt is in progress. However the exact fill progression is not known.
4. The estimates are **always corrected to ground truth** on each `MiningRefined` event:
   that commodity's "in-bin" estimate resets to 0% and starts accumulating again.
5. On the Status dashboard, refinery bins are shown as a simple per-commodity "last
   refined X ago + total refined" row rather than a fill-level bar, to avoid displaying
   speculative data as authoritative.

If the game ever exposes a bin-state file (similar to `Cargo.json`), this can be upgraded.

---

## Comparison to Prospector Plugin

| Feature | Prospector Observatory | Daedalus Mining |
|---|---|---|
| Ring type detection on FSS Scan | ✓ | ✓ (Rings page) |
| DSS hotspot display | ✓ (grid row) | ✓ (Rings page + Status) |
| Prospector limpet results | ✓ (grid + notification) | ✓ (Status dashboard) |
| Cargo tracking | ✓ (sticky notification) | ✓ (Cargo bar on Status) |
| Limpet tracking | ✓ (included in cargo notification) | ✓ (dedicated Limpet bar) |
| Good rock detection | ✓ | ✓ |
| Core/motherlode detection | ✓ | ✓ |
| Session stats | ✓ (basic %) | ✓ (Statistics page) |
| Refinery bin tracking | ✗ (game limitation) | History + correction sync |
| Synth mat rich bodies | ✓ | Not planned (Engineering section handles mats) |
| Raw material inventory | ✓ | Not planned (Ship Inventory section handles this) |
| "Don't forget limpets" alert | ✓ (notification) | ✓ (inline warning on Status) |
| Visual dashboard | ✗ | ✓ (Status page — main differentiator) |
| Per-commodity history | ✗ | ✓ (Log + Statistics) |

---

## Resolved Decisions

| Decision | Resolution |
|---|---|
| Autoswitch scope | Only fires when already on `/mining/*`. `SupercruiseExit` → Status, `SupercruiseEntry` → Rings |
| Refinery bins | History + correction sync approach (see Refinery Bin Estimation section) |
| Session reset | On `LoadGame` (login) only. FSDJumps keep cargo/limpets; rings/session stats reset on system change |
| Rings page URL | `/mining/rings` ✓ |
| Limpet icon | Not needed; reuse `barrel` from existing font |
