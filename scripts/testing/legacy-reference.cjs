// Render actual legacy panel components with fabricated data; never start Node's
// game service. This reference HTML is test-only and is not part of preview ZIPs.
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')
function legacyReference (root) {
  const cache = new Map()
  const load = filename => {
    if (cache.has(filename)) return cache.get(filename)
    if (filename.endsWith('.json')) return require(filename)
    const code = transformSync(fs.readFileSync(filename, 'utf8'), { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
    const mod = new Module(filename, module); mod.paths = module.paths
    mod.require = name => {
      if (name === 'next/router') return { useRouter: () => ({ pathname: '/nav/map', push: () => {} }) }
      if (name === 'lib/socket') return { useSocket: () => ({ connected: true, ready: true, active: false }), socketOptions: {}, sendEvent: async () => null }
      if (name === 'lib/window') return {}
      if (name === 'lib/use-audio-toggle') return { __esModule: true, default: () => ({ audioEnabled: false }) }
      if (name === 'lib/use-landing-pad') return { __esModule: true, default: () => ({ landingPadEnabled: false }) }
      if (['components/settings', 'components/landing-pad-overlay', 'lib/notification'].includes(name)) return { __esModule: true, default: () => null, Settings: () => null }
      if (name.startsWith('components/') || name.startsWith('lib/')) return load(path.join(root, 'src/client', name + '.js'))
      if (name.startsWith('.')) { const target = path.resolve(path.dirname(filename), name); return load(path.extname(target) ? target : target + '.js') }
      return require(name)
    }
    mod._compile(code, filename); cache.set(filename, mod.exports); return mod.exports
  }
  const component = name => load(path.join(root, 'src/client/components', name + '.js')).default
  const SystemMap = require(path.join(root, 'src/service/lib/system-map.js'))
  const bodies = [{ id: 1000, id64: '1000', bodyId: 0, name: 'Fixture Alpha', type: 'Star', subType: 'G (White-Yellow) Star', radius: 696340, distanceToArrival: 0, parents: [] }, { id: 1001, id64: '1001', bodyId: 1, name: 'Fixture Alpha 1', type: 'Planet', subType: 'Rocky body', radius: 1000, distanceToArrival: 850, isLandable: true, gravity: 1, temperature: 250, atmosphereType: 'Thin carbon dioxide atmosphere', volcanismType: 'No volcanism', parents: [{ Star: 0 }], signals: { biological: 3 } }, { id: 1002, id64: '1002', bodyId: 2, name: 'Fixture Alpha 1 a', type: 'Planet', subType: 'Icy body', radius: 750, distanceToArrival: 1200, isLandable: true, parents: [{ Planet: 1 }, { Star: 0 }] }]
  const map = new SystemMap({ name: 'Fixture Alpha', bodies, stations: [], scanPercentComplete: 1, isCurrentLocation: true, distance: 0 })
  const system = { ...map.detail, ...map, bodies: map.objectsInSystem.filter(b => b.type === 'Star' || b.type === 'Planet') }
  const Header = component('header'); const Layout = component('layout'); const Panel = component('panel'); const MapPanel = component('panels/nav/navigation-system-map-panel'); const Inspector = component('panels/nav/navigation-inspector-panel')
  const nav = load(path.join(root, 'src/client/lib/navigation-items.js')).NavPanelNavItems('Map', {}).filter(i => i.url)
  const html = renderToStaticMarkup(React.createElement(React.Fragment, null, React.createElement(Header), React.createElement(Layout, { connected: true, ready: true }, React.createElement(Panel, { navigation: nav }, React.createElement(MapPanel, { system, systemObject: system.bodies[1], setSystemObject: () => {}, cmdrStatus: { flags: { fsdJump: false } } }), React.createElement(Inspector, { systemObject: system.bodies[1], setSystemObjectByName: () => {} })))))
  const styles = fs.readdirSync(path.join(root, 'build/client/_next/static/css')).filter(n => n.endsWith('.css')).map(n => `<link rel="stylesheet" href="/_next/static/css/${n}">`).join('')
  return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">${styles}<title>Legacy DAEDALUS — fabricated reference</title></head><body>${html}</body></html>`
}
module.exports = { legacyReference }
