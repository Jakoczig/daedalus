const test = require('node:test')
const assert = require('node:assert/strict')
const { harness, exploration } = require('../helpers/go-panel-harness.cjs')
const body = (id, kind, parent) => ({ bodyId: id, name: 'Fixture ' + id, details: { isStar: kind === 'Star', parents: parent ? [parent] : [] } })
test('G2H orbital topology has horizontal planets, vertical moons, barycentre groups and no inferred links', () => {
  const { chartScene } = harness('go-terminal-chart').exports
  const b = [body('0', 'Star'), body('1', 'Planet', { kind: 'Star', bodyId: '0' }), body('2', 'Planet', { kind: 'Star', bodyId: '0' }), body('3', 'Planet', { kind: 'Planet', bodyId: '1' }), body('4', 'Planet', { kind: 'Planet', bodyId: '1' }), body('5', 'Star', { kind: 'Null', bodyId: '9' }), body('6', 'Planet', { kind: 'Null', bodyId: '9' }), body('7', 'Planet', { kind: 'Planet', bodyId: '999' })]
  const s = chartScene(b); const n = id => s.nodes.find(n => n.body.bodyId === id)
  assert.equal(n('1').y, n('0').y); assert.equal(n('2').y, n('0').y); assert.ok(n('2').x > n('1').x)
  assert.equal(n('3').x, n('1').x); assert.ok(n('4').y > n('3').y)
  assert.equal(n('5').parent.body.bodyId, n('6').parent.body.bodyId)
  assert.equal(n('7').parent, null); assert.equal(n('7').unresolved, true)
  assert.equal(new Set(s.nodes.map(n => n.x + ',' + n.y)).size, s.nodes.length)
  const filtered = chartScene([b[3]], b); assert.equal(filtered.nodes[0].x, n('3').x)
})
test('G2H Navigation removes body strip, organism details and unknown facts while retaining Exploration', async () => {
  const value = { ...exploration, bodies: [{ ...exploration.bodies[0], organisms: [{ species: 'Fixture species', samples: 1 }], details: { radiusKM: 1000 } }] }
  const h = harness('go-terminal-survey', value); h.props.path = '/nav/map'; h.html(); const html = await h.flush()
  assert.doesNotMatch(html, /go-body-strip|Biological observations|Fixture species|Pressure|Atmosphere not reported/)
  assert.match(html, /Radius/)
  h.props.path = '/exploration/system'; assert.match(h.html(), /Biological observations|Fixture species/)
})

test('G2H statistics never substitute base power for unknown engineered values', () => {
  const { powerStatistics } = harness('go-ship-statistics').exports
  const n = value => ({ label: 'PowerDraw', value })
  const modules = [{ slot: 'PowerPlant', item: 'plant', on: true, engineering: { modifiers: [{ label: 'PowerCapacity', value: 20 }] } }, { slot: 'MainEngines', on: true, priority: 0, reference: { powerMW: 5 }, engineering: { modifiers: [] } }]
  assert.equal(powerStatistics(modules).deployed, null)
  modules[1].engineering.modifiers = [n(6)]
  let v = powerStatistics(modules); assert.equal(v.deployed, 6); assert.equal(v.percent, 30)
  modules.push({ slot: 'LargeHardpoint1', on: true, priority: 2, reference: { powerMW: 3 } })
  v = powerStatistics(modules); assert.equal(v.stowed, 6); assert.equal(v.deployed, 9); assert.equal(v.priorities[2], 3)
  modules[1].engineering.modifiers = [n(NaN)]; assert.equal(powerStatistics(modules).deployed, null)
})

test('G2H Galaxy route framing handles short/coincident routes and missing destination without NaNs', () => {
  const { routeFrame, zoomFrame, panFrame } = harness('go-galaxy').exports
  const full = { x: [-65000, 65000], y: [-15000, 15000], z: [-35000, 95000] }
  const a = { coordinates: [0, 0, 0] }; const b = { coordinates: [12, 3, -10] }
  const f = routeFrame(a, b, full); for (const axis of ['x', 'y', 'z']) { assert.ok(f[axis][1] - f[axis][0] >= 100); assert.ok(f[axis].every(Number.isFinite)) }
  assert.deepEqual(routeFrame(a, null, full), full)
  assert.ok(zoomFrame(f, 0.5).x[1] - zoomFrame(f, 0.5).x[0] >= 10)
  assert.notDeepEqual(panFrame(f, 'xz', 0.2, 0), f)
})

test('G2H statistics render fuel and cargo pairs without formatting them as NaN', () => {
  const h = harness('go-ship-statistics'); h.props.workspace = { parity: { ship: { fuelMainT: 16, fuelCapacityT: 32, cargoCapacityT: 64 }, cargo: { count: 4 }, modules: [] } }
  const html = h.html(); assert.doesNotMatch(html, /NaN/); assert.match(html, /16 \/ 32 T/); assert.match(html, /4 \/ 64 T/)
})
