const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

function harness (role = 'viewer', initial = { sourceState: 'live', currentSystem: 'Fixture <Alpha>', lastEventType: 'Location', lastEventAt: '', importInProgress: false, eventsRead: 3 }) {
  const filename = path.resolve(__dirname, '../../src/client/components/go-journal-status.js')
  const code = transformSync(fs.readFileSync(filename, 'utf8'), { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
  const states = []; const effects = []; const pending = new Map(); const calls = []; const requests = []
  let cursor = 0; let listener; let unsubscribed = 0; let tree
  const mod = new Module(filename, module); mod.filename = filename; mod.paths = module.paths
  mod.require = name => name === 'react'
    ? {
        ...React,
        useState: initial => { const index = cursor++; if (!(index in states)) states[index] = initial; return [states[index], next => { states[index] = typeof next === 'function' ? next(states[index]) : next }] },
        useRef: initial => { const index = cursor++; if (!(index in states)) states[index] = { current: initial }; return states[index] },
        useEffect: (callback, deps) => { const index = cursor++; if (!effects[index] || deps.some((v, i) => v !== effects[index].deps[i])) pending.set(index, { callback, deps }) }
      }
    : require(name)
  mod._compile(`module.exports = function (window) { const module = { exports: {} }; const exports = module.exports; ${code}; return module.exports }`, filename)
  const browser = { fetch: async (url, options) => { requests.push({ url, options }); return { ok: true, json: async () => ({ Directory: 'C:\\fixture\\journals', Override: '', Error: '' }) } } }
  const component = mod.exports(browser).default
  const props = { role, connected: true, ready: true, sendEvent: async name => { calls.push(name); return initial }, eventListener: (name, callback) => { assert.equal(name, 'journalSummaryChanged'); listener = callback; return () => { unsubscribed++ } } }
  const render = () => { cursor = 0; tree = component(props); return renderToStaticMarkup(tree) }
  return {
    props,
    browser,
    calls,
    requests,
    html: render,
    async flush () { for (const [index, effect] of pending) { effects[index]?.cleanup?.(); effects[index] = { deps: effect.deps, cleanup: effect.callback() } } pending.clear(); await new Promise(resolve => setImmediate(resolve)); return render() },
    broadcast (value) { listener(value) },
    elements (type) { const found = []; const visit = node => { if (!React.isValidElement(node)) return; if (node.type === type) found.push(node); React.Children.forEach(node.props.children, visit) }; visit(tree); return found },
    get unsubscribed () { return unsubscribed }
  }
}

test('G2A owner and viewer show sanitized current system without inventing game features', async () => {
  for (const role of ['owner', 'viewer']) {
    const h = harness(role); h.html(); const html = await h.flush()
    assert.match(html, /Fixture &lt;Alpha&gt;/)
    assert.match(html, /Location/)
    assert.match(html, /provider uploads/i)
    assert.deepEqual(h.calls, ['getJournalSummary'])
    if (role === 'viewer') { assert.equal(h.requests.length, 0); assert.doesNotMatch(html, /C:|Journal directory|Save journal directory/) } else assert.match(html, /Save journal directory/)
  }
})

test('G2A missing source and disconnect are distinct from service readiness', async () => {
  const h = harness('viewer', { sourceState: 'waiting', currentSystem: '', eventsRead: 0 }); h.html()
  assert.match(await h.flush(), /Waiting for Elite Dangerous journal/)
  h.broadcast({ sourceState: 'permission_error', directory: 'PRIVATE_PATH', currentSystem: '' })
  assert.match(h.html(), /permission|unavailable/i); assert.doesNotMatch(h.html(), /PRIVATE_PATH/)
  h.props.connected = false; h.props.ready = false; h.html(); const html = await h.flush()
  assert.match(html, /disconnected/i); assert.equal(h.unsubscribed, 1)
  assert.equal(h.calls.length, 1, 'disconnected browser started another poll')
})

test('G2A owner directory control uses authenticated local POST and viewers have none', async () => {
  const h = harness('owner'); h.html(); await h.flush()
  const input = h.elements('input')[0]; input.props.onChange({ target: { value: 'D:\\Synthetic Journals' } }); h.html()
  await h.elements('form')[0].props.onSubmit({ preventDefault () {} })
  const request = h.requests.at(-1)
  assert.equal(request.url, '/owner/journal'); assert.equal(request.options.method, 'POST'); assert.equal(request.options.credentials, 'same-origin')
  assert.deepEqual(JSON.parse(request.options.body), { action: 'selectDir', directory: 'D:\\Synthetic Journals' })
})

test('G2A replay status does not claim a game connection or finished dashboard', async () => {
  const h = harness('viewer', { sourceState: 'replaying', currentSystem: '', importInProgress: true, eventsRead: 100 }); h.html(); const html = await h.flush()
  assert.match(html, /Replay|replay/)
  assert.match(html, /100/)
  assert.doesNotMatch(html, /Game connected|Commander:|exploration is ready/i)
})

test('G2A live summary updates without browser polling', async () => {
  const h = harness(); h.html(); await h.flush()
  h.broadcast({ sourceState: 'live', currentSystem: 'Fixture Beta', lastEventType: 'FSDJump', eventsRead: 4 })
  assert.match(h.html(), /Fixture Beta/)
  assert.equal(h.calls.length, 1)
})

test('G2A successful reconnect clears a failed journal request alert', async () => {
  const h = harness()
  h.props.sendEvent = async () => { throw new Error('synthetic outage') }
  h.html(); assert.match(await h.flush(), /Journal status unavailable/)
  h.props.connected = false; h.props.ready = false; h.html(); await h.flush()
  h.props.sendEvent = async () => ({ sourceState: 'live', currentSystem: 'Fixture Recovered' })
  h.props.connected = true; h.props.ready = true; h.html()
  const html = await h.flush()
  assert.match(html, /Fixture Recovered/)
  assert.doesNotMatch(html, /Journal status unavailable/)
})

test('G2A delayed initial owner status cannot replace a newer directory selection', async () => {
  const h = harness('owner')
  let resolveStatus
  h.browser.fetch = async (url, options) => {
    const body = JSON.parse(options.body)
    if (body.action === 'status') return new Promise(resolve => { resolveStatus = resolve })
    return { ok: true, json: async () => ({ Directory: 'D:\\NEW', Override: 'D:\\NEW' }) }
  }
  h.html(); await h.flush()
  h.elements('input')[0].props.onChange({ target: { value: 'D:\\NEW' } }); h.html()
  await h.elements('form')[0].props.onSubmit({ preventDefault () {} })
  resolveStatus({ ok: true, json: async () => ({ Directory: 'C:\\OLD', Override: 'C:\\OLD' }) })
  const html = await h.flush()
  assert.match(html, /D:\\NEW/)
  assert.doesNotMatch(html, /C:\\OLD/)
})

test('G2B source card describes the supported survey without claiming full exploration parity', async () => {
  const h = harness('viewer'); h.props.stage = 'G2B'; h.html(); const html = await h.flush()
  assert.match(html, /navigation route and current-system survey are journal-only/i)
  assert.doesNotMatch(html, /Exploration, ship and engineering features remain unimplemented/)
})
