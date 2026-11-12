'use strict'

const crypto = require('crypto')
const { exec } = require('child_process')
const { preferencesStore } = require('../preferences-store')
const _axios = require('axios')

// Lazy-built axios instance — deferred so appVersion() resolves after package.json loads
let _axiosInstance = null
function axios () {
  if (!_axiosInstance) {
    _axiosInstance = _axios.create({ headers: { 'User-Agent': `DAEDALUS Terminal/${appVersion()}` } })
  }
  return _axiosInstance
}

// Frontier CAPI
const FRONTIER_AUTH_SERVER = 'https://auth.frontierstore.net'
const CAPI_SERVER = 'https://companion.orerve.net'
const CAPI_CLIENT_ID = 'bf038cd7-8bbf-45dd-bd78-0ceb05c58b95'
const CAPI_REDIRECT_URI = 'daedalus://capi-callback'

// External service endpoints
const EDSM_API_URL = 'https://www.edsm.net'
const EDDN_UPLOAD_URL = 'https://eddn.edcd.io:4430/upload/'
const INARA_API_URL = 'https://inara.cz/inapi/v1/'

// Journal events that EDSM is interested in receiving
const EDSM_JOURNAL_EVENTS = new Set([
  'Bounty', 'CapShipBond', 'CommitCrime', 'CommunityGoalJoin', 'CommunityGoalReward',
  'CrewAssign', 'CrewFire', 'CrewHire', 'CrimeVictim', 'Died', 'Docked',
  'DockingCancelled', 'DockingDenied', 'DockingGranted', 'DockingRequested',
  'DockingTimeout', 'EngineerApply', 'EngineerContribution', 'EngineerCraft',
  'EngineerLegacyConvert', 'EngineerProgress', 'EscapeInterdiction', 'FactionKillBond',
  'FSDJump', 'FSSAllBodiesFound', 'FSSDiscoveryScan', 'FSSBodySignals',
  'FSSSignalDiscovered', 'HeatWarning', 'HullDamage', 'Interdicted', 'Interdiction',
  'JetConeBoost', 'LaunchFighter', 'LaunchSRV', 'LeaveBody', 'Liftoff',
  'LoadGame', 'Location', 'MissionAbandoned', 'MissionAccepted', 'MissionCompleted',
  'MissionFailed', 'MissionRedirected', 'ModuleBuy', 'ModuleRetrieve', 'ModuleSell',
  'ModuleSellRemote', 'ModuleStore', 'ModuleSwap', 'MultiSellExplorationData',
  'NavBeaconScan', 'PowerplayCollect', 'PowerplayDefect', 'PowerplayDeliver',
  'PowerplayFastTrack', 'PowerplayJoin', 'PowerplayLeave', 'PowerplayMerge',
  'PowerplaySalary', 'PowerplayVote', 'PowerplayVoucher', 'PVPKill', 'Rank',
  'RebootRepair', 'ReceiveText', 'RedeemVoucher', 'RefuelAll', 'RefuelPartial',
  'Repair', 'RepairAll', 'RepairDrone', 'Resurrect', 'SAAScanComplete', 'Scan',
  'ScanOrganic', 'Scanned', 'SearchAndRescue', 'SelfDestruct', 'SellDrones',
  'SellExplorationData', 'SellMicroResources', 'SetUserShipName', 'ShieldState',
  'ShipTargeted', 'ShipyardBuy', 'ShipyardNew', 'ShipyardSell', 'ShipyardSwap',
  'ShipyardTransfer', 'StartJump', 'SupercruiseEntry', 'SupercruiseExit', 'Synthesis',
  'TechnologyBroker', 'Touchdown', 'Undocked', 'UseOrDiscard', 'VehicleSwitch',
  'WingAdd', 'WingInvite', 'WingJoin', 'WingLeave'
])

// Journal events that EDDN journal/1 schema accepts
const EDDN_JOURNAL_EVENTS = new Set([
  'Docked', 'FSDJump', 'Location', 'Scan', 'SAASignalsFound', 'SAAScanComplete',
  'NavBeaconScan', 'FSSDiscoveryScan', 'FSSAllBodiesFound', 'FSSBodySignals',
  'FSSSignalDiscovered', 'ApproachSettlement'
])

