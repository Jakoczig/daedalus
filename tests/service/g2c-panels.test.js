const test = require('node:test')
const assert = require('node:assert/strict')
const { harness, exploration } = require('../helpers/go-panel-harness.cjs')
const survey = { ...exploration, bodies: [{ ...exploration.bodies[0], details: { radiusKM: 1000, parents: [{ kind: 'Star', bodyId: '0' }], rings: [{ name: 'Fixture Ring', class: 'Rocky', innerKM: 2000, outerKM: 3000 }] }, organisms: [{ genus: 'Bacterium', species: 'Confirmed <species>', stage: 'sampled', samples: 2, complete: false, lastObservedAt: '2026-01-01T00:00:02Z' }] }] }
test('G2C actual survey workspace reuses body renderer and shows typed details and sampling', async () => {
  const h = harness('go-survey-explorer', survey); h.html(); const html = await h.flush()
  assert.deepEqual(h.calls, ['getGoExploration']); assert.match(html, /Selectable journal system map/); assert.match(html, /system-map__planet-surface/); assert.match(html, /1[,\s]?000 km/); assert.match(html, /Fixture Ring/); assert.match(html, /Confirmed &lt;species&gt;/); assert.match(html, /2\/3 sample stages/); assert.match(html, /Previously mapped/)
  assert.doesNotMatch(html, /https?:|predicted species:|estimated payout|PRIVATE_/)
  h.props.biology = true; assert.doesNotMatch(h.html(), /Selectable journal system map/); assert.match(h.html(), /Exobiology/)
  h.props.connected = false; assert.doesNotMatch(h.html(), /Fixture Ring|Confirmed &lt;species&gt;/); assert.match(h.html(), /disconnected/)
})
test('G2C survey worklist bounds rows and clears old system observations', async () => {
  const many = { ...survey, bodies: Array.from({ length: 100 }, (_, i) => ({ ...survey.bodies[0], bodyId: String(i), name: `Fixture ${i}` })) }
  const h = harness('go-survey-explorer', many); h.html(); let html = await h.flush(); assert.equal(h.elements('tr').length, 26); assert.match(html, /Page 1 \/ 4/)
  h.broadcast({ ...survey, systemName: 'Other system', bodies: [] }); html = h.html(); assert.doesNotMatch(html, /Fixture Ring|Confirmed &lt;species&gt;/); assert.match(html, /Other system/)
})
test('G2C enhanced navigation pages long routes without a polling loop or host action', async () => {
  const { navigation } = require('../helpers/go-panel-harness.cjs')
  const h = harness('go-navigation', { ...navigation, hops: Array.from({ length: 200 }, (_, i) => ({ name: `Waypoint ${i}`, starClass: 'G', isNext: i === 100 })) }); h.props.enhanced = true; h.html(); const html = await h.flush()
  assert.equal(h.elements('li').length, 80); assert.match(html, /Waypoint 100/); assert.match(html, /fuel-scoop class/); assert.deepEqual(h.calls, ['getGoNavigation'])
})

test('G2C renderer translates journal star codes into the existing CSS classes', async () => {
  for (const [kind, subtype] of [['M', 'M Star'], ['N', 'Neutron Star'], ['H', 'Black Hole'], ['DA', 'DA White Dwarf']]) {
    const h = harness('go-survey-explorer', { ...survey, bodies: [{ ...survey.bodies[0], kind, details: { isStar: true } }] }); h.html(); const html = await h.flush(); assert.ok(html.includes(`data-system-object-sub-type="${subtype}"`), `missing legacy subtype ${subtype}`)
  }
})
test('G2C filtering does not discard known parent provenance', async () => {
  const bodies = [{ bodyId: '0', name: 'Fixture Star', details: { isStar: true } }, { ...survey.bodies[0], bodyId: '1', landable: true, details: { parents: [{ kind: 'Star', bodyId: '0' }] } }]
  const h = harness('go-survey-explorer', { ...survey, bodies }); h.html(); await h.flush(); h.elements('select')[0].props.onChange({ target: { value: 'landable' } }); const html = h.html(); assert.doesNotMatch(html, /Parent unobserved \/ ambiguous|, parent unknown/)
})
