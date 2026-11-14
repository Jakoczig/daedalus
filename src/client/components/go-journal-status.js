import { useEffect, useRef, useState } from 'react'

export async function journalAction (action, values = {}) {
  const response = await window.fetch('/owner/journal', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...values, action }) })
  if (!response.ok) throw new Error('Journal directory change failed. Choose an existing readable absolute folder, or repair the private Go profile.')
  return response.json()
}

export default function GoJournalStatus ({ stage, connected, ready, role, sendEvent, eventListener }) {
  const [summary, setSummary] = useState(null)
  const [error, setError] = useState('')
  const [source, setSource] = useState(null)
  const [directory, setDirectory] = useState('')
  const [pending, setPending] = useState(false)
  const [statusError, setStatusError] = useState('')
  const sourceVersion = useRef(0)
  useEffect(() => {
    if (!connected || !ready) { setSummary(null); return }
    let active = true
    let liveSeen = false
    setStatusError('')
    const stop = eventListener('journalSummaryChanged', value => { liveSeen = true; if (active) { setSummary(value); setStatusError('') } })
    sendEvent('getJournalSummary').then(value => { if (active && !liveSeen) { setSummary(value); setStatusError('') } }).catch(() => { if (active && !liveSeen) setStatusError('Journal status unavailable. Reconnect to the Go service.') })
    return () => { active = false; stop() }
  }, [connected, ready])
  useEffect(() => {
    if (role !== 'owner' || !connected || !ready) return
    let active = true
    const version = ++sourceVersion.current
    journalAction('status').then(value => { if (active && version === sourceVersion.current) { setSource(value); setDirectory(value.Override || value.Directory || '') } }).catch(() => { if (active && version === sourceVersion.current) setError('Local journal settings unavailable.') })
    return () => { active = false }
  }, [role, connected, ready])
  const change = async (action) => {
    const version = ++sourceVersion.current
    setPending(true); setError('')
    try { const result = await journalAction(action, action === 'selectDir' ? { directory: directory.trim() } : {}); if (version === sourceVersion.current) { setSource(result); setDirectory(result.Override || result.Directory || '') } } catch (error) { if (version === sourceVersion.current) setError(error.message) } finally { setPending(false) }
  }
  let status = 'Loading journal status…'
  if (!connected) status = 'Service disconnected. Journal status is unavailable until reconnect.'
  else if (!ready) status = 'Waiting for the Go service to become ready.'
  else if (summary?.importInProgress) status = 'Replaying local journal history…'
  else if (summary?.sourceState === 'waiting') status = 'Waiting for Elite Dangerous journal. Start the game or ask the local owner to select its journal directory.'
  else if (summary?.sourceState === 'waiting_game') status = 'Game session ended. Showing the last known system; waiting for the next session.'
  else if (['permission_error', 'source_error'].includes(summary?.sourceState)) status = 'Journal source unavailable or permission denied. The local owner can repair the directory or permissions.'
  else if (summary?.sourceState === 'partial') status = 'Waiting for a journal line to finish writing.'
  else if (summary?.sourceState === 'parse_warning') status = 'Reading local journal; malformed or oversized records were skipped.'
  else if (summary) status = 'Local journal available. This is independent of service connection readiness.'
  return (
    <section aria-label='Journal status' style={{ padding: '1rem', border: '1px solid currentColor', position: 'relative', zIndex: 20, background: 'var(--color-background, #100d12)' }}>
      <h2>Current system — local journal preview</h2>
      <p role='status'>{status}</p>
      {connected && ready && summary?.currentSystem && <p>Current / last known system: <strong>{summary.currentSystem}</strong></p>}
      {connected && ready && summary && <p>Events read: {summary.eventsRead || 0}. Last safe event: {summary.lastEventType || 'none'} {summary.lastEventAt && <time dateTime={summary.lastEventAt}>{summary.lastEventAt}</time>}</p>}
      <p>{stage === 'G2C' ? 'No provider uploads. Local routes, system maps, body details and confirmed biology are supported. Other ship, engineering and provider features remain unfinished.' : stage === 'G2B' ? 'No provider uploads. The navigation route and current-system survey are journal-only. Other ship, engineering and exploration features remain unfinished.' : 'No provider uploads. Exploration, ship and engineering features remain unimplemented; keep the separate legacy app for those workflows.'}</p>
      {error && <p role='alert'>{error}</p>}
      {statusError && <p role='alert'>{statusError}</p>}
      {role === 'owner' && (
        <details>
          <summary>Journal directory (local owner)</summary>
          <p>Automatic Windows Saved Games / Linux Steam-Proton discovery is used unless overridden. Select the game journal folder, not the Go profile. No files are written to the game directory.</p>
          {source?.Error && <p role='alert'>{source.Error}</p>}
          {source?.Directory && <p>Selected source: <code style={{ overflowWrap: 'anywhere' }}>{source.Directory}</code></p>}
          <form onSubmit={event => { event.preventDefault(); return change('selectDir') }}>
            <label>Journal directory <input required disabled={pending || !connected || !ready} value={directory} onChange={event => { sourceVersion.current++; setDirectory(event.target.value) }} style={{ width: '100%' }} /></label>
            <button disabled={pending || !connected || !ready}>Save journal directory</button>
            <button type='button' disabled={pending || !connected || !ready} onClick={() => change('clearOverride')}>Use automatic discovery</button>
          </form>
        </details>
      )}
    </section>
  )
}
