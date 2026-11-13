/* global WebSocket, CustomEvent */
import { createContext, useState, useContext, useEffect } from 'react'
import Router, { useRouter } from 'next/router'
import notification from 'lib/notification'
import PrimaryNavigation from 'components/primary-navigation'
import Layout from 'components/layout'
import Panel from 'components/panel'
import RemoteDisplays from 'components/remote-displays'
import GoJournalStatus from 'components/go-journal-status'
import * as navigation from 'lib/navigation-items'

let connecting = false
let socket = null// Store socket connection (defaults to null)
let callbackHandlers = Object.create(null) // Pending responses, without inherited names
let reconnectTimer = null
let connectionGeneration = 0
let deferredEventQueue = [] // Store events waiting to be sent (used when server is not ready yet or offline)
let recentBroadcastEvents = 0
let broadcastActivityTimeout = null

const defaultSocketState = {
  role: null,
  needsPairing: false,
  prototype: null,
  connected: false, // Boolean to indicate current connection status
  active: false, // Boolean to indicate if any pending requests
  ready: false // Boolean to indicate if the service is ready and loaded
}

const socketOptions = {
  notifications: false,
  explorationAutoSwitch: false,
  audioEnabled: false,
  landingPadEnabled: false,
  _autoSwitchJumping: false,
  _autoSwitchCooldown: 0
}

const PERSISTED_OPTIONS = ['notifications', 'explorationAutoSwitch', 'landingPadEnabled', 'audioEnabled']
const STORAGE_KEY = 'daedalus-socket-options'
const AUTOSWITCH_STORAGE_KEY = 'daedalus-autoswitch-state'

function setSocketOption (key, value) {
  socketOptions[key] = value
  if (typeof window !== 'undefined' && PERSISTED_OPTIONS.includes(key)) {
    try {
      const saved = JSON.parse(window.localStorage.getItem(STORAGE_KEY) || '{}')
      saved[key] = value
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(saved))
    } catch (e) { /* ignore */ }
  }
}

function persistAutoSwitchState () {
  if (typeof window === 'undefined') return
  try {
    window.sessionStorage.setItem(AUTOSWITCH_STORAGE_KEY, JSON.stringify({
      jumping: socketOptions._autoSwitchJumping,
      cooldown: socketOptions._autoSwitchCooldown
    }))
  } catch (e) { /* ignore */ }
}

// Restore persisted toggle states from localStorage
if (typeof window !== 'undefined') {
  try {
    const saved = JSON.parse(window.localStorage.getItem(STORAGE_KEY))
    if (saved) {
      if (typeof saved.explorationAutoSwitch === 'boolean') socketOptions.explorationAutoSwitch = saved.explorationAutoSwitch
      if (typeof saved.notifications === 'boolean') socketOptions.notifications = saved.notifications
      if (typeof saved.landingPadEnabled === 'boolean') socketOptions.landingPadEnabled = saved.landingPadEnabled
      if (typeof saved.audioEnabled === 'boolean') socketOptions.audioEnabled = saved.audioEnabled
    }
  } catch (e) { /* ignore */ }
  // Restore autoswitch navigation state from sessionStorage
  try {
    const autoState = JSON.parse(window.sessionStorage.getItem(AUTOSWITCH_STORAGE_KEY))
    if (autoState) {
      socketOptions._autoSwitchJumping = autoState.jumping || false
      socketOptions._autoSwitchCooldown = autoState.cooldown || 0
    }
  } catch (e) { /* ignore */ }
}

const isDev = process.env.NODE_ENV === 'development'

/** Timestamped dev-mode logger — matches server-side format for correlation */
function devLog (...args) {
  if (!isDev) return
  const now = new Date()
  const ts = now.toTimeString().slice(0, 8) + '.' + String(now.getMilliseconds()).padStart(3, '0')
  console.log(ts, ...args)
}

function socketDebugMessage () {
  devLog('[SOCKET]', arguments[0]) // Never log credential or pairing payloads
}