// Runtime status — in memory only, not persisted
const integrationStatus = {
  edsm: { enabled: false, status: 'disconnected', lastError: null, lastSuccess: null },
  eddn: { enabled: false, status: 'disconnected', lastError: null, lastSuccess: null },
  inara: { enabled: false, status: 'disconnected', lastError: null, lastSuccess: null },
  capi: { enabled: false, status: 'disconnected', lastError: null, lastSuccess: null, cmdrName: null }
}

// Lazy-load package version to avoid circular requires during init
let _appVersion = null
function appVersion () {
  if (!_appVersion) {
    try { _appVersion = require('../../../../package.json').version } catch { _appVersion = '0.0.0' }
  }
  return _appVersion
}

class Integrations {
  constructor ({ eliteLog, eliteJson }) {
    this.eliteLog = eliteLog
    this.eliteJson = eliteJson

    // Preferences cache — updated by notifyPreferencesChanged
    this._prefs = null

    // CAPI OAuth2 state
    this._capiVerifier = null
    this._capiState = null
    this._capiCallbackServer = null
    this._capiAccessToken = null
    this._capiAccessTokenExpiry = null

    // Inara event queue + flush timer
    this._inaraQueue = []
    this._inaraFlushTimer = null

    // Current game state enrichment data (populated from journal events)
    this._gameState = {
      systemName: null,
      systemAddress: null,
      starPos: null,
      stationName: null,
      stationMarketId: null,
      cmdrName: null,
      horizons: false,
      odyssey: false,
      gameVersion: null,
      gameBuild: null,
      fid: null
    }

    // Load initial preferences from disk asynchronously
    this._loadPreferences()
  }

  async _loadPreferences () {
    try {
      this._prefs = await preferencesStore.read()
      this._syncEnabledFromPrefs()
    } catch {
      this._prefs = {}
    }
  }

  // Called by EventHandlers when setPreferences is called by the client
  notifyPreferencesChanged (newPrefs) {
    this._prefs = newPrefs
    this._syncEnabledFromPrefs()
  }

  _syncEnabledFromPrefs () {
    const i = this._prefs?.integrations || {}
    integrationStatus.edsm.enabled = !!i.edsmEnabled
    integrationStatus.eddn.enabled = !!i.eddnEnabled
    integrationStatus.inara.enabled = !!i.inaraEnabled
    integrationStatus.capi.enabled = !!i.capiEnabled
    if (i.capiRefreshToken && integrationStatus.capi.status === 'disconnected') {
      integrationStatus.capi.status = 'ok'
    }
    if (!i.capiRefreshToken) {
      integrationStatus.capi.status = 'disconnected'
      this._capiAccessToken = null
      this._capiAccessTokenExpiry = null
    }
  }

  // Called by EventHandlers.logEventHandler for every journal event
  logEventHandler (logEvent) {
    const eventName = logEvent.event
    if (!eventName) return

    this._updateGameState(logEvent)

    if (integrationStatus.edsm.enabled && EDSM_JOURNAL_EVENTS.has(eventName)) {
      this._sendToEdsm(logEvent).catch(() => {
        integrationStatus.edsm.status = 'error'
        integrationStatus.edsm.lastError = 'Provider request failed'
      })
    }

    if (integrationStatus.eddn.enabled && EDDN_JOURNAL_EVENTS.has(eventName)) {
      this._sendToEddn(logEvent).catch(() => {
        integrationStatus.eddn.status = 'error'
        integrationStatus.eddn.lastError = 'Provider request failed'
      })
    }

    if (integrationStatus.inara.enabled) {
      this._queueForInara(logEvent)
    }
  }

  _updateGameState (entry) {
    switch (entry.event) {
      case 'LoadGame':
        this._gameState.cmdrName = entry.Commander ?? null
        this._gameState.horizons = entry.Horizons ?? false
        this._gameState.odyssey = entry.Odyssey ?? false
        this._gameState.gameVersion = entry.gameversion ?? null
        this._gameState.gameBuild = entry.build ?? null
        this._gameState.fid = entry.FID ?? null
        break
      case 'Fileheader':
        this._gameState.gameVersion = entry.gameversion ?? this._gameState.gameVersion
        this._gameState.gameBuild = entry.build ?? this._gameState.gameBuild
        break
      case 'Location':
        this._gameState.systemName = entry.StarSystem ?? null
        this._gameState.systemAddress = entry.SystemAddress ?? null
        this._gameState.starPos = entry.StarPos ?? null
        this._gameState.stationName = entry.StationName ?? null
        this._gameState.stationMarketId = entry.MarketID ?? null
        break
      case 'FSDJump':
        this._gameState.systemName = entry.StarSystem ?? null
        this._gameState.systemAddress = entry.SystemAddress ?? null
        this._gameState.starPos = entry.StarPos ?? null
        this._gameState.stationName = null
        this._gameState.stationMarketId = null
        break
      case 'Docked':
        this._gameState.stationName = entry.StationName ?? null
        this._gameState.stationMarketId = entry.MarketID ?? null
        break
      case 'Undocked':
        this._gameState.stationName = null
        this._gameState.stationMarketId = null
        break
    }
  }

