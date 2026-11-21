const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
test('close-zoom symbols represent occupied L7 bin centres with unchanged RGBA and bounded output', () => {
  const h = harness('go-atlas-canvas')
  try {
    const rgba = new Uint8ClampedArray(4 * 4 * 4); rgba.set([40, 180, 150, 210], (1 * 4 + 2) * 4)
    const bins = h.exports.occupiedBins(rgba, 4, 4, { sx: 1, sy: 0, sw: 2, sh: 4, width: 100, height: 200 }, 12000)
    assert.equal(bins.length, 1); assert.deepEqual(bins[0], { x: 75, y: 75, color: [40, 180, 150, 210] })
    assert.equal(h.exports.occupiedBins(rgba, 4, 4, { sx: 0, sy: 0, sw: 4, sh: 4, width: 100, height: 100 }, 0), null)
    assert.equal(h.exports.useBinSymbols({ level: 7, sourceLevel: 4 }, 10), false)
    assert.equal(h.exports.useBinSymbols({ level: 7, sourceLevel: 7 }, 2), true)
    assert.equal(h.exports.useBinSymbols({ level: 7, sourceLevel: 7 }, 1), false)
    assert.equal(h.exports.useBinSymbols({ level: 7, sourceLevel: 7 }, 8, 1), false)
    const dense = new Uint8ClampedArray(256 * 256 * 4).fill(255)
    assert.equal(h.exports.occupiedBins(dense, 256, 256, { sx: 0, sy: 0, sw: 256, sh: 256, width: 1024, height: 1024 }), null)
  } finally { h.unmount() }
})
test('route visibility preserves coordinates and markers, using thin constant-pixel strokes', () => {
  const h = harness('go-galaxy', null, 'Projection')
  h.props.axes = 'xz'; h.props.frame = { x: [-10, 10], y: [-10, 10], z: [-10, 10] }; h.props.current = { name: 'Fixture', coordinates: [0, 0, 0] }; h.props.route = [h.props.current, { coordinates: [5, 0, 5] }]
  try {
    const before = h.html(); assert.match(before, /data-marker="waypoint" data-axes="xz" data-position="0.5,0.5"/); assert.match(before, /go-galaxy-route/); assert.match(before, /non-scaling-stroke/); assert.match(before, /data-position="0.5,0.5"/)
    h.props.showRoute = false; const hidden = h.html(); assert.doesNotMatch(hidden, /go-galaxy-route/); assert.match(hidden, /data-marker="current"/); assert.match(hidden, /data-position="0.5,0.5"/)
  } finally { h.unmount() }
})
test('LOD diagnostics distinguish verified full detail, fallback and missing tiles', () => {
  const h = harness('go-atlas')
  try {
    assert.match(h.exports.atlasLODLabel({ tiles: [{ level: 7, sourceLevel: 4 }], missing: 2, pending: true }), /Requested L7 · Displayed L4 fallback.*2 pending/)
    assert.match(h.exports.atlasLODLabel({ tiles: [{ level: 7, sourceLevel: 7 }], missing: 0 }), /Requested L7 · Displayed L7/)
    assert.match(h.exports.atlasLODLabel({ tiles: [], requestedLevels: { xz: 7 }, missing: 3 }), /Requested L7 · Displayed none.*3 unavailable/)
  } finally { h.unmount() }
})
