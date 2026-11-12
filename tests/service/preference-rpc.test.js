const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const os = require('node:os')
const path = require('node:path')
const { createPreferencesStore } = require('../../src/service/lib/preferences-store')
global.BROADCAST_EVENT = () => {}
const EventHandlers = require('../../src/service/lib/event-handlers')

test('real preference RPC never returns stored credentials and preserves them on full-view saves', async t => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'daedalus-rpc-'))
  t.after(() => fs.rm(dir, { recursive: true, force: true }))
  const store = createPreferencesStore(path.join(dir, 'Preferences.json'))
  await store.update(() => ({ integrations: { inaraApiKey: 'synthetic-inara', capiRefreshToken: 'synthetic-refresh' } }))
  const instance = Object.assign(Object.create(EventHandlers.prototype), {
    preferencesStore: store,
    _handlerInstances: [],
    textToSpeech: { invalidatePreferencesCache () {} },
    integrations: { notifyPreferencesChanged () {}, disconnectCapi: () => store.disconnectCapi() }
  })
  const handlers = instance.getEventHandlers()
  const before = await handlers.getPreferences()
  assert.equal(JSON.stringify(before).includes('synthetic'), false)
  const result = await handlers.setIntegrationSecret({ provider: 'edsm', secret: 'synthetic-edsm' })
  assert.deepEqual(result, { configured: true })
  const saved = await handlers.setPreferences({ ...before, covasExtendedEnabled: true })
  assert.equal(JSON.stringify(saved).includes('synthetic'), false)
  const raw = await store.read()
  assert.equal(raw.integrations.inaraApiKey, 'synthetic-inara')
  assert.equal(raw.integrations.capiRefreshToken, 'synthetic-refresh')
  assert.equal(raw.integrations.edsmApiKey, 'synthetic-edsm')
  await handlers.disconnectCapi()
  assert.equal((await store.read()).integrations.capiRefreshToken, undefined)
})
