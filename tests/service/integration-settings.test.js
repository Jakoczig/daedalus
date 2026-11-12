const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const os = require('node:os')
const path = require('node:path')
const { createPreferencesStore } = require('../../src/service/lib/preferences-store')
const { saveIntegrationPreference } = require('../../src/client/lib/integration-settings')
global.BROADCAST_EVENT = () => {}
const EventHandlers = require('../../src/service/lib/event-handlers')
const Integrations = require('../../src/service/lib/event-handlers/integrations')

test('saving and reopening owner integration settings shows the new commander without returning credentials', async t => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'daedalus-settings-'))
  t.after(() => fs.rm(dir, { recursive: true, force: true }))
  const store = createPreferencesStore(path.join(dir, 'Preferences.json'))
  const raw = await store.update(() => ({ integrations: { edsmCmdrName: 'Old Synthetic', edsmApiKey: 'synthetic-secret' } }))
  const integrations = Object.assign(Object.create(Integrations.prototype), { _prefs: raw })
  const instance = Object.assign(Object.create(EventHandlers.prototype), {
    preferencesStore: store,
    _handlerInstances: [integrations],
    integrations,
    textToSpeech: { invalidatePreferencesCache () {} }
  })
  const handlers = instance.getEventHandlers()
  const sendEvent = (name, message) => handlers[name](message)
  const result = await saveIntegrationPreference({ sendEvent, preferences: await sendEvent('getPreferences'), key: 'edsmCmdrName', value: 'New Synthetic' })
  assert.equal(result.status.edsm.cmdrName, 'New Synthetic')
  assert.equal(JSON.stringify(result).includes('synthetic-secret'), false)
  assert.equal((await sendEvent('getIntegrationStatus')).edsm.cmdrName, 'New Synthetic')
  assert.equal((await sendEvent('getPreferences')).integrations.edsmCmdrName, undefined)
})
