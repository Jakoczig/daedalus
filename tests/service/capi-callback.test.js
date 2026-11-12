const test = require('node:test')
const assert = require('node:assert/strict')
const Integrations = require('../../src/service/lib/event-handlers/integrations')

test('wrong or missing OAuth state cannot consume pending owner authentication', async () => {
  const integration = Object.create(Integrations.prototype)
  integration._capiState = 'synthetic-state'
  const pending = integration._waitForCapiCallback()
  const receiveAuth = global.HANDLE_CAPI_CALLBACK
  receiveAuth({ code: 'synthetic-code', state: 'wrong' })
  assert.equal(global.HANDLE_CAPI_CALLBACK, receiveAuth)
  receiveAuth({ error: 'denied' })
  assert.equal(global.HANDLE_CAPI_CALLBACK, receiveAuth)
  receiveAuth({ code: 'synthetic-code', state: 'synthetic-state' })
  assert.deepEqual(await pending, { code: 'synthetic-code', state: 'synthetic-state' })
  assert.equal(global.HANDLE_CAPI_CALLBACK, null)
})
