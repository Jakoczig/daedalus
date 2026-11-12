require('dotenv').config()

process.title = 'daedalus-service'

const os = require('os')
const fs = require('fs')
const path = require('path')
const exec = require('child_process').exec
const connect = require('connect')
const serveStatic = require('serve-static')
const http = require('http')
const https = require('https')
const { createSocketBoundary } = require('./lib/socket-server')
const { isLoopback } = require('./lib/socket-auth')
const yargs = require('yargs')
const packageJson = require('../../package.json')

const commandLineArgs = yargs
  .help()
  .usage('Usage: $0 --save-game-dir <directory> [--port <port>]')
  .option('port', {
    type: 'number',
    alias: 'p',
    description: 'Port to listen on (default: 3300)'
  })
  .option('save-game-dir', {
    type: 'string',
    alias: 's',
    description: 'Elite Dangerous Save Game Directory'
  })
  .option('parent-pid', {
    type: 'number',
    description: 'PID of parent app process (for orphan detection)'
  })
  .version(packageJson.version)
  .alias('v', 'version')
  .alias('h', 'help')
  .argv

console.log(`DAEDALUS Terminal Service ${packageJson.version}`)

// Parse command line arguments
const PORT = commandLineArgs.port || commandLineArgs.p || 3300 // Port to listen on
const DEVELOPMENT = commandLineArgs.dev || false // Development mode
const WEB_DIR = (() => {
  // In pkg builds, __dirname is a virtual snapshot path — try multiple candidates
  const candidates = [
    path.join(__dirname, '..', '..', 'build', 'client'), // pkg snapshot (assets bundled)
    path.join(path.dirname(process.execPath), 'build', 'client'), // next to exe
    path.join(process.cwd(), 'build', 'client') // working directory
  ]
  return candidates.find(d => fs.existsSync(d)) || candidates[0]
})()
const LOG_DIR = getLogDir()
const RUNTIME_DIR = path.join(os.homedir(), 'AppData', 'Local', 'DAEDALUS Terminal', 'runtime')
const SYSTEM_MEDIA_ARTWORK_DIR = path.join(RUNTIME_DIR, 'system-media-artwork')

if (!fs.existsSync(LOG_DIR)) {
  console.error('ERROR: No save game data found in', LOG_DIR, '\n')
  yargs.showHelp()
  process.exit(1)
} else {
  console.log('Loading save game data from', LOG_DIR)
}

function getLogDir () {
  // Hard coded fallback
  const FALLBACK_LOG_DIR = path.join(os.homedir(), 'Saved Games', 'Frontier Developments', 'Elite Dangerous')
  let logDir = FALLBACK_LOG_DIR

  // For development (or on Unix platforms) you can use the LOG_DIR environment
  // variable to specify the direct path (relative or absolute). You can also
  // use a .env file. This is overriden by command line args.
  if (process.env.LOG_DIR) {
    logDir = process.env.LOG_DIR.startsWith('/') ? process.env.LOG_DIR : path.join(__dirname, process.env.LOG_DIR)
  }
  // Use provided Save Game dir as base path to look for the the files we need
  // This must be obtained via native OS APIs so is typically passed by the client.
  const commandLineSaveGameDir = commandLineArgs.s || commandLineArgs['save-game-dir']
  if (commandLineSaveGameDir) {
    // The option can be a path to a Windows Save Game directory (in which case
    // we append 'Frontier Developments\Elite Dangerous') or the direct path.
    // If it doesn't look like a valid Save Game dir then it is treated it as a
    // direct path.
    const FullPathToSaveGameDir = path.join(commandLineSaveGameDir, 'Frontier Developments', 'Elite Dangerous')
    if (fs.existsSync(FullPathToSaveGameDir)) {
      logDir = FullPathToSaveGameDir
    } else {
      logDir = commandLineSaveGameDir
    }
  }

  return logDir
}

// Export globals BEFORE loading libraries that use them
global.PORT = PORT
global.LOG_DIR = LOG_DIR
global.BROADCAST_EVENT = broadcastEvent
global.DEV_MODE = DEVELOPMENT

