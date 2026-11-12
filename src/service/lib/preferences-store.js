const fs = require('fs').promises
const path = require('path')
const os = require('os')

const PUBLIC_KEYS = new Set([
  'covasVoiceoverEnabled', 'covasExtendedEnabled', 'soundsEnabled',
  'explorationMinBodyValue', 'explorationMinBioValue', 'explorationIncludeNonValuable',
  'voicepackDir', 'covasDir'
])
const INTEGRATION_KEYS = new Set(['edsmEnabled', 'eddnEnabled', 'inaraEnabled', 'capiEnabled', 'spanshEnabled'])
const SECRET_KEYS = { edsm: 'edsmApiKey', inara: 'inaraApiKey' }

function publicPreferences (raw = {}) {
  const result = {}
  for (const key of PUBLIC_KEYS) {
    if (!Object.hasOwn(raw, key)) continue
    if (key === 'soundsEnabled') {
      result[key] = Object.fromEntries(Object.entries(raw[key] || {}).filter(([name, value]) => /^[a-z0-9_]+$/i.test(name) && typeof value === 'boolean'))
    } else if (['voicepackDir', 'covasDir'].includes(key) && typeof raw[key] === 'string') result[key] = raw[key]
    else if (['boolean', 'number'].includes(typeof raw[key])) result[key] = raw[key]
  }
  result.integrations = Object.fromEntries(Object.entries(raw.integrations || {}).filter(([key, value]) => INTEGRATION_KEYS.has(key) && typeof value === 'boolean'))
  return result
}

function createPreferencesStore (filename) {
  let pending = Promise.resolve()
  async function read () {
    try { return JSON.parse(await fs.readFile(filename, 'utf8')) } catch (e) {
      if (e.code === 'ENOENT') return {}
      throw e
    }
  }
  function update (change) {
    const operation = pending.then(async () => {
      const next = await change(await read())
      await fs.mkdir(path.dirname(filename), { recursive: true })
      const temporary = `${filename}.tmp`
      await fs.writeFile(temporary, JSON.stringify(next), { mode: 0o600 })
      await fs.rename(temporary, filename)
      return next
    })
    pending = operation.catch(() => {})
    return operation
  }
  return {
    read,
    update,
    async savePublic (input) {
      if (!input || typeof input !== 'object' || Array.isArray(input)) throw new Error('Invalid preferences')
      const integration = input.integrations || {}
      for (const key of Object.keys(integration)) {
        if (!INTEGRATION_KEYS.has(key) && !['edsmCmdrName', 'inaraCmdrName'].includes(key)) throw new Error('Use the credential operation')
      }
      const safe = publicPreferences(input)
      const result = await update(raw => ({
        ...raw,
        ...Object.fromEntries(Object.entries(safe).filter(([key]) => key !== 'integrations')),
        integrations: {
          ...raw.integrations,
          ...safe.integrations,
          ...Object.fromEntries(['edsmCmdrName', 'inaraCmdrName'].filter(key => typeof integration[key] === 'string').map(key => [key, integration[key].slice(0, 128)]))
        }
      }))
      return publicPreferences(result)
    },
    setSecret (provider, secret) {
      if (!Object.hasOwn(SECRET_KEYS, provider) || typeof secret !== 'string' || secret.length > 4096) return Promise.reject(new Error('Invalid credential'))
      return update(raw => {
        const integrations = { ...raw.integrations }
        if (secret) integrations[SECRET_KEYS[provider]] = secret
        else delete integrations[SECRET_KEYS[provider]]
        return { ...raw, integrations }
      })
    },
    disconnectCapi () {
      return update(raw => {
        const integrations = { ...raw.integrations, capiEnabled: false }
        delete integrations.capiRefreshToken
        return { ...raw, integrations }
      })
    }
  }
}

const preferencesStore = createPreferencesStore(path.join(os.homedir(), 'AppData', 'Local', 'DAEDALUS Terminal', 'Preferences.json'))
module.exports = { preferencesStore, createPreferencesStore, publicPreferences }
