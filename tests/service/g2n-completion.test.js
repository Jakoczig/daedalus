const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')

test('goal cards join exact Frontier IDs without inventing participation and prefer fresh global metadata', () => {
  const h = harness('go-community-goals')
  try {
    const page = { goalMembership: { known: true, ids: ['9007199254740993'], observedAt: '2026-01-01' }, current: [{ entity: '9007199254740993', title: 'progress_snapshot', at: '2026-01-01', details: { fields: { Title: 'Old title', PlayerContribution: '42', CurrentTotal: '10' } } }], reference: [{ field: 'communityGoals', source: 'frontier', stale: false, fetchedAt: '2026-01-02', value: { global: [{ id: '9007199254740993', title: 'Live goal', globalTotal: '9007199254740997', targetTotal: '9999999999999999' }, { id: '9007199254740994', title: 'Other goal' }] } }] }
    const cards = h.exports.goalCards(page)
    assert.equal(cards.length, 2); assert.equal(cards[0].title, 'Live goal'); assert.equal(cards[0].contribution, '42'); assert.equal(cards[0].total, '9007199254740997'); assert.equal(cards[1].contribution, null)
    page.reference[0].stale = true
    assert.equal(h.exports.goalCards(page)[0].total, '10')
  } finally { h.unmount() }
})

test('Community Goals has one owner-controlled Frontier refresh and no export or automatic online request', async () => {
  const h = harness('go-community-goals', () => ({ rows: [] }))
  Object.assign(h.props, { role: 'owner', page: { goalMembership: { known: true, ids: [] }, rows: [] } })
  try { const html = h.html(); await h.flush(); assert.match(html, /Refresh Frontier goals/); assert.doesNotMatch(html, /export|commander select|local journal observation/i); assert.ok(h.calls.every(n => n === 'getGoProviders')) } finally { h.unmount() }
})

test('Fleet has owned/sold filters, exact value formatting and bounded pagination controls', () => {
  const h = harness('go-fleet'); Object.assign(h.props, { page: { rows: [{ id: '1', entity: '1', state: 'known_active', details: { shipName: 'Fixture Ship', modelLabel: 'Asp Explorer', Value: '9007199254740993' } }, { id: '2', entity: '2', state: 'sold', details: { shipName: 'SOLD FIXTURE' } }], next: '2' } })
  try { const html = h.html(); assert.match(html, /Fixture Ship/); assert.doesNotMatch(html, /SOLD FIXTURE/); assert.match(html, /Fleet view/); assert.match(html, /Older ships/) } finally { h.unmount() }
})

test('activity groups never add influence ratings or mix award and redemption', () => {
  const h = harness('go-field-operations', {}, 'BGSTally')
  try {
    const rows = [{ category: 'bounty_awarded', faction: 'F', system: '1', period: 'day', unit: 'CR', amount: '9007199254740993' }, { category: 'bounty_awarded', faction: 'F', system: '1', period: 'day', unit: 'CR', amount: '2' }, { category: 'bounty_redeemed', faction: 'F', system: '1', period: 'day', unit: 'CR', amount: '8' }, { category: 'mission_influence', faction: 'F', system: '1', period: 'day', unit: 'INF rating', amount: '+++' }]
    const groups = h.exports.activityGroups(rows)
    assert.equal(groups.length, 3); assert.equal(groups[0].total, '9007199254740995'); assert.equal(groups[1].total, '8'); assert.equal(groups[2].total, null)
  } finally { h.unmount() }
})