async function connect (socketState, setSocketState) {
  if (socket !== null || connecting) return
  connecting = true
  const generation = ++connectionGeneration
  const controller = new AbortController()
  const bootstrapTimer = setTimeout(() => controller.abort(), 10000)
  try {
    const response = await window.fetch('/auth/session', { method: 'POST', credentials: 'same-origin', signal: controller.signal })
    if (generation !== connectionGeneration) return
    if (!response.ok) {
      const needsPairing = response.status === 401 || response.status === 403
      let pairingCodeLength = 32 // Preserve the legacy backend's pairing format.
      if (needsPairing) {
        try { if ((await response.json()).pairingCodeLength === 6) pairingCodeLength = 6 } catch {}
      }
      if (generation !== connectionGeneration) return
      setSocketState(previous => ({ ...previous, role: needsPairing ? null : previous.role, pairingCodeLength, needsPairing, connected: false, ready: false }))
      if (!needsPairing) scheduleReconnect(socketState, setSocketState)
      return
    }
    const { role, prototype } = await response.json()
    if (generation !== connectionGeneration) return
    if (!['owner', 'viewer'].includes(role)) throw new Error('Invalid session response')
    const preview = (['G1A', 'G1B'].includes(prototype?.stage) && prototype?.offline === true) || (prototype?.stage === 'G2A' && prototype?.offline === false)
    setSocketState(previous => ({ ...previous, role, prototype: preview ? prototype : null, needsPairing: false }))
  } catch {
    if (generation === connectionGeneration) scheduleReconnect(socketState, setSocketState)
    return
  } finally { clearTimeout(bootstrapTimer); if (generation === connectionGeneration) connecting = false }

  // Reset on reconnect
  cancelPending()
  deferredEventQueue = []
  if (broadcastActivityTimeout) {
    clearTimeout(broadcastActivityTimeout)
    broadcastActivityTimeout = null
  }
  recentBroadcastEvents = 0

  socket = new WebSocket((window.location.protocol === 'https:' ? 'wss://' : 'ws://') + window.location.host)
  const connection = socket
  const current = () => socket === connection && generation === connectionGeneration
  if (process.env.NODE_ENV === 'development') devLog('[SOCKET] Connecting to service')
  socket.onmessage = (event) => {
    if (!current()) return
    let requestId, name, message
    try {
      ({ requestId, name, message } = JSON.parse(event.data))
    } catch (e) {
      console.error('Failed to parse socket message:', e)
      return
    }
    // Invoke callback to handler (if there is one)
    if (requestId && callbackHandlers[requestId]) callbackHandlers[requestId](event, setSocketState)

    // Updating resync whern loading completes tells any components to resync
    // with the server. it is useful for remote clients that disconnects then
    // reconnects to tell them to update once the service is ready.
    if (name === 'loadingProgress') {
      if (process.env.NODE_ENV === 'development') devLog(`[SOCKET] loadingProgress: complete=${message.loadingComplete} inProgress=${message.loadingInProgress}`)
      if (message.loadingComplete) {
        if (process.env.NODE_ENV === 'development') devLog('[SOCKET] Ready state → true (via loadingProgress)')
        setSocketState(prevState => ({
          ...prevState,
          ready: true
        }))
      }
    }

    // Broadcast event to anything that is listening for an event with this name
    if (!requestId && name) {
      if (process.env.NODE_ENV === 'development') {
        const summary = name === 'gameStateChange'
          ? `file=${message?._changedFile}`
          : name === 'newLogEntry'
            ? `event=${message?.event}`
            : ''
        devLog(`[RECV-BROADCAST] ${name}  ${summary}`)
      }
      window.dispatchEvent(new CustomEvent(`socketEvent_${name}`, { detail: message }))

      // When a broadcast message is received, use recentBroadcastEvents to
      // track recent requests so the activity monitor in the UI can reflect
      // that there is activity and that the client is receiving events.
      recentBroadcastEvents++
      if (!broadcastActivityTimeout) {
        broadcastActivityTimeout = setTimeout(() => {
          broadcastActivityTimeout = null
          recentBroadcastEvents = 0
          setSocketState(prevState => ({
            ...prevState,
            active: socketRequestsPending()
          }))
        }, 500)
      }

      // Trigger notifications for key actions
      // TODO Refactor out into a seperate handler
      try { // Don't crash if fails because properties are missing
        if (socketOptions.notifications === true && name === 'newLogEntry') {
          if (message.event === 'StartJump' && message.StarSystem) notification(`Jumping to ${message.StarSystem}`)
          if (message.event === 'FSDJump') notification(`Arrived in ${message.StarSystem}`)
          if (message.event === 'ApproachBody') notification(`Approaching ${message.Body}`)
          if (message.event === 'LeaveBody') notification(`Leaving ${message.Body}`)
          if (message.event === 'NavRoute') notification('New route plotted')
          if (message.event === 'DockingGranted') notification(`Docking at ${message.StationName}`)
          if (message.event === 'Docked') notification(`Docked at ${message.StationName}`)
          if (message.event === 'Undocked') notification(`Now leaving ${message.StationName}`)
          if (message.event === 'ApproachSettlement') notification(`Approaching ${message.Name}`)
          if (message.event === 'ReceiveText' && message.From) notification(() => <p style={{ width: '100%' }}><span className='text-primary'>{message.From_Localised || message.From}</span><br /><span className='text-info text-no-transform'>{message.Message_Localised || message.Message}</span></p>)
          if (message.event === 'MarketBuy') notification(`Purchased ${message.Type_Localised || message.Type} (${message.Count})`)
          if (message.event === 'MarketSell') notification(`Sold ${message.Type_Localised || message.Type} (${message.Count})`)
          if (message.event === 'BuyDrones') notification(`Purchased Limpet ${message.Count === 1 ? 'Drone' : `Drones (${message.Count})`}`)
          if (message.event === 'SellDrones') notification(`Sold Limpet ${message.Count === 1 ? 'Drone' : `Drones (${message.Count})`}`)
          if (message.event === 'CargoDepot' && message.UpdateType === 'Collect') notification(`Collected ${message.CargoType.replace(/([a-z])([A-Z])/g, '$1 $2')} (${message.Count})`)
          if (message.event === 'CargoDepot' && message.UpdateType === 'Deliver') notification(`Delivered ${message.CargoType.replace(/([a-z])([A-Z])/g, '$1 $2')} (${message.Count})`)
          if (message.event === 'Scanned') notification('Scan detected')
        }
      } catch (e) { console.log('NOTIFICATION_ERROR', e) }

      // ── Auto-switch logic ──
      // Rules:
      // 1. FSD hyperdrive charging (in supercruise or not) → switch to route
      // 2. Jump charge cancelled → switch to system (or bio if in atmosphere)
      // 3. Not in supercruise + hyperspace jump begins → switch to route (journal fallback)
      // 4. Enter a body's atmosphere → switch to bioscanner
      // 5. Leave a body's atmosphere → switch to system

      // Early auto-switch via Status flags (rule 1: FSD hyperdrive charging = jump)
      try {
        if (socketOptions.explorationAutoSwitch && name === 'gameStateChange' && message?._changedFile === 'Status') {
          const statusFlags = message?.Status?.Flags ?? 0
          const statusFlags2 = message?.Status?.Flags2 ?? 0
          const fsdCharging = (statusFlags & 131072) !== 0
          const fsdHyperdriveCharging = (statusFlags2 & 524288) !== 0
          const fsdJump = (statusFlags & 1073741824) !== 0
          const hasLatLon = (statusFlags & 2097152) !== 0
          const path = Router.asPath

          if (path.startsWith('/exploration')) {
            // Skip during post-arrival cooldown to avoid re-triggering
            const inCooldown = socketOptions._autoSwitchCooldown && Date.now() - socketOptions._autoSwitchCooldown < 5000

            // Rule 1: FSD hyperdrive charging → switch to route
            // Fires for both supercruise and normal-space jumps.
            // fsdHyperdriveCharging distinguishes a jump from a supercruise entry.
            if (!inCooldown && !socketOptions._autoSwitchJumping) {
              if (fsdCharging && fsdHyperdriveCharging) {
                socketOptions._autoSwitchJumping = true
                socketOptions._autoSwitchCooldown = 0
                persistAutoSwitchState()
                if (path !== '/exploration/route') Router.push('/exploration/route')
              }
            }

            // Rule 2: Charge cancelled — was jumping but FSD no longer charging and not mid-jump
            if (socketOptions._autoSwitchJumping && !fsdCharging && !fsdJump) {
              socketOptions._autoSwitchJumping = false
              persistAutoSwitchState()
              if (hasLatLon) {
                if (path !== '/exploration/biologicals') Router.push('/exploration/biologicals')
              } else {
                if (path !== '/exploration/system') Router.push('/exploration/system')
              }
            }
          }
        }
      } catch (e) { console.log('AUTO_SWITCH_STATUS_ERROR', e) }

      // Rule 3: Not in supercruise + hyperspace jump begins (StartJump journal event)
      // Also acts as a fallback if Status flags fire too late
      try {
        if (socketOptions.explorationAutoSwitch && name === 'newLogEntry') {
          const path = Router.asPath
          if (path.startsWith('/exploration')) {
            if (message.event === 'StartJump' && message.JumpType === 'Hyperspace') {
              socketOptions._autoSwitchJumping = true
              socketOptions._autoSwitchCooldown = 0
              persistAutoSwitchState()
              if (path !== '/exploration/route') Router.push('/exploration/route')
            }
          }
        }
      } catch (e) { console.log('AUTO_SWITCH_CHARGE_ERROR', e) }

      // Arrival: switch to system (or bio if on a body surface)
      try {
        if (socketOptions.explorationAutoSwitch && name === 'newLogEntry') {
          if (message.event === 'FSDJump' || message.event === 'Location') {
            socketOptions._autoSwitchJumping = false
            socketOptions._autoSwitchCooldown = Date.now()
            persistAutoSwitchState()
            const path = Router.asPath
            if (path.startsWith('/exploration')) {
              if (path !== '/exploration/system') Router.push('/exploration/system')
            }
          }
        }
      } catch (e) { console.log('AUTO_SWITCH_JUMP_ERROR', e) }

      // Rules 4 & 5: Atmosphere enter/leave (suppressed while mid-jump)
      try {
        if (socketOptions.explorationAutoSwitch && !socketOptions._autoSwitchJumping && name === 'newLogEntry') {
          const path = Router.asPath
          if (path.startsWith('/exploration')) {
            if (message.event === 'ApproachBody') {
              if (path !== '/exploration/biologicals') Router.push('/exploration/biologicals')
            }
            if (message.event === 'LeaveBody') {
              if (path !== '/exploration/system') Router.push('/exploration/system')
            }
          }
        }
      } catch (e) { console.log('AUTO_SWITCH_BODY_ERROR', e) }
    }
    socketDebugMessage('Message received from socket server', requestId, name, message)
  }
  socket.onopen = async (e) => {
    if (!current()) return
    if (process.env.NODE_ENV === 'development') devLog('[SOCKET] Connection opened')
    setSocketState(prevState => ({
      ...prevState,
      active: socketRequestsPending(),
      connected: true
    }))

    // While connection remains open and there are queued messages, try to
    // deliver. The readyState check matters because otherwise if a connection
    // does down just after going up we want to catch that scenario, and try to
    // send the message again when the open event is next fired.
    while (socket.readyState === WebSocket.OPEN && deferredEventQueue.length > 0) {
      const { requestId, name, message } = deferredEventQueue[0]
      try {
        socket.send(JSON.stringify({ requestId, name, message }))
        setSocketState(prevState => ({
          ...prevState,
          active: socketRequestsPending()
        }))
        deferredEventQueue.shift() // Remove message from queue once delivered
        socketDebugMessage('Queued message sent to socket server', requestId, name, message)
      } catch (e) {
        // Edge case for flakey connections
        socketDebugMessage('Failed to deliver queued message socket server', requestId, name, message)
      }
    }

    // If we are fully loaded, then set 'ready' state to true, otherwise wait
    // until get a loadingProgress event that indicates the service is loaded
    try {
      const loadingStats = await sendEvent('getLoadingStatus')
      if (!current()) return
      if (process.env.NODE_ENV === 'development') devLog(`[SOCKET] getLoadingStatus: complete=${loadingStats.loadingComplete} inProgress=${loadingStats.loadingInProgress}`)
      if (loadingStats.loadingComplete) {
        if (process.env.NODE_ENV === 'development') devLog('[SOCKET] Ready state → true (via getLoadingStatus)')
        setSocketState(prevState => ({
          ...prevState,
          ready: true
        }))
      }
    } catch (err) {
      if (current()) console.error('getLoadingStatus failed:', err)
    }
  }
  socket.onclose = (e) => {
    if (!current()) return
    socket = null
    connectionGeneration++
    cancelPending()
    if (broadcastActivityTimeout) { clearTimeout(broadcastActivityTimeout); broadcastActivityTimeout = null }
    recentBroadcastEvents = 0
    if (process.env.NODE_ENV === 'development') devLog('[SOCKET] Connection closed — reconnecting in 5s')
    setSocketState(prevState => ({
      ...prevState,
      active: socketRequestsPending(),
      connected: false,
      ready: false
    }))
    scheduleReconnect(socketState, setSocketState)
  }

  socket.onerror = function (err) {
    if (!current()) return
    socketDebugMessage('Socket error', err.message)
    socket.close()
  }
}

