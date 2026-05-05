# Mining Dashboard Overlap Layout Plan

## Problem Summary

The mining dashboard currently has only the two left prospector cards using the circle-overlap notch treatment.

The next requested change is to extend that same visual language so more panels appear to overlap the center mining circle and be cut away by it.

Based on the mockup, the intended end state is:

- Left side keeps the existing two large prospector panels with circle cut-outs.
- A small top-center box also overlaps and cuts into the circle.
- A small bottom-center box also overlaps and cuts into the circle.
- The Statistics panel on the upper-right side overlaps and cuts into the circle.
- The Refined panel on the lower-right side overlaps and cuts into the circle.
- The center circle remains the visual anchor and all adjacent boxes should feel aligned to its edge.

## Current Known State

- The left prospector cards already use a working notch solution.
- That solution is implemented with `clip-path` on the card plus an SVG overlay path for the missing border edge and arc.
- The dashboard uses a fixed design surface of `110rem x 56rem`.
- The circle is already positioned between the left and center areas using overlap and negative margin.
- Statistics and Refined currently render as ordinary rectangular cards in the center column.
- There is not yet a dedicated top small box or bottom small box near the circle.

## Main Risk

The previous attempts timed out because too much of the layout was being redesigned in one pass.

The safest approach is to reuse the existing notch technique and introduce one overlap region at a time, validating after each step.

## Visual Interpretation Of The Mockup

The mockup suggests five separate notched panels around the circle:

1. Top-left large panel with a bottom-right notch.
2. Bottom-left large panel with a top-right notch.
3. Top small panel above the circle with a bottom-left or bottom-center notch behavior depending on final alignment.
4. Top-right Statistics panel with a bottom-left notch.
5. Bottom-right Refined panel with a top-left notch.

The small bottom-center box likely acts as a configuration or auxiliary information panel and should visually mirror the small top box.

## Implementation Strategy

### Phase 1 ✅ DONE

Add `margin-right: -4rem` to `.mining-panel__circle-wrap` so the circle overlaps into the right-side panels, mirroring the existing left-side overlap.
Add `position: relative; z-index: 1` to `.mining-panel__center-col` so the circle (z-index: 2) renders on top.

Result: Statistics and Refined cards are now visually behind/under the circle's right edge by 4rem. No notch geometry yet — just layout overlap established.

### Phase 2 ✅ DONE (combined with Phases 3-5 into single implementation pass)

See geometry calculations and implementation plan below.

---

## Geometry Reference (all values computed, ready to implement)

### Dashboard dimensions

- Total: 110rem × 56rem. Padding: 1rem top/bottom, 1.5rem left/right.
- Inner content area: 107rem wide × 54rem tall.
- Circle: 22rem diameter, margin-left: -4rem, margin-right: -4rem → 14rem effective flex width, 3rem visual overlap on each side.
- Circle centre in inner area: y = 27rem (vertically centred). x = 39 + W_cl from left inner edge (where W_cl = centre-left col width ≈ 18.5rem).

### New column structure

```
[left-col:30rem] [gap:1] [centre-left:flex:1≈18.5rem] [gap:1] [circle:14rem-eff] [gap:1] [centre-right:18.5rem] [gap:1] [cargo:fit-content≈14.5rem] [gap:1] [limpets:6.5rem]
```

Budget check: 30+1+18.5+1+14+1+18.5+1+14.5+1+6.5 = 107rem ✓

### Card dimensions

- centre-left and centre-right: 18.5rem wide. Height: (54 − 0.5rem gap) / 2 = 26.75rem each.
- Two cards per column, 0.5rem gap.

### Circle centre in card-local coordinates

Distance from right edge of left-side columns to circle centre = 8rem (constant regardless of col width, because gap=1 + margin=-4 + radius=11 = 8).

- centre-left top card: cx = 18.5 + 8 = 26.5rem right of card left edge; cy = 27rem
- centre-left bottom card: cx = 26.5rem; cy = 27 − 26.75 − 0.5 = −0.25rem
- centre-right top card: cx = −8rem left of card left edge; cy = 27rem
- centre-right bottom card: cx = −8rem; cy = −0.25rem

### Arc intersections (r = 12rem, both sides)

For top cards (cy = 27rem):
- Vertical edge at y = 27 − √(144−64) = 18.056rem (67.5% of 26.75rem)
- Horizontal edge at x = cx − √(144−0.063) = cx ± 11.997rem
  - CL right edge x: 26.5 − 12.0 = 14.5rem = 78.4% of 18.5rem
  - CR left edge x: −8 + 12.0 = 4.0rem = 21.6% of 18.5rem

For bottom cards (cy = −0.25rem):
- Vertical edge at y = −0.25 + √80 = 8.694rem (32.5% of 26.75rem)
- Horizontal edge: same x as top (14.5rem CL, 4.0rem CR)

