# Mining Dashboard — Notch & Layout Geometry Plan

## Goal (from mockup)

```
┌────────────────────┐  ┌──────────┐  ┌─────────────────────────┐
│  RECENT PROSPECT   │  │  CONFIG  │  │       STATISTICS         │
│                    │  │          │  │                          │
│                    ╰──╯          ╰──╯                          │  ← row1 (15rem)
│                                                                 │
├──────────────────────────────────────────────────────────────── ┤
│ LASER       ╭────────────────╮                 (row2, 22rem)   │
│ POWER       │    ⬤ circle   │                                  │
│             ╰────────────────╯                                  │
├──────────────────────────────────────────────────────────────── ┤
│  PROSPECT   │  ╭──╮          ╭──╮  │  REFINED                  │  ← row3 (15rem)
│  HISTORY   ╰──╯  TARGET      │   ╰─╯                           │
│                               │                                 │
└────────────────────────────────────────────────────────────────┘
```

Key changes needed:
1. **Circle** needs to move further left — prospector cards need their right-side arc notches
2. **Config / Target** cards are full col2 width (22rem) but should visually be ~70% the circle diameter, centred on the circle
3. **Config / Target** notch arcs must be **concave** (arc bows away from the card, into the card)
4. **Statistics** (row1+2) and **Refined** (row2+3) — merge into single tall cells, notched on left side facing circle
5. **Prospector cards** — restore right-side arc notch cutouts

---

## Grid Geometry (exact values)

Dashboard: `110rem × 56rem`, padding `1rem 1.5rem`, `gap: 1rem`.

### Columns (total inner = 107rem, 4 gaps = 4rem → content = 103rem)

| Area       | Width  | Left edge (from dashboard left, after padding) |
|------------|--------|------------------------------------------------|
| prosp      | 30rem  | 1.5rem                                         |
| config/circle/target | 22rem | 32.5rem (= 1.5 + 30 + 1)            |
| stats/refined | 18.5rem | 55.5rem (= 32.5 + 22 + 1)          |
| cargo      | 26rem  | 75rem                                           |
| limpets    | 6.5rem | 102rem                                          |

### Rows (total inner = 56rem, 2×1rem padding, 2 gaps → 52rem content)
Row1 + Row3 = 52 - 22 = 30rem → **Row1 = Row3 = 15rem**

| Area   | Height | Top edge (from dashboard top, after padding) |
|--------|--------|----------------------------------------------|
| row1   | 15rem  | 1rem                                          |
| row2   | 22rem  | 17rem (= 1 + 15 + 1)                         |
| row3   | 15rem  | 40rem (= 17 + 22 + 1)                        |

### Circle centre

- Grid cell col2 occupies x: 32.5–54.5rem (width=22rem)
- Circle is 22rem wide, centred in 22rem cell → natural centre x = 32.5 + 11 = 43.5rem
- With `margin-left: X` the centre shifts to 43.5 + X/2 (for centering), or we use `justify-self:center` + margin-left to shift

**Simpler framing**: Circle occupies the full 22rem cell width at `justify-self: center`. With no margin the circle right edge aligns with col2 right = col3 left = 54.5rem. The prospector card right edge is at 1.5+30=31.5rem. Gap between prospector right and circle left = 32.5-11=21.5rem when centred — that is far too much.

**Required**: Circle left edge must be flush or ~1rem from prospector right edge (32.5rem).
- Circle left edge = 32.5 → centre = 32.5 + 11 = 43.5rem. That's exactly where it naturally lands when justified-centre in col2.
- But the circle overlaps into col3 on the right: right edge = 43.5+11=54.5 = exactly col3 left edge.

So: **no margin needed** for the circle touching prospector — just `justify-self: start` with `margin-left: 0`. Actually `justify-self: center` in a 22rem cell with a 22rem circle means the circle fills the entire col2 cell edge to edge. The gap between prospector right and col2 left is the 1rem grid gap — that is the notch space.

**Re-check with actual screenshot**: The circle in the screenshot is well away from the prospectors. This means the current `justify-self: center` + no offset is wrong because the grid cell (22rem) is wider than the circle appears, OR `margin-left: -4rem` is too small. 

**New plan**: Use `margin-left: -11rem` on circle-wrap so the circle left edge aligns to col2 left edge → circle fills col2 exactly → circle right edge is at 54.5rem, which is at col3 left. Col3 holds stats/refined, so stats/refined need left-side notches.

---

## Revised Grid Template

### Changes to `grid-template-areas`:
- **Stats** currently spans only row1. Needs to span row1+row2 → use `grid-row: span 2` or restructure.
- **Refined** currently spans only row2. Needs to span row2+row3 → `grid-row: span 2`.
- This means removing the `.` empty cell in row3 col3 and merging.

New template:
```css
grid-template-areas:
  "prosp-top  config   stats-refined  cargo  limpets"
  "laser      circle   stats-refined  cargo  limpets"
  "prosp-bot  target   stats-refined  cargo  limpets";
```

But we need **two** cards (Statistics above, Refined below) in col3. Options:
1. Single grid area `stats-col` spanning all 3 rows, with internal flex layout splitting into two cards.
2. Keep separate grid areas but use `grid-row` span.

**Option 2 is cleaner** — keep two separate elements, span them:
```css
.mining-panel__info-card-wrap--stats   { grid-area: stats;   grid-row: 1 / 3; grid-column: 3; }
.mining-panel__info-card-wrap--refined { grid-area: refined; grid-row: 2 / 4; grid-column: 3; }
```
But they overlap in row2! Stats would be rows 1-2, Refined rows 2-3 → both claim row2 col3.

**Option 1 — single wrapper**: Use one grid item spanning all 3 rows, containing two inner cards with `flex: 1` each.
```css
grid-template-areas:
  "prosp-top  config   right-col  cargo  limpets"
  "laser      circle   right-col  cargo  limpets"
  "prosp-bot  target   right-col  cargo  limpets";

.mining-panel__right-col { grid-area: right-col; display: flex; flex-direction: column; gap: 1rem; }
```
Inner children:
```jsx
<div className='mining-panel__right-col'>
  <MiningSessionCard ... />   {/* flex: 1 → ~23.5rem tall */}
  <MiningRefinedCard ... />   {/* flex: 1 → ~23.5rem tall */}
</div>
```
Each inner card takes half: (52rem - 1rem gap) / 2 = 25.5rem. But the circle is 22rem, so both cards together equal the panel height.

**This is the right approach.**

---

## Notch Geometry — All Positions

### Coordinate system: cell-local (0,0 = top-left of card)

### A. Prospector cards (col1, rows 1 and 3) — right-side notch facing circle

Card dimensions: **30rem × 15rem**

Circle centre in col1-local coords:
- Circle centre x = col2_left + 11 - 11 = col2_left = 32.5rem → relative to col1 left (1.5rem) = 31rem from card left. But card is only 30rem wide → circle centre is **1rem to the right of the card right edge**.
- Circle centre y relative to prosp-top card: centre of row2 = 17 + 11 = 28rem from dashboard top. Card top = 1rem, card bottom = 16rem. So circle centre is **12rem below the card bottom edge**.

Arc radius r = 11rem (circle outer edge) + 1rem gap = **12rem**.

