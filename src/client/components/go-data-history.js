import { useEffect, useState } from 'react'
import GoHistoryControls from './go-history-controls'

const pages = ['history', 'ships', 'scans', 'biology', 'sales', 'bgs', 'colonisation', 'squadron', 'goals']
function HistoryWorkspace ({ sendEvent }) {
  const [status, setStatus] = useState(null)
  const [fid, setFID] = useState('')
  const [kind, setKind] = useState('history')
  const [page, setPage] = useState(null)
  const [policy, setPolicy] = useState(null)
  const [mode, setMode] = useState('Never')
  const [providers, setProviders] = useState([])
  const [provider, setProvider] = useState('eddn')
  const [category, setCategory] = useState('travel')
  const [path, setPath] = useState('')
  const [ack, setAck] = useState(false)
  const [publicationAck, setPublicationAck] = useState(false)
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const refresh = async () => { const v = await sendEvent('getGoDataStatus'); setStatus(v); setFID(previous => previous || v.commanders?.[0]?.fid || ''); return v }
  useEffect(() => {
    let active = true
    const load = async () => { try { const v = await sendEvent('getGoDataStatus'); if (active) { setStatus(v); setFID(previous => previous || v.commanders?.[0]?.fid || '') } } catch { if (active) setNotice('Database status unavailable; history has not been reset.') } }
    load()
    sendEvent('getGoProviders').then(v => { if (active) setProviders(v.filter(p => p.capability.upload?.length)) }).catch(() => {})
    const timer = setInterval(load, 3000)
    return () => { active = false; clearInterval(timer) }
  }, [sendEvent])
  useEffect(() => {
    let active = true
    setPage(null); setPolicy(null)
    if (fid) {
      sendEvent('queryGoHistory', { fid, kind, limit: 25 }).then(v => { if (active) setPage(v) }).catch(() => { if (active) setNotice('History query unavailable while database recovery is pending.') })
      sendEvent('getGoPublicationPolicy', { fid }).then(v => { if (active) { setPolicy(v); setMode(v.mode) } }).catch(() => {})
    }
    return () => { active = false }
  }, [fid, kind, sendEvent])
  const action = async (name, values) => {
    setBusy(true); setNotice('')
    try {
      const v = await sendEvent(name, values)
      setNotice(v.sha256 ? `Verified export: ${v.path} · SHA-256 ${v.sha256}` : v.notice || 'Saved. History remains private unless independently authorized.')
      if (v.mode) setPolicy(v)
      await refresh()
    } catch (e) { setNotice(e.message || 'Action failed; original history retained.') } finally { setBusy(false) }
  }
  const categories = providers.find(p => p.capability.id === provider)?.capability.upload || []
  const summary = status?.summary
  const database = status?.database
  return (
    <div className='go-history-workspace'>
      <p role='status'><strong>{summary?.stage || 'RESTORING_CACHE'}</strong> · {summary?.cached ? 'Last-known cached data' : 'Journal synchronization'} · last source event {summary?.lastEventAt || 'unknown'}</p>
      <p>UI readiness is separate from journal synchronization. No network login is needed for local history.</p>
      <dl className='go-history-health'><div><dt>Database</dt><dd>{database?.State || 'not ready'} · {database?.DatabaseBytes == null ? 'size unknown' : (database.DatabaseBytes / 1048576).toFixed(1) + ' MiB (DB + WAL)'} · schema {database?.Schema ?? '—'}</dd></div><div><dt>Events / sources</dt><dd>{database?.Events ?? '—'} / {database?.Files ?? '—'}</dd></div><div><dt>Committed journal bytes</dt><dd>{database?.Bytes ?? '—'} · last offset {database?.LastOffset ?? '—'}</dd></div><div><dt>Last stored event</dt><dd>{database?.LastEvent || 'unknown'}</dd></div></dl>
      {status?.notice && <p role='alert'>{status.notice}</p>}
      {status?.restorePending && <p role='alert'>Verified restore pending. Quit and restart the native application. Provider uploads will require renewed consent.</p>}
      <div className='go-history-queries'><label>Commander <select aria-label='History commander' value={fid} onChange={e => setFID(e.target.value)}><option value=''>Select commander</option>{status?.commanders?.map(c => <option key={c.fid} value={c.fid}>{c.name || c.fid}</option>)}</select></label><label>History <select aria-label='History category' value={kind} onChange={e => setKind(e.target.value)}>{pages.map(p => <option key={p}>{p}</option>)}</select></label></div>
      <div className='go-history-rows'>{page?.rows?.map(r => <details key={r.id}><summary>{r.type} · {r.observedAt || 'time unknown'} · {r.systemName || r.systemAddress || 'location not reported'}</summary><pre>{r.json}</pre>{r.truncated && <p>Preview truncated ({r.rawBytes} original bytes). The complete record remains in the private database export.</p>}</details>)}</div>
      {page && !page.rows.length && <p>No retained observations in this category for the selected commander.</p>}
      {page?.next && <button type='button' disabled={busy} onClick={() => sendEvent('queryGoHistory', { fid, kind, before: page.next, limit: 25 }).then(setPage).catch(() => setNotice('Next page unavailable'))}>Older records</button>}
      <details><summary>Exploration publication policy · {policy?.mode || 'Never'}</summary><p>Never is the default. Each recipient and category also needs independent upload consent. After sale is hold + manual review only. No automatic sale-triggered uploads. Manual review cannot authorize unsupported delayed recipients, including EDDN, EDSM and EDAstro. Ambiguous biological sales remain private.</p><label>Timing <select aria-label='Exploration publication timing' value={mode} onChange={e => setMode(e.target.value)}>{['Never', 'Live', 'After sale'].map(v => <option key={v} value={v}>{v === 'After sale' ? 'After sale — hold + manual review only' : v}</option>)}</select></label><label><input type='checkbox' checked={publicationAck} onChange={e => setPublicationAck(e.target.checked)} /> I reviewed the timing and understand that Live publication or exceptions can disclose current locations.</label><button type='button' disabled={busy || !fid || !publicationAck} onClick={() => action('setGoPublicationPolicy', { fid, mode, acknowledge: publicationAck })}>Save timing policy</button><p>{Object.entries(policy?.counts || {}).map(([key, value]) => `${key}: ${value}`).join(' · ') || 'No publication candidates'}</p>{policy?.notice && <p role='status'>{policy.notice}</p>}<h4>Separate live-only exception</h4><p>Bypasses an After sale location hold only for this exact recipient/category and fresh live events. It cannot authorize historical replay or a disabled provider.</p><label>Recipient <select value={provider} onChange={e => { setProvider(e.target.value); setCategory(providers.find(p => p.capability.id === e.target.value)?.capability.upload?.[0] || '') }}>{providers.map(p => <option key={p.capability.id}>{p.capability.id}</option>)}</select></label><label>Category <select value={category} onChange={e => setCategory(e.target.value)}>{categories.map(c => <option key={c}>{c}</option>)}</select></label><button type='button' disabled={busy || !fid || !publicationAck || !category} onClick={() => action('setGoPublicationException', { fid, provider, category, enabled: true, acknowledge: publicationAck })}>Allow explicit live disclosure</button>{policy?.exceptions?.filter(e => e.enabled).map(e => <p key={e.provider + e.category}>{e.provider} / {e.category} · LIVE EXCEPTION <button type='button' disabled={busy} onClick={() => action('setGoPublicationException', { fid, provider: e.provider, category: e.category, enabled: false, acknowledge: false })}>Revoke exception</button></p>)}</details>
      <GoHistoryControls key={fid} sendEvent={sendEvent} fid={fid} />
      <details><summary>Backup, export and recovery · local owner only</summary><p>Exports contain private commander, location and gameplay history. They contain no API credentials. Store privately; no periodic backups are created. Migration and pre-restore backups are automatic only for those operations.</p><label>Private destination / selected restore file <input aria-label='Database backup or restore path' maxLength={4096} value={path} onChange={e => setPath(e.target.value)} placeholder='Blank for a backup inside the private profile' /></label><label><input type='checkbox' checked={ack} onChange={e => setAck(e.target.checked)} /> I understand the private-history exposure and selected recovery action.</label><div className='go-history-actions'><button type='button' disabled={busy || !ack || !database} onClick={() => action('backupGoHistory', { path, acknowledge: ack })}>Create verified backup / portable export</button><button type='button' disabled={busy || !ack || !path} onClick={() => action('stageGoRestore', { path, acknowledge: ack })}>Verify and stage restore</button><button type='button' disabled={busy || !ack || !database} onClick={() => action('rebuildGoHistory', { acknowledge: ack })}>Rebuild derived views</button></div><p>Restore takes effect after restart, preserves the previous database and holds publication for renewed approval. Rebuild retains original events.</p></details>
      <details><summary>Sources · offsets and recovery</summary>{status?.sources?.map(source => <div className='go-history-source' key={source.id}><strong>{source.state}</strong><code>{source.path}</code><small>{source.offset} / {source.size} bytes · {source.lastSeen}</small>{source.state === 'verified' && <button type='button' disabled={busy || !ack} onClick={() => action('backfillGoOperationsSource', { id: source.id, acknowledge: ack })}>Recover G2L history from hash-verified original</button>}{source.state === 'quarantined' && <button type='button' disabled={busy || !ack} onClick={() => action('reimportGoSource', { source: source.id, acknowledge: ack })}>Reimport this source; retain original generation</button>}</div>)}</details>
      {notice && <p role='status'>{notice}</p>}
    </div>
  )
}
export default function GoDataHistory (props) {
  const [open, setOpen] = useState(false)
  if (props.role !== 'owner') return null
  return <details className='go-data-history' onToggle={e => setOpen(e.currentTarget.open)}><summary>Data / History · SQLite</summary>{open && <HistoryWorkspace {...props} />}</details>
}