  async _sendToEdsm (journalEntry) {
    const prefs = this._prefs?.integrations
    if (!prefs?.edsmApiKey || !prefs?.edsmCmdrName) return

    const params = new URLSearchParams()
    params.append('commanderName', prefs.edsmCmdrName)
    params.append('apiKey', prefs.edsmApiKey)
    params.append('fromSoftware', 'DAEDALUS Terminal')
    params.append('fromSoftwareVersion', appVersion())
    params.append('message', JSON.stringify(journalEntry))

    const response = await axios().post(`${EDSM_API_URL}/api-journal-v1`, params.toString(), {
      timeout: 10000,
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
    })

    const msgnum = response.data?.msgnum
    if (msgnum >= 200) {
      throw new Error(response.data.msg || `EDSM error ${msgnum}`)
    }
    integrationStatus.edsm.status = 'ok'
    integrationStatus.edsm.lastSuccess = Date.now()
    integrationStatus.edsm.lastError = null
  }

  async _sendToEddn (journalEntry) {
    const cmdrName = this._prefs?.integrations?.eddnCmdrName ||
      this._gameState.cmdrName || 'UNKNOWN'

    const enriched = Object.fromEntries(
      Object.entries({
        ...journalEntry,
        _marketId: this._gameState.stationMarketId || undefined,
        _systemAddress: this._gameState.systemAddress || undefined,
        _stationName: this._gameState.stationName || undefined
      }).filter(([, v]) => v !== undefined)
    )

    const payload = {
      $schemaRef: 'https://eddn.edcd.io/schemas/journal/1',
      header: {
        uploaderID: cmdrName,
        softwareName: 'DAEDALUS Terminal',
        softwareVersion: appVersion(),
        gameversion: this._gameState.gameVersion || '',
        gamebuild: this._gameState.gameBuild || ''
      },
      message: enriched
    }

    await axios().post(EDDN_UPLOAD_URL, JSON.stringify(payload), {
      timeout: 10000,
      headers: { 'Content-Type': 'application/json' }
    })

    integrationStatus.eddn.status = 'ok'
    integrationStatus.eddn.lastSuccess = Date.now()
    integrationStatus.eddn.lastError = null
  }

  _queueForInara (journalEntry) {
    let inaraEvent = null

    if (journalEntry.event === 'FSDJump') {
      inaraEvent = {
        eventName: 'addCommanderFlightLog',
        eventTimestamp: journalEntry.timestamp,
        eventData: {
          starsystemName: journalEntry.StarSystem,
          starsystemCoords: journalEntry.StarPos
            ? { x: journalEntry.StarPos[0], y: journalEntry.StarPos[1], z: journalEntry.StarPos[2] }
            : undefined,
          starsystemAddress: journalEntry.SystemAddress,
          isLanded: false,
          isDocked: false
        }
      }
    } else if (journalEntry.event === 'Location') {
      inaraEvent = {
        eventName: 'addCommanderFlightLog',
        eventTimestamp: journalEntry.timestamp,
        eventData: {
          starsystemName: journalEntry.StarSystem,
          starsystemCoords: journalEntry.StarPos
            ? { x: journalEntry.StarPos[0], y: journalEntry.StarPos[1], z: journalEntry.StarPos[2] }
            : undefined,
          starsystemAddress: journalEntry.SystemAddress,
          isLanded: false,
          isDocked: journalEntry.Docked || false
        }
      }
    } else if (journalEntry.event === 'LoadGame' && journalEntry.Credits !== undefined) {
      inaraEvent = {
        eventName: 'setCommanderCredits',
        eventTimestamp: journalEntry.timestamp,
        eventData: {
          commanderCredits: journalEntry.Credits,
          commanderLoan: 0
        }
      }
    }

    if (inaraEvent) {
      this._inaraQueue.push(inaraEvent)
      if (!this._inaraFlushTimer) {
        this._inaraFlushTimer = setTimeout(() => this._flushInara(), 35000)
      }
    }
  }