For a circle at (31rem, 12+15=27rem) relative to prosp-top card top-left:
- Intersection with right edge (x=30rem): 12² = (31-30)² + (27-y)² → 144 = 1 + (27-y)² → (27-y)=√143=11.96 → y=15.04rem (just off bottom) ... hmm.

Re-check: circle centre relative to the **prosp-top** card coordinate system:
- Card top-left is at dashboard (1.5rem, 1rem)
- Circle centre is at dashboard (43.5rem, 17+11=28rem)  
- Local: x = 43.5 - 1.5 = 42rem, y = 28 - 1 = 27rem

Card is 30×15rem. Circle (42, 27) with r=12:
- Right edge x=30: Δx=42-30=12, Δy²=144-144=0 → y=27rem. Card height is 15rem → y=27 is **12rem below the card**, no intersection on right edge.

**The circle simply doesn't reach the prospector card** with the current layout at all. The gap in x alone equals 12rem (the arc radius), meaning the circle is tangent to the right edge vertically at y=27rem — which is outside the card.

**This is the fundamental problem.** The prospector card needs to physically overlap with or abut the circle for a notch to make sense.

---

## Correct Layout for Notches to Work

From the **mockup image**, the circle's left side is notched into the prospector cards. This requires:

> The **circle must physically overlap** the prospector column or the gap must be ≤ arc radius.

### Solution: Change column widths

Currently col1=30rem, col2=22rem. If we give the circle `margin-left: -X` to push it left into the prospector column:

**Target**: Circle left edge at prospector right edge minus 0rem (flush). 
- Prospector right edge at x=31.5rem (1.5 + 30rem).
- Circle left edge = centre - 11rem.
- For left edge = 31.5 → centre = 42.5rem.
- Col2 left = 32.5rem, col2 centre = 43.5rem.
- Required `margin-left` on circle-wrap = shifts justify-self:center leftward.
  - Natural centre in col2 = 43.5. Need 42.5. Shift = -1rem? No — `margin-left` on a `justify-self:center` item shifts the item by that amount. So `margin-left: -1rem` moves circle centre from 43.5 to 42.5rem.
  - Circle left edge = 42.5 - 11 = 31.5rem = prospector right edge ✓

**But the gap between col1 and col2 is 1rem** — so the circle left edge at 31.5rem means it's within the gap, touching the prospector right edge. The notch is carved into the prospector at the circle boundary.

**For a visible notch**: Use `margin-left: -3rem` → centre = 40.5rem → left edge = 29.5rem → 0.5rem inside the prospector card. Circle overlaps prospector by 0.5rem on the left. The notch is carved out of the prospector's right side.

**Recommended: `margin-left: -5rem`** → centre = 38.5rem → left edge = 27.5rem → 2.5rem into prospector. Good visible notch.

### Recompute all notch geometry with `margin-left: -5rem`:

Circle centre: x = 43.5 - 5 = **38.5rem** from dashboard left, y = **28rem** from dashboard top.

---

## Notch Details (with circle centre at x=38.5rem, y=28rem dashboard-absolute)

Arc radius **r = 12rem** (11rem circle + 1rem border gap).

### Prospector Top card (30rem × 15rem, top-left at 1.5rem, 1rem)
Local circle centre: (38.5-1.5, 28-1) = (37, 27)
- Intersection right edge (x=30): Δx=7, Δy²=144-49=95 → Δy=9.75 → y=27±9.75 → y=17.25 (below card) and y=17.25 ... wait: y=27-9.75=17.25 (below card bottom=15), y=27+9.75=36.75 (way below). 

Hmm — bottom intersection only. Let's find bottom edge (y=15) intersection:
- Δy=27-15=12, Δx²=144-144=0 → x=37. So the arc is tangent to the bottom edge at x=37rem (local) — but card only extends to x=30. The arc is entirely to the right of the card.

**The circle still doesn't intersect the prospector card's right edge or bottom edge.** With `margin-left: -5rem` the circle centre is at dashboard x=38.5 and the prospector right edge is at x=31.5. Distance = 7rem < 12rem ✓ — the arc does intersect! Let me redo:

Right edge x=30 (local). Circle local centre (37, 27). Distance from centre to right edge line x=30: 37-30=7rem < 12rem ✓.
- Intersection y: (30-37)²+(y-27)²=144 → 49+(y-27)²=144 → (y-27)²=95 → y=27±9.75
  - y₁ = 17.25 (below card, card ends at y=15)
  - y₂ = 36.75 (far below)
- Neither intersection is within the card (0–15rem height). **Still no intersection on the right edge.**

Bottom edge y=15 (local). Circle local centre (37, 27). Distance = 12. Intersection x: (x-37)²+(15-27)²=144 → (x-37)²=144-144=0 → x=37 (outside card width 0-30). **Just tangent, outside card.**

**Conclusion**: With the current 30rem+1rem gap+22rem column arrangement, the circle can only notch into the prospector if the margin-left moves the circle far enough into the prospector. Needed distance for right-edge intersection:

For right edge (x=30 local, card width=30):
- Need circle local centre x < 30 + 12 = 42. Centre is at 37 ✓ already for the 5rem case.
- Wait — I got centre at local (37, 27) with `margin-left: -5rem`. That IS within 12rem of x=30!
- The issue is the y range: intersections at y=17.25 and y=36.75, both outside 0-15.

The fundamental problem: **the circle centre is 12rem BELOW the card bottom** (local y=27, card height=15). The arc at the right edge only intersects at y=17.25 — which is 2.25rem below the card. The notch would be on the bottom-right corner area.

**For the notch to show on the right edge within the card**, we need the circle centre to be within the card's y-range ± 12rem i.e. within -12 to 27. Circle centre local y=27-12=15 = card bottom. So the arc barely reaches the card bottom-right corner.

The notch only appears on the **bottom-right corner** of the prospector-top card and **top-right corner** of prospector-bottom card. That's exactly what the mockup shows — a concave quarter-arc at the inner corner.

With `margin-left: -5rem`, circle local y=27 in prosp-top card coords. Arc hits bottom edge y=15 at x=37 (outside). Arc hits right edge x=30 at y=17.25 (outside). So the arc misses the card entirely!

**Need bigger margin-left.** For arc to reach the card corner (30, 15):
- Distance from circle local centre to corner (30, 15): √((cx-30)²+(cy-15)²) = 12
- cy = 27 (fixed by grid). So (cx-30)²+(27-15)²=144 → (cx-30)²=144-144=0 → cx=30.
- Circle local centre must be at x=30 for arc to exactly touch corner.
- Local x = dashboard_x - card_left = dashboard_x - 1.5 = 30 → dashboard_x = 31.5rem.
- Col2 centre (natural justify-self:center) = 43.5rem. Needed = 31.5rem. margin-left = 31.5-43.5 = **-12rem**.

With `margin-left: -12rem`:
- Circle centre dashboard x = 43.5 - 12 = 31.5rem
- Circle left edge = 31.5 - 11 = 20.5rem — **9.5rem inside the prospector card**
- Circle local centre in prosp-top: (31.5-1.5, 28-1) = (30, 27)
- Arc intersects right edge (x=30): x=cx=30, y=27±12 → y=15 and y=39. y=15 is exactly card bottom corner!
- Arc intersects bottom edge (y=15): y=15=cy-12, x=cx=30. Same point (30,15).