### Arc polygon points (8 points from edge-intercept to corner-intercept)

y% progression for top-card notches (right/left edge → bottom edge):
67.5%, 71.2%, 75.4%, 79.8%, 84.4%, 89.2%, 94.6%, 100%

y% progression for bottom-card notches (top edge → right/left edge):
0%, 5.4%, 10.8%, 15.6%, 20.2%, 24.6%, 28.8%, 32.5%

x% for CL (100% → 78.4%): 100%, 94.7%, 89.9%, 86.1%, 82.9%, 80.6%, 79.0%, 78.4%
x% for CR (0% → 21.6%): 0%, 5.4%, 10.1%, 13.9%, 17.1%, 19.4%, 21.0%, 21.6%

### CSS clip-path polygons

**CL notch-br** (top card, right-side cut):
```
0% 0%, 100% 0%, 100% 67.5%,
94.7% 71.2%, 89.9% 75.4%, 86.1% 79.8%, 82.9% 84.4%, 80.6% 89.2%, 79.0% 94.6%, 78.4% 100%,
0% 100%
```
border-right: none; border-bottom: none

**CL notch-tr** (bottom card, right-side cut):
```
0% 0%, 78.4% 0%,
79.0% 5.4%, 80.6% 10.8%, 82.9% 15.6%, 86.1% 20.2%, 89.9% 24.6%, 94.7% 28.8%, 100% 32.5%,
100% 100%, 0% 100%
```
border-right: none; border-top: none

**CR notch-bl** (top card, left-side cut):
```
0% 0%, 100% 0%, 100% 100%,
21.6% 100%, 21.0% 94.6%, 19.4% 89.2%, 17.1% 84.4%, 13.9% 79.8%, 10.1% 75.4%, 5.4% 71.2%, 0% 67.5%
```
border-left: none; border-bottom: none

**CR notch-tl** (bottom card, left-side cut):
```
21.6% 0%, 21.0% 5.4%, 19.4% 10.8%, 17.1% 15.6%, 13.9% 20.2%, 10.1% 24.6%, 5.4% 28.8%, 0% 32.5%,
0% 100%, 100% 100%, 100% 0%
```
border-left: none; border-top: none

### SVG arc border paths (viewBox "0 0 18.5 26.75", preserveAspectRatio="none")

```
CL-notch-br: M 18.5 0 L 18.5 18.056 A 12 12 0 0 0 14.503 26.75 L 0 26.75
CL-notch-tr: M 0 0 L 14.503 0 A 12 12 0 0 0 18.5 8.694 L 18.5 26.75
CR-notch-bl: M 0 0 L 0 18.056 A 12 12 0 0 1 4.0 26.75 L 18.5 26.75
CR-notch-tl: M 0 26.75 L 0 8.694 A 12 12 0 0 1 4.0 0 L 18.5 0
```

CL uses sweep=0 (same as existing left-col cards). CR uses sweep=1 (mirrored).

---

## File changes needed

### CSS (`mining-panel.css`)

1. Rename `.mining-panel__center-col` → `.mining-panel__center-right-col`, change `flex:1` to `width:18.5rem; flex-shrink:0`.
2. Add `.mining-panel__center-left-col` (same width, `flex:1` so it absorbs cargo-sizing variance).
3. Add `.mining-panel__info-card-wrap` (position:relative, flex:1, min-height:0, flex-direction:column).
4. Add `.mining-panel__info-card-arc` and its path stroke rule (same as prospector-arc).
5. Add four `mining-panel__info-card--notch-*` clip-path classes.

### JS (`mining-status/index.js`)

1. Modify `MiningSessionCard` to accept and pass through `className` prop.
2. Modify `MiningRefinedCard` to accept and pass through `className` prop.
3. In dashboard JSX:
   - Add `mining-panel__center-left-col` div before circle with two placeholder cards + SVG arcs.
   - Rename `mining-panel__center-col` → `mining-panel__center-right-col`.
   - Wrap statistics card in `info-card-wrap`, add SVG arc, pass `mining-panel__info-card--notch-bl` className.
   - Wrap refined card in `info-card-wrap`, add SVG arc, pass `mining-panel__info-card--notch-tl` className.

## Concrete File Targets

- `src/client/components/panels/mining/mining-status/index.js`
- `src/client/css/panels/mining-panel.css`

## First Small Change To Make

The first implementation change should be layout-only.

That means:

- No new visual notch geometry yet for every panel.
- Only introduce the structural wrappers needed to place the future top, right, and bottom boxes around the circle.
- Validate that the dashboard still builds before adding any more arcs.

## Working Rule

From here, changes should be made one part at a time:

1. Layout structure.
2. Upper-right notch.
3. Lower-right notch.
4. Small top box.
5. Small bottom box.
6. Final visual tuning.