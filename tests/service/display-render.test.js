const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

// Exercise the actual page JSX with deterministic service snapshots. Only app
// chrome/socket transport are replaced; no browser, OS helper or network runs.
function render (page, role, firstState) {
  const filename = path.resolve(__dirname, `../../src/client/pages/media/${page}.js`)
  const compiled = transformSync(fs.readFileSync(filename, 'utf8'), {
    babelrc: false,
    configFile: false,
    presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]],
    plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')]
  }).code
  const mod = new Module(filename, module)
  mod.filename = filename
  mod.paths = module.paths
  let stateIndex = 0
  mod.require = name => {
    if (name === 'react') {
      return {
        ...React,
        useState: value => {
          const index = stateIndex++
          if (page === 'galnet' && index === 2) value = false // Loaded fixture, not an active fetch.
          return React.useState(index === 0 && firstState !== undefined ? firstState : value)
        }
      }
    }
    if (name === 'lib/socket') return { useSocket: () => ({ role, connected: true, ready: true }), sendEvent: () => { throw new Error('Unexpected RPC during render') }, eventListener: () => () => {} }
    if (name === 'components/layout' || name === 'components/panel') return ({ children }) => React.createElement('div', null, children)
    if (name === 'lib/navigation-items') return { MediaPanelNavItems: () => [] }
    if (name === 'lib/video-playback') return require('../../src/client/lib/video-playback')
    return require(name)
  }
  mod._compile(compiled, filename)
  return renderToStaticMarkup(React.createElement(mod.exports.default))
}

test('music observation renders for a viewer with host transport disabled; owner controls remain usable', () => {
  const state = { current: { title: 'Synthetic Track', artist: 'Synthetic', playbackStatus: 'Playing', controls: { canPrevious: true, canNext: true, canPlay: true, canPause: true } } }
  const viewer = render('music', 'viewer', state)
  const owner = render('music', 'owner', state)
  assert.match(viewer, /Synthetic Track/)
  const viewerButtons = viewer.match(/<button[^>]*music-panel__control-button[^>]*>/g)
  const ownerButtons = owner.match(/<button[^>]*music-panel__control-button[^>]*>/g)
  assert.equal(viewerButtons.length, 3)
  assert.ok(viewerButtons.every(button => button.includes('disabled=')))
  assert.ok(ownerButtons.every(button => !button.includes('disabled=')))
})

test('GalNet remains readable for viewers with forced refresh disabled', () => {
  const articles = [{ id: 'synthetic', title: 'Synthetic GalNet Article', content: 'Fixture text' }]
  assert.match(render('galnet', 'viewer', articles), /Synthetic GalNet Article/)
  assert.match(render('galnet', 'viewer', articles), /<button[^>]*disabled=""[^>]*title="Forced refresh/)
  assert.match(render('galnet', 'owner', articles), /<button title="Refresh articles"/)
})

test('viewer video page offers browsing and explains the local-download boundary', () => {
  const html = render('video', 'viewer')
  assert.match(html, /Search YouTube or paste a link/)
  assert.match(html, /Downloads are managed on the PC/)
})
