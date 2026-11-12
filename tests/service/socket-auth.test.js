const test = require('node:test')
const assert = require('node:assert/strict')
const { createSocketAuth, authorizeRpc } = require('../../src/service/lib/socket-auth')

const request = (extra = {}) => ({
  headers: { host: 'localhost:3300', origin: 'http://localhost:3300' },
  socket: { remoteAddress: '127.0.0.1' },
  ...extra
})

test('anonymous and forged roles cannot reach RPC; viewer has explicit display access', () => {
  for (const principal of [null, {}, { role: 'admin' }]) {
    assert.equal(authorizeRpc({ principal, handlerName: 'setPreferences' }), false)
  }
  const auth = createSocketAuth()
  const owner = auth.localSession(request())
  assert.equal(owner.role, 'owner')
  assert.equal(authorizeRpc({ principal: owner, handlerName: 'setPreferences' }), true)
  const viewer = auth.pair(auth.createPairingCode(owner).code)
  for (const name of ['getPreferences', 'getMiningStatus', 'getNavRoute', 'getBlueprints', 'getKeybindFiles', 'getKeybinds', 'getSystemMediaSession', 'videoSearch', 'videoGetInfo', 'videoGetChannelVideos', 'videoGetCached', 'videoCheckStatus', 'galnetGetArticles']) {
    assert.equal(authorizeRpc({ principal: viewer, handlerName: name }), true)
  }
  for (const name of ['setPreferences', 'beginCapiAuth', 'setIntegrationSecret', 'syncMessage', 'testMessage', 'systemMediaTransport', 'videoDownload', 'videoCancelDownload', 'galnetRefresh', 'constructor', '__proto__', 'futureHandler']) {
    assert.equal(authorizeRpc({ principal: viewer, handlerName: name }), false)
  }
})

test('local owner bootstrap rejects cross-site, missing origin, rebinding and remote peers', () => {
  const auth = createSocketAuth()
  for (const req of [
    request({ headers: { host: 'localhost:3300', origin: 'https://evil.example' } }),
    request({ headers: { host: 'localhost:3300' } }),
    request({ headers: { host: 'evil.example', origin: 'http://evil.example' } }),
    request({ socket: { remoteAddress: '192.0.2.1' } }),
    request({ headers: { host: 'localhost:3300', origin: 'null' } })
  ]) assert.throws(() => auth.localSession(req))
})

test('pairing is single-use, expires, stays read-only and revokes existing sessions', () => {
  let now = 1000
  const auth = createSocketAuth({ clock: () => now })
  const owner = auth.localSession(request())
  assert.throws(() => auth.createPairingCode({ role: 'owner' }))
  const { code } = auth.createPairingCode(owner)
  const viewer = auth.pair(code)
  assert.equal(viewer.role, 'viewer')
  assert.throws(() => auth.pair(code))
  const cookieReq = request({ headers: { ...request().headers, cookie: `daedalus_session=${viewer.token}` } })
  assert.equal(auth.principal(cookieReq), viewer)
  auth.revokeViewers(owner)
  assert.equal(auth.principal(cookieReq), null)
  const expiring = auth.createPairingCode(owner).code
  now += 5 * 60 * 1000
  assert.throws(() => auth.pair(expiring))
  const session = auth.localSession(request())
  now += 12 * 60 * 60 * 1000
  assert.equal(auth.valid(session), false)
})

test('owner cookie is never accepted from remote transport or peer', () => {
  const auth = createSocketAuth()
  const owner = auth.localSession(request())
  const headers = { ...request().headers, cookie: `daedalus_session=${owner.token}` }
  assert.equal(auth.principal(request({ headers, socket: { remoteAddress: '192.0.2.1', encrypted: true } })), null)
  assert.equal(auth.principal(request({ headers, socket: { remoteAddress: '127.0.0.1', encrypted: true } })), null)
})
