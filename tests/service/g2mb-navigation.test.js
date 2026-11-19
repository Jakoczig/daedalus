const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
test('Spansh chooser offers only verified public plotters, fixed URLs, and local route history', () => {
  const h = harness('go-spansh')
  const { plotters, plannerText } = h.exports
  assert.deepEqual(plotters.map(p => p.id), ['plotter', 'exact-plotter', 'fleet-carrier', 'trade', 'tourist', 'riches', 'ammonia', 'earth', 'rocky-metal', 'exobiology', 'colonisation', 'history'])
  for (const p of plotters) if (p.id !== 'history') assert.equal(p.url, 'https://spansh.co.uk/' + p.id)
  const html = h.html()
  assert.match(html, /Spansh Plotter/)
  assert.match(html, /manual web workflow/)
  assert.doesNotMatch(html, /api\/|iframe|Import selected CSV/)
  assert.match(plannerText(plotters[0], { from: 'Sol', to: 'Colonia', range: '50' }), /Sol/)
  assert.match(plannerText(plotters[0], { from: 'Sol', to: 'Colonia', range: '50' }), /50/)
  assert.equal(h.calls.length, 0, 'opening chooser must not issue provider or owner RPCs')
})
test('native primary navigation hides Controls without changing legacy navigation', () => {
  const legacy = harness('primary-navigation')
  assert.match(legacy.html(), /Controls/)
  legacy.props.native = true
  assert.doesNotMatch(legacy.html(), /Controls/)
})
test('context activity requires owner and does not enumerate or choose the first commander', async () => {
  const h = harness('go-field-operations', [], 'BGSTally')
  h.props.context = 'missions'
  assert.match(h.html(), /read-only/)
  await h.flush()
  assert.equal(h.calls.length, 0)
  h.props.role = 'owner'
  h.html(); await h.flush()
  assert.ok(h.calls.includes('getGoBGSTally'))
  assert.ok(!h.calls.includes('getGoDataStatus'))
  h.unmount()
})
