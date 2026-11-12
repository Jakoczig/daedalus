const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const os = require('node:os')
const path = require('node:path')
const { createPreferencesStore } = require('../../src/service/lib/preferences-store')
const Integrations = require('../../src/service/lib/event-handlers/integrations')

async function fixture (t) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'daedalus-capi-'))
  t.after(() => fs.rm(dir, { recursive: true, force: true }))
  const store = createPreferencesStore(path.join(dir, 'Preferences.json'))
  const raw = await store.update(() => ({ integrations: { capiEnabled: true, capiRefreshToken: 'synthetic-original' } }))
  let respond, started
  const requestStarted = new Promise(resolve => { started = resolve })
  const response = new Promise(resolve => { respond = resolve })
  const integration = Object.assign(Object.create(Integrations.prototype), {
    preferencesStore: store,
    _prefs: raw,
    _capiGeneration: 0,
    _capiHttp: { post: () => { started(); return response } },
    _openCapiAuthUrl () {}
  })
  return { store, integration, requestStarted, respond }
}

test('disconnect prevents an in-flight refresh response from resurrecting credentials', async t => {
  const { store, integration, requestStarted, respond } = await fixture(t)
  const pending = integration._getCapiAccessToken()
  await requestStarted
  await integration.disconnectCapi()
  respond({ data: { access_token: 'synthetic-access', refresh_token: 'synthetic-rotated' } })
  await assert.rejects(pending, /Authentication changed/)
  assert.equal((await store.read()).integrations.capiRefreshToken, undefined)
  assert.equal(integration._capiAccessToken, null)
})

test('disconnect prevents a completed OAuth exchange from restoring a removed account', async t => {
  const { store, integration, requestStarted, respond } = await fixture(t)
  const pending = integration.beginCapiAuth()
  global.HANDLE_CAPI_CALLBACK({ code: 'synthetic-code', state: integration._capiState })
  await requestStarted
  await integration.disconnectCapi()
  respond({ data: { access_token: 'synthetic-access', refresh_token: 'synthetic-new' } })
  await assert.rejects(pending, /Authentication changed/)
  assert.equal((await store.read()).integrations.capiRefreshToken, undefined)
})
