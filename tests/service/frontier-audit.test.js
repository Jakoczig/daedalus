const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
test('Frontier fleet reference is displayed without implying live ownership or verified engineering units', () => {
  const h = harness('go-fleet')
  h.props.page = { rows: [{ id: 'capi:9007199254740993', entity: '9007199254740993', state: 'capi_last_known', details: { shipName: 'Fixture vessel', modelLabel: 'Asp Explorer', Value: '9007199254740995', capiReference: { fetchedAt: '2026-01-01T00:00:00Z', currentReference: false }, capiModules: [{ slot: 'FrameShiftDrive', item: 'int_hyperdrive_size5_class5', name: 'Frame Shift Drive', modifiers: [{ name: 'OutfittingFieldType_FSDOptimalMass', value: '1.25' }] }] } }] }
  const html = h.html()
  assert.match(html, /Fixture vessel/)
  assert.match(html, /Frontier owned-fleet reference/)
  assert.match(html, /observation time unknown/)
  assert.match(html, /units.*unverified/)
  assert.doesNotMatch(html, /Last witnessed active/)
})
test('Frontier global goals never establish personal subscription or contribution', () => {
  const h = harness('go-community-goals')
  h.props.page = { goalMembership: { known: true, ids: [] }, reference: [{ field: 'communityGoals', source: 'frontier', fetchedAt: '2026-01-01', value: { global: [{ id: '9007199254740993', title: 'Fixture public goal', globalTotal: '100', targetTotal: '1000' }] } }] }
  const html = h.html()
  assert.match(html, /0 subscribed goals as last witnessed/)
  assert.match(html, /Fixture public goal/)
  assert.match(html, /Global progress does not establish your subscription/)
  assert.match(html, /Global target/)
  assert.doesNotMatch(html, /Your contribution/)
})
