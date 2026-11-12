const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

function loadClientModule (relativePath, dependencies, browser = {}, testExports = '') {
  const filename = path.resolve(__dirname, '../../src/client', relativePath)
  const code = transformSync(fs.readFileSync(filename, 'utf8'), {
    babelrc: false,
    configFile: false,
    presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]],
    plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')]
  }).code
  const mod = new Module(filename, module)
  mod.filename = filename
  mod.paths = module.paths
  mod.require = dependencies
  mod._compile(`module.exports = function (window, WebSocket, setTimeout, clearTimeout, CustomEvent) { const module = { exports: {} }; const exports = module.exports; ${code}\n${testExports}\nreturn module.exports }`, filename)
  return mod.exports(browser.window, browser.WebSocket, browser.setTimeout, browser.clearTimeout, browser.CustomEvent)
}

function harness (initialState, options = {}) {
  let state = initialState
  let effect
  const timers = new Map()
  const sockets = []
  const browser = {
    window: {
      location: { protocol: 'http:', host: '127.0.0.1:54395' },
      fetch: options.fetch || (async () => ({ ok: true, json: async () => ({ role: 'owner', prototype: { stage: 'G1A', offline: true } }) })),
      localStorage: { getItem: () => null },
      sessionStorage: { getItem: () => null },
      dispatchEvent: () => {}
    },
    setTimeout: (callback, delay) => { const id = {}; timers.set(id, { callback, delay }); return id },
    clearTimeout: id => timers.delete(id),
    CustomEvent: class {}
  }
  browser.WebSocket = class {
    static OPEN = 1
    constructor () { this.readyState = 0; this.sent = []; sockets.push(this) }
    send (raw) { this.sent.push(JSON.parse(raw)) }
    close () { this.readyState = 3; this.onclose() }
    message (value) { this.onmessage({ data: JSON.stringify(value) }) }
  }
  const loader = loadClientModule('components/loader.js', require).default
  const layout = loadClientModule('components/layout.js', name => name === 'components/loader' ? loader : require(name)).default
  const socket = loadClientModule('lib/socket.js', name => {
    if (name === 'react') {
      return {
        ...React,
        useState: initial => {
          state ??= initial
          return [state, update => { state = typeof update === 'function' ? update(state) : update }]
        },
        useEffect: callback => { effect ??= callback }
      }
    }
    if (name === 'next/router') return { __esModule: true, default: {}, useRouter: () => ({ pathname: '/exploration/system' }) }
    if (name === 'lib/notification') return () => {}
    if (name === 'components/primary-navigation') return () => React.createElement('nav', null, 'Existing primary navigation')
    if (name === 'components/layout') return layout
    if (name === 'components/panel') return ({ children }) => React.createElement('div', null, children)
    if (name === 'components/remote-displays') return () => React.createElement('section', null, 'Owner remote display settings')
    if (name === 'lib/navigation-items') return require('../../src/client/lib/navigation-items')
    return require(name)
  }, browser)
  return {
    render: child => renderToStaticMarkup(React.createElement(socket.SocketProvider, null, child)),
    start: async () => { effect(); await new Promise(resolve => setImmediate(resolve)) },
    reconnect: async () => {
      const timer = [...timers.entries()].find(([, value]) => value.delay === 5000)
      assert.ok(timer, 'disconnect did not schedule reconnection')
      timers.delete(timer[0])
      await timer[1].callback()
      await new Promise(resolve => setImmediate(resolve))
    },
    get state () { return state },
    sockets,
    sendEvent: socket.sendEvent,
    timers
  }
}

function render (prototype, child, connection = { connected: true, ready: true }) {
  return harness({ role: 'owner', prototype, ...connection }).render(child)
}