  async _flushInara () {
    this._inaraFlushTimer = null
    if (this._inaraQueue.length === 0) return
    const events = this._inaraQueue.splice(0)

    const prefs = this._prefs?.integrations
    if (!prefs?.inaraApiKey) return

    const payload = {
      header: {
        appName: 'DAEDALUS Terminal',
        appVersion: appVersion(),
        isDeveloped: false,
        APIkey: prefs.inaraApiKey,
        commanderName: prefs.inaraCmdrName || this._gameState.cmdrName || ''
      },
      events
    }

    try {
      const response = await axios().post(INARA_API_URL, payload, {
        timeout: 30000,
        headers: { 'Content-Type': 'application/json' }
      })
      if (response.data?.header?.eventStatus === 200) {
        integrationStatus.inara.status = 'ok'
        integrationStatus.inara.lastSuccess = Date.now()
        integrationStatus.inara.lastError = null
      } else {
        integrationStatus.inara.status = 'error'
        integrationStatus.inara.lastError = 'Provider request failed'
      }
    } catch (err) {
      integrationStatus.inara.status = 'error'
      integrationStatus.inara.lastError = 'Provider request failed'
    }
  }

  // ─── CAPI OAuth2 helpers ─────────────────────────────────────────────────

  _generateVerifier () {
    return crypto.randomBytes(32).toString('base64url')
  }

  _generateState () {
    return crypto.randomBytes(16).toString('hex')
  }

  _codeChallenge (verifier) {
    return crypto.createHash('sha256').update(verifier).digest('base64url')
  }

  async _getCapiAccessToken () {
    const generation = this._capiGeneration || 0
    const prefs = this._prefs?.integrations
    if (!prefs?.capiRefreshToken) return null

    // Return cached token if still valid (with 60 s buffer)
    if (this._capiAccessToken && this._capiAccessTokenExpiry &&
        Date.now() < this._capiAccessTokenExpiry - 60000) {
      return this._capiAccessToken
    }

    const response = await (this._capiHttp || axios()).post(
      `${FRONTIER_AUTH_SERVER}/token`,
      new URLSearchParams({
        grant_type: 'refresh_token',
        client_id: CAPI_CLIENT_ID,
        refresh_token: prefs.capiRefreshToken
      }).toString(),
      {
        timeout: 15000,
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
      }
    )

    const data = response.data
    if (generation !== (this._capiGeneration || 0)) throw new Error('Authentication changed')

    if (data.refresh_token && data.refresh_token !== prefs.capiRefreshToken) {
      this._prefs = await (this.preferencesStore || preferencesStore).update(raw => {
        if (generation !== (this._capiGeneration || 0) || raw.integrations?.capiRefreshToken !== prefs.capiRefreshToken) throw new Error('Authentication changed')
        return { ...raw, integrations: { ...raw.integrations, capiRefreshToken: data.refresh_token } }
      })
    }
    if (generation !== (this._capiGeneration || 0)) throw new Error('Authentication changed')
    this._capiAccessToken = data.access_token
    this._capiAccessTokenExpiry = Date.now() + (data.expires_in || 7200) * 1000

    return this._capiAccessToken
  }

  // Called by the HTTP server route in main.js when the OS redirects
  // daedalus://capi-callback?code=...&state=... back to the app.
  handleCapiCallback (code, state, error) {
    if (!global.HANDLE_CAPI_CALLBACK) return
    const handler = global.HANDLE_CAPI_CALLBACK
    handler({ code, state, error })
  }

