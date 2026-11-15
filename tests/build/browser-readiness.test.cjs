const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const { pageTextReady } = require('../../scripts/testing/g2d-visual.cjs')
test('browser navigation readiness safely waits through a missing body', () => {
  const expression = pageTextReady('Native Companion')
  for (const body of [null, undefined, { innerText: '' }, { innerText: 'Old page' }]) assert.equal(vm.runInNewContext(expression, { document: { body } }), false)
  assert.equal(vm.runInNewContext(expression, { document: { body: { innerText: 'NATIVE COMPANION' } } }), true)
})
test('service-ready navigation still requires both authenticated readiness and the requested page', () => {
  const expression = pageTextReady('Ship Statistics', true)
  for (const text of ['SERVICE READY', 'Ship Statistics', 'SERVICE READY wrong page']) assert.equal(vm.runInNewContext(expression, { document: { body: { innerText: text } } }), false)
  assert.equal(vm.runInNewContext(expression, { document: { body: { innerText: 'SERVICE READY Ship Statistics' } } }), true)
})