The arc is tangent to the corner (30,15) — just touches. Still no visible notch inside the card.

**Need margin-left slightly beyond -12rem** for the arc to cut into both edges. Try `margin-left: -14rem`:
- Circle dashboard x = 43.5 - 14 = 29.5rem (1rem inside prospector card right edge)
- Circle left edge = 29.5 - 11 = 18.5rem
- Local centre in prosp-top: (29.5-1.5, 27) = (28, 27)

Right edge (x=30): (30-28)²+(y-27)²=144 → (y-27)²=140 → y=27±11.83 → y=15.17 and y=38.83
- y=15.17 is just 0.17rem below card (out of bounds)

Bottom edge (y=15): (x-28)²+(15-27)²=144 → (x-28)²=144-144=0... wait: 12²=144, (15-27)²=144. So still tangent at (28,15) — which is within x=[0,30] ✓

So with `-14rem`, arc is tangent to bottom edge at (28, 15) local — bottom edge, at x=28rem from card left. Just touches. The visible arc starts from within the card.

**Correct approach**: The notch arc sits at the **bottom-right corner** of the prospector-top card. Points:
- Arc enters from bottom edge (y=15) at some x<30
- Arc exits from right edge (x=30) at some y<15

For this, the circle centre must be positioned so both intersection points are within card bounds. Let's try `margin-left: -16rem`:
- Dashboard circle x = 43.5 - 16 = 27.5rem, local in prosp-top = (26, 27)
- Right edge (x=30): (30-26)²+(y-27)²=144 → (y-27)²=128 → y=27±11.31 → y=15.69 (outside) and y=38.31 (outside). Still outside!

The right-edge intersection y is always > 15 (card height) because cy=27, card height=15, and 27-15=12 = r. Intersection y = 27-√(144-(30-cx)²). For this to be ≤ 15: 27-√(144-(30-cx)²) ≤ 15 → √(144-(30-cx)²) ≥ 12 → 144-(30-cx)² ≥ 144 → (30-cx)² ≤ 0 → cx=30.

**Mathematical conclusion**: The right-edge arc intersection is **never within** the prosp-top card (height 0-15rem) because the circle centre is always at local y=27 (12rem below card bottom). The arc can only produce a notch on the **bottom edge** only.

---

## Correct Layout to Enable Side Notches

The mockup clearly shows notches on the right side of the prospector cards (not the bottom). This means the circle centre must be at the same y-height as the prospector cards' midpoints, which requires:

**The circle must span into the row1 and row3 areas** — i.e., the circle visually overlaps the prospector cards vertically, not just be adjacent below/above them.

### Solution: Prospector cards and circle in the same rows

Restructure so prosp-top and prosp-bot are in the same row as the circle (row2), stacked vertically, with the circle beside them.

**New approach — single left column spanning all 3 rows, containing two prospector cards:**

```css
grid-template-areas:
  "prosp-col  config     right-col  cargo  limpets"
  "prosp-col  circle     right-col  cargo  limpets"
  "prosp-col  target     right-col  cargo  limpets";
```

- `prosp-col` spans rows 1-3 (full height 52rem), contains two prospector cards (each ~25.5rem)
- Circle (22rem) in row2 → circle centre local y = 11rem
- Prosp card 1 occupies local y 0-25.5rem, prosp card 2 occupies local y 26.5-52rem

Circle centre in prosp-col coords:
- Circle dashboard centre: x=?, y = 1(padding) + 15(row1) + 1(gap) + 11(half circle) = 28rem from top
- Prosp-col top = 1rem (padding), so local y = 28 - 1 = 27rem

Prosp card 1 bottom ≈ 25.5rem. Circle centre local y=27 is **1.5rem into prosp card 2** — close to the card boundary. Still awkward.

**The fundamental issue**: With a 3-row grid where row1=row3=15rem and row2=22rem, the circle centre (row2 centre) is at y=28rem dashboard. The prospector cards span rows 1+2 (prosp1, 0-37rem) and rows 2+3 (prosp2, 17-52rem) in absolute coords — their midpoints are at 18.5rem and 34.5rem respectively. Circle centre y=28rem is within both spans.

**This is the right solution.** Each prospector card spans 2 rows. The circle is in row2 only. The circle centre is vertically near the bottom of prosp1 and near the top of prosp2.

---

## Final Recommended Layout

### Grid

```css
grid-template-columns: 30rem 22rem 18.5rem 1fr 6.5rem;
grid-template-rows: 1fr 22rem 1fr;
gap: 1rem;
grid-template-areas:
  "prosp-top  config    right-col  cargo  limpets"
  "laser      circle    right-col  cargo  limpets"
  "prosp-bot  target    right-col  cargo  limpets";
```

**New element**: `.mining-panel__right-col` spans `right-col` across all 3 rows:
```css
.mining-panel__right-col { grid-area: right-col; display: flex; flex-direction: column; gap: 1rem; }
```
Contains Statistics card (flex:1) and Refined card (flex:1).

**Prospector cards**: Keep as separate row1/row3 items (30rem × 15rem). To get the notch on the bottom-right (prosp-top) and top-right (prosp-bot), the circle needs to be at the right x-position.

### Circle X position

Notch target: bottom-right corner of prosp-top (local 30rem, 15rem). Circle must pass through or near this corner.

Circle centre must be at distance ≤ r=12 from corner (30, 15) in prosp-top local coords.
- prosp-top local origin = dashboard (1.5rem, 1rem)
- Corner dashboard coords = (1.5+30, 1+15) = (31.5, 16)
- Circle centre dashboard y = row2 centre = 1 + 15 + 1 + 11 = 28rem
- Distance from circle centre (cx, 28) to corner (31.5, 16): √((cx-31.5)²+(28-16)²) = 12
- (cx-31.5)²+144=144 → cx=31.5

**Circle centre must be at dashboard x=31.5rem.** Col2 centre (natural) = 43.5rem. Required `margin-left`:
- With `justify-self:start` and `margin-left: X`: circle left at 32.5+X, centre at 32.5+X+11
- 32.5 + X + 11 = 31.5 → X = -12rem

**`margin-left: -12rem`** places circle centre at x=31.5rem, exactly touching the corner. The arc is tangent at the corner — just a point. We want a **visible notch**, so use `margin-left: -14rem` to push slightly further, creating a real arc cut.

With `margin-left: -14rem`:
- Circle centre dashboard x = 32.5 + (-14) + 11 = 29.5rem
- Circle left edge = 29.5 - 11 = 18.5rem (11.5rem inside prospector card)

In prosp-top local coords: centre = (29.5-1.5, 28-1) = (28, 27)

Bottom edge (y=15) intersections: (x-28)²+(15-27)²=144 → (x-28)²=0 → x=28. Tangent at (28,15). One point only.

We need cx < 28 for two intersection points on the bottom edge. Try `margin-left: -16rem`:
- Centre x = 32.5 - 16 + 11 = 27.5rem dashboard
- Local: (26, 27)
- Bottom (y=15): (x-26)²+144=144 → x=26. Still tangent!