test('viewer manual pairing accepts six visible uppercase characters and submits without owner authority', async () => {
  const values = []
  let cursor = 0
  let submitted
  let reloaded = false
  const browser = { window: { fetch: async (url, options) => { submitted = { url, options }; return { ok: true } }, location: { reload: () => { reloaded = true } } } }
  const mod = loadClientModule('lib/socket.js', name => {
    if (name === 'react') return { ...React, useState: initial => { const index = cursor++; values[index] ??= initial; return [values[index], value => { values[index] = value }] } }
    if (name === 'next/router') return { __esModule: true, default: {} }
    if (name === 'lib/navigation-items') return require('../../src/client/lib/navigation-items')
    if (name.startsWith('components/') || name === 'lib/notification') return () => null
    return require(name)
  }, browser, 'module.exports.PairDisplay = PairDisplay')
  const draw = (shortCode = true) => { cursor = 0; return mod.PairDisplay({ shortCode }) }
  let tree = draw()
  const children = React.Children.toArray(tree.props.children)
  let form = children.find(child => child.type === 'form')
  let inputs = React.Children.toArray(form.props.children).filter(child => child.type === 'label').map(label => React.Children.toArray(label.props.children).find(child => child.type === 'input'))
  assert.equal(inputs[0].props.type, 'text')
  assert.equal(inputs[0].props.maxLength, 6)
  assert.equal(inputs[0].props.minLength, 6)
  assert.equal(inputs[0].props.autoCapitalize, 'characters')
  assert.equal(inputs[0].props.spellCheck, false)
  assert.equal(inputs[0].props.inputMode, 'text')
  assert.equal(inputs[0].props.required, true)
  const pattern = new RegExp(`^(?:${inputs[0].props.pattern})$`)
  assert.ok(pattern.test('7K4M9P'))
  assert.ok(!pattern.test('7I4M9P') && !pattern.test('7K4M9PP'))
  inputs[0].props.onChange({ target: { value: '7k4m9p' } })
  tree = draw()
  form = React.Children.toArray(tree.props.children).find(child => child.type === 'form')
  inputs = React.Children.toArray(form.props.children).filter(child => child.type === 'label').map(label => React.Children.toArray(label.props.children).find(child => child.type === 'input'))
  assert.equal(inputs[0].props.value, '7K4M9P')
  await form.props.onSubmit({ preventDefault () {} })
  assert.equal(submitted.url, '/auth/pair')
  assert.equal(submitted.options.credentials, 'same-origin')
  assert.deepEqual(JSON.parse(submitted.options.body), { code: '7K4M9P', name: 'Browser display' })
  assert.ok(reloaded)
  values[0] = ''
  tree = draw(false)
  form = React.Children.toArray(tree.props.children).find(child => child.type === 'form')
  inputs = React.Children.toArray(form.props.children).filter(child => child.type === 'label').map(label => React.Children.toArray(label.props.children).find(child => child.type === 'input'))
  assert.notEqual(inputs[0].props.maxLength, 6, 'legacy long pairing code was truncated')
  inputs[0].props.onChange({ target: { value: 'abcdef0123456789abcdef0123456789' } })
  tree = draw(false)
  form = React.Children.toArray(tree.props.children).find(child => child.type === 'form')
  await form.props.onSubmit({ preventDefault () {} })
  assert.equal(JSON.parse(submitted.options.body).code, 'abcdef0123456789abcdef0123456789')
})

test('unauthenticated G1B hint selects short entry without granting a role; legacy keeps long entry', async () => {
  for (const hint of [6, undefined]) {
    const client = harness(undefined, { fetch: async () => ({ ok: false, status: 401, json: async () => ({ pairingCodeLength: hint }) }) })
    client.render()
    await client.start()
    assert.equal(client.state.pairingCodeLength, hint === 6 ? 6 : 32)
    assert.equal(client.state.role, null)
    assert.equal(client.state.needsPairing, true)
    assert.equal(client.sockets.length, 0)
  }
})

test('G1A uses the React navigation shell without mounting an unported page', () => {
  function Unported () { throw new Error('Unported game page mounted') }
  const html = render({ stage: 'G1A', offline: true }, React.createElement(Unported))
  assert.match(html, /G1A/)
  assert.match(html, /offline/i)
  assert.match(html, /not implemented/i)
  assert.match(html, /Existing primary navigation/)
  assert.match(html, /exploration\/system/)
})

test('legacy service still renders its existing page when no prototype marker is present', () => {
  for (const connection of [{ connected: true, ready: true }, { connected: false, ready: false }, { connected: true, ready: false }]) {
    const html = render(null, React.createElement('p', null, 'Existing live interface'), connection)
    assert.match(html, /Existing live interface/)
    assert.doesNotMatch(html, /G1A|role="status"|id="loader"/)
  }
})

function assertStatus (client, connected, ready, message) {
  assert.equal(client.state.connected, connected)
  assert.equal(client.state.ready, ready)
  const html = client.render()
  const status = html.match(/<p role="status">([^<]+)<\/p>/)
  assert.ok(status, 'missing visible connection status')
  assert.match(status[1], message)
  assert.match(status[1], /offline/i)
  // The real Layout fades its children while loading/disconnected. Status must
  // precede it so it remains visible, and the real loader must reflect state.
  assert.ok(html.indexOf(status[0]) < html.indexOf('class="layout"'))
  assert.match(html, new RegExp(`id="loader" style="opacity:${connected && ready ? '0' : '0.75'}"`))
  assert.match(html, new RegExp(`class="layout__main " style="opacity:${connected && ready ? '1' : '0'}"`))
}