// Dev-mode timestamped logger — all server [TAG] lines go through this
// so they can be correlated with journal timestamps and client-side logs.
global.devLog = DEVELOPMENT
  ? (...args) => {
      const now = new Date()
      const ts = now.toTimeString().slice(0, 8) + '.' + String(now.getMilliseconds()).padStart(3, '0')
      console.log(ts, ...args)
    }
  : () => {}

// Initalise simple in-memory object cache (reset when program restarted)
// LRU cache for systems to prevent unbounded memory growth in long sessions.
// Uses a Proxy so consumers can use normal object syntax: CACHE.SYSTEMS[key]
const LRU_MAX_ENTRIES = 500

function createLRUCache (maxEntries) {
  const cache = new Map()
  return new Proxy({}, {
    get (_, key) {
      if (key === Symbol.iterator || key === 'length') return undefined
      if (!cache.has(key)) return undefined
      // Move to end (most recently used)
      const value = cache.get(key)
      cache.delete(key)
      cache.set(key, value)
      return value
    },
    set (_, key, value) {
      if (cache.has(key)) cache.delete(key)
      cache.set(key, value)
      // Evict oldest entry if over limit
      if (cache.size > maxEntries) {
        const oldest = cache.keys().next().value
        cache.delete(oldest)
      }
      return true
    },
    has (_, key) {
      return cache.has(key)
    },
    deleteProperty (_, key) {
      return cache.delete(key)
    }
  })
}

global.CACHE = {
  SYSTEMS: createLRUCache(LRU_MAX_ENTRIES)
}

// Don't load events till globals are set
const { eventHandlers, init } = require('./lib/events')

// Middleware to serve voiceline WAV files at /voicelines/ for client-side audio playback
const covasPlayer = require('./lib/covas-player')
function voicelinesMiddleware (req, res, next) {
  if (req.url && req.url.startsWith('/voicelines/')) {
    const fileName = path.basename(decodeURIComponent(req.url.slice('/voicelines/'.length)))
    // Only serve .wav files, sanitize to prevent path traversal
    if (!fileName || fileName.includes('..') || !fileName.toLowerCase().endsWith('.wav')) {
      res.writeHead(400)
      return res.end()
    }
    const voicelinesDir = covasPlayer.getVoicelinesDir()
    const filePath = path.join(voicelinesDir, fileName)
    if (fs.existsSync(filePath)) {
      const stat = fs.statSync(filePath)
      res.writeHead(200, {
        'Content-Type': 'audio/wav',
        'Content-Length': stat.size,
        'Cache-Control': 'public, max-age=86400'
      })
      fs.createReadStream(filePath).pipe(res)
    } else {
      res.writeHead(404)
      res.end()
    }
    return
  }
  next()
}

// Middleware to serve cached video files at /videos/ with Range request support
const videoManager = require('./lib/video-manager')
function videosMiddleware (req, res, next) {
  if (req.url && req.url.startsWith('/videos/')) {
    const fileName = path.basename(decodeURIComponent(req.url.slice('/videos/'.length)))
    if (!fileName || fileName.includes('..') || !/\.(mp4|webm|mkv)$/i.test(fileName)) {
      res.writeHead(400)
      return res.end()
    }
    const filePath = path.join(videoManager.getCacheDir(), fileName)
    if (!fs.existsSync(filePath)) {
      res.writeHead(404)
      return res.end()
    }
    const stat = fs.statSync(filePath)
    const total = stat.size
    const ext = path.extname(fileName).toLowerCase()
    const mimeTypes = { '.mp4': 'video/mp4', '.webm': 'video/webm', '.mkv': 'video/x-matroska' }
    const contentType = mimeTypes[ext] || 'application/octet-stream'

    // Support Range requests for seeking
    const range = req.headers.range
    if (range) {
      const parts = range.replace(/bytes=/, '').split('-')
      const start = parseInt(parts[0], 10)
      const end = parts[1] ? parseInt(parts[1], 10) : total - 1
      if (start >= total || end >= total || start > end) {
        res.writeHead(416, { 'Content-Range': `bytes */${total}` })
        return res.end()
      }
      res.writeHead(206, {
        'Content-Range': `bytes ${start}-${end}/${total}`,
        'Accept-Ranges': 'bytes',
        'Content-Length': end - start + 1,
        'Content-Type': contentType
      })
      fs.createReadStream(filePath, { start, end }).pipe(res)
    } else {
      res.writeHead(200, {
        'Content-Length': total,
        'Content-Type': contentType,
        'Accept-Ranges': 'bytes',
        'Cache-Control': 'public, max-age=86400'
      })
      fs.createReadStream(filePath).pipe(res)
    }
    return
  }
  next()
}