**The problem**: cy=27, card bottom y=15, distance=12=r. The circle is always tangent to the card bottom, never secant, because the circle centre is EXACTLY r=12 below the card bottom (grid geometry forces this: circle centre is always at y=28rem, card bottom at y=16rem, difference = 12rem = r).

**To get a concave notch on the bottom edge**: We either need to use a slightly smaller radius (r<12) OR adjust the grid so the circle centre is closer to the card (reduce the gap, or increase circle margin to bring it up).

**OR**: Accept that the notch is on the **right edge** only (not the bottom), which requires the circle centre to be closer in y to the card. This means the prospector cards need to extend into row2.

---

## THE ACTUAL CORRECT SOLUTION

Looking at the mockup again carefully: the prospector notch in the mockup is on the **bottom-right of prosp-top** and **top-right of prosp-bot** — meaning the arc curves across the corner where the right and bottom (or top) edges meet.

For this to work geometrically, the notch requires the arc to cut through BOTH the right edge and the bottom/top edge of the card. This is a corner notch.

**The existing working notch CSS already does this** — the `clip-path: polygon` for `notch-br` cuts points along the bottom-right area. The issue is that these percentages were calculated for a **different card height** than the current 15rem.

The existing notch-br was computed for a card ~25.1rem tall (based on viewBox "0 0 30 25.1"). The current row height is 15rem. **The percentages are wrong for the new height.**

### Recalculate for 30rem × 15rem cards

Circle centre dashboard (31.5, 28). Prosp-top local centre (30, 27). r=12.

At local x=30 (right edge): y = 27 ± √(144-0) = 27 ± 12 → y=15 (card corner!) and y=39.
At local y=15 (bottom edge): x = 30 ± √(144-144) = 30. Same corner point.

The arc only hits the card at **exactly the corner (30, 15)** — the bottom-right corner. To create a visible notch, we need the arc to pass INSIDE the card bounds.

**Circle must be closer in x:** If centre is at local (27, 27):
- Right edge (x=30): y=27±√(144-9)=27±11.62 → y=15.38 (just outside, 0.38rem below card bottom)
- Bottom edge (y=15): x=27±√(144-144)=27. Still tangent!

The y-distance (27-15=12=r) makes the bottom edge always tangent or secant in degenerate cases. **We must reduce r or adjust vertical geometry.**

### Grid Row Height Adjustment

If row1 height increases to 18rem (giving r-distance=10rem < 12rem):
- Row1+Row3 = 52-22 = 30rem. If row1=18, row3=12.
- Circle centre y = 1+18+1+11 = 31rem
- Prosp-top bottom edge y = 1+18 = 19rem. Local bottom = 18rem.
- Circle local (in prosp-top): cy = 31-1 = 30. Distance to bottom = 30-18 = 12 = r. Still tangent!

The distance is always `r` because: circle centre y = padding + row1 + gap + r = 1 + row1 + 1 + 12 = row1 + 14. Card bottom y = 1 + row1 = row1 + 1. Distance = (row1+14) - (row1+1) - ... wait, in local coords:

- Local circle cy = (dashboard circle y) - (card top y) = (1 + row1 + 1 + 11) - 1 = row1 + 12
- Card bottom in local = row1
- Distance = cy - card_bottom = (row1+12) - row1 = **12 = r** always.

**This is a fixed geometric constraint** given the 1rem gap between rows and the 22rem circle (radius 11) plus 1rem notch gap (total r=12). The circle arc will always be tangent to the card bottom edge — a notch of zero width.

### The Real Fix: Use r < 12 (don't add extra gap)

Use **r = 11rem** (exact circle edge, no gap). Then distance = 11, card bottom distance = 12. 

Actually that makes it worse. We need r > 12 to cut into both edges.

### Final Fix: Prospector cards span into row2

**Make prospector cards span rows 1+2 (for prosp-top) and 2+3 (for prosp-bot).** Then:

```css
grid-template-areas:
  "prosp-top  config    right-col  cargo  limpets"
  "prosp-top  circle    right-col  cargo  limpets"
  "prosp-bot  target    right-col  cargo  limpets"
  "prosp-bot  .         right-col  cargo  limpets";
```

No — this changes the row count. Better: keep 3 rows, use CSS `grid-row: span 2` for prospector cards in JSX/CSS:

```css
.mining-panel__prospector-wrap--notch-br { grid-area: prosp-top; grid-row: 1 / 3; }
.mining-panel__prospector-wrap--notch-tr { grid-area: prosp-bot; grid-row: 2 / 4; }
```

Both prospector cards now span 2 rows each. Prosp-top: rows 1-2 (height=15+1+22=38rem). Prosp-bot: rows 2-3 (38rem).

Circle is in row2. Circle centre y = 28rem. Prosp-top spans y=1 to 39rem. Local cy = 28-1=27rem. Card height = 38rem.

Right edge (x=30 local). Centre (cx, 27). For a proper notch: need two intersections on right edge within [0,38]:
- y = 27 ± √(144-(30-cx)²)
- For both to be in [0,38]: 27-√... ≥ 0 and 27+√... ≤ 38

The upper intersection y₁=27+√(144-(30-cx)²) ≤ 38 → √... ≤ 11 → (30-cx)² ≥ 144-121=23 → |cx-30| ≥ 4.8

So if cx ≤ 25.2 (i.e., cx at most 25.2 in local coords, dashboard x ≤ 26.7):
- With `margin-left: -18rem`: centre x = 32.5-18+11=25.5rem dashboard → local cx = 24rem
- y₁ = 27+√(144-(30-24)²) = 27+√(144-36) = 27+√108 = 27+10.39 = 37.39 ✓ (within 38)
- y₂ = 27-10.39 = 16.61 ✓ (within 38)

**With `margin-left: -18rem` and prosp cards spanning 2 rows:**
- Circle left edge = 25.5-11=14.5rem (within prosp card, 14.5rem inside the 30rem card)
- That's a huge overlap — too much visually.

**Let's try the original working proportions from the old code.** The old notch-br viewBox was "0 0 30 25.1" — so the card was 30×25.1rem. At that height, the circle centre y in local coords was: looking at the arc path `M 30 0 L 30 18.056 A 12 12 0 0 0 26.15 25.1` — the arc starts at (30, 18.056) on the right edge and ends at (26.15, 25.1) on the bottom edge. Circle centre = (30, 30) would be at... let me solve: arc with r=12 from (30,18.056) to (26.15,25.1): centre at (cx,cy) equidistant. Midpoint = (28.075, 21.578). Perpendicular bisector approach, or: since both points are on the circle, cx²+cy² equations.

For the path, the arc goes from the right edge to the bottom edge at the bottom-right corner. The circle must be centred at the bottom-right "outside" corner:
- Pt1 (30, 18.056): (cx-30)²+(cy-18.056)²=144
- Pt2 (26.15, 25.1): (cx-26.15)²+(cy-25.1)²=144

With cx=30 (right edge extended): (0)+(cy-18.056)²=144 → cy=18.056±12. And (3.85)²+(cy-25.1)²=144 → (cy-25.1)²=129.18 → cy=25.1±11.37. Intersection: cy=30.056 or cy=6.056 vs cy=36.47 or cy=13.73. cy=30.056? Check pt2: (30-26.15)²+(30.056-25.1)²=14.82+24.56=39.38≠144. 