  _waitForCapiCallback () {
    this._cancelCapiAuth?.()
    const expectedState = this._capiState
    return new Promise((resolve, reject) => {
      const finish = (error, result) => {
        clearTimeout(timer)
        if (global.HANDLE_CAPI_CALLBACK === callback) global.HANDLE_CAPI_CALLBACK = null
        this._cancelCapiAuth = null
        if (error) reject(new Error(error))
        else resolve(result)
      }
      const callback = ({ code, state, error }) => {
        // Reject unsolicited callbacks before consuming the pending request.
        if (!expectedState || state !== expectedState) return
        if (error || !code) finish('Frontier authentication failed')
        else finish(null, { code, state })
      }
      const timer = setTimeout(() => finish('CAPI authentication timed out (5 min)'), 5 * 60 * 1000)
      timer.unref?.()
      this._cancelCapiAuth = () => finish('Authentication superseded')
      global.HANDLE_CAPI_CALLBACK = callback
    })
  }

  _openCapiAuthUrl (authUrl) {
    const openCmd = process.platform === 'win32'
      ? `start "" "${authUrl.replace(/"/g, '\\"')}"`
      : process.platform === 'darwin'
        ? `open "${authUrl}"`
        : `xdg-open "${authUrl}"`

    exec(openCmd, (err) => {
      if (err) console.error('[CAPI] Failed to open browser:', err.message)
    })
  }

  async beginCapiAuth () {
    const generation = this._capiGeneration = (this._capiGeneration || 0) + 1
    this._capiVerifier = this._generateVerifier()
    this._capiState = this._generateState()
    const challenge = this._codeChallenge(this._capiVerifier)

    const authUrl = (
      `${FRONTIER_AUTH_SERVER}/auth` +
      '?response_type=code' +
      '&audience=frontier,steam,epic' +
      '&scope=auth%20capi' +
      `&client_id=${CAPI_CLIENT_ID}` +
      `&code_challenge=${challenge}` +
      '&code_challenge_method=S256' +
      `&state=${this._capiState}` +
      `&redirect_uri=${encodeURIComponent(CAPI_REDIRECT_URI)}`
    )

    integrationStatus.capi.status = 'pending'

    this._openCapiAuthUrl(authUrl)

    const { code, state } = await this._waitForCapiCallback()

    if (state !== this._capiState) {
      integrationStatus.capi.status = 'error'
      integrationStatus.capi.lastError = 'OAuth state mismatch'
      throw new Error('OAuth state mismatch')
    }

    const tokenResponse = await (this._capiHttp || axios()).post(
      `${FRONTIER_AUTH_SERVER}/token`,
      new URLSearchParams({
        grant_type: 'authorization_code',
        client_id: CAPI_CLIENT_ID,
        code_verifier: this._capiVerifier,
        code,
        redirect_uri: CAPI_REDIRECT_URI
      }).toString(),
      {
        timeout: 15000,
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
      }
    )

    const tokenData = tokenResponse.data
    if (generation !== this._capiGeneration) throw new Error('Authentication changed')

    // Persist refresh token
    this._prefs = await (this.preferencesStore || preferencesStore).update(raw => {
      if (generation !== this._capiGeneration) throw new Error('Authentication changed')
      return { ...raw, integrations: { ...raw.integrations, capiRefreshToken: tokenData.refresh_token } }
    })
    if (generation !== this._capiGeneration) throw new Error('Authentication changed')
    // Broadcast so other components (and getPreferences cache) know prefs changed
    const broadcastEvent = global.BROADCAST_EVENT
    if (broadcastEvent) broadcastEvent('syncMessage', { name: 'preferences' })

    this._capiAccessToken = tokenData.access_token
    this._capiAccessTokenExpiry = Date.now() + (tokenData.expires_in || 7200) * 1000

    integrationStatus.capi.status = 'ok'
    integrationStatus.capi.lastSuccess = Date.now()
    integrationStatus.capi.lastError = null

    return { success: true }
  }

  async getCapiProfile () {
    const token = await this._getCapiAccessToken()
    if (!token) throw new Error('Not authenticated with Frontier CAPI')

    const response = await axios().get(`${CAPI_SERVER}/profile`, {
      headers: { Authorization: `Bearer ${token}` },
      timeout: 15000
    })

    integrationStatus.capi.status = 'ok'
    integrationStatus.capi.lastSuccess = Date.now()
    integrationStatus.capi.cmdrName = response.data?.commander?.name ?? null

    return response.data
  }

  async disconnectCapi () {
    this._capiGeneration = (this._capiGeneration || 0) + 1
    this._cancelCapiAuth?.()
    this._capiState = null
    this._capiVerifier = null
    this._capiAccessToken = null
    this._capiAccessTokenExpiry = null
    const raw = await (this.preferencesStore || preferencesStore).disconnectCapi()
    this.notifyPreferencesChanged(raw)
    return raw
  }

