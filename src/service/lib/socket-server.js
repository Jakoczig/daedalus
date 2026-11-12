const WebSocket = require('ws')
const { createSocketAuth, authorizeRpc, sameOrigin, localRequest } = require('./socket-auth')

function createSocketBoundary ({ handlers, auth = createSocketAuth() }) {
  const wss = new WebSocket.Server({ noServer: true, maxPayload: 1024 * 1024 })
  let attempts = 0
  let attemptWindow = 0
  const cookie = (p, secure) => `${secure ? '__Host-daedalus_session' : 'daedalus_session'}=${p.token}; Path=/; HttpOnly; SameSite=Strict; Max-Age=43200${secure ? '; Secure' : ''}`
  const reply = (res, status, data, headers = {}) => {
    res.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store', ...headers })
    res.end(JSON.stringify(data))
  }
  async function middleware (req, res, next) {
    // Never allow browsers to expose local bootstrap endpoints through framing
    // or send pairing codes in Referer headers.
    res.setHeader('Referrer-Policy', 'no-referrer')
    res.setHeader('X-Frame-Options', 'DENY')
    const route = req.url.split('?')[0]
    if (!route.startsWith('/auth/')) {
      if (/^\/(videos|system-media-artwork)\//.test(route) && !auth.principal(req)) return reply(res, 403, { error: 'Forbidden' })
      return next()
    }
    if (req.method !== 'POST' || !sameOrigin(req)) return reply(res, 403, { error: 'Forbidden' })
    try {
      let p = auth.principal(req)
      if (route === '/auth/session') {
        if (!p) p = auth.localSession(req)
      } else if (route === '/auth/pair' && req.socket.encrypted) {
        if (Date.now() - attemptWindow > 60000) { attempts = 0; attemptWindow = Date.now() }
        if (++attempts > 10) return reply(res, 429, { error: 'Try again in a minute' })
        if (req.headers['content-type'] !== 'application/json') return reply(res, 400, { error: 'Invalid request' })
        let body = ''
        for await (const chunk of req) {
          body += chunk
          if (body.length > 1024) return reply(res, 413, { error: 'Request too large' })
        }
        p = auth.pair(JSON.parse(body).code)
      } else return reply(res, 403, { error: 'Forbidden' })
      return reply(res, 200, { role: p.role }, { 'Set-Cookie': cookie(p, !!req.socket.encrypted) })
    } catch { return reply(res, 403, { error: 'Pair this display from the local owner window' }) }
  }
  function attach (server) {
    server.on('upgrade', (req, socket, head) => {
      const p = auth.principal(req)
      // HTTP is exclusively the local native/browser interface. No proxy-header
      // trust: remote sessions must terminate TLS in this service.
      if (req.url !== '/' || !sameOrigin(req) || !p || (!req.socket.encrypted && !localRequest(req))) {
        socket.end('HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\n')
        return
      }
      wss.handleUpgrade(req, socket, head, ws => {
        ws.principal = p
        wss.emit('connection', ws)
      })
    })
  }
  wss.on('connection', socket => {
    socket.on('error', () => {})
    socket.on('message', async raw => {
      let requestId
      try {
        const input = JSON.parse(raw.toString())
        requestId = typeof input.requestId === 'string' ? input.requestId.slice(0, 128) : undefined
        const name = input.name
        if (typeof name !== 'string' || !authorizeRpc({ principal: socket.principal, handlerName: name })) throw new Error('Forbidden')
        let data
        if (name === 'createPairingCode') data = auth.createPairingCode(socket.principal)
        else if (name === 'revokePairedDevices') data = auth.revokeViewers(socket.principal)
        else {
          if (!Object.hasOwn(handlers, name) || typeof handlers[name] !== 'function') throw new Error('Forbidden')
          data = await handlers[name](input.message || {})
        }
        if (auth.valid(socket.principal) && socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ requestId, name, message: data }))
      } catch (e) {
        // Handler/provider exceptions can contain URLs, paths or credentials.
        if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ requestId, error: e.message === 'Forbidden' ? 'Forbidden' : 'Request failed' }))
      }
    })
  })
  function broadcast (name, message) {
    for (const socket of wss.clients) {
      if (!auth.valid(socket.principal)) { socket.close(1008, 'Session expired or revoked'); continue }
      if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ name, message }))
    }
  }
  return { auth, middleware, attach, broadcast, close: () => { for (const socket of wss.clients) socket.terminate(); wss.close() } }
}

module.exports = { createSocketBoundary }
