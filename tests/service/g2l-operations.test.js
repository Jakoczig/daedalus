const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

function harness (panel = 'go-operations', initial = {}) {
  const states = []; const effects = []; const pending = new Map(); const calls = []; const listeners = new Map(); const copied = []
  let cursor = 0; let tree; let unsubscribed = 0
  const react = {
    ...React,
    useState: initial => { const i = cursor++; if (!(i in states)) states[i] = initial; return [states[i], next => { states[i] = typeof next === 'function' ? next(states[i]) : next }] },
    useRef: initial => { const i = cursor++; if (!(i in states)) states[i] = { current: initial }; return states[i] },
    useEffect: (callback, deps) => { const i = cursor++; if (!effects[i] || deps.some((v, j) => v !== effects[i].deps[j])) pending.set(i, { callback, deps }) }
  }
  const browser = { clipboard: { writeText: async value => { copied.push(value) } } }
  const cache = new Map()
  const load = filename => {
    if (cache.has(filename)) return cache.get(filename)
    const code = transformSync(fs.readFileSync(filename, 'utf8'), { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
    const mod = new Module(filename, module); mod.filename = filename; mod.paths = module.paths
    mod.require = name => name === 'react' ? react : name.startsWith('.') ? load(path.resolve(path.dirname(filename), name + '.js')) : require(name)
    mod._compile(`module.exports = function (navigator) { const module = { exports: {} }; const exports = module.exports; ${code}; return module.exports }`, filename)
    const result = mod.exports(browser); cache.set(filename, result); return result
  }
  const component = load(path.resolve(__dirname, '../../src/client/components/' + panel + '.js')).default
  const props = { connected: true, ready: true, role: 'viewer', sendEvent: async name => { calls.push(name); return initial }, eventListener: (name, callback) => { listeners.set(name, callback); return () => { listeners.delete(name); unsubscribed++ } } }
  const render = () => { cursor = 0; tree = component(props); return renderToStaticMarkup(tree) }
  return {
    props,
    calls,
    browser,
    copied,
    html: render,
    async flush () { for (const [i, e] of pending) { effects[i]?.cleanup?.(); effects[i] = { deps: e.deps, cleanup: e.callback() } } pending.clear(); await new Promise(resolve => setImmediate(resolve)); return render() },
    broadcast (value) { for (const cb of listeners.values()) cb(value) },
    unmount () { for (const e of effects) e?.cleanup?.(); pending.clear() },
    get unsubscribed () { return unsubscribed },
    elements (type) { const found = []; const visit = node => { if (!React.isValidElement(node)) return; if (node.type === type) found.push(node); React.Children.forEach(node.props.children, visit) }; visit(tree); return found }
  }
}

test('G2L viewer screens do not request private operations or providers', async () => {
  const h = harness(); h.props.path = '/ship/fleet'; h.props.connected = true; h.props.ready = true
  assert.match(h.html(), /LOCAL OWNER ONLY/); await h.flush(); assert.deepEqual(h.calls, [])
})
test('G2L owner uses only bounded local reads, preserves exact IDs and stops after unmount', async () => {
  const h = harness('go-operations', { rows: [{ id: '1', entity: '18446744073709551615', title: 'Fixture Explorer', at: '2026-01-01', state: 'last_known', kind: 'Fleet', details: { shipName: 'Fixture Explorer', modelLabel: 'Asp Explorer', Value: '9007199254740993' } }], next: '', notice: 'Observed journal' })
  h.props.path = '/ship/fleet'; h.props.role = 'owner'; h.props.ready = true
  h.html(); const html = await h.flush(); h.unmount()
  assert.match(html, /18446744073709551615/); assert.match(html.replace(/[ ,\u00a0]/g, ''), /9007199254740993/)
  assert.ok(h.calls.includes('getGoOperations')); assert.ok(!h.calls.includes('getGoDataStatus'), 'Fleet must not request other commander identities')
  assert.ok(h.calls.every(n => ['getGoOperations', 'getGoDataStatus', 'getGoProviders'].includes(n)))
})

test('CG membership distinguishes known empty, unknown and historical goals without losing rewards', async () => {
  for (const known of [false, true]) {
    const h = harness('go-operations', { rows: [{ id: '1', entity: '9007199254740993', title: 'CommunityGoalReward', kind: 'CommunityGoalReward', at: '2026-01-01', state: 'historical_goal_event', details: { Reward: '100' } }], current: [{ id: '2', entity: '9007199254740993', title: 'progress_snapshot', at: '2026-01-01', state: known ? 'historical_goal' : 'membership_unknown_no_complete_snapshot', details: {} }], goalMembership: { known, ids: [], observedAt: '2026-01-02', notice: 'Complete snapshot; historical rewards retained' } })
    h.props.path = '/cmdr/community-goals'; h.props.role = 'owner'; h.props.ready = true
    h.html(); const html = await h.flush(); h.unmount()
    assert.match(html, known ? /0 subscribed goals as last witnessed/ : /Subscribed goals unknown/)
    assert.match(html, /Received reward 100/); assert.match(html, /9007199254740993/)
    assert.match(html, /Historical participation/)
  }
})
