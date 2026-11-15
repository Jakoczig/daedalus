const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

function renderHistory (role) {
  let stateIndex = 0
  function load (name) {
    const filename = path.resolve(__dirname, '../../src/client/components', name + '.js')
    const code = transformSync(fs.readFileSync(filename, 'utf8'), { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
    const mod = new Module(filename, module); mod.filename = filename; mod.paths = module.paths
    mod.require = name => {
      if (name === 'react') return { ...React, useState: value => React.useState(stateIndex++ === 0 ? true : value) }
      if (name === './go-history-controls') return load('go-history-controls')
      return require(name)
    }
    mod._compile(code, filename)
    return mod.exports
  }
  return renderToStaticMarkup(React.createElement(load('go-data-history').default, { role, sendEvent: () => { throw new Error('Unexpected request during render') } }))
}

test('After sale is explicitly a manual hold and source fetch time is not observation time', () => {
  const owner = renderHistory('owner')
  assert.match(owner, /After sale[^<]*hold \+ manual review only/)
  assert.match(owner, /No automatic sale-triggered uploads/)
  assert.match(owner, /newer source observation/)
  assert.match(owner, /Manual review cannot authorize unsupported delayed recipients/)
  assert.equal(renderHistory('viewer'), '')
})