// Receives the OAuth2 callback forwarded by the Go app when Frontier redirects
// to daedalus://capi-callback?code=...&state=...
// The Go app opens http://localhost:{PORT}/capi-callback?... on the running service.
function capiCallbackMiddleware (req, res, next) {
  if (!req.url?.startsWith('/capi-callback')) return next()

  if (req.url.split('?')[0] !== '/capi-callback' || req.method !== 'GET' || !isLoopback(req.socket.remoteAddress) || req.socket.encrypted) {
    res.writeHead(403)
    return res.end()
  }
  const urlParsed = new URL(req.url, `http://localhost:${PORT}`)
  const code = urlParsed.searchParams.get('code')
  const state = urlParsed.searchParams.get('state')
  const error = urlParsed.searchParams.get('error')

  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
  res.end([
    '<!DOCTYPE html><html><head><meta charset="utf-8">',
    '<title>DAEDALUS Terminal — Auth Complete</title></head>',
    '<body style="background:#111;color:#4DFFC9;font-family:monospace;',
    'text-align:center;padding:4rem;margin:0">',
    '<h2>DAEDALUS Terminal</h2>',
    '<p>Authentication complete. You may close this tab.</p>',
    '</body></html>'
  ].join(''))

  if (global.HANDLE_CAPI_CALLBACK) {
    global.HANDLE_CAPI_CALLBACK({ code, state, error })
  }
}

function systemMediaArtworkMiddleware (req, res, next) {
  if (req.url && req.url.startsWith('/system-media-artwork/')) {
    const requestPath = new URL(req.url, 'http://localhost').pathname
    const fileName = path.basename(decodeURIComponent(requestPath.slice('/system-media-artwork/'.length)))
    if (!fileName || fileName.includes('..') || !/\.(png|jpe?g|webp|bmp)$/i.test(fileName)) {
      res.writeHead(400)
      return res.end()
    }

    const filePath = path.join(SYSTEM_MEDIA_ARTWORK_DIR, fileName)
    if (!fs.existsSync(filePath)) {
      res.writeHead(404)
      return res.end()
    }

    const stat = fs.statSync(filePath)
    const ext = path.extname(fileName).toLowerCase()
    const mimeTypes = {
      '.png': 'image/png',
      '.jpg': 'image/jpeg',
      '.jpeg': 'image/jpeg',
      '.webp': 'image/webp',
      '.bmp': 'image/bmp'
    }

    res.writeHead(200, {
      'Content-Type': mimeTypes[ext] || 'application/octet-stream',
      'Content-Length': stat.size,
      'Cache-Control': 'public, max-age=300'
    })
    fs.createReadStream(filePath).pipe(res)
    return
  }

  next()
}

