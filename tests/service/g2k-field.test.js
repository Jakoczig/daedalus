const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
test('G2K itinerary is explicitly separate and viewers have no import or host clipboard', async () => {
  const h = harness('go-field-operations', { state: 'off_route', completed: 1, total: 3, next: { name: 'Fixture next', neutron: true }, gameTarget: 'Different' }, 'ImportedItinerary')
  h.html(); const html = await h.flush()
  assert.match(html, /IMPORTED ITINERARY/); assert.match(html, /GAME ROUTE/); assert.match(html, /Fixture next/); assert.match(html, /Progress paused/)
  assert.doesNotMatch(html, /Copy next to owner PC|type="file"|Activate \/ correct/)
  h.props.role = 'owner'; assert.match(h.html(), /Copy next to owner PC|Import selected CSV/)
})
test('G2K BGS private export is unavailable on paired displays', () => {
  const h = harness('go-field-operations', null, 'BGSTally')
  assert.match(h.html(), /only to the local owner/); assert.doesNotMatch(h.html(), /Export selected activity/)
})
