'use strict'

const crypto = require('crypto')
const { exec } = require('child_process')
const os = require('os')
const fs = require('fs').promises
const path = require('path')
const _axios = require('axios')

// Lazy-built axios instance — deferred so appVersion() resolves after package.json loads
let _axiosInstance = null
function axios () {
  if (!_axiosInstance) {
    _axiosInstance = _axios.create({ headers: { 'User-Agent': `DAEDALUS Terminal/${appVersion()}` } })
  }
  return _axiosInstance
}

const PREFERENCES_DIR = path.join(os.homedir(), 'AppData', 'Local', 'DAEDALUS Terminal')
const PREFERENCES_FILE = path.join(PREFERENCES_DIR, 'Preferences.json')

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
      const data = await fs.readFile(PREFERENCES_FILE, 'utf8')
      this._prefs = JSON.parse(data)
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
      this._sendToEdsm(logEvent).catch(err => {
        integrationStatus.edsm.status = 'error'
        integrationStatus.edsm.lastError = err.message
      })
    }

    if (integrationStatus.eddn.enabled && EDDN_JOURNAL_EVENTS.has(eventName)) {
      this._sendToEddn(logEvent).catch(err => {
        integrationStatus.eddn.status = 'error'
        integrationStatus.eddn.lastError = err.message
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
        const msg = response.data?.header?.eventStatusText || 'Inara error'
        integrationStatus.inara.status = 'error'
        integrationStatus.inara.lastError = msg
      }
    } catch (err) {
      integrationStatus.inara.status = 'error'
      integrationStatus.inara.lastError = err.message
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
    const prefs = this._prefs?.integrations
    if (!prefs?.capiRefreshToken) return null

    // Return cached token if still valid (with 60 s buffer)
    if (this._capiAccessToken && this._capiAccessTokenExpiry &&
        Date.now() < this._capiAccessTokenExpiry - 60000) {
      return this._capiAccessToken
    }

    const response = await axios().post(
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
    this._capiAccessToken = data.access_token
    this._capiAccessTokenExpiry = Date.now() + (data.expires_in || 7200) * 1000

    if (data.refresh_token && data.refresh_token !== prefs.capiRefreshToken) {
      const newPrefs = JSON.parse(JSON.stringify(this._prefs || {}))
      if (!newPrefs.integrations) newPrefs.integrations = {}
      newPrefs.integrations.capiRefreshToken = data.refresh_token
      this._prefs = newPrefs
      await fs.mkdir(PREFERENCES_DIR, { recursive: true })
      await fs.writeFile(PREFERENCES_FILE, JSON.stringify(newPrefs))
    }

    return this._capiAccessToken
  }

  // Called by the HTTP server route in main.js when the OS redirects
  // daedalus://capi-callback?code=...&state=... back to the app.
  handleCapiCallback (code, state, error) {
    if (!global.HANDLE_CAPI_CALLBACK) return
    const handler = global.HANDLE_CAPI_CALLBACK
    global.HANDLE_CAPI_CALLBACK = null
    handler({ code, state, error })
  }

  _waitForCapiCallback () {
    // Cancel any stale pending auth
    if (global.HANDLE_CAPI_CALLBACK) {
      global.HANDLE_CAPI_CALLBACK({ error: 'superseded' })
      global.HANDLE_CAPI_CALLBACK = null
    }

    return new Promise((resolve, reject) => {
      let settled = false

      global.HANDLE_CAPI_CALLBACK = ({ code, state, error }) => {
        if (settled) return
        settled = true
        global.HANDLE_CAPI_CALLBACK = null
        if (error) {
          reject(new Error(`Frontier auth error: ${error}`))
        } else if (!code) {
          reject(new Error('No code in OAuth callback'))
        } else {
          resolve({ code, state })
        }
      }

      // Auto-cancel after 5 minutes if the user doesn't complete auth
      setTimeout(() => {
        if (global.HANDLE_CAPI_CALLBACK && !settled) {
          settled = true
          global.HANDLE_CAPI_CALLBACK = null
          reject(new Error('CAPI authentication timed out (5 min)'))
        }
      }, 5 * 60 * 1000)
    })
  }

  async beginCapiAuth () {
    this._capiVerifier = this._generateVerifier()
    this._capiState = this._generateState()
    const challenge = this._codeChallenge(this._capiVerifier)

    const authUrl = (
      `${FRONTIER_AUTH_SERVER}/auth` +
      `?response_type=code` +
      `&audience=frontier,steam,epic` +
      `&scope=auth%20capi` +
      `&client_id=${CAPI_CLIENT_ID}` +
      `&code_challenge=${challenge}` +
      `&code_challenge_method=S256` +
      `&state=${this._capiState}` +
      `&redirect_uri=${encodeURIComponent(CAPI_REDIRECT_URI)}`
    )

    integrationStatus.capi.status = 'pending'

    const openCmd = process.platform === 'win32'
      ? `start "" "${authUrl.replace(/"/g, '\\"')}"`
      : process.platform === 'darwin'
        ? `open "${authUrl}"`
        : `xdg-open "${authUrl}"`

    exec(openCmd, (err) => {
      if (err) console.error('[CAPI] Failed to open browser:', err.message)
    })

    const { code, state } = await this._waitForCapiCallback()

    if (state !== this._capiState) {
      integrationStatus.capi.status = 'error'
      integrationStatus.capi.lastError = 'OAuth state mismatch'
      throw new Error('OAuth state mismatch')
    }

    const tokenResponse = await axios().post(
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

    // Persist refresh token
    const newPrefs = JSON.parse(JSON.stringify(this._prefs || {}))
    if (!newPrefs.integrations) newPrefs.integrations = {}
    newPrefs.integrations.capiRefreshToken = tokenData.refresh_token
    this._prefs = newPrefs
    await fs.mkdir(PREFERENCES_DIR, { recursive: true })
    await fs.writeFile(PREFERENCES_FILE, JSON.stringify(newPrefs))
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
        const msg = response.data?.msg || `EDSM error ${msgnum}`
        integrationStatus.edsm.status = 'error'
        integrationStatus.edsm.lastError = msg
        return { success: false, error: msg }
      }
    } catch (err) {
      integrationStatus.edsm.status = 'error'
      integrationStatus.edsm.lastError = err.message
      return { success: false, error: err.message }
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
        const msg = response.data?.header?.eventStatusText || `Inara error ${response.data?.header?.eventStatus ?? ''}`
        integrationStatus.inara.status = 'error'
        integrationStatus.inara.lastError = msg
        return { success: false, error: msg }
      }
    } catch (err) {
      integrationStatus.inara.status = 'error'
      integrationStatus.inara.lastError = err.message
      return { success: false, error: err.message }
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
          integrationStatus.capi.lastError = err.message
          return { success: false, error: err.message }
        }
      },

      refreshCapiProfile: async () => {
        try {
          const data = await this.getCapiProfile()
          return { success: true, cmdrName: data?.commander?.name || null }
        } catch (err) {
          integrationStatus.capi.status = 'error'
          integrationStatus.capi.lastError = err.message
          return { success: false, error: err.message }
        }
      },

      testEdsmConnection: async () => this.testEdsm(),

      testInaraConnection: async () => this.testInara()
    }
  }
}

module.exports = Integrations
