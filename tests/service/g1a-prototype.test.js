const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

function loadClientModule (relativePath, dependencies, browser = {}) {
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
  mod._compile(`module.exports = function (window, WebSocket, setTimeout, clearTimeout, CustomEvent) { const module = { exports: {} }; const exports = module.exports; ${code}\nreturn module.exports }`, filename)
  return mod.exports(browser.window, browser.WebSocket, browser.setTimeout, browser.clearTimeout, browser.CustomEvent)
}

function harness (initialState) {
  let state = initialState
  let effect
  const timers = new Map()
  const sockets = []
  const browser = {
    window: {
      location: { protocol: 'http:', host: '127.0.0.1:54395' },
      fetch: async () => ({ ok: true, json: async () => ({ role: 'owner', prototype: { stage: 'G1A', offline: true } }) }),
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
    sockets
  }
}

function render (prototype, child, connection = { connected: true, ready: true }) {
  return harness({ role: 'owner', prototype, ...connection }).render(child)
}

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
