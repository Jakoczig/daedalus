const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
test('atlas tiles accept only constrained authenticated local references', () => {
  const h = harness('go-atlas')
  try { const { safeTileURL } = h.exports; assert.equal(safeTileURL('/atlas/assets/0123456789abcdef01234567/tiles/xz/7/4-5.png'), true); for (const value of ['https://eridion.fitoserver.org/a', '/atlas/assets/../../private', '/images/galaxy-density/galaxy-xz.png', '/atlas/assets/0123456789abcdef01234567/xz/7/0-0.png?x=1']) assert.equal(safeTileURL(value), false) } finally { h.unmount() }
})
test('atlas settings are absent for viewers and ordinary mount never checks a remote service', async () => {
  for (const role of ['owner', 'viewer']) { const h = harness('go-atlas', () => ({ automatic: false, details: false }), 'AtlasSettings'); h.props.role = role; try { const html = h.html(); await h.flush(); if (role === 'viewer') { assert.equal(html, ''); assert.equal(h.calls.length, 0) } else { assert.match(html, /Galaxy Atlas/); assert.ok(h.calls.every(n => n === 'getGoAtlasStatus')) } } finally { h.unmount() } }
})

test('atlas settings explicitly reports the protected working-set budget exception', async () => {
  const h = harness('go-atlas', () => ({ detailBytes: 140 * 1048576, visibleDetailBytes: 140 * 1048576, detailBudgetBytes: 140 * 1048576, workingSetLimitBytes: 2304 * 1048576 }), 'AtlasSettings')
  h.props.role = 'owner'
  try { h.html(); await h.flush(); const html = h.html(); assert.match(html, /140.00 MiB/); assert.match(html, /128 MiB target/); assert.match(html, /Only this working set may exceed/); assert.match(html, /2304.00 MiB/); assert.ok(h.calls.every(n => n === 'getGoAtlasStatus')) } finally { h.unmount() }
})