function scheduleReconnect (socketState, setSocketState) {
  if (reconnectTimer) return
  reconnectTimer = setTimeout(() => { reconnectTimer = null; connect(socketState, setSocketState) }, 5000)
}

function cancelPending () {
  for (const callback of Object.values(callbackHandlers)) callback.cancel()
  callbackHandlers = Object.create(null)
  deferredEventQueue = []
}

const SocketContext = createContext()

function SocketProvider ({ children }) {
  const [socketState, setSocketState] = useState(defaultSocketState)

  useEffect(() => {
    if (typeof window !== 'undefined' && typeof WebSocket !== 'undefined') {
      connect(socketState, setSocketState)
    }
  }, [])

  return (
    <SocketContext.Provider value={socketState}>
      {socketState.role ? socketState.prototype ? <PrototypeShell /> : children : socketState.needsPairing ? <PairDisplay shortCode={socketState.pairingCodeLength === 6} /> : <p>Connecting to DAEDALUS…</p>}
    </SocketContext.Provider>
  )
}

function PrototypeShell () {
  const { connected, ready, role, prototype } = useSocket()
  const stage = prototype?.stage || 'G1A'
  const router = useRouter()
  const status = !connected
    ? 'Disconnected. Waiting to reconnect to the local Go service.'
    : !ready
        ? 'Connected. Waiting for the local Go service to become ready.'
        : stage === 'G2A' ? 'Connected and ready. Journal availability is shown separately.' : 'Connected and ready. Game features remain unimplemented.'
  const [, group, page = ''] = router.pathname.split('/')
  const menus = {
    cmdr: navigation.CmdrPanelNavItems,
    ship: navigation.ShipPanelNavItems,
    nav: navigation.NavPanelNavItems,
    exploration: navigation.ExplorationPanelNavItems,
    trade: navigation.TradePanelNavItems,
    mining: navigation.MiningPanelNavItems,
    missions: navigation.MissionsPanelNavItems,
    eng: navigation.EngineeringPanelNavItems,
    media: navigation.MediaPanelNavItems,
    controls: navigation.ControlsPanelNavItems
  }
  const items = menus[group] ? menus[group](page, {}).filter(item => item.url) : []
  return (
    <>
      <header><h1>DAEDALUS Terminal</h1><PrimaryNavigation /></header>
      <p role='status'>{stage} {stage === 'G2A' ? 'local journal' : 'offline'} preview: {status}</p>
      {['G1B', 'G2A'].includes(stage) && role === 'owner' && <RemoteDisplays />}
      {stage === 'G2A' && <GoJournalStatus connected={connected} ready={ready} role={role} sendEvent={sendEvent} eventListener={eventListener} />}
      <Layout connected={connected} ready={ready}>
        <Panel navigation={items}>
          <h2>{stage} — {stage === 'G2A' ? 'limited local journal preview' : 'offline desktop foundation'}</h2>
          <p>{stage === 'G2A' ? 'Only local journal ingestion and the sanitized current-system card are implemented. Other gameplay pages remain unimplemented. No provider calls or uploads run.' : 'Game features are not implemented in this preview. No journals are read, no commander data is loaded, and no provider calls or uploads run.'}</p>
          <p>Selected route: <code>{router.pathname}</code></p>
          <p>{stage !== 'G1A' ? 'Paired displays are read-only. Remote access is controlled by the local owner.' : 'Remote displays and pairing are not enabled in G1A.'} Navigation and reload test the existing exported interface only. Keep using the separate legacy application for game features.</p>
        </Panel>
      </Layout>
    </>
  )
}

