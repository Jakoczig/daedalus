const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')

function load (snapshot, fetch) {
  const filename = path.resolve(__dirname, '../../src/client/components/remote-displays.js')
  const code = transformSync(fs.readFileSync(filename, 'utf8'), { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
  const mod = new Module(filename, module)
  mod.filename = filename; mod.paths = module.paths
  let call = 0
  mod.require = name => name === 'react' ? { ...React, useState: initial => [call++ === 0 ? snapshot : initial, () => {}], useEffect: () => {} } : require(name)
  mod._compile(`module.exports = function (window) { const module = { exports: {} }; const exports = module.exports; ${code}; return module.exports }`, filename)
  return mod.exports({ fetch })
}

test('owner remote settings explain TLS trust and expose only safe device controls', () => {
  const mod = load({ enabled: true, url: 'https://192.168.1.10:3443', ca: 'PUBLIC CERTIFICATE', fingerprint: 'fixture-fingerprint', devices: [{ id: 'fixture', name: '<tablet>', expires: '2026-11-12' }] })
  const html = renderToStaticMarkup(React.createElement(mod.default))
  assert.match(html, /Disable remote displays/)
  assert.match(html, /Create pairing code/)
  assert.match(html, /Rename/)
  assert.match(html, /Revoke/)
  assert.match(html, /Refresh devices/)
  assert.match(html, /certificate/i)
  assert.match(html, /fixture-fingerprint/)
  assert.match(html, /&lt;tablet&gt;/)
  assert.match(html, /https:\/\/192.168.1.10:3443/)
  assert.doesNotMatch(html, /PRIVATE KEY|Skip verification/)
})

test('owner action uses same-origin authenticated POST and sanitized failures', async () => {
  let captured
  const mod = load({}, async (url, options) => { captured = { url, options }; return { ok: true, json: async () => ({ enabled: false }) } })
  assert.deepEqual(await mod.remoteAction('disable'), { enabled: false })
  assert.equal(captured.url, '/owner/remote')
  assert.equal(captured.options.method, 'POST')
  assert.equal(captured.options.credentials, 'same-origin')
  assert.deepEqual(JSON.parse(captured.options.body), { action: 'disable' })
  const denied = load({}, async () => ({ ok: false, text: async () => 'PRIVATE KEY leaked by fixture' }))
  await assert.rejects(denied.remoteAction('enable'), error => !error.message.includes('PRIVATE KEY') && /failed/i.test(error.message))
})

test('recovery warning remains visible without opening settings and prevents unsafe enable', () => {
  const mod = load({ enabled: false, recoveryRequired: true, notice: 'Remote displays disabled. Local desktop available; back up and repair the Go profile.', devices: [] })
  const html = renderToStaticMarkup(React.createElement(mod.default))
  assert.match(html, /role="alert"/)
  assert.ok(html.indexOf('back up and repair') < html.indexOf('<details'), 'recovery warning hidden in collapsed settings')
  assert.doesNotMatch(html, /Enable HTTPS remote displays/)
})
