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
    useMemo: callback => callback(),
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
    mod.require = name => {
      if (name === 'react') return react
      if (name === 'next/router') return { useRouter: () => ({ pathname: '/nav/map', push: () => {}, replace: () => {} }) }
      if (name.startsWith('.') || (name.startsWith('lib/') || name.startsWith('components/'))) {
        const target = (name.startsWith('lib/') || name.startsWith('components/')) ? path.resolve(__dirname, '../../src/client', name) : path.resolve(path.dirname(filename), name)
        return target.endsWith('.json') ? require(target) : load(target + '.js')
      }
      return require(name)
    }
    mod._compile(`module.exports = function (navigator) { const document = { addEventListener () {}, removeEventListener () {} }; const sessionStorage = { setItem () {}, getItem () { return null } }; const module = { exports: {} }; const exports = module.exports; ${code}; return module.exports }`, filename)
    const result = mod.exports(browser); cache.set(filename, result); return result
  }
  const component = load(path.resolve(__dirname, '../../src/client/components/' + panel + '.js')).default
  const props = { connected: true, ready: true, role: 'viewer', sendEvent: async name => { calls.push(name); return typeof initial === 'function' ? initial(name) : initial }, eventListener: (name, callback) => { listeners.set(name, callback); return () => { listeners.delete(name); unsubscribed++ } } }
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

module.exports = { harness, navigation, exploration }
