const { BROADCAST_EVENT: broadcastEvent } = global
const axios = require('axios')

const ARDENT_API_BASE = 'https://api.ardent-insight.com/v2'

// Default target materials — can be overridden via preferences
const DEFAULT_TARGET_MATERIALS = [
  'tritium', 'painite', 'platinum', 'lowtemperaturediamond',
  'opal', 'musgravite', 'monazite', 'rhodplumsite', 'serendibite',
  'benitoite', 'alexandrite', 'grandidierite', 'void opals'
]

const DEFAULT_MIN_PERCENT = 10

// Mining laser / limpet module patterns to detect a mining-capable ship
const MINING_MODULE_PATTERNS = [
  'mininglaser', 'dronecontrol_collection', 'dronecontrol_prospector',
  'refinery', 'mrascanner'
]

// How many limpets a Synthesis produces
const SYNTHESIS_LIMPET_COUNTS = {
  basic: 4,
  standard: 4,
  premium: 8
}

class Mining {
  constructor ({ eliteLog, eliteJson }) {
    this.eliteLog = eliteLog
    this.eliteJson = eliteJson
    this._state = this._freshState()
    this._priceCache = new Map() // key → avgSellPrice CR
    this._initDone = false
  }

  // Populate baseline state from the journal DB (runs once, on first getMiningStatus call).
  // logEventHandler is NOT called during initial journal load, so we must seed state here.
  async _initFromJournal () {
    if (this._initDone) return
    this._initDone = true
    try {
      // CargoCapacity from the most recent Loadout
      const loadout = await this.eliteLog.getEvent('Loadout')
      if (loadout?.CargoCapacity) {
        this._state.cargo.max = loadout.CargoCapacity
      }
      if (loadout?.Modules) {
        this._state.hasMiningModules = loadout.Modules.some(m => {
          const item = (m.Item ?? '').toLowerCase()
          return MINING_MODULE_PATTERNS.some(p => item.includes(p))
        })
      }
    } catch (e) {}
    try {
      // Current cargo contents — prefer Cargo.json (live file, always has full Inventory)
      // and fall back to the most recent journal Cargo event.
      let cargoData = null
      try {
        const jsonFiles = await this.eliteJson.json(true)
        if (jsonFiles?.Cargo?.Inventory) cargoData = jsonFiles.Cargo
      } catch (e) {}
      if (!cargoData) {
        cargoData = await this.eliteLog.getEvent('Cargo')
      }
      if (cargoData) {
        if (cargoData.Count != null) this._state.cargo.current = cargoData.Count
        if (cargoData.Inventory) {
          const items = {}
          let limpetCount = 0
          for (const item of cargoData.Inventory) {
            const key = (item.Name ?? '').toLowerCase()
            const count = item.Count ?? 0
            if (key === 'drones') { limpetCount = count } else { items[key] = count }
          }
          this._state.cargo.items = items
          this._state.limpets.remaining = limpetCount
        }
      }
    } catch (e) {}
    try {
      // Current system
      const location = await this.eliteLog.getEvent('Location')
      if (location?.StarSystem) this._state.systemName = location.StarSystem
    } catch (e) {}
  }

  _freshState () {
    return {
      active: false,
      systemName: null,
      sessionStart: null,
      hasMiningModules: false,
      weaponPips: { halfPips: 8, isFull: true }, // from Status.json Pips[2]
      cargo: {
        max: 0,
        current: 0,
        items: {}         // { [commodityKey]: count } — commodities only (not limpets)
      },
      limpets: {
        remaining: 0,
        used: 0,
        synthed: 0,
        abandoned: 0,
        warnNoLimpets: false
      },
      rings: [],            // [{ bodyName, ringType, distanceLs, hotspots:{key:count} }]
      activeProspectors: [], // most recent 2 ProspectedAsteroid results (newest first)
      session: {
        goodRocks: 0,
        totalRocks: 0,
        recentResults: []   // last 50, newest first
      },
      refined: {
        items: {},          // { [commodityKey]: count }
        totalTons: 0,
        lastRefinedKey: null,
        lastRefinedAt: null
      }
    }
  }

  _partialReset () {
    // FSDJump: keep cargo/limpets, reset mining-session data
    this._state.rings = []
    this._state.activeProspectors = []
    this._state.session = { goodRocks: 0, totalRocks: 0, recentResults: [] }
    this._state.refined = { items: {}, totalTons: 0, lastRefinedKey: null, lastRefinedAt: null }
  }

