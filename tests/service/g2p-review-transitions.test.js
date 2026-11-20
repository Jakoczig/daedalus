const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')

function provider (state) {
  const h = harness('go-provider-settings', null, 'CompactProvider')
  const calls = []
  h.props.value = state
  h.props.refresh = async () => {}
  h.props.sendEvent = async (name, q) => {
    calls.push({ name, q })
    if (name === 'configureGoProvider') state.config = { ...q.config }
    if (name === 'setGoProviderCredential') state.credentialStored = true
    if (name === 'disconnectGoProvider') { state.credentialStored = false; state.config = { paused: true, read: false, upload: false, categories: [] } }
  }
  const toggle = (name, checked) => { h.html(); const label = h.elements('label').find(l => l.props.children?.[1] === ' ' + name); assert.ok(label, name); return label.props.children[0].props.onChange({ target: { checked } }) }
  return { h, calls, toggle }
}
test('credential and Write transitions follow persisted state through deletion and re-entry', async () => {
  const previous = global.window; global.window = { confirm: () => true }
  const state = { capability: { id: 'edsm', read: ['flightlog'], upload: ['travel'] }, config: { read: false, upload: false, categories: [] }, credentialStored: false }
  const { h, calls, toggle } = provider(state)
  try {
    await toggle('Read', true); assert.equal(state.config.read, true); assert.match(h.html(), /type="password"/)
    const fields = h.elements('input').filter(e => e.props.type !== 'checkbox')
    fields[0].props.onChange({ target: { value: 'Synthetic A' } }); fields[1].props.onChange({ target: { value: 'SYNTHETIC-KEY' } }); h.html()
    await h.elements('form')[0].props.onSubmit({ preventDefault () {} })
    assert.equal(state.credentialStored, true); assert.match(h.html(), /Delete credentials/); assert.doesNotMatch(h.html(), /Re-enter credentials|type="password"/)
    await toggle('Write', true); assert.equal(state.config.upload, false); assert.match(h.html(), /Write setup pending/)
    assert.equal(h.elements('input')[1].props.checked, false)
    await h.elements('input').at(-1).props.onChange({ target: { checked: true } }); h.html()
    assert.equal(state.config.upload, true); assert.equal(h.elements('input')[1].props.checked, true)
    await h.elements('input').at(-1).props.onChange({ target: { checked: false } }); h.html()
    assert.equal(state.config.upload, false); assert.deepEqual(state.config.categories, []); assert.equal(h.elements('input')[1].props.checked, false); assert.doesNotMatch(h.html(), /Write setup pending/)
    await toggle('Write', true); h.html(); await h.elements('button').find(b => b.props.children === 'Delete credentials').props.onClick(); h.html()
    assert.equal(state.credentialStored, false); assert.equal(state.config.paused, true); assert.equal(h.elements('input')[1].props.checked, false)
    assert.match(h.html(), /Re-enter credentials/); assert.doesNotMatch(h.html(), /Write setup pending|type="password"/)
    h.elements('button').find(b => b.props.children === 'Re-enter credentials').props.onClick(); assert.match(h.html(), /type="password"/)
    assert.equal(state.config.upload, false); assert.ok(calls.every(c => !['readGoProvider', 'connectGoFrontier'].includes(c.name)))
  } finally { h.unmount(); global.window = previous }
})
test('reopening another account has no credential draft or pending Write from the previous account', async () => {
  const previous = global.window; global.window = { confirm: () => true }
  const a = provider({ capability: { id: 'edsm', read: ['flightlog'], upload: ['travel'] }, config: {}, credentialStored: false })
  try {
    await a.toggle('Write', true); assert.match(a.h.html(), /Write setup pending/); a.h.unmount()
    const state = { capability: { id: 'edsm', read: ['flightlog'], upload: ['travel'] }, config: { paused: true, read: false, upload: false, categories: [] }, credentialStored: true }
    const b = provider(state)
    try { assert.match(b.h.html(), /Delete credentials/); assert.doesNotMatch(b.h.html(), /Re-enter credentials|Write setup pending|type="password"/); assert.equal(b.h.elements('input')[1].props.checked, false); assert.equal(b.calls.length, 0) } finally { b.h.unmount() }
  } finally { a.h.unmount(); global.window = previous }
})
test('startup progress describes local status review and never initiates provider traffic', async () => {
  const originalSet = global.setTimeout; const originalClear = global.clearTimeout; let advance
  global.setTimeout = callback => { advance = callback; return 1 }; global.clearTimeout = () => {}
  const h = harness('go-launcher-status', () => [{ capability: { id: 'frontier' }, config: { read: true } }])
  try { h.html(); await h.flush(); await h.flush(); assert.match(h.html(), /Reviewing enabled services/); assert.doesNotMatch(h.html(), /Checking Frontier|All services OK/); assert.equal(typeof advance, 'function'); advance(); assert.match(h.html(), /1 not checked/); assert.deepEqual(h.calls, ['getGoProviders']) } finally { h.unmount(); global.setTimeout = originalSet; global.clearTimeout = originalClear }
})

test('account-switch notification closes the old provider editor before loading the next account', async () => {
  const previous = global.window; const listeners = new Map()
  global.window = { addEventListener: (n, cb) => listeners.set(n, cb), removeEventListener: n => listeners.delete(n) }
  let current = { capability: { id: 'edsm', read: ['flightlog'], upload: ['travel'] }, config: { read: true }, credentialStored: false }
  const h = harness('go-provider-settings', () => [current]); h.props.role = 'owner'
  try {
    h.html(); await h.flush(); h.elements('button')[0].props.onClick(); assert.match(h.html(), /type="password"/)
    current = { ...current, config: { paused: true, upload: false, categories: [] }, credentialStored: true }
    listeners.get('eridion-account-changed')(); assert.doesNotMatch(h.html(), /go-provider-expanded|type="password"/)
    await h.flush(); assert.doesNotMatch(h.html(), /go-provider-expanded/)
    h.elements('button')[0].props.onClick(); assert.match(h.html(), /Delete credentials/); assert.doesNotMatch(h.html(), /Re-enter credentials|Write setup pending|type="password"/)
  } finally { h.unmount(); global.window = previous }
})
