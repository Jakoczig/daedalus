import { useEffect, useRef, useState } from 'react'
import useGoProjection from './go-projection'

const labels = {
  no_route: 'No route is attested in the local journal.',
  cleared: 'Route cleared. Previous plotted waypoints are no longer active.',
  unverified: 'Route unverified. Waiting for matching journal and sidecar evidence.',
  stale: 'Route stale or source unavailable. No active plotted route is claimed.',
  partial: 'Partial route: a size or data limit was reached. No final destination is inferred.',
  waiting_game: 'Waiting for game. Previous route is not active.',
  replaying: 'Replaying local journal history…',
  verified: 'Route verified against this session’s journal marker.',
  verified_last_known: 'Last-known journal-verified route. Current game activity is not attested by replay.'
}
export default function GoNavigation (props) {
  const { value, error } = useGoProjection(props, 'getGoNavigation', 'goNavigationChanged')
  const [copyNotice, setCopyNotice] = useState('')
  const focus = useRef(null)
  useEffect(() => { focus.current?.scrollIntoView?.({ block: 'nearest' }); setCopyNotice('') }, [value, props.connected])
  if (!props.connected) return <section aria-label='Navigation route'><h2>Navigation — plotted game route</h2><p role='status'>Service disconnected. Route unavailable until reconnect.</p></section>
  if (!props.ready) return <p role='status'>Waiting for the Go service to become ready.</p>
  const copy = async name => {
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard unavailable')
      await navigator.clipboard.writeText(name)
      setCopyNotice('Copied on this display only.')
    } catch { setCopyNotice('Copy unavailable. Select the system name and copy it manually on this device.') }
  }
  const nextExists = value?.hops?.some(hop => hop.isNext)
  return (
    <section aria-label='Navigation route' style={{ minWidth: 0 }}>
      <h2>Navigation — plotted game route</h2>
      <p>Journal-only route and target observations. This read-only screen cannot control the game.</p>
      <p role='status'>{labels[value?.routeState] || 'Loading local route evidence…'}</p>
      {error && <p role='alert'>{error}</p>}
      {value?.truncated && <p role='alert'>Route exceeds the display budget or is incomplete. Do not infer a destination from a partial list.</p>}
      {value && (
        <>
          <dl style={{ overflowWrap: 'anywhere' }}>
            <dt>Current / last-known system</dt><dd>{value.currentSystem || 'Unknown'} {value.currentSystem && <button type='button' onClick={() => copy(value.currentSystem)}>Copy system name</button>}</dd>
            <dt>Next target</dt><dd>{value.targetSystem || 'Unknown'} {value.targetSystem && <button type='button' onClick={() => copy(value.targetSystem)}>Copy target name</button>}</dd>
            <dt>Target evidence</dt><dd>{value.targetEvidence === 'journal_target' ? 'Journal FSDTarget' : value.targetEvidence === 'verified_route_next' ? 'Next matched plotted waypoint' : value.targetEvidence === 'route_conflict' ? 'Route / target disagreement' : value.targetEvidence === 'last_known_target' ? 'Last-known journal target' : 'Unknown'}</dd>
            <dt>Plotted destination</dt><dd>{value.destinationSystem || 'Unknown'}</dd>
            <dt>Remaining plotted hops</dt><dd>{value.remainingJumps ?? 'Unknown'}</dd>
            <dt>Game-reported remaining jumps</dt><dd>{value.reportedRemainingJumps ?? 'Unknown'} (separate journal observation)</dd>
          </dl>
          {value.targetEvidence === 'route_conflict' && <p role='alert'>The selected target differs from the next attested route hop. It is shown separately.</p>}
          {value.hops?.length > 0 && (
            <div style={{ overflowX: 'auto', maxHeight: '55vh', overflowY: 'auto' }}>
              <ol aria-label='Plotted waypoints' style={{ paddingInlineStart: '2rem' }}>
                {value.hops.map((hop, index) => (
                  <li key={index} ref={hop.isNext || (!nextExists && hop.isCurrent) ? focus : null} aria-current={hop.isCurrent ? 'location' : undefined} style={{ padding: '.6rem', overflowWrap: 'anywhere', borderBottom: '1px solid currentColor' }}>
                    <strong>{hop.name}</strong> {hop.isCurrent && <span> — Current</span>} {hop.isNext && <span> — Next</span>}
                    <p>Star class: {hop.starClass || 'Unknown'} · Previous hop: {hop.distanceFromPreviousLY == null ? 'Unknown' : `${hop.distanceFromPreviousLY.toFixed(2)} LY`}</p>
                    <button type='button' onClick={() => copy(hop.name)}>Copy waypoint name</button>
                  </li>
                ))}
              </ol>
            </div>
          )}
        </>
      )}
      {copyNotice && <p role='status'>{copyNotice}</p>}
      <p>Distances are approximate geometry, not fuel or jump-safety advice. No uploads, route optimization or automatic targeting.</p>
    </section>
  )
}