  async testEdsm () {
    const prefs = this._prefs?.integrations
    if (!prefs?.edsmApiKey || !prefs?.edsmCmdrName) {
      return { success: false, error: 'Missing API key or commander name' }
    }
    try {
      // get-credits always returns valid JSON for any known commander;
      // get-position returns HTTP 404 when no position has been recorded yet.
      const response = await axios().get(`${EDSM_API_URL}/api-commander-v1/get-credits`, {
        params: { commanderName: prefs.edsmCmdrName, apiKey: prefs.edsmApiKey },
        timeout: 15000
      })
      const msgnum = response.data?.msgnum
      if (msgnum === 100) {
        integrationStatus.edsm.status = 'ok'
        integrationStatus.edsm.lastSuccess = Date.now()
        integrationStatus.edsm.lastError = null
        return { success: true, system: response.data.system || null }
      } else {
        integrationStatus.edsm.status = 'error'
        integrationStatus.edsm.lastError = 'Provider request failed'
        return { success: false, error: 'Provider request failed' }
      }
    } catch (err) {
      integrationStatus.edsm.status = 'error'
      integrationStatus.edsm.lastError = 'Provider request failed'
      return { success: false, error: 'Provider request failed' }
    }
  }

  async testInara () {
    const prefs = this._prefs?.integrations
    if (!prefs?.inaraApiKey) {
      return { success: false, error: 'Missing API key' }
    }
    try {
      const payload = {
        header: {
          appName: 'DAEDALUS Terminal',
          appVersion: appVersion(),
          isDeveloped: false,
          APIkey: prefs.inaraApiKey,
          commanderName: prefs.inaraCmdrName || this._gameState.cmdrName || ''
        },
        events: [{
          eventName: 'getCommanderProfile',
          eventTimestamp: new Date().toISOString(),
          eventData: { searchName: prefs.inaraCmdrName || this._gameState.cmdrName || 'unknown' }
        }]
      }
      const response = await axios().post(INARA_API_URL, payload, {
        timeout: 10000,
        headers: { 'Content-Type': 'application/json' }
      })
      if (response.data?.header?.eventStatus === 100) {
        integrationStatus.inara.status = 'ok'
        integrationStatus.inara.lastSuccess = Date.now()
        integrationStatus.inara.lastError = null
        return { success: true }
      } else {
        integrationStatus.inara.status = 'error'
        integrationStatus.inara.lastError = 'Provider request failed'
        return { success: false, error: 'Provider request failed' }
      }
    } catch (err) {
      integrationStatus.inara.status = 'error'
      integrationStatus.inara.lastError = 'Provider request failed'
      return { success: false, error: 'Provider request failed' }
    }
  }

  // ─── IPC handlers registered with EventHandlers ─────────────────────────

  getHandlers () {
    return {
      getIntegrationStatus: async () => {
        const prefs = this._prefs?.integrations || {}
        return {
          edsm: {
            ...integrationStatus.edsm,
            configured: !!(prefs.edsmApiKey && prefs.edsmCmdrName),
            cmdrName: prefs.edsmCmdrName || null
          },
          eddn: {
            ...integrationStatus.eddn,
            configured: true
          },
          inara: {
            ...integrationStatus.inara,
            configured: !!prefs.inaraApiKey,
            cmdrName: prefs.inaraCmdrName || null
          },
          capi: {
            ...integrationStatus.capi,
            configured: !!prefs.capiRefreshToken
          }
        }
      },

      beginCapiAuth: async () => {
        try {
          return await this.beginCapiAuth()
        } catch (err) {
          integrationStatus.capi.status = 'error'
          integrationStatus.capi.lastError = 'Provider request failed'
          return { success: false, error: 'Provider request failed' }
        }
      },

      refreshCapiProfile: async () => {
        try {
          const data = await this.getCapiProfile()
          return { success: true, cmdrName: data?.commander?.name || null }
        } catch (err) {
          integrationStatus.capi.status = 'error'
          integrationStatus.capi.lastError = 'Provider request failed'
          return { success: false, error: 'Provider request failed' }
        }
      },

      testEdsmConnection: async () => this.testEdsm(),

      testInaraConnection: async () => this.testInara()
    }
  }
}

module.exports = Integrations