Let me just trust the old geometry worked visually. The key data:
- Old card was 30rem wide, ~25.1rem tall
- Circle extended 8rem past the card right edge (from arc data: notch starts at x=30 (right edge), so circle was to the right)

Wait — re-reading: "circle center at (38rem, 27rem) in left-col coords" from the CSS comment in the existing code! The prospector card is 30rem wide, circle centre at local x=38 — that's 8rem to the right of the card right edge. NOT overlapping at all. And y=27 in a 25.1rem tall card — that's 1.9rem below the card.

So the old working notch had the circle 8rem to the right and ~2rem below the card bottom-right corner. The arc passed near the corner. With the circle at (38,27) and r=12:
- Right edge (x=30): (30-38)²+(y-27)²=144 → 64+(y-27)²=144 → (y-27)²=80 → y=27±8.94 → y=18.06 ✓ and y=35.94 (outside 25.1). So right-edge intersection at y=18.06 (72% of height).
- Bottom edge (y=25.1): (x-38)²+(25.1-27)²=144 → (x-38)²=144-3.61=140.39 → x=38±11.85 → x=26.15 ✓ (within 30) and x=49.85 (outside).

**This is the working geometry!** The card height is 25.1rem, circle centre is at (38, 27) local — 8rem right and 1.9rem below card bottom.

**The reason the old code worked**: Card height ~25.1rem (spanning 2 grid rows with gap), not 15rem (single row).

---

## DEFINITIVE PLAN

### 1. Prospector cards must span 2 rows each

- `prosp-top` spans grid rows 1-2 (height = 15+1+22 = 38rem) — **too tall**, will cover the circle
- Need **4-row grid**? Or accept prospectors span half:

Actually from the CSS comment: "Geometry: card = 30rem × ~25.1rem". This was the old layout where the cards were in a single column filling 25.1rem each. Our current row1=15rem gives shorter cards. **We need row heights that give ~25rem prospector cards.**

If total dashboard height = 56rem, padding = 2rem, circle = 22rem, prospectors need 25rem each:
- 2(prosp) + circle + 2(gaps) + 2(padding) = 25+25+22+2+2 = 76rem > 56rem. Impossible in 3 rows.

**Old layout didn't use a 3-row grid** — it likely used flex/absolute positioning so the prospectors could overlap with the circle row. 

---

## REVISED FINAL PLAN — Grid Template with 5 Rows

Use a 5-row grid where prospectors span 3 rows each, overlapping the circle row:

```
Row heights: 6rem, 1rem, 22rem, 1rem, 6rem  (rows 1,2 = top prosp, row3=circle, rows 4,5=bot prosp)
```

No — this becomes complex. 

**Simplest solution that matches the mockup**: Keep 3-row grid, but make the circle **not** tied to row dimensions. Instead use `grid-row: 1 / 4` (span all rows) on circle-wrap, and position it absolutely or with align-self + specific margins.

OR: **Increase row1 and row3 to 25.1rem** and accept taller prospector cards:
- Row1=25.1rem, Row2=22rem, Row3=25.1rem
- Total = 25.1+22+25.1+2gaps+2padding = 76.2rem > 56rem. Impossible.

**We cannot fit three rows of that total height in 56rem.**

**The mockup and the screenshot both show prospector cards that are roughly the same height as the circle, with the notch near the middle of the right edge.** This requires prospectors that are ~22rem tall, not 15rem.

### Final answer: Prospectors span rows 1+2 (top) and 2+3 (bottom)

```css
grid-template-areas:
  "prosp-top  config    right-col  cargo  limpets"
  "prosp-top  circle    right-col  cargo  limpets"
  "prosp-bot  target    right-col  cargo  limpets"
```
— but this puts config+circle+target in a single column with config in row1, circle in row2, target in row3. Prosp-top in rows 1+2, prosp-bot in rows 2+3.

With `grid-row: span 2`:
```css
.mining-panel__prospector-wrap--notch-br { grid-column: 1; grid-row: 1 / 3; }
.mining-panel__prospector-wrap--notch-tr { grid-column: 1; grid-row: 2 / 4; }
```

These overlap at row2! CSS grid items can overlap; stacking order controlled by z-index.

Actually no — two items at the same grid position simply overlap. The prospectors would visually overlap each other at row2. Not intended.

**The correct 3-row layout** must have the prospectors each fill either 1 or 2 rows, with no overlap. If each fills exactly 1.5 rows — impossible in a regular grid.

**Resolution: Use asymmetric row heights** so there are 4 rows:

```css
grid-template-rows: 11rem 11rem 22rem 11rem 11rem;  /* 5 rows — but that's 5 */
```

Or with `subgrid`, or with **explicit span + 4-row grid**:

```
Row1: 11rem (prosp-top upper half)
Row2: 11rem (prosp-top lower half + circle top = circle spans row2+row3)
Row3: 11rem (circle middle)
Row4: 11rem (circle bottom + prosp-bot upper)
Row5: 11rem (prosp-bot lower)
```

This is getting complex. Let's look at this pragmatically.

---

## PRAGMATIC IMPLEMENTATION PLAN

The simplest change that gets everything matching the mockup:

### Step 1: Make prosp cards span rows 1+2 and rows 2+3 in a 3-row grid (overlapping at row2 is fine — they're on left side, circle in col2)

```css
/* Prospector top: rows 1-2, col 1 */
.mining-panel__prospector-wrap--notch-br { grid-column: 1; grid-row: 1 / 3; }
/* Prospector bot: rows 2-3, col 1 */  
.mining-panel__prospector-wrap--notch-tr { grid-column: 1; grid-row: 2 / 4; }
```

These are in col1, config/circle/target are in col2 — **they don't overlap in the grid**. Within col1, prosp-top and prosp-bot both claim row2 in col1 — **grid allows overlap within the same column** but typically shows both stacked. We need them to split the column:

Use `align-self: start` on prosp-top and `align-self: end` on prosp-bot. They each take half the column height.

Actually the cleanest: **use a single wrapper** for both prospector cards.

```
"prosp-col   config  right-col  cargo  limpets"
"prosp-col   circle  right-col  cargo  limpets"
"prosp-col   target  right-col  cargo  limpets"
```

`.mining-panel__prosp-col` spans all 3 rows, is a flex column, contains the two prospector cards with `flex: 1` each. Each card = (52-1gap)/2 = 25.5rem tall. **This matches the old geometry closely (old cards were 25.1rem)!**

### Step 2: Right-col for Stats+Refined

Already in plan above. Single wrapper, flex column, each card `flex:1`.

### Step 3: Circle margin-left

With prosp-col right edge at x=31.5rem and circle in col2 (32.5-54.5rem):
- Natural circle centre x (justify-self:center) = 43.5rem
- Old working geometry had circle centre at local (38, 27) in prosp card coords
- Prosp card local origin = dashboard (1.5, 1). Local (38, 27) = dashboard (39.5, 28).
- Required circle centre x = 39.5rem. Natural = 43.5. margin-left = -4rem (restore original!)