function PairDisplay ({ shortCode }) {
  const [code, setCode] = useState('')
  const [name, setName] = useState('Browser display')
  const [error, setError] = useState('')
  const pair = async event => {
    event.preventDefault()
    try {
      const response = await window.fetch('/auth/pair', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: code.trim(), name: name.trim() })
      })
      if (!response.ok) throw new Error('Pairing failed. Check the code and HTTPS connection, or try again in a minute.')
      window.location.reload()
    } catch (e) { setError(e.message) }
  }
  return (
    <main style={{ maxWidth: '36rem', margin: '4rem auto', padding: '2rem' }}>
      <h1>Pair this display</h1>
      <p>Open DAEDALUS settings on the host desktop to create a pairing code. Remote displays require the configured HTTPS address and a trusted certificate.</p>
      <form onSubmit={pair}>
        <label>Pairing code <input type={shortCode ? 'text' : 'password'} required minLength={shortCode ? 6 : undefined} maxLength={shortCode ? 6 : undefined} pattern={shortCode ? '[0-9A-HJKMNP-TV-Z]{6}' : undefined} inputMode='text' autoCapitalize={shortCode ? 'characters' : 'none'} autoCorrect='off' spellCheck={false} autoComplete='off' placeholder={shortCode ? '7K4M9P' : undefined} style={shortCode ? { fontSize: '2rem', letterSpacing: '.2em', width: '9em', maxWidth: '100%', boxSizing: 'border-box' } : undefined} value={code} onChange={e => setCode(shortCode ? e.target.value.toUpperCase() : e.target.value)} /></label>
        {shortCode && <p>Enter the six-character code from the owner. It expires after 5 minutes and can be used once. Letters I, L, O and U are not used.</p>}
        <label>Device name <input maxLength={80} value={name} onChange={e => setName(e.target.value)} /></label>
        <button type='submit'>Pair display</button>
      </form>
      {error && <p role='alert'>{error}</p>}
    </main>
  )
}

