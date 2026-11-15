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

test('G2H waiting journal uses a focused empty state, not an empty orbital map', async () => {
  const h = harness('go-terminal-survey', { systemName: '', surveyState: 'waiting_game', bodies: [] })
  h.props.path = '/nav/map'
  h.html()
  const html = await h.flush()
  assert.match(html, /Current system not available yet/)
  assert.doesNotMatch(html, /go-chart__viewport|No matching observations|Find observed body/)
})

test('G2H community stations are visible as selectable system-level map markers', () => {
  const h = harness('go-terminal-chart')
  const star = body('0', 'Star')
  Object.assign(h.props, { bodies: [star], contextBodies: [star], systemKey: 'Fixture Alpha', onSelect () {}, onSelectStation () {}, stations: [{ name: 'Garratt Ring', type: 'Coriolis Starport', distanceLS: 977 }, { name: 'Vinge Silo', type: 'Planetary settlement', distanceLS: 978 }] })
  const html = h.html()
  assert.match(html, /Community station markers on system map/)
  assert.match(html, /STATIONS · 2 EDSM/)
  assert.match(html, /Garratt Ring|Vinge Silo/)
  assert.match(html, /orbital positions not available/)
  assert.match(html, /aria-pressed="false"/)
})

test('G2H valid empty system differs from filtered observations and journal startup', async () => {
  const h = harness('go-terminal-survey', { systemName: 'Empty Fixture', surveyState: 'observed', bodies: [] })
  h.props.path = '/nav/map'; h.html(); const html = await h.flush()
  assert.match(html, /No body observations available/)
  assert.doesNotMatch(html, /Awaiting local journal|No matching observations|Selectable star and body map|Find observed body/)
  h.broadcast(exploration, 'goExplorationChanged')
  assert.match(h.html(), /Selectable star and body map/)
  h.elements('input').find(e => e.props['aria-label'] === 'Find observed body or genus').props.onChange({ target: { value: 'no-such-fixture' } })
  assert.match(h.html(), /No matching observations/)
  assert.doesNotMatch(h.html(), /No body observations available/)
  h.broadcast({ systemName: '', bodies: [] }, 'goExplorationChanged')
  assert.match(h.html(), /Current system not available yet/)
  h.unmount()
})

test('G2H historical requests never display a previously requested system while loading', async () => {
  const h = harness('go-terminal-survey', name => name === 'getGoSystem' ? { local: { systemName: 'History Beta', bodies: [] }, availability: 'retained' } : exploration)
  Object.assign(h.props, { path: '/nav/map', system: 'History Beta' })
  h.html(); await h.flush(); const loaded = await h.flush()
  assert.match(loaded, /History Beta/)
  h.props.system = 'History Gamma'
  assert.doesNotMatch(h.html(), /History Beta/)
  assert.match(h.html(), /History Gamma/)
  h.unmount()
})

test('G2H station inspector retains identity across cache reordering and exposes freshness', async () => {
  const a = { name: 'Fixture Station A', type: 'Coriolis Starport', distanceLS: 50, updatedAt: '2026-01-01' }
  const b = { name: 'Fixture Station B', type: 'Outpost', distanceLS: null }
  let community = { enabled: true, status: 'cached', systemName: exploration.systemName, stations: [a, b], stationsFetchedAt: '2026-11-15', expiresAt: '2026-11-16' }
  const h = harness('go-terminal-survey', name => name === 'getGoCommunity' ? community : exploration)
  Object.assign(h.props, { path: '/nav/map', stage: 'G2E' })
  h.html(); await h.flush(); await h.flush()
  const select = () => h.elements('button').find(e => e.props.children?.[0]?.props?.children === a.name).props.onClick()
  select(); let html = h.html(); assert.match(html, /Station inspector/); assert.match(html, /Stations fetched: 2026-11-15/)
  community = { ...community, status: 'stale', stations: [b, a] }; h.props.communityVersion = 1
  await h.flush(); html = await h.flush()
  assert.match(html, /<h2 class="text-info">Fixture Station A/)
  assert.match(html, /STALE CACHE/)
  community = { ...community, enabled: false }; h.props.communityVersion = 2
  await h.flush(); html = await h.flush(); assert.doesNotMatch(html, /Station inspector|Community station markers/)
  h.unmount()
})
