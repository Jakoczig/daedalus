const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
test('local volume uses exact XYZ, synchronized projections and bounded drawing', () => {
  const h = harness('go-spatial', null, 'SpatialSettings')
  try {
    const f = h.exports.spatialFrame([0, 0, 0], 200, 100)
    assert.deepEqual(f, { x: [-100, 100], y: [-50, 50], z: [-100, 100] })
    assert.deepEqual(h.exports.spatialFrame([20, 30, 40], 400, 50, 100), { x: [-180, 220], y: [5, 55], z: [-10, 90] })
    const p = [-0.03125, -0.0625, -0.125]
    assert.deepEqual(h.exports.spatialPixel(p, 'xz', f, 200, 200), [99, 100])
    assert.deepEqual(h.exports.spatialPixel(p, 'xy', f, 200, 100), [99, 50])
    assert.deepEqual(h.exports.spatialPixel(p, 'yz', f, 100, 200), [49, 100])
    assert.equal(h.exports.spatialPixel([200, 0, 0], 'xz', f, 200, 200), null)
    assert.deepEqual(h.exports.spatialSegments([{ coordinates: [-200, 0, 0] }, { coordinates: [200, 0, 0] }], f), [[[-100, 0, 0], [100, 0, 0]]])
    assert.deepEqual(h.exports.spatialSegments([{ coordinates: [-200, 200, 0] }, { coordinates: [200, 200, 0] }], f), [])
    assert.doesNotMatch(h.html(), /Permit selective spatial downloads/)
  } finally { h.unmount() }
})
test('local side panels preserve world aspect and round navigation markers', () => {
  const h = harness('go-galaxy', null, 'Projection')
  try {
    Object.assign(h.props, { axes: 'yz', localGeometry: true, frame: { x: [-100, 100], y: [-50, 50], z: [-100, 100] } })
    assert.match(h.html(), /viewBox="0 0 50 100"/)
    h.props.frame.x = [-200, 200]
    assert.match(h.html(), /viewBox="0 0 25 50"/)
  } finally { h.unmount() }
})