function useSocket () { return useContext(SocketContext) }

function sendEvent (name, message = null, timeout = 30000) {
  return new Promise((resolve, reject) => {
    if (Object.keys(callbackHandlers).length >= 128 || deferredEventQueue.length >= 128) { reject(new Error('Request limit reached')); return }
    const requestId = generateUuid()
    const sendStart = (process.env.NODE_ENV === 'development') ? Date.now() : 0
    if (process.env.NODE_ENV === 'development') devLog(`[SEND] → ${name}  id=${requestId.slice(0, 8)}`)
    let timer = null
    callbackHandlers[requestId] = (event, setSocketState) => {
      if (timer) clearTimeout(timer)
      const parsed = JSON.parse(event.data)
      delete callbackHandlers[requestId]
      if (process.env.NODE_ENV === 'development') devLog(`[SEND] ← ${name}  id=${requestId.slice(0, 8)}  ${Date.now() - sendStart}ms`)
      setSocketState(prevState => ({
        ...prevState,
        active: socketRequestsPending()
      }))
      if (parsed.error) {
        reject(new Error(parsed.error))
      } else {
        resolve(parsed.message)
      }
    }
    callbackHandlers[requestId].cancel = () => { if (timer) clearTimeout(timer); reject(new Error('Connection interrupted; retry request')) }
    if (timeout > 0) {
      timer = setTimeout(() => {
        delete callbackHandlers[requestId]
        deferredEventQueue = deferredEventQueue.filter(request => request.requestId !== requestId)
        if (process.env.NODE_ENV === 'development') devLog(`[SEND] TIMEOUT ${name}  id=${requestId.slice(0, 8)}  ${timeout}ms`)
        reject(new Error(`sendEvent '${name}' timed out after ${timeout}ms`))
      }, timeout)
    }
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ requestId, name, message }))
    } else {
      deferredEventQueue.push({ requestId, name, message })
    }
  })
}

function eventListener (eventName, callback) {
  const eventHandler = (e) => { callback(e.detail) }
  window.addEventListener(`socketEvent_${eventName}`, eventHandler)
  return () => window.removeEventListener(`socketEvent_${eventName}`, eventHandler)
}

function socketRequestsPending () {
  return !!((Object.keys(callbackHandlers).length > 0 || deferredEventQueue.length > 0 || recentBroadcastEvents > 0))
}

function generateUuid () {
  return Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15)
}

module.exports = {
  SocketProvider,
  useSocket,
  sendEvent,
  eventListener,
  socketOptions,
  setSocketOption,
  devLog
}
