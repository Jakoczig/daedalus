const test = require('node:test')
const assert = require('node:assert/strict')
const http = require('node:http')
const https = require('node:https')
const fs = require('node:fs')
const path = require('node:path')
const { once } = require('node:events')
const WebSocket = require('ws')
const { createSocketBoundary } = require('../../src/service/lib/socket-server')

async function fixture (t) {
  const boundary = createSocketBoundary({
    handlers: {
      getMiningStatus: () => ({ active: true }),
      setPreferences: () => ({ saved: true }),
      explode: () => { throw new Error('synthetic-secret-error') }
    }
  })
  const server = http.createServer((req, res) => boundary.middleware(req, res, () => { res.end('public UI') }))
  boundary.attach(server)
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  t.after(() => { boundary.close(); server.closeAllConnections(); server.close() })
  const origin = `http://127.0.0.1:${server.address().port}`
  const bootstrap = await fetch(`${origin}/auth/session`, { method: 'POST', headers: { Origin: origin } })
  assert.equal(bootstrap.status, 200)
  const cookie = bootstrap.headers.get('set-cookie').split(';')[0]
  return { boundary, server, origin, cookie }
}

async function connect (origin, cookie, extra = {}) {
  const ws = new WebSocket(origin.replace('http', 'ws'), { headers: { Origin: origin, Cookie: cookie, ...extra } })
  await once(ws, 'open')
  return ws
}

async function rpc (ws, name, message) {
  const response = new Promise(resolve => {
    function receive (raw) {
      const data = JSON.parse(raw)
      if (data.requestId !== 'fixture') return
      ws.off('message', receive)
      resolve(data)
    }
    ws.on('message', receive)
  })
  ws.send(JSON.stringify({ requestId: 'fixture', name, message }))
  return response
}

test('real HTTP/WS rejects missing auth, cross-site upgrades, inherited methods and leaked errors', async t => {
  const { origin, cookie } = await fixture(t)
  for (const headers of [{ Origin: origin }, { Origin: 'https://evil.example', Cookie: cookie }]) {
    const ws = new WebSocket(origin.replace('http', 'ws'), { headers })
    const [error] = await once(ws, 'error')
    assert.match(error.message, /403/)
  }
  const owner = await connect(origin, cookie)
  t.after(() => owner.terminate())
  assert.deepEqual((await rpc(owner, 'setPreferences')).message, { saved: true })
  assert.equal((await rpc(owner, 'constructor')).error, 'Forbidden')
  assert.equal((await rpc(owner, 'explode')).error, 'Request failed')
  assert.equal((await fetch(`${origin}/auth/session`, { method: 'POST', headers: { Origin: 'https://evil.example' } })).status, 403)
  assert.equal((await fetch(`${origin}/system-media-artwork/example.png`)).status, 403)
})

test('paired display receives journal broadcasts and read RPC; revoke blocks an already-open socket', async t => {
  const { origin, cookie, boundary } = await fixture(t)
  const owner = await connect(origin, cookie)
  t.after(() => owner.terminate())
  const pairing = (await rpc(owner, 'createPairingCode')).message
  // TLS-only HTTP pairing is separately tested below. Mint the session through
  // the same auth object here to exercise real WebSocket policy on loopback.
  const viewerSession = boundary.auth.pair(pairing.code)
  const viewer = await connect(origin, `daedalus_session=${viewerSession.token}`)
  t.after(() => viewer.terminate())
  assert.equal((await rpc(viewer, 'getMiningStatus')).message.active, true)
  assert.equal((await rpc(viewer, 'setPreferences')).error, 'Forbidden')
  assert.equal((await rpc(viewer, 'createPairingCode')).error, 'Forbidden')
  const broadcast = once(viewer, 'message')
  boundary.broadcast('newLogEntry', { event: 'FSDJump', StarSystem: 'Synthetic' })
  assert.equal(JSON.parse((await broadcast)[0]).message.event, 'FSDJump')
  // Drain owner's copy of the broadcast before requesting revocation.
  await new Promise(resolve => setImmediate(resolve))
  assert.equal((await rpc(owner, 'revokePairedDevices')).message.revoked, true)
  assert.equal((await rpc(viewer, 'getMiningStatus')).error, 'Forbidden')
  assert.equal(boundary.auth.valid(viewerSession), false)
})

test('pairing endpoint refuses plaintext transport even with valid code', async t => {
  const { origin, cookie } = await fixture(t)
  const owner = await connect(origin, cookie)
  t.after(() => owner.terminate())
  const { code } = (await rpc(owner, 'createPairingCode')).message
  const response = await fetch(`${origin}/auth/pair`, { method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json' }, body: JSON.stringify({ code }) })
  assert.equal(response.status, 403)
})

test('direct TLS pairing sets secure cookie, ignores requested role, and rejects reuse and brute force', async t => {
  const { boundary, origin, cookie } = await fixture(t)
  const cert = fs.readFileSync(path.join(__dirname, '../fixtures/tls/cert.pem'))
  const tlsServer = https.createServer({ cert, key: fs.readFileSync(path.join(__dirname, '../fixtures/tls/key.pem')) }, (req, res) => boundary.middleware(req, res, () => res.end('UI')))
  boundary.attach(tlsServer)
  tlsServer.listen(0, '127.0.0.1')
  await once(tlsServer, 'listening')
  t.after(() => { tlsServer.closeAllConnections(); tlsServer.close() })
  const remoteOrigin = `https://127.0.0.1:${tlsServer.address().port}`
  const post = body => new Promise((resolve, reject) => {
    const req = https.request(`${remoteOrigin}/auth/pair`, { method: 'POST', ca: cert, headers: { Origin: remoteOrigin, 'Content-Type': 'application/json' } }, res => {
      let text = ''
      res.on('data', chunk => { text += chunk })
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: JSON.parse(text) }))
    })
    req.on('error', reject)
    req.end(JSON.stringify(body))
  })
  const owner = await connect(origin, cookie)
  t.after(() => owner.terminate())
  const { code } = (await rpc(owner, 'createPairingCode')).message
  const paired = await post({ code, role: 'owner' })
  assert.equal(paired.status, 200)
  assert.equal(paired.body.role, 'viewer')
  assert.match(paired.headers['set-cookie'][0], /^__Host-daedalus_session=.*; HttpOnly; SameSite=Strict; Max-Age=43200; Secure$/)
  const ws = new WebSocket(remoteOrigin.replace('https', 'wss'), { ca: cert, headers: { Origin: remoteOrigin, Cookie: paired.headers['set-cookie'][0].split(';')[0] } })
  await once(ws, 'open')
  t.after(() => ws.terminate())
  assert.equal((await rpc(ws, 'getMiningStatus')).message.active, true)
  assert.equal((await rpc(ws, 'beginCapiAuth')).error, 'Forbidden')
  assert.equal((await post({ code })).status, 403)
  for (let i = 0; i < 8; i++) assert.equal((await post({ code: 'wrong' })).status, 403)
  assert.equal((await post({ code: 'wrong' })).status, 429)
})