With `margin-left: -4rem`, circle centre at dashboard x=39.5rem.
- prosp-col spans rows 1-3, total height = 52rem. Prosp card 1 = top 25.5rem (local y 0-25.5).
- Local centre in prosp1 = (39.5-1.5, 28-1) = (38, 27). ✓ This matches the old working geometry!

### Step 4: Notch geometry for right-col (Stats+Refined)

`.mining-panel__right-col` left edge at 55.5rem dashboard. Card height = each 25.5rem.
- Stats card: local y 0-25.5rem
- Refined card: local y 26.5-51.5rem (after 1rem gap)

Circle centre in stats-local: (39.5-55.5, 28-1) = (-16, 27). Circle is 16rem to the LEFT of the card.

For left edge (x=0) of stats card: Δx=16, (y-27)²=144-256 < 0. No intersection! Distance from circle centre to left edge = 16 > 12 = r. **No notch possible with current column widths.**

We need col3 (right-col) to be closer to the circle. Currently col3 starts at 55.5rem. Circle right edge = 39.5+11=50.5rem. Gap = 55.5-50.5=5rem. Need gap ≤ 0 for notch, or decrease col3 left edge.

**Reduce col2 width**: If col2 = 14rem (down from 22rem), col3 left edge = 1.5+30+1+14+1=47.5rem. Circle (justify-self:center in 14rem col) centre at 1.5+30+1+7=39.5rem (same!). margin-left=-4 → centre at 37.5rem. Hmm.

**The issue**: stats/refined (right-col) are too far right from the circle. In the mockup they appear directly adjacent to the circle's right edge.

**Reduce gap between circle and right-col**: 
- Circle right edge = cx+11 = 39.5+11 = 50.5rem
- For right-col left edge to touch circle right edge: col3 starts at 50.5+1gap = 51.5rem
- That means col1+col2+2gaps+padding = 1.5+30+1+col2+1 = 33.5+col2 = 51.5 → col2=18rem

**With col2=18rem**: circle in 18rem col, justify-self:center → centre at 1.5+30+1+9=41.5rem. margin-left=-4 → 39.5rem. Right edge = 50.5rem. right-col starts at 50.5+1=51.5rem ✓.

Actually the right-col notch just needs: col3 left ≤ cx+r. With cx=39.5, r=12: col3 left ≤ 51.5. Currently col3 left = 55.5 (col2=22). With col2=18: col3 left=51.5 ✓. With col2=16: col3 left=49.5, so circle overlaps 1rem into right-col ✓.

### Step 5: Config/Target notches

Config card (col2, row1): 22rem wide × 15rem tall.
Circle centre local in config: x = cx - col2_left = 39.5 - 32.5 = 7rem, y = 28 - 1 = 27rem (12rem below card bottom).

The arc is tangent to the card bottom at x=7rem. Same geometry problem as before.

**For visible bottom notch on config**: need arc centre closer. With col2=18rem, col2_left=32.5 (same), config width=18rem.
- Circle local in config: x=7rem, y=27rem (same — the gap is still 12rem = r)

The arc is still tangent to the bottom edge. To get a deeper notch, **use a larger arc radius** in the SVG without changing the clip-path circle radius. Or accept the tangent as a subtle notch.

**Alternatively**: Don't notch config/target on their edges — instead notch the circle wrap edges. Or accept the design without config/target notches and focus on prospector + right-col notches.

**From the mockup**: Config and Target appear to have gentle concave curves on their circle-facing edges (bottom of config, top of target). These are subtle. A tangent arc (r slightly > 12) would achieve this with 1 point of contact, or r=11 (going inside the card) for a visible notch.

Use **r=10rem** for config/target notch (shallower, shows a real cutout):
- Bottom edge (y=15): (x-7)²+(15-27)²=100 → (x-7)²=100-144 < 0. No intersection! r=10<12, still no intersection.

Use **r=13rem** (larger than 12):
- Bottom (y=15): (x-7)²+144=169 → (x-7)²=25 → x=7±5 → x=2 and x=12. Width of notch = 10rem.
- Depth: max depth at x=7: y_arc = 27-√(169-0)=27-13=14rem. Notch depth = 15-14 = 1rem.

**r=13 gives a 1rem deep notch, from x=2 to x=12 along the bottom of config card.** This is correct — visible concave notch. Use negative arc sweep to curve away from the card interior.

---

## IMPLEMENTATION SUMMARY

### CSS Changes

1. **Grid columns**: `30rem 18rem 18.5rem 1fr 6.5rem` (reduce col2 from 22 to 18rem to bring right-col closer to circle)

2. **Grid areas**: Introduce `prosp-col` and `right-col` spanning all 3 rows:
```css
grid-template-areas:
  "prosp-col  config   right-col  cargo  limpets"
  "prosp-col  circle   right-col  cargo  limpets"
  "prosp-col  target   right-col  cargo  limpets";
```

3. **New wrapper classes**:
```css
.mining-panel__prosp-col  { grid-area: prosp-col; display:flex; flex-direction:column; gap:1rem; }
.mining-panel__right-col  { grid-area: right-col; display:flex; flex-direction:column; gap:1rem; position:relative; }
```

4. **Circle**: `margin-left: -4rem` (original working value), `justify-self: center`

5. **Prospector cards** (inside prosp-col, each `flex:1`, ~25.5rem tall):
   - `notch-br` percentages for 30rem × 25.5rem card, circle at (38, 27) local:
     - Right edge (x=30): y=27±√(144-64)=27±8.94 → y=18.06 (70.8%) and y=35.94 (140.7%, outside)
     - Bottom edge (y=25.5): (x-38)²+(25.5-27)²=144 → x=38±√141.75=38±11.91 → x=26.09 (87%) and x=49.91 (outside)
   - clip-path for notch-br (right+bottom): cut from (100%, 70.8%) curving to (87%, 100%)
   - SVG arc: `M 30 0 L 30 18.06 A 12 12 0 0 0 26.09 25.5 L 0 25.5` (viewBox "0 0 30 25.5")

6. **right-col cards** (each `flex:1`, ~25.5rem tall), circle at dashboard (39.5, 28):
   - Stats card top at y=1rem. Local circle: x=39.5-55.5+... need new col3 left.
   - With col2=18: col3 left = 1.5+30+1+18+1=51.5rem. Local circle in stats: x=39.5-51.5=-12, y=28-1=27.
   - Arc at left edge (x=0): Δx=12, (y-27)²=144-144=0 → y=27. Tangent at (0,27) = 70.8% height. 1 point.
   - Need r>12: use r=13: (y-27)²=169-144=25 → y=27±5 → y=22 and y=32 (outside 25.5).
   - Intersection at top edge (y=0): (x+12)²+729=169... no. x=(-12±√(169-729)) < 0.
   - Arc at left edge (x=0), r=13: y=22 (86.3%) and y=32 (outside). 1 intersection on left edge.
   - Bottom edge (y=25.5): (x+12)²+(25.5-27)²=169 → (x+12)²=169-2.25=166.75 → x=-12±12.91 → x=0.91 and x=-24.91. x=0.91 is within card (width=18.5)!
   - So with r=13: left-edge y=22rem (86.3%), bottom-edge x=0.91rem (4.9%)

   Notch-bl for stats card (bottom-left corner):
   - clip-path: cut from (0%, 86.3%) to (4.9%, 100%)
   - SVG path: similar to prospector but mirrored

   Refined card top at y=1+25.5+1=27.5rem. Local circle: x=-12, y=28-27.5=0.5rem.
   - Top edge (y=0): (x+12)²+(0-0.5)²=169 → (x+12)²=168.75 → x=0.99rem (within 18.5). 
   - Left edge (x=0): (0+12)²+(y-0.5)²=169 → (y-0.5)²=25 → y=5.5 and y=-4.5(outside).
   - Notch-tl for refined card (top-left corner):
     - Top edge x=0.99rem (5.4%), left edge y=5.5rem (21.6%)

