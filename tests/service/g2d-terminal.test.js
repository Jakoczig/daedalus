const test = require('node:test')
const assert = require('node:assert/strict')
const { harness, exploration, navigation } = require('../helpers/go-panel-harness.cjs')
const facts = { ...exploration, bodies: [{ ...exploration.bodies[0], details: { radiusKM: 1000, temperatureK: 250, isStar: false, parents: [{ kind: 'Star', bodyId: '0' }] }, organisms: [{ genus: 'Bacterium', species: 'Fixture organism', stage: 'sampled', samples: 2, complete: false }] }, { bodyId: '0', name: 'Fixture Alpha', kind: 'G', details: { isStar: true, radiusKM: 696340 }, genuses: [] }] }
const ship = { shipType: 'asp', evidence: 'journal_last_known', fuelMainT: 16, fuelCapacityT: 32, hullHealth: 0.82, maxJumpRangeLY: 45, cargoCapacityT: 64 }
const fixture = name => name === 'getGoShip' ? ship : name === 'getGoNavigation' ? navigation : name === 'getJournalSummary' ? { currentSystem: 'Fixture Alpha', sourceState: 'live' } : facts
for (const [path, title] of [['/nav/map', 'System'], ['/nav/route', 'Route'], ['/exploration/biologicals', 'Biolog'], ['/ship/status', 'Ship']]) {
  test('G2D terminal restores styled, readonly ' + path, async () => { const h = harness('go-terminal', fixture); h.props.path = path; h.html(); const html = await h.flush(); assert.match(html, /go-terminal/); assert.match(html, /DAEDALUS/); assert.match(html, new RegExp(title, 'i')); assert.match(html, /READ.ONLY DISPLAY/); assert.doesNotMatch(html, /owner\/remote|Save journal directory|Game features are not implemented/); assert.ok(h.calls.every(c => ['getJournalSummary', 'getGoShip', 'getGoExploration', 'getGoNavigation'].includes(c))) })
}
test('G2D body inspector and sampling use facts without invented rewards', async () => { const h = harness('go-terminal-survey', fixture); h.props.path = '/exploration/biologicals'; h.html(); const html = await h.flush(); assert.match(html, /Fixture organism/); assert.match(html, /sample/i); assert.match(html, /inspector/); assert.doesNotMatch(html, /Estimated.*credits|PRIVATE_|Latitude|Longitude/); h.props.connected = false; assert.doesNotMatch(h.html(), /Fixture organism/) })

test('G2D chart allocates nonoverlapping sibling and nested moon subtrees', () => {
  const body = (bodyId, parent, isStar = false) => ({ bodyId, name: `Fixture ${bodyId}`, details: { isStar, parents: parent == null ? [] : [{ kind: parent === '0' ? 'Star' : 'Planet', bodyId: parent }] } })
  const bodies = [body('0', null, true), body('1', '0'), body('9007199254740992', '1'), body('9007199254740993', '1'), body('4', '9007199254740992'), body('5', '9007199254740992'), body('6', '0')]
  const { chartScene } = harness('go-terminal-chart').exports
  const scene = chartScene(bodies)
  assert.equal(scene.nodes.length, bodies.length)
  assert.equal(new Set(scene.nodes.map(n => `${n.x},${n.y}`)).size, bodies.length, 'glyphs and pointer targets overlap')
  for (const n of scene.nodes) {
    assert.ok(n.x > n.radius && n.x + n.radius < scene.width && n.y > n.radius && n.y + n.radius + 880 < scene.height)
    const parent = scene.nodes.find(p => p.body.bodyId === n.parent?.body.bodyId)
    if (n.depth > 1) assert.ok(n.y >= parent.y + 2600, 'child must be below its parent')
    for (const other of scene.nodes) if (other !== n && other.x === n.x) assert.ok(Math.abs(other.y - n.y) >= 2600, 'subtree rows overlap')
  }
  const filtered = chartScene([bodies[2], bodies[3]], bodies)
  assert.ok(filtered.nodes.every(n => !n.unresolved), 'filtering must retain known parent provenance')
  assert.notEqual(filtered.nodes[0].y, filtered.nodes[1].y)
})