for (const { connected, ready, message } of [
  { connected: false, ready: false, message: /disconnected.*reconnect/i },
  { connected: true, ready: false, message: /waiting.*ready/i },
  { connected: true, ready: true, message: /connected and ready/i }
]) {
  test(`G1A displays socket status connected=${connected}, ready=${ready}`, () => {
    assertStatus(harness({ role: 'owner', prototype: { stage: 'G1A', offline: true }, connected, ready }), connected, ready, message)
  })
}

test('G1A reports service disconnection and reconnects only after the new socket is ready', async () => {
  const client = harness()
  client.render()
  await client.start()
  let connection = client.sockets[0]
  connection.readyState = 1
  const opened = connection.onopen()
  assertStatus(client, true, false, /waiting.*ready/i)
  const request = connection.sent[0]
  assert.equal(request.name, 'getLoadingStatus')
  connection.message({ ...request, message: { loadingComplete: true } })
  await opened
  assertStatus(client, true, true, /connected and ready/i)

  connection.close()
  assertStatus(client, false, false, /disconnected.*reconnect/i)
  await client.reconnect()
  assert.equal(client.sockets.length, 2)
  assertStatus(client, false, false, /disconnected.*reconnect/i)

  connection = client.sockets[1]
  connection.readyState = 1
  const reopened = connection.onopen()
  assertStatus(client, true, false, /waiting.*ready/i)
  connection.message({ ...connection.sent[0], message: { loadingComplete: true } })
  await reopened
  assertStatus(client, true, true, /connected and ready/i)
  assert.match(client.render(), /not implemented/)
})

test('G1B viewer stays offline and never receives owner settings', () => {
  const viewer = harness({ role: 'viewer', prototype: { stage: 'G1B', offline: true }, connected: true, ready: true })
  function Unported () { throw new Error('Gameplay mounted') }
  const html = viewer.render(React.createElement(Unported))
  assert.match(html, /G1B/)
  assert.match(html, /read.only/i)
  assert.doesNotMatch(html, /Owner remote display settings|not enabled in G1A/)
  const owner = harness({ role: 'owner', prototype: { stage: 'G1B', offline: true }, connected: true, ready: true })
  assert.match(owner.render(), /Owner remote display settings/)
})

test('G1B bootstrap survives transient service failure instead of requesting pairing', async () => {
  let attempts = 0
  const client = harness(undefined, { fetch: async () => ++attempts === 1 ? { ok: false, status: 503 } : { ok: true, json: async () => ({ role: 'viewer', prototype: { stage: 'G1B', offline: true } }) } })
  client.render(); await client.start()
  assert.equal(client.state.needsPairing, false)
  await client.reconnect()
  assert.equal(client.state.role, 'viewer')
  assert.equal(client.state.prototype.stage, 'G1B')
  assert.equal(client.sockets.length, 1)
})

test('disconnect cancels pending work and stale readiness cannot affect the new socket', async () => {
  const client = harness(); client.render(); await client.start()
  const first = client.sockets[0]; first.readyState = 1
  const opened = first.onopen()
  const pending = client.sendEvent('hostInfo')
  const rejected = assert.rejects(pending, /interrupted/i)
  const oldRequest = first.sent[0]
  first.close(); await rejected; await opened
  await client.reconnect()
  const second = client.sockets[1]; second.readyState = 1
  const reopened = second.onopen()
  first.message({ ...oldRequest, message: { loadingComplete: true } })
  assert.equal(client.state.ready, false)
  second.message({ ...second.sent[0], message: { loadingComplete: true } }); await reopened
  assert.equal(client.state.ready, true)
  assert.ok([...client.timers.values()].every(timer => timer.delay !== 30000), 'pending request timers leaked')
})

test('offline RPC queue is bounded', async () => {
  const client = harness()
  const pending = []
  for (let i = 0; i < 128; i++) pending.push(client.sendEvent('hostInfo').catch(() => {}))
  await assert.rejects(client.sendEvent('hostInfo'), /limit/i)
  client.render(); await client.start(); client.sockets[0].close()
  await Promise.all(pending)
})

test('timed-out offline work is removed and is never replayed on connect', async () => {
  const client = harness()
  client.render(); await client.start()
  for (let round = 0; round < 3; round++) {
    const pending = client.sendEvent('staleAction')
    const rejected = assert.rejects(pending, /timed out/i)
    const timer = [...client.timers.entries()].find(([, value]) => value.delay === 30000)
    client.timers.delete(timer[0]); timer[1].callback(); await rejected
  }
  const connection = client.sockets[0]; connection.readyState = 1
  const opened = connection.onopen()
  assert.ok(connection.sent.every(request => request.name === 'getLoadingStatus'), 'timed-out action was replayed')
  connection.message({ ...connection.sent[0], message: { loadingComplete: true } }); await opened
})
