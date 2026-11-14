import { useEffect, useState } from 'react'
export function useCommunity (props, local) {
  const [value, setValue] = useState(null)
  useEffect(() => {
    setValue(null)
    if (props.stage !== 'G2E' || !props.connected || !props.ready || !local?.systemName) return
    let active = true
    const load = async () => {
      try {
        let next = await props.sendEvent('getGoCommunity')
        if (props.role === 'owner' && next.enabled && !['cached', 'waiting_for_journal'].includes(next.status)) next = await props.sendEvent('refreshGoCommunity')
        if (active) setValue(next)
      } catch { if (active) setValue(null) }
    }
    load()
    const timer = setInterval(load, 15000)
    return () => { active = false; clearInterval(timer) }
  }, [props.stage, props.connected, props.ready, props.role, props.sendEvent, props.communityVersion, local])
  return value
}
export default function GoCommunitySettings ({ sendEvent, onChange }) {
  const [value, setValue] = useState(null); const [error, setError] = useState(''); const [busy, setBusy] = useState(false)
  useEffect(() => { let active = true; sendEvent('getGoCommunity').then(v => { if (active) setValue(v) }).catch(() => { if (active) setError('Community settings unavailable') }); return () => { active = false } }, [sendEvent])
  const setEnabled = async enabled => {
    setBusy(true); setError('')
    try { setValue(await sendEvent('setGoCommunityEnabled', { enabled })); onChange() } catch { setError('Preference could not be saved. Check private profile permissions.') } finally { setBusy(false) }
  }
  return <section className='go-community' aria-label='Optional EDSM enrichment'><h3>Community system map · EDSM</h3><p>Off by default. Enabling sends the current or owner-requested system name and your network IP to EDSM when the local owner opens the map. No commander, credentials, journals or surface positions are sent. Read-only lookup; no uploads.</p><p>Six-hour shared memory cache · up to 16 systems · no viewer-triggered requests. Provider data can be outdated and does not prove your scans or discoveries.</p>{value && <button disabled={busy} onClick={() => setEnabled(!value.enabled)}>{value.enabled ? 'Disable EDSM lookups' : 'Enable EDSM lookups'}</button>}{value?.enabled && <p>Cache: {value.status} · fetched {value.fetchedAt || 'not yet'} · expires {value.expiresAt || 'unknown'} · stations fetched {value.stationsFetchedAt || 'not yet'}</p>}<strong>{value?.enabled ? 'Enabled' : 'Disabled'}</strong>{value?.notice && <p role='status'>{value.notice}</p>}{error && <p role='alert'>{error}</p>}</section>
}
