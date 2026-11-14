import { useEffect, useState } from 'react'
// Cached owner-only snapshots; rendering never initiates an external request.
export default function GoProviderReference ({ role, sendEvent }) {
  const [snapshots, setSnapshots] = useState([])
  useEffect(() => { let active = true; if (role === 'owner') sendEvent('getGoProviders').then(values => { if (active) setSnapshots(values.filter(v => v.snapshot?.activeCommander).map(v => v.snapshot)) }).catch(() => {}); return () => { active = false } }, [role, sendEvent])
  if (role !== 'owner' || !snapshots.length) return null
  return <details className='go-provider-reference'><summary>Supplemental provider snapshots · separate from journal</summary>{snapshots.map((s, i) => <section key={i}><h3>{s.source}</h3><small>Fetched {s.fetchedAt} · expires {s.expiresAt}</small><dl>{s.facts.map((f, i) => <div key={i}><dt>{f.name}</dt><dd>{f.value}</dd></div>)}</dl></section>)}</details>
}
