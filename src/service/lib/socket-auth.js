const crypto = require('crypto')

// Only this module can mint a principal. Client-supplied role objects are inert.
const principals = new WeakMap()
const DISPLAY_RPC = new Set([
  'getPreferences', 'getLoadingStatus', 'getCmdr', 'getLogEntries',
  'getCmdrStatus', 'getSystem', 'getShipStatus', 'getMaterials', 'getEngineers',
  'getInventory', 'getBlueprints', 'getNavRoute', 'getExplorationRoute',
  'getExplorationSystem', 'getExplorationBiologicals', 'getExplorationInventory',
  'getPowerplay', 'getPowerplayNearbySources', 'getMaterialSources',
  'getMaterialTraders', 'getEngineerPrerequisites', 'getEngineerUnlockStations',
  'getMiningStatus', 'galnetGetArticles'
])

function authorizeRpc ({ principal, handlerName }) {
  return !!(principal && principals.get(principal)?.() &&
    (principal.role === 'owner' || DISPLAY_RPC.has(handlerName)))
}

function isLoopback (address) {
  return ['127.0.0.1', '::1', '::ffff:127.0.0.1'].includes(address)
}

function sameOrigin (req) {
  try {
    const expected = new URL(`${req.socket.encrypted ? 'https' : 'http'}://${req.headers.host}`)
    return expected.host === req.headers.host && req.headers.origin === expected.origin
  } catch { return false }
}

function localRequest (req) {
  if (!isLoopback(req.socket.remoteAddress) || req.socket.encrypted) return false
  try {
    return ['localhost', '127.0.0.1', '[::1]'].includes(new URL(`http://${req.headers.host}`).hostname)
  } catch { return false }
}

function createSocketAuth ({ clock = Date.now } = {}) {
  const sessions = new Map()
  let pairing = null
  const valid = p => !!p && sessions.get(p.token) === p && p.expires > clock()
  function mint (role) {
    for (const [token, p] of sessions) if (!valid(p)) sessions.delete(token)
    if (sessions.size >= 128) throw new Error('Session limit reached')
    const p = Object.freeze({ role, token: crypto.randomBytes(32).toString('hex'), expires: clock() + 12 * 60 * 60 * 1000 })
    sessions.set(p.token, p)
    principals.set(p, () => valid(p))
    return p
  }
  function requireOwner (p) {
    if (!valid(p) || p.role !== 'owner') throw new Error('Forbidden')
  }
  function principal (req) {
    const cookieName = req.socket.encrypted ? '__Host-daedalus_session' : 'daedalus_session'
    const token = (req.headers.cookie || '').split(';').map(s => s.trim()).find(s => s.startsWith(`${cookieName}=`))?.slice(cookieName.length + 1)
    const p = sessions.get(token)
    if (!valid(p) || (p.role === 'owner' && !localRequest(req))) return null
    return p
  }
  return {
    valid,
    principal,
    localSession (req) {
      if (!localRequest(req) || !sameOrigin(req)) throw new Error('Forbidden')
      return principal(req) || mint('owner')
    },
    createPairingCode (p) {
      requireOwner(p)
      pairing = { code: crypto.randomBytes(24).toString('hex'), expires: clock() + 5 * 60 * 1000 }
      return { ...pairing }
    },
    pair (code) {
      if (typeof code !== 'string' || !pairing || pairing.expires <= clock() || code.length !== pairing.code.length ||
        !crypto.timingSafeEqual(Buffer.from(code), Buffer.from(pairing.code))) throw new Error('Invalid or expired pairing code')
      const p = mint('viewer')
      pairing = null
      return p
    },
    revokeViewers (p) {
      requireOwner(p)
      pairing = null
      for (const [token, session] of sessions) if (session.role === 'viewer') sessions.delete(token)
      return { revoked: true }
    }
  }
}

module.exports = { createSocketAuth, authorizeRpc, isLoopback, sameOrigin, localRequest }
