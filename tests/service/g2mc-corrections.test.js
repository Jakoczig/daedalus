const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const vm = require('node:vm')
const { harness } = require('../helpers/go-panel-harness.cjs')

test('global shortcuts preserve editing, native selectors, modified keys and IME', () => {
  const source = fs.readFileSync('src/client/pages/_app.js', 'utf8')
  const code = source.slice(source.indexOf('const handleKeyPress'), source.indexOf('export default class'))
  let navigated = 0; let blurred = 0
  const document = { body: { click: () => blurred++ }, querySelector: () => ({ dataset: { secondaryNavigation: '1', primaryNavigation: '1' }, click: () => navigated++ }) }
  const handler = vm.runInNewContext(code + ';handleKeyPress', { document })
  for (const tagName of ['INPUT', 'SELECT', 'TEXTAREA', 'BUTTON', 'OPTION', 'DIV']) {
    document.activeElement = { tagName, isContentEditable: tagName === 'DIV', closest: () => true }
    for (const key of ['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', '1', 'Escape']) handler({ key, target: document.activeElement, getModifierState: () => false })
  }
  document.activeElement = { tagName: 'BODY', closest: () => null }
  for (const fields of [{ ctrlKey: true }, { metaKey: true }, { shiftKey: true }, { altKey: true }, { isComposing: true }, { defaultPrevented: true }, { keyCode: 229 }]) handler({ key: 'ArrowDown', target: document.activeElement, getModifierState: () => false, ...fields })
  assert.equal(navigated, 0, 'editing or modified keys navigated away')
  assert.equal(blurred, 0, 'editing arrows or Escape blurred the input')
  handler({ key: 'ArrowDown', target: document.activeElement, getModifierState: () => false })
  assert.equal(navigated, 1, 'plain page shortcut stopped working')
})

test('native Trade root renders usable comparison and preserves explicit subpages', () => {
  for (const path of ['/trade', '/trade/commodities']) {
    const h = harness('go-terminal', () => ({})); Object.assign(h.props, { path, stage: 'G2E', role: 'owner' })
    try { const html = h.html(); assert.doesNotMatch(html, /has not been migrated/); assert.match(html, path === '/trade' ? /Two-market quoted comparison/i : /Station commodity observations/i) } finally { h.unmount() }
  }
})

test('Local Activity is implemented for owners and never mounts private history for viewers', async () => {
  for (const role of ['owner', 'viewer']) {
    const h = harness('go-terminal', () => ({})); Object.assign(h.props, { path: '/missions/local-activity', stage: 'G2E', role })
    try {
      const html = h.html(); assert.doesNotMatch(html, /has not been migrated/)
      if (role === 'owner') assert.match(html, /Private commander activity/)
      else assert.doesNotMatch(html, /go-private-activity|go-context-activity/)
      await h.flush(); assert.ok(!h.calls.includes('getGoBGSTally'), 'private ledger queried without explicit owner open')
    } finally { h.unmount() }
  }
})

test('missing route target is explicit', async () => {
  const route = harness('go-terminal-route', { targetSystem: '', routeState: 'unverified', hops: [] })
  try { route.html(); await route.flush(); assert.match(route.html(), /Not reported/) } finally { route.unmount() }
})

test('successful Frontier read wording separates observed success from fixture evidence', async () => {
  const value = { state: 'last_request_succeeded', lastSuccess: '2026-11-19T10:00:00Z', config: { read: true }, capability: { id: 'frontier', read: ['profile'], upload: [], evidence: 'Contract-fixture tested; live authentication/publication acceptance not established', intervalSeconds: 60 } }
  const h = harness('go-provider-settings', name => name === 'getGoProviders' ? [value] : {}, 'ProviderDiagnostics')
  try {
    h.html(); await h.flush()
    const html = h.html(); assert.match(html, /authorized read succeeded/i)
    assert.doesNotMatch(html, /live acceptance remains unverified|live authentication\/publication acceptance not established/)
    assert.ok(h.calls.every(name => ['getGoProviders', 'getGoCommunity'].includes(name)), 'status rendering caused an online action')
  } finally { h.unmount() }
})

test('owner EDSM settings follow shared cache metadata without refreshing the provider', async () => {
  const originalSet = global.setInterval; const originalClear = global.clearInterval
  let poll; let cleared = false; let cached = false
  global.setInterval = (fn, ms) => { assert.equal(ms, 15000); poll = fn; return 71 }
  global.clearInterval = id => { assert.equal(id, 71); cleared = true }
  const h = harness('go-community', () => ({ enabled: true, status: cached ? 'cached' : 'waiting_for_journal', fetchedAt: cached ? '2026-11-19T10:00:00Z' : '', expiresAt: cached ? '2026-11-19T16:00:00Z' : '' }))
  try {
    h.html(); await h.flush(); assert.match(h.html(), /waiting_for_journal/)
    cached = true; await poll(); assert.match(h.html(), /Cache: cached/); assert.match(h.html(), /2026-11-19T16:00:00Z/)
    assert.deepEqual(h.calls, ['getGoCommunity', 'getGoCommunity'])
  } finally { h.unmount(); global.setInterval = originalSet; global.clearInterval = originalClear }
  assert.equal(cleared, true)
})