  // "$painite_name;" → "painite", "painite" → "painite"
  _normalizeKey (type) {
    if (!type) return ''
    return type.replace(/^\$/, '').replace(/_name;$/, '').toLowerCase().replace(/\s+/g, '')
  }

  // "$AsteroidMaterialContent_Medium;" → "Medium"
  _parseContentLevel (content) {
    if (!content) return 'Unknown'
    const match = content.match(/AsteroidMaterialContent_(\w+)/)
    return match ? match[1] : 'Unknown'
  }

  // "eRingClass_Icy" → "Icy"; handle the game's "Metalic" typo
  _parseRingType (ringClass) {
    if (!ringClass) return 'Unknown'
    const raw = ringClass.replace('eRingClass_', '')
    return raw === 'Metalic' ? 'Metallic' : raw
  }

  // Called from gameStateChangeHandler when Status.json changes
  updateFromStatus (statusJson) {
    if (!statusJson) return
    const pips = statusJson.Pips
    if (!Array.isArray(pips) || pips.length < 3) return
    const halfPips = pips[2]
    const isFull = halfPips >= 8
    const prev = this._state.weaponPips
    if (prev.halfPips === halfPips && prev.isFull === isFull) return
    this._state.weaponPips = { halfPips, isFull }
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  // Fetch best NPC-station sell price from Ardent API (fleet carriers excluded)
  async _fetchPrice (key) {
    if (this._priceCache.has(key)) return
    this._priceCache.set(key, null) // mark as in-flight to prevent duplicate fetches
    try {
      const res = await axios.get(
        `${ARDENT_API_BASE}/commodity/name/${encodeURIComponent(key)}/imports?fleetCarriers=false`,
        { timeout: 10000 }
      )
      const price = Array.isArray(res.data) && res.data.length > 0
        ? res.data[0].sellPrice
        : null
      if (price != null && price > 0) {
        this._priceCache.set(key, price)
        broadcastEvent('miningStateChange', this.getMiningStatus())
      }
    } catch (e) {
      this._priceCache.delete(key) // allow retry on next refine
    }
  }

  logEventHandler (event) {
    switch (event.event) {
      case 'LoadGame': return this._onLoadGame(event)
      case 'FSDJump': return this._onFSDJump(event)
      case 'Location': return this._onLocation(event)
      case 'Loadout': return this._onLoadout(event)
      case 'Cargo': return this._onCargo(event)
      case 'ProspectedAsteroid': return this._onProspectedAsteroid(event)
      case 'MiningRefined': return this._onMiningRefined(event)
      case 'LaunchDrone': return this._onLaunchDrone(event)
      case 'BuyDrones': return this._onBuyDrones(event)
      case 'SellDrones': return this._onSellDrones(event)
      case 'EjectCargo': return this._onEjectCargo(event)
      case 'Synthesis': return this._onSynthesis(event)
      case 'Scan': return this._onScan(event)
      case 'SAASignalsFound': return this._onSAASignalsFound(event)
      case 'AsteroidCracked': return this._onAsteroidCracked(event)
      case 'SupercruiseEntry': return this._onSupercruiseEntry(event)
      case 'SupercruiseExit': return this._onSupercruiseExit(event)
      case 'Undocked': return this._onUndocked(event)
    }
  }

  _onLoadGame (event) {
    this._state = this._freshState()
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onFSDJump (event) {
    if (event.StarSystem) this._state.systemName = event.StarSystem
    this._partialReset()
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onLocation (event) {
    if (event.StarSystem) this._state.systemName = event.StarSystem
    // Don't reset session — covers carrier jumps and restarts mid-session
  }

  _onLoadout (event) {
    if (event.CargoCapacity) this._state.cargo.max = event.CargoCapacity
    this._state.hasMiningModules = (event.Modules ?? []).some(m => {
      const item = (m.Item ?? '').toLowerCase()
      return MINING_MODULE_PATTERNS.some(p => item.includes(p))
    })
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onCargo (event) {
    // Always update the total count
    if (event.Count != null) this._state.cargo.current = event.Count

    if (event.Inventory) {
      // Full inventory sync (fires at login / ship switch)
      const items = {}
      let limpetCount = 0
      for (const item of event.Inventory) {
        const key = (item.Name ?? '').toLowerCase()
        const count = item.Count ?? 0
        if (key === 'drones') {
          limpetCount = count
        } else {
          items[key] = count
        }
      }
      this._state.cargo.items = items
      this._state.limpets.remaining = limpetCount
    }

    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onProspectedAsteroid (event) {
    this._state.active = true
    if (!this._state.sessionStart) this._state.sessionStart = event.timestamp

    const targetMaterials = DEFAULT_TARGET_MATERIALS
    const minPercent = DEFAULT_MIN_PERCENT

    const materials = (event.Materials ?? []).map(m => ({
      key: (m.Name ?? '').toLowerCase(),
      name: m.Name,
      pct: m.Proportion ?? 0,
      isTarget: targetMaterials.includes((m.Name ?? '').toLowerCase())
    }))

    const motherlode = event.MotherlodeMaterial
      ? this._normalizeKey(event.MotherlodeMaterial)
      : null
    const contentLevel = this._parseContentLevel(event.Content)
    const remaining = event.Remaining ?? 100

    const isGood = materials.some(m => m.isTarget && m.pct >= minPercent) ||
      (motherlode != null && targetMaterials.includes(motherlode))

    if (isGood) this._state.session.goodRocks++
    this._state.session.totalRocks++

    const result = {
      timestamp: event.timestamp,
      materials,
      motherlode,
      contentLevel,
      remaining,
      isGood
    }

    this._state.session.recentResults.unshift(result)
    if (this._state.session.recentResults.length > 50) {
      this._state.session.recentResults.pop()
    }

    // Rolling 2-slot buffer: newest at front
    this._state.activeProspectors.unshift(result)
    if (this._state.activeProspectors.length > 2) {
      this._state.activeProspectors.pop()
    }

    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onMiningRefined (event) {
    this._state.active = true
    if (!this._state.sessionStart) this._state.sessionStart = event.timestamp

    const key = this._normalizeKey(event.Type)
    if (!key) return

    const items = this._state.refined.items
    items[key] = (items[key] ?? 0) + 1
    this._state.refined.totalTons = Object.values(items).reduce((a, b) => a + b, 0)
    this._state.refined.lastRefinedKey = key
    this._state.refined.lastRefinedAt = event.timestamp

    // Keep cargo.items in sync — the Cargo journal event only includes Inventory at
    // login/ship-switch, so we track each refined ton ourselves
    const cargoItems = this._state.cargo.items
    cargoItems[key] = (cargoItems[key] ?? 0) + 1
    // Recalculate current from known items + limpets (the Cargo Count event may lag)
    const itemTons = Object.values(cargoItems).reduce((a, b) => a + b, 0)
    this._state.cargo.current = itemTons + this._state.limpets.remaining

    // Kick off price fetch for this commodity (fire-and-forget)
    this._fetchPrice(key).catch(() => {})

    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onLaunchDrone (event) {
    // Track limpet usage
    this._state.limpets.used++
    this._state.limpets.remaining = Math.max(0, this._state.limpets.remaining - 1)
    // cargo.current will be updated by the subsequent Cargo event
  }

  _onBuyDrones (event) {
    const count = event.Count ?? 0
    this._state.limpets.remaining += count
    // cargo.current updated by subsequent Cargo event
  }

  _onSellDrones (event) {
    const count = event.Count ?? 0
    this._state.limpets.remaining = Math.max(0, this._state.limpets.remaining - count)
  }

  _onEjectCargo (event) {
    const type = (event.Type ?? '').toLowerCase()
    const count = event.Count ?? 1
    if (type === 'drones') {
      this._state.limpets.remaining = Math.max(0, this._state.limpets.remaining - count)
      this._state.limpets.abandoned += count
    } else {
      const items = this._state.cargo.items
      if (items[type] != null) {
        items[type] = Math.max(0, items[type] - count)
        if (items[type] === 0) delete items[type]
      }
    }
  }

  _onSynthesis (event) {
    const name = (event.Name ?? '').toLowerCase()
    if (!name.includes('limpet')) return

    let count = SYNTHESIS_LIMPET_COUNTS.basic
    if (name.includes('premium')) count = SYNTHESIS_LIMPET_COUNTS.premium
    this._state.limpets.remaining += count
    this._state.limpets.synthed++
  }

  _onScan (event) {
    if (!event.Rings || event.Rings.length === 0) return

    let changed = false
    for (const ring of event.Rings) {
      // Only track actual rings (not belts — belts aren't minable with limpets)
      if (!ring.Name) continue
      if (ring.Name.includes('Belt')) continue

      const existing = this._state.rings.find(r => r.bodyName === ring.Name)
      if (existing) {
        // Update ring type if we didn't have it yet
        if (!existing.ringType) existing.ringType = this._parseRingType(ring.RingClass)
      } else {
        this._state.rings.push({
          bodyName: ring.Name,
          ringType: this._parseRingType(ring.RingClass),
          distanceLs: null,
          hotspots: {}
        })
        changed = true
      }
    }

    if (changed) broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onSAASignalsFound (event) {
    const bodyName = event.BodyName
    const signals = event.Signals ?? []

    let ring = this._state.rings.find(r => r.bodyName === bodyName)
    if (!ring) {
      // SAASignalsFound before any Scan event for this body
      ring = { bodyName, ringType: null, distanceLs: null, hotspots: {} }
      this._state.rings.push(ring)
    }

    for (const signal of signals) {
      // Signal.Type is the plain commodity name, e.g. "Painite"
      const key = (signal.Type ?? '').toLowerCase().replace(/\s+/g, '')
      if (key) ring.hotspots[key] = signal.Count ?? 1
    }

    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onAsteroidCracked (event) {
    // Core mining: seismic charge cracked the asteroid — it's depleted, clear prospectors
    this._state.activeProspectors = []
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onSupercruiseEntry (event) {
    this._state.activeProspectors = []
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onSupercruiseExit (event) {
    // Used by client for autoswitch to /mining/status
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  _onUndocked (event) {
    const noLimpets = this._state.hasMiningModules && this._state.limpets.remaining === 0
    this._state.limpets.warnNoLimpets = noLimpets
    broadcastEvent('miningStateChange', this.getMiningStatus())
  }

  getMiningStatus () {
    const s = this._state
    const { limpets, cargo, session, refined, rings, activeProspectors } = s

    const totalLaunched = limpets.used
    const wastedPercent = totalLaunched > 0
      ? Math.round((limpets.abandoned / totalLaunched) * 1000) / 10
      : 0

    const sessionDurationHours = s.sessionStart
      ? (Date.now() - new Date(s.sessionStart).getTime()) / 3600000
      : 0
    const ratePerHour = sessionDurationHours > 0.001
      ? Math.round((refined.totalTons / sessionDurationHours) * 10) / 10
      : 0

    const goodRockPercent = session.totalRocks > 0
      ? Math.round((session.goodRocks / session.totalRocks) * 1000) / 10
      : 0

    const fillPercent = cargo.max > 0
      ? Math.round((cargo.current / cargo.max) * 1000) / 10
      : 0

    // Estimated profit from cached Ardent prices
    let estimatedProfit = 0
    for (const [key, count] of Object.entries(refined.items)) {
      const price = this._priceCache.get(key)
      if (price) estimatedProfit += price * count
    }

    return {
      active: s.active,
      systemName: s.systemName,
      sessionStart: s.sessionStart,
      hasMiningModules: s.hasMiningModules,
      weaponPips: s.weaponPips,
      estimatedProfit,

      cargo: {
        max: cargo.max,
        current: cargo.current,
        fillPercent,
        items: Object.entries(cargo.items)
          .filter(([, count]) => count > 0)
          .map(([key, count]) => ({ key, count })),
        limpetCount: limpets.remaining
      },

      limpets: {
        remaining: limpets.remaining,
        used: limpets.used,
        synthed: limpets.synthed,
        abandoned: limpets.abandoned,
        warnNoLimpets: limpets.warnNoLimpets,
        wastedPercent
      },

      rings: rings.map(r => {
        const hotspotList = Object.entries(r.hotspots)
          .map(([key, count]) => ({
            key,
            count,
            isTarget: DEFAULT_TARGET_MATERIALS.includes(key)
          }))
          .sort((a, b) => (b.isTarget ? 1 : 0) - (a.isTarget ? 1 : 0))
        return {
          bodyName: r.bodyName,
          ringType: r.ringType,
          distanceLs: r.distanceLs,
          hotspots: hotspotList,
          hasTargetHotspots: hotspotList.some(h => h.isTarget)
        }
      }),

      activeProspectors: activeProspectors.map((r, i) => r ? { id: i, ...r } : null),

      session: {
        goodRocks: session.goodRocks,
        totalRocks: session.totalRocks,
        goodRockPercent,
        recentResults: session.recentResults.slice(0, 10)
      },

      refined: {
        items: Object.entries(refined.items)
          .filter(([, count]) => count > 0)
          .map(([key, count]) => ({
            key,
            count,
            lastRefinedAt: key === refined.lastRefinedKey ? refined.lastRefinedAt : null
          }))
          .sort((a, b) => b.count - a.count),
        totalTons: refined.totalTons,
        ratePerHour
      }
    }
  }

  getHandlers () {
    return {
      getMiningStatus: async () => {
        await this._initFromJournal()
        return this.getMiningStatus()
      }
    }
  }
}

module.exports = Mining