7. **Config notch** (width=18rem, height=15rem, circle local (7,27)):
   - r=13, bottom edge: x=2,12 → 11.1% and 66.7%, depth 1rem
   - Concave notch on bottom (notch-bottom): arc sweeps upward (A 13 13 0 0 1 ...)
   - SVG path: `M 0 15 L 2 15 A 13 13 0 0 1 12 15 L 18 15` — wait, this would bow UP (into card) only if sweep flag is correct.
   - For concave (bowl shape in bottom edge, arc bowing INTO card): use sweep-flag=0 for left-to-right upward arc.
   - Path: `M 0 15 L 2 15 A 13 13 0 0 0 12 15 L 18 15` — arc from (2,15) to (12,15) with r=13, sweep 0 → centre above → bows upward ✓

8. **Target notch**: mirror of config on top edge.
   - Arc: `M 0 0 L 2 0 A 13 13 0 0 1 12 0 L 18 0` — sweep 1 → centre below → bows downward (into card from top) ✓

---

## Config/Target Width

The mockup shows config/target taking up ~70% of the circle diameter. Circle diameter = 22rem (we're keeping 22rem circle even though col2=18rem — the circle uses `width:22rem` and overflows the col with negative margin).

70% of 22rem = 15.4rem. Config/target should be ~15rem wide, centred on the circle centre.

Circle centre at x=39.5rem. Config card occupies col2 (left=32.5, width=18rem). To centre a 15rem card on x=39.5: left = 39.5-7.5=32rem. Offset from col2 left = 32-32.5 = -0.5rem.

Easiest: give config/target `width: 15rem; align-self: center; justify-self: center` within their grid cells. Or `margin: 0 auto` with `width: 15rem`.

Actually: give them `max-width: 15.4rem` and `align-self: stretch; justify-self: center`.

---

## Action Items (Implementation)

### CSS (`mining-panel.css`)
- [ ] Change `grid-template-columns` to `30rem 18rem 18.5rem 1fr 6.5rem`
- [ ] Change `grid-template-areas` to use `prosp-col` and `right-col`
- [ ] Remove `prosp-top`/`prosp-bot` grid area assignments
- [ ] Add `.mining-panel__prosp-col` and `.mining-panel__right-col` flex containers
- [ ] Update `circle-wrap` to `margin-left: -4rem`
- [ ] Update prospector `clip-path` polygons for 30rem × 25.5rem (recalculated percentages)
- [ ] Update prospector SVG viewBox to `"0 0 30 25.5"`
- [ ] Add `notch-bl` clip-path for stats card (bottom-left corner, r=13)
- [ ] Add `notch-tl` clip-path for refined card (top-left corner, r=13)  
- [ ] Fix `notch-bottom` on config (x=2 to 12, r=13, correct sweep)
- [ ] Fix `notch-top` on target (mirror)
- [ ] Config/target `max-width: 15.4rem; align-self: center` (centred in their cell)

### JSX (`mining-status/index.js`)
- [ ] Wrap both prospector cards in `<div className='mining-panel__prosp-col'>`
- [ ] Wrap Stats and Refined cards in `<div className='mining-panel__right-col'>`
- [ ] Remove separate `mining-panel__info-card-wrap--stats` and `--refined` grid items
- [ ] Update prospector SVG arc paths with new coordinates
- [ ] Add SVG arcs to stats (notch-bl) and refined (notch-tl) inside right-col
- [ ] Fix config SVG arc path and sweep direction
- [ ] Fix target SVG arc path and sweep direction

---

## Exact Clip-Path Polygons (Final)

All percentages based on final card dimensions.

### Prospector cards (30rem × 25.5rem)
```
notch-br (right+bottom):
  Circle local centre (38, 27). r=12.
  Right edge y-entry: 18.06rem = 70.8%
  Bottom edge x-exit: 26.09rem = 86.97%

  polygon(
    0% 0%, 100% 0%, 100% 70.8%,
    [arc points...],
    86.97% 100%, 0% 100%
  )
  
  Arc points (~7 segments between (100%, 70.8%) and (86.97%, 100%)):
  Right edge at 70.8%, then curving to bottom at 87%.
  Angle at pt1: atan2(27-18.06, 38-30) = atan2(8.94, 8) = 48.2° (from positive x)
  Angle at pt2: atan2(27-25.5, 38-26.09) = atan2(1.5, 11.91) = 7.2°
  Sweep: 48.2° to 7.2° = ~41° in 7 steps of ~5.9°
  
  θ: 48.2°, 42.3°, 36.4°, 30.5°, 24.6°, 18.7°, 12.8°, 7.2° (8 points including endpoints)
  x(θ) = 38 - 12*cos(θ); y(θ) = 27 - 12*sin(θ)
  As %: x/30, y/25.5

  θ=48.2°: x=38-12*0.665=38-7.98=30.02/30=100.07%, y=27-12*0.747=27-8.96=18.04/25.5=70.75%
  θ=42.3°: x=38-12*0.740=38-8.88=29.12/30=97.1%,   y=27-12*0.673=27-8.07=18.93/25.5=74.2%
  θ=36.4°: x=38-12*0.805=38-9.66=28.34/30=94.5%,   y=27-12*0.593=27-7.12=19.88/25.5=78.0%
  θ=30.5°: x=38-12*0.861=38-10.33=27.67/30=92.2%,  y=27-12*0.509=27-6.11=20.89/25.5=81.9%
  θ=24.6°: x=38-12*0.909=38-10.91=27.09/30=90.3%,  y=27-12*0.416=27-4.99=22.01/25.5=86.3%
  θ=18.7°: x=38-12*0.947=38-11.36=26.64/30=88.8%,  y=27-12*0.320=27-3.84=23.16/25.5=90.8%
  θ=12.8°: x=38-12*0.975=38-11.70=26.30/30=87.7%,  y=27-12*0.222=27-2.66=24.34/25.5=95.4%
  θ=7.2°:  x=38-12*0.992=38-11.90=26.10/30=87.0%,  y=27-12*0.125=27-1.50=25.50/25.5=100%
```

### Stats card (18.5rem × 25.5rem)
```
notch-bl (left+bottom corner):
  Circle local centre (-12, 27). r=13.
  Left edge y-entry: 22rem = 86.3%
  Bottom edge x-exit: 0.91rem = 4.9%

  Angles:
  Pt1 (0, 22): atan2(27-22, -12-0) = atan2(5, -12) = 180°-22.6° = 157.4° (from +x)
  Pt2 (0.91, 25.5): atan2(27-25.5, -12-0.91) = atan2(1.5, -12.91) = 180°-6.6° = 173.4°

  Arc points from 157.4° to 173.4° in 5 steps of ~4°:
  x(θ) = -12 - 13*cos(θ); y(θ) = 27 - 13*sin(θ)
  As %: x/18.5, y/25.5

  θ=157.4°: x=-12-13*(-0.921)=-12+11.97=-0.03≈0, y=27-13*0.383=27-4.98=22.02 → (0%, 86.4%)
  θ=161.4°: x=-12-13*(-0.947)=-12+12.31=0.31, y=27-13*0.321=27-4.17=22.83 → (1.7%, 89.5%)
  θ=165.4°: x=-12-13*(-0.965)=-12+12.55=0.55, y=27-13*0.255=27-3.32=23.68 → (3.0%, 92.9%)
  θ=169.4°: x=-12-13*(-0.983)=-12+12.78=0.78, y=27-13*0.169=27-2.19=24.81 → (4.2%, 97.3%)
  θ=173.4°: x=-12-13*(-0.993)=-12+12.91=0.91, y=27-13*0.114=27-1.48=25.52 → (4.9%,100.1%)

  polygon(
    0% 0%, 100% 0%, 100% 100%,
    4.9% 100%, 4.2% 97.3%, 3.0% 92.9%, 1.7% 89.5%, 0% 86.4%
  )
  border-left: none; border-bottom: none;
```

### Refined card (18.5rem × 25.5rem)
```
notch-tl (left+top corner):
  Circle local centre (-12, 0.5). r=13.
  Top edge x-exit: 0.99rem = 5.3%
  Left edge y-entry: 5.5rem = 21.6%

  Symmetric mirror of stats notch (essentially same geometry, different y-origin).
  
  Angles:
  Pt1 (0.99, 0): atan2(0.5-0, -12-0.99) = atan2(0.5, -12.99) = 180°-2.2° = 177.8°
  Pt2 (0, 5.5): atan2(0.5-5.5, -12-0) = atan2(-5, -12) = 180°+22.6° = 202.6° → wraps below x-axis

  Use simpler mirror: same polygon as stats but flipped vertically.
  polygon(
    0% 21.6%, 1.7% 10.5%, 3.0% 7.1%, 4.2% 2.7%, 5.3% 0%,
    100% 0%, 100% 100%, 0% 100%
  )
  border-left: none; border-top: none;
```

### Config card (18rem × 15rem) — notch-bottom
```
  Circle local centre (7, 27). r=13.
  Bottom edge: x=2 to x=12, depth 1rem.
  
  polygon(
    0% 0%, 100% 0%, 100% 100%,
    66.7% 100%, 63.3% 97.3%, 58.9% 95.0%, 54.4% 93.3%,
    50.0% 93.3%, 44.4% 93.3%, 40.0% 95.0%, 36.7% 97.3%,
    11.1% 100%, 0% 100%
  )
  
  Exact arc points: circle (7,27), r=13, bottom edge y=15 → x=7±5 → 2 to 12.
  At x=7 (mid): y=27-13=14rem (depth=1rem from bottom=15: 14/15=93.3%)
  Points along arc (x=2..12):
  x(θ)=7-13cos(θ), y(θ)=27-13sin(θ), need y<15:
  x=2: cos(θ)=5/13, θ=67.4°, y=27-13*0.923=27-12=15=edge (endpoint)
  x=7: cos(θ)=0, θ=90°, y=27-13=14 → 93.3%
  x=12: cos(θ)=-5/13, θ=112.6°, y=15 (endpoint)

  Mid-arc points at 5 steps:
  θ=67.4°,75.4°,83.4°,91.4°,99.4°,107.4°,112.6°
  x= 7-13cos(θ): 2.0, 3.6, 5.4, 7.2, 9.1, 10.7, 12.0
  y= 27-13sin(θ): 15.0, 14.4, 14.1, 14.0, 14.1, 14.4, 15.0
  As %: x/18, y/15

  polygon(
    0% 0%, 100% 0%, 100% 100%,
    66.7% 100%,  [arc...]  11.1% 100%, 0% 100%
  )
  Intermediate points (x%, y%):
  (3.6/18=20%,  14.4/15=96.0%)
  (5.4/18=30%,  14.1/15=94.0%)
  (7.2/18=40%,  14.0/15=93.3%)  ← deepest
  (9.1/18=50.5%, 14.1/15=94.0%)
  (10.7/18=59.4%, 14.4/15=96.0%)

  Full polygon:
  polygon(
    0% 0%, 100% 0%, 100% 100%,
    66.7% 100%, 59.4% 96.0%, 50.5% 94.0%, 40.0% 93.3%,
    30.0% 94.0%, 20.0% 96.0%, 11.1% 100%,
    0% 100%
  )
```

### Target card (18rem × 15rem) — notch-top (mirror of config)
```
  polygon(
    0% 0%, 11.1% 0%, 20.0% 4.0%, 30.0% 6.0%, 40.0% 6.7%,
    50.5% 6.0%, 59.4% 4.0%, 66.7% 0%,
    100% 0%, 100% 100%, 0% 100%
  )
```

---

## SVG Arc Paths

All use `vector-effect: non-scaling-stroke; stroke-width: 1`.

### Prospector notch-br (viewBox "0 0 30 25.5")
```
M 30 0 L 30 18.06 A 12 12 0 0 0 26.09 25.5 L 0 25.5
```
Sweep flag 0 = counterclockwise = concave notch bowing left/up ✓

### Prospector notch-tr (viewBox "0 0 30 25.5")
```
M 0 0 L 26.09 0 A 12 12 0 0 0 30 7.44 L 30 25.5
```
(7.44 = 25.5-18.06)

### Stats notch-bl (viewBox "0 0 18.5 25.5")
```
M 0 0 L 0 22.02 A 13 13 0 0 0 0.91 25.5 L 18.5 25.5
```
Sweep 0 = concave notch curving left ✓

### Refined notch-tl (viewBox "0 0 18.5 25.5")
```
M 18.5 0 L 0.99 0 A 13 13 0 0 0 0 5.5 L 0 25.5
```
Sweep 0 = concave notch curving left ✓

### Config notch-bottom (viewBox "0 0 18 15")
```
M 0 15 L 2 15 A 13 13 0 0 0 12 15 L 18 15
```
Sweep 0 = arc bows upward (away from bottom, into card) = concave ✓

### Target notch-top (viewBox "0 0 18 15")
```
M 0 0 L 6 0 A 13 13 0 0 1 12 0 L 18 0
```
Sweep 1 = arc bows downward (into card from top) = concave ✓

---

## Summary Table

| Card | Notch side | r | clip-path class | SVG viewBox |
|------|-----------|---|-----------------|-------------|
| Prospector top | bottom-right | 12 | `notch-br` (updated %) | `0 0 30 25.5` |
| Prospector bot | top-right | 12 | `notch-tr` (updated %) | `0 0 30 25.5` |
| Config | bottom | 13 | `notch-bottom` (new) | `0 0 18 15` |
| Target | top | 13 | `notch-top` (new) | `0 0 18 15` |
| Stats | bottom-left | 13 | `notch-bl` (new) | `0 0 18.5 25.5` |
| Refined | top-left | 13 | `notch-tl` (new) | `0 0 18.5 25.5` |
