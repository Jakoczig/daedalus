import { useEffect, useState } from 'react'
import { eliteDateTime } from '../lib/format'
export const show = (value, unit = '', digits = 1) => value == null ? '—' : `${Number(value).toLocaleString(undefined, { maximumFractionDigits: digits })}${unit ? ` ${unit}` : ''}`
export function Icon ({ name, className = '' }) { return <i aria-hidden='true' className={`icon daedalus-terminal-${name} ${className}`} /> }
export function Metric ({ label, value, unit, icon, tone = '' }) { return <div className={`go-metric ${tone}`}><span className='go-metric__label'>{icon && <Icon name={icon} />} {label}</span><strong>{typeof value === 'number' ? show(value, unit) : value ?? '—'}</strong>{value == null && <small className='text-muted'>Unknown / not reported</small>}</div> }
export function Badge ({ children, tone = '' }) { return <span className={`go-badge ${tone}`}>{children}</span> }
export function Clock () {
  const [time, setTime] = useState(eliteDateTime())
  useEffect(() => { if (typeof window === 'undefined') return; const id = setInterval(() => setTime(eliteDateTime()), 1000); return () => clearInterval(id) }, [])
  return <span className='go-clock'><strong>{time.time}</strong><small>{time.day} {time.month} {time.year}</small></span>
}
export function SampleProgress ({ organism }) {
  const count = organism.samples
  return (
    <div className='go-sampling' aria-label={`${organism.species}: ${organism.stage}`}>
      <div className='go-sampling__steps'>{['Log', 'Sample', 'Analyse'].map((label, i) => <span key={label} className={organism.complete || (count != null && count > i) ? 'is-complete' : ''}><b>{i + 1}</b><small>{label}</small></span>)}</div>
      <span className={organism.complete ? 'text-success' : 'text-primary'}>{organism.complete ? 'Analysis witnessed' : organism.stage === 'interrupted' ? 'Interrupted — new log needed' : count == null ? 'Start not witnessed' : `${count}/3 stages witnessed`}</span>
    </div>
  )
}
export function Evidence ({ state }) { const names = { verified: 'JOURNAL VERIFIED', verified_last_known: 'LAST KNOWN ROUTE', observed: 'PARTIAL SURVEY', scanner_complete: 'FSS COMPLETE', partial: 'PARTIAL EVIDENCE', journal_last_known: 'LAST JOURNAL FACTS', cleared: 'ROUTE CLEARED', no_route: 'NO PLOTTED ROUTE', replaying: 'REPLAYING', in_transit_last_known: 'JUMP PENDING', stale_source: 'SOURCE UNAVAILABLE', unknown: 'AWAITING JOURNAL', unverified: 'UNVERIFIED', waiting_game: 'GAME STOPPED' }; return <Badge tone={['verified', 'scanner_complete'].includes(state) ? 'text-success' : 'text-primary'}>{names[state] || 'LAST KNOWN'}</Badge> }
