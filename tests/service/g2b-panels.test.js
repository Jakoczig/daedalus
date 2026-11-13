const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

const navigation = { currentSystem: 'Fixture <Alpha>', targetSystem: 'Fixture Beta', destinationSystem: 'Fixture Gamma', routeState: 'verified', targetEvidence: 'journal_target', remainingJumps: 2, reportedRemainingJumps: 2, truncated: false, hops: [{ name: 'Fixture <Alpha>', starClass: 'G', distanceFromPreviousLY: null, isCurrent: true, isNext: false }, { name: 'Fixture Beta', starClass: 'M', distanceFromPreviousLY: 5, isCurrent: false, isNext: true }] }
const exploration = { systemName: 'Fixture Alpha', surveyState: 'observed', reportedBodyCount: 12, fssProgress: 0.25, allBodiesFound: false, scannerBodyCount: null, truncated: false, bodies: [{ bodyId: '9007199254740997', name: 'Fixture Alpha 1', kind: 'Rocky body', landable: null, terraformable: null, gravityG: null, distanceLS: null, bioSignals: 3, geoSignals: 0, previouslyMapped: true, previouslyDiscovered: null, dssMapped: false, genuses: ['Bacterium'] }] }

function harness (panel = 'go-navigation', initial = navigation) {
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
    mod.require = name => name === 'react' ? react : name.startsWith('./') ? load(path.resolve(path.dirname(filename), name + '.js')) : require(name)
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

test('TestG2BRoutePageUsesOnlyGoProjection', async () => {
  const h = harness(); h.html(); const html = await h.flush()
  assert.deepEqual(h.calls, ['getGoNavigation']); assert.match(html, /Fixture &lt;Alpha&gt;/); assert.match(html, /Fixture Beta/); assert.match(html, /Fixture Gamma/); assert.match(html, /5.*LY/); assert.match(html, /journal.only/i); assert.match(html, /Current|Next/)
  assert.doesNotMatch(html, /EDSM|Spansh|automatically target|credits|scoopable/i)
})
test('TestG2BExplorationPageDoesNotInventSpeciesOrRevenue', async () => {
  const h = harness('go-exploration-system', exploration); h.html(); const html = await h.flush()
  assert.deepEqual(h.calls, ['getGoExploration']); assert.match(html, /12/); assert.match(html, /25%/); assert.match(html, /Unknown/); assert.match(html, /3/); assert.match(html, /Bacterium/); assert.match(html, /genus/i); assert.match(html, /Previously mapped/); assert.match(html, /DSS/)
  assert.doesNotMatch(html, /Estimated|payout|credits|first footfall|predicted species|you mapped it/i)
  assert.equal(h.elements('tr').length, 2)
})
test('TestG2BMissingOrClearedRouteStates', async () => {
  const h = harness(); h.html(); await h.flush()
  for (const routeState of ['no_route', 'cleared', 'unverified', 'stale', 'partial', 'waiting_game']) { h.broadcast({ routeState, hops: [], targetEvidence: 'unknown', truncated: routeState === 'partial' }); const html = h.html(); assert.match(html, /No route|cleared|unverified|stale|partial|Waiting/i); assert.doesNotMatch(html, /Fixture Beta|Fixture Gamma/) }
})
test('TestG2BDisconnectAndReconnectNoStaleResponse', async () => {
  for (const [panel, initial] of [['go-navigation', navigation], ['go-exploration-system', exploration]]) {
    const h = harness(panel, initial); let resolveOld
    h.props.sendEvent = () => new Promise(resolve => { resolveOld = resolve })
    h.html(); await h.flush(); h.props.connected = false; h.props.ready = false
    assert.doesNotMatch(h.html(), /Fixture/); assert.match(await h.flush(), /disconnected/i)
    h.props.sendEvent = async () => initial; h.props.connected = true; h.props.ready = true; h.html(); await h.flush()
    resolveOld({ ...initial, currentSystem: 'OLD_SESSION', systemName: 'OLD_SESSION' }); const html = await h.flush(); assert.doesNotMatch(html, /OLD_SESSION/); assert.match(html, /Fixture/); assert.equal(h.unsubscribed, 1)
    h.broadcast({ ...initial, currentSystem: 'Live newest', systemName: 'Live newest' }); assert.match(h.html(), /Live newest/)
    h.unmount(); assert.equal(h.unsubscribed, 2)
  }
})
test('TestG2BViewerCopyIsLocalOnly', async () => {
  const h = harness(); h.html(); await h.flush(); await h.elements('button')[0].props.onClick(); assert.deepEqual(h.copied, ['Fixture <Alpha>']); assert.deepEqual(h.calls, ['getGoNavigation'])
  h.browser.clipboard.writeText = async () => { throw new Error('denied') }; await h.elements('button')[0].props.onClick(); assert.match(h.html(), /select.*name|copy unavailable/i)
})
test('G2B live notification wins over delayed bootstrap reply', async () => {
  const h = harness(); let resolveReply
  h.props.sendEvent = () => new Promise(resolve => { resolveReply = resolve }); h.html(); await h.flush(); h.broadcast({ ...navigation, currentSystem: 'Newest live system' }); resolveReply({ ...navigation, currentSystem: 'Stale bootstrap' })
  const html = await h.flush(); assert.match(html, /Newest live system/); assert.doesNotMatch(html, /Stale bootstrap/)
})
