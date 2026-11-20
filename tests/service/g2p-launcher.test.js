const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
test('provider consent persists only after confirmation and write remains staged without categories', async () => {
  const previous = global.window
  const requests = []; let accept = false
  global.window = { confirm: () => accept }
  const h = harness('go-provider-settings', null, 'CompactProvider')
  h.props.value = { capability: { id: 'eddn', read: ['reference'], upload: ['travel'] }, config: { read: false, upload: false, categories: [] } }
  h.props.refresh = async () => {}
  h.props.sendEvent = async (name, data) => { requests.push({ name, data }) }
  try {
    h.html(); await h.elements('input')[0].props.onChange({ target: { checked: true } }); assert.equal(requests.length, 0)
    accept = true; await h.elements('input')[0].props.onChange({ target: { checked: true } }); assert.equal(requests[0].name, 'configureGoProvider'); assert.equal(requests[0].data.config.read, true)
    h.html(); await h.elements('input')[1].props.onChange({ target: { checked: true } }); h.html(); assert.equal(requests.length, 1)
    const category = h.elements('input').at(-1); assert.equal(category.props.checked, false)
    await category.props.onChange({ target: { checked: true } }); assert.equal(requests[1].data.config.upload, true); assert.deepEqual(requests[1].data.config.categories, ['travel'])
  } finally { h.unmount(); global.window = previous }
})
test('failed consent persistence retains unchecked persisted state and reports failure', async () => {
  const previous = global.window; global.window = { confirm: () => true }
  const h = harness('go-provider-settings', null, 'CompactProvider'); h.props.value = { capability: { id: 'spansh', read: ['system'], upload: [] }, config: {} }; h.props.refresh = async () => {}; h.props.sendEvent = async () => { throw Error('Storage unavailable') }
  try { h.html(); await h.elements('input')[0].props.onChange({ target: { checked: true } }); assert.match(h.html(), /Storage unavailable/); assert.equal(h.elements('input')[0].props.checked, false) } finally { h.unmount(); global.window = previous }
})
test('single-account settings always show the active verified card and only launcher offers portrait editing', async () => {
  for (const launcher of [false, true]) {
    const h = harness('go-accounts', () => ({ settings: { multiple: false }, cards: [{ fid: 'FIXTURE-A', name: 'Fixture A', shipType: 'Cobra', system: 'Sol', active: true }] }))
    h.props.launcher = launcher
    try { h.html(); await h.flush(); const html = h.html(); assert.match(html, /Fixture A/); assert.match(html, /Cobra/); assert.match(html, /Sol/); assert.equal(html.includes('Change portrait'), launcher); assert.doesNotMatch(html, /type="file"/) } finally { h.unmount() }
  }
})
test('compact provider cards expose no requests or credential fields until expanded and are absent for viewers', async () => {
  const h = harness('go-provider-settings', () => [{ capability: { id: 'frontier', read: ['profile'], upload: [] }, config: {}, state: 'disabled' }])
  try {
    assert.equal(h.html(), '')
    h.props.role = 'owner'; h.html(); await h.flush()
    const html = h.html(); assert.match(html, /Frontier/); assert.doesNotMatch(html, /Read profile|Save consent|type="password"/)
    assert.ok(h.calls.every(n => n === 'getGoProviders'))
  } finally { h.unmount() }
})
test('launcher journal labels distinguish cached history from a running game', () => {
  const h = harness('go-launcher-status')
  try {
    const label = h.exports.journalLabel
    assert.equal(label({ sourceState: 'waiting_game', cached: true }), 'Waiting for game to start')
    assert.equal(label({ sourceState: 'live', cached: true }), 'Cached journal history')
    assert.equal(label({ sourceState: 'live', importInProgress: true }), 'Loading journal history')
    assert.equal(label({ sourceState: 'live' }), 'Journal connected')
    assert.match(label({ sourceState: 'permission_error' }), /permission/i)
  } finally { h.unmount() }
})
test('service health never treats disabled or historical success as a current verified check', () => {
  const h = harness('go-launcher-status')
  try {
    const health = h.exports.providerHealth
    assert.equal(health({ config: {}, lastSuccess: new Date().toISOString() }, 0), 'disabled')
    assert.equal(health({ config: { read: true }, lastSuccess: '2020-01-01T00:00:00Z' }, Date.now()), 'unverified')
    assert.equal(health({ config: { read: true }, lastError: 'Failure', lastSuccess: new Date().toISOString() }, 0), 'error')
  } finally { h.unmount() }
})
