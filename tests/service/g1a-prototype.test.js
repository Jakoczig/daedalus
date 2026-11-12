const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

function render (prototype, child) {
  const filename = path.resolve(__dirname, '../../src/client/lib/socket.js')
  const code = transformSync(fs.readFileSync(filename, 'utf8'), {
    babelrc: false,
    configFile: false,
    presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]],
    plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')]
  }).code
  const mod = new Module(filename, module)
  mod.filename = filename
  mod.paths = module.paths
  mod.require = name => {
    if (name === 'react') return { ...React, useState: initial => React.useState({ ...initial, role: 'owner', connected: true, prototype }) }
    if (name === 'next/router') return { __esModule: true, default: {}, useRouter: () => ({ pathname: '/exploration/system' }) }
    if (name === 'lib/notification') return () => {}
    if (name === 'components/primary-navigation') return () => React.createElement('nav', null, 'Existing primary navigation')
    if (name === 'components/layout' || name === 'components/panel') return ({ children }) => React.createElement('div', null, children)
    if (name === 'lib/navigation-items') return require('../../src/client/lib/navigation-items')
    return require(name)
  }
  mod._compile(code, filename)
  return renderToStaticMarkup(React.createElement(mod.exports.SocketProvider, null, child))
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
  assert.match(render(null, React.createElement('p', null, 'Existing live interface')), /Existing live interface/)
})
