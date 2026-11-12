// Owner settings need both the sanitized save response and the fresh identity
// status. Do not rely on a racing preferences broadcast to refresh the input.
async function saveIntegrationPreference ({ sendEvent, preferences, key, value }) {
  const next = JSON.parse(JSON.stringify(preferences || {}))
  next.integrations = { ...next.integrations, [key]: value }
  const saved = await sendEvent('setPreferences', next)
  const status = await sendEvent('getIntegrationStatus')
  return { preferences: saved, status }
}

module.exports = { saveIntegrationPreference }
