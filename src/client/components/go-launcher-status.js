import { useEffect, useState } from 'react'

export function journalLabel (v) {
  if (!v) return 'Waiting for journal status'
  if (v.importInProgress || v.sourceState === 'replaying') return 'Loading journal history'
  const labels = { waiting_game: 'Waiting for game to start', waiting: 'Waiting for journal files', permission_error: 'Journal permission denied', source_error: 'Journal directory unavailable', db_error: 'Journal history unavailable', partial: 'Waiting for journal record', parse_warning: 'Journal connected · some records unreadable', stopped: 'Journal monitoring stopped' }
  return labels[v.sourceState] || (v.cached ? 'Cached journal history' : v.sourceState === 'live' ? 'Journal connected' : 'Journal status unavailable')
}
export function providerHealth (v, since) {
  if ((!v.config?.read && !v.config?.upload) || v.config?.paused || v.capability?.gate) return 'disabled'
  if (v.lastError) return 'error'
  return Date.parse(v.lastSuccess) >= since ? 'verified' : 'unverified'
}
export default function LauncherServices ({ sendEvent, connected, ready, eventListener }) {
  const [values, setValues] = useState([]); const [index, setIndex] = useState(0); const [error, setError] = useState('')
  const [started] = useState(Date.now())
  useEffect(() => {
    if (!connected || !ready) return
    let active = true
    // Owner-local telemetry only: never refresh credentials or probe a provider.
    sendEvent('getGoProviders').then(v => { if (active) setValues(v.filter(p => (p.config.read || p.config.upload) && !p.config.paused && !p.capability.gate)) }).catch(() => { if (active) setError('Services not checked') })
    return () => { active = false }
  }, [sendEvent, connected, ready])
  useEffect(() => { if (index >= values.length) return; const timer = setTimeout(() => setIndex(index + 1), 650); return () => clearTimeout(timer) }, [index, values])
  useEffect(() => eventListener?.('goProviderStatusChanged', v => { setError(''); setValues(v.filter(p => (p.config.read || p.config.upload) && !p.config.paused && !p.capability.gate)) }), [eventListener])
  if (error) return <span>{error}</span>
  if (index < values.length) return <span role='status'>Reviewing enabled services… {index + 1}/{values.length} · saved status only</span>
  const current = values.filter(v => (v.config.read || v.config.upload) && !v.config.paused && !v.capability.gate)
  const states = current.map(v => providerHealth(v, started)); const errors = states.filter(s => s === 'error').length; const verified = states.filter(s => s === 'verified').length
  return <span>{current.length === 0 ? 'No live services enabled' : verified === current.length ? 'All services OK' : `${errors} service errors · ${verified} ready · ${current.length - errors - verified} not checked`}</span>
}