const socketBoundary = createSocketBoundary({ handlers: eventHandlers })
let requestHandler
if (DEVELOPMENT) {
  // If DEVELOPMENT is specified then HTTP requests other than web socket
  // requests will be forwarded to a web server which is started on localhost
  // to allow UI changes to be tested without rebuilding the app.
  const devServer = exec('npx next src/client')
  devServer.on('error', (err) => console.error('Dev server error:', err))
  devServer.stderr?.on('data', (data) => console.error('Dev server:', data.toString()))
  process.on('exit', () => devServer.kill())
  const httpProxy = require('http-proxy')
  const proxy = httpProxy.createProxyServer({})
  proxy.on('error', (err, req, res) => {
    if (err) console.error('Development proxy unavailable')
    if (res.writeHead) res.writeHead(502, { 'Content-Type': 'text/plain' })
    if (res.end) res.end('Waiting for Next.js dev server...')
  })
  requestHandler = (req, res) => {
    capiCallbackMiddleware(req, res, () => voicelinesMiddleware(req, res, () => videosMiddleware(req, res, () => systemMediaArtworkMiddleware(req, res, () => proxy.web(req, res, { target: 'http://localhost:3000' })))))
  }
} else {
  // The default behaviour (i.e. production) is to serve static assets. When the
  // application is compiled to a native executable these assets will be bundled
  // with the executable in a virtual file system.
  const webServer = connect().use(capiCallbackMiddleware).use(voicelinesMiddleware).use(videosMiddleware).use(systemMediaArtworkMiddleware).use(serveStatic(WEB_DIR, { extensions: ['html'] }))
  requestHandler = webServer
}

const httpServer = http.createServer((req, res) => socketBoundary.middleware(req, res, () => requestHandler(req, res)))
socketBoundary.attach(httpServer)

// The same authentication and RPC boundary applies to every listener.
function broadcastEvent (name, message) {
  socketBoundary.broadcast(name, message)
  if (name === 'loadingProgress' && message.loadingComplete === true) {
    console.log(`Scanned ${message.numberOfFiles} files`)
    console.log(`Imported ${message.numberOfEventsImported} events`)
  }
}

// Remote display access is opt-in and requires direct TLS (no proxy headers).
// The existing Go launcher continues to use HTTP on localhost.
const remotePort = Number(process.env.DAEDALUS_REMOTE_PORT || 0)
if (remotePort) {
  if (!process.env.DAEDALUS_TLS_CERT || !process.env.DAEDALUS_TLS_KEY) throw new Error('Remote access requires TLS certificate and key paths')
  const remoteServer = https.createServer({
    cert: fs.readFileSync(process.env.DAEDALUS_TLS_CERT),
    key: fs.readFileSync(process.env.DAEDALUS_TLS_KEY),
    minVersion: 'TLSv1.2'
  }, (req, res) => socketBoundary.middleware(req, res, () => requestHandler(req, res)))
  socketBoundary.attach(remoteServer)
  remoteServer.on('error', () => { console.error('Remote TLS listener failed'); process.exit(1) })
  remoteServer.listen(remotePort, process.env.DAEDALUS_REMOTE_HOST || '0.0.0.0')
  console.log(`Paired HTTPS displays enabled on port ${remotePort}`)
}

httpServer.on('error', function (error) {
  if (error.code && error.code === 'EADDRINUSE') {
    console.error(`Failed to start service, port ${PORT} already in use.`)
    process.exit(1)
  }
})

// Start server
httpServer.listen(PORT, '127.0.0.1')
console.log(`Listening on port ${PORT}…`)

// Failsafe: if the parent app dies without cleaning up, detect it and exit.
// Polls every 5 seconds to check if the parent process is still alive.
const PARENT_PID = commandLineArgs['parent-pid']
if (PARENT_PID) {
  const parentCheckInterval = setInterval(() => {
    try {
      // process.kill with signal 0 checks existence without killing
      process.kill(PARENT_PID, 0)
    } catch (e) {
      // ESRCH = process not found; parent has exited
      console.log('Parent process no longer running, shutting down service.')
      clearInterval(parentCheckInterval)
      process.exit(0)
    }
  }, 5000)
  parentCheckInterval.unref() // Don't prevent natural exit
}

// Initialize app - start parsing data and watching for game state changes
setTimeout(() => init().catch(e => {
  console.error('Fatal: init failed', e)
  process.exit(1)
}), 500)
