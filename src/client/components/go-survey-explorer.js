import { useMemo, useState } from 'react'
import useGoProjection from './go-projection'
import GoSystemMap from './go-system-map'
import { layoutBodies, selectBodies, surveySummary } from '../lib/go-survey'
const format = (n, unit = '') => n == null ? 'Unknown' : `${Number(n).toLocaleString(undefined, { maximumFractionDigits: 3 })} ${unit}`
const truth = n => n == null ? 'Unknown' : n ? 'Yes' : 'No'
function Organisms ({ body }) {
  return (
    <div aria-label='Confirmed organic sampling'>
      <p>Biological signals: {body.bioSignals ?? 'Unknown'} · Geological signals: {body.geoSignals ?? 'Unknown'}</p>
      <p>Observed genus: {body.genuses?.join(', ') || 'Not reported'}</p>
      {body.organisms?.length > 0
        ? <ul>{body.organisms.map((o, i) => <li key={i}><strong>{o.species}</strong> · {o.genus}{o.variant && ` · ${o.variant}`}<p>{o.complete ? 'Analysis complete — witnessed Analyse' : o.stage === 'interrupted' ? 'Interrupted by another organism — start a fresh log' : o.samples == null ? 'Partial history — start not witnessed' : `${o.samples}/3 sample stages witnessed`}</p><p>Last journal observation: {o.lastObservedAt}. {o.complete ? 'Historical completion, not a claim that data is unsold.' : 'Last-known sampler evidence; check the game after restart.'}</p></li>)}</ul>
        : <p>No species identified by an organic sampling event. Signals/genus alone do not identify species.</p>}
    </div>
  )
}
function Details ({ body }) {
  const d = body.details || {}
  const values = [['Class', body.kind || 'Unknown'], ['Arrival distance', format(body.distanceLS, 'LS')], ['Landable', truth(body.landable)], ['Terraformable', truth(body.terraformable)], ['Gravity', format(body.gravityG, 'g')], ['Radius', format(d.radiusKM, 'km')], ['Temperature', format(d.temperatureK, 'K')], ['Surface pressure', format(d.pressureATM, 'atm')], ['Mass', d.isStar ? format(d.massSolar, 'solar masses') : format(d.massEarths, 'Earth masses')], ['Atmosphere', body.atmosphere || 'Unknown / not reported'], ['Volcanism', body.volcanism || 'Unknown / not reported'], ['Orbital period', format(d.orbitalDays, 'days')], ['Semi-major axis', format(d.semiMajorAU, 'AU')], ['Rotation period', format(d.rotationHours, 'hours (negative = retrograde)')], ['Eccentricity', format(d.eccentricity)], ['Axial tilt', format(d.axialTiltDegrees, 'degrees')], ['Tidally locked', truth(d.tidalLock)], ['Luminosity', d.luminosity || 'Unknown'], ['Stellar age', format(d.ageMY, 'million years')], ['Previously discovered', `${truth(body.previouslyDiscovered)} (source fact)`], ['Previously mapped', `${truth(body.previouslyMapped)} (source fact)`], ['Your DSS scan', body.dssMapped ? 'Witnessed SAAScanComplete' : 'Not witnessed']]
  return (
    <section aria-label='Selected celestial body' style={{ border: '1px solid currentColor', padding: '1rem', marginBlock: '1rem', overflowWrap: 'anywhere' }}>
      <h3>{body.name || `Body ${body.bodyId}`} · exact body ID {body.bodyId}</h3>
      <dl style={{ display: 'grid', gridTemplateColumns: 'minmax(8rem, 1fr) minmax(8rem, 2fr)', gap: '.35rem' }}>{values.map(([label, value]) => <div key={label} style={{ display: 'contents' }}><dt>{label}</dt><dd style={{ margin: 0 }}>{value}</dd></div>)}</dl>
      <p>Parent chain (nearest first): {d.parents?.map(p => `${p.kind} #${p.bodyId}`).join(' → ') || 'Not reported'}</p>
      <p>Composition: {d.composition?.map(c => `${c.name} ${format(c.percent, '%')}`).join(', ') || 'Not reported'}</p>
      <p>Atmosphere composition: {d.atmosphereComposition?.map(c => `${c.name} ${format(c.percent, '%')}`).join(', ') || 'Not reported'}</p>
      {d.rings?.length > 0 && <ul aria-label='Observed rings'>{d.rings.map((r, i) => <li key={i}>{r.name} · {r.class} · {format(r.innerKM, 'km')} to {format(r.outerKM, 'km')}</li>)}</ul>}
      <Organisms body={body} />
    </section>
  )
}
export default function GoSurveyExplorer (props) {
  const { value, error } = useGoProjection(props, 'getGoExploration', 'goExplorationChanged')
  const [query, setQuery] = useState(''); const [filter, setFilter] = useState('all'); const [page, setPage] = useState(0); const [selection, setSelection] = useState(null)
  const bodies = value?.bodies || []
  const filtered = useMemo(() => selectBodies(bodies, query, props.biology ? 'biology' : filter), [bodies, query, filter, props.biology])
  const nodes = useMemo(() => layoutBodies(filtered), [filtered])
  const summary = useMemo(() => surveySummary(bodies), [bodies])
  const currentPage = Math.min(page, Math.max(0, Math.ceil(filtered.length / 25) - 1))
  // Only the ID is retained; all displayed facts come from the current DTO.
  const selected = filtered.find(b => b.bodyId === selection) || filtered[0]
  if (!props.connected) return <p role='status'>Service disconnected. Survey and biology unavailable until reconnect.</p>
  if (!props.ready) return <p role='status'>Waiting for the Go service to become ready.</p>
  const choose = id => setSelection(id)
  return (
    <section aria-label={props.biology ? 'Exobiology journal workflow' : 'Current-system exploration workspace'} style={{ minWidth: 0 }}>
      <h2>{props.biology ? 'Exobiology — confirmed observations' : 'System map and exploration'} — {value?.systemName || 'Unknown system'}</h2>
      <p><a href='/nav/route'>Plotted route</a> · <a href='/exploration/system'>System map</a> · <a href='/exploration/biologicals'>Exobiology</a></p>
      <p role='status'>{value ? `Journal evidence: ${value.surveyState}.` : 'Loading local survey…'} {['unverified', 'stale_source', 'in_transit_last_known', 'replaying'].includes(value?.surveyState) && 'These observations are not a current verified game state.'}</p>
      {error && <p role='alert'>{error}</p>}
      <p>Reported bodies: {value?.reportedBodyCount ?? 'Unknown'} · Observed rows: {bodies.length} · Your DSS: {summary.mapped} · Landable: {summary.landable} · Biological bodies: {summary.biologicalBodies} · Analysed organisms: {summary.analysed}</p>
      <p>FSS progress: {value?.fssProgress == null ? 'Unknown' : `${Math.round(value.fssProgress * 100)}%`} · All-bodies completion: {value?.allBodiesFound ? 'Witnessed' : 'Not witnessed'}. Scan rows can still be incomplete.</p>
      {value?.truncated && <p role='alert'>Cache or display budget reached; the displayed survey is incomplete.</p>}
      <label>Find body / class / genus / species <input value={query} onChange={e => { setQuery(e.target.value); setPage(0) }} /></label>{' '}
      {!props.biology && <label>Show <select value={filter} onChange={e => { setFilter(e.target.value); setPage(0) }}><option value='all'>All observed bodies</option><option value='landable'>Landable</option><option value='unmapped'>No witnessed DSS</option><option value='biology'>Biological evidence</option></select></label>}
      <p>Showing {filtered.length} matching observations. Map and table are paged to bound tablet rendering.</p>
      {!props.biology && !props.listOnly && <><p>Schematic parent hierarchy, not orbital position or physical scale. Unobserved parents are marked, never guessed.</p><GoSystemMap nodes={nodes.slice(currentPage * 25, currentPage * 25 + 25)} selected={selected?.bodyId} onSelect={choose} /></>}
      {selected && <Details body={selected} />}
      <div style={{ overflowX: 'auto' }}><table aria-label='Survey worklist' style={{ width: '100%' }}><thead><tr><th>Body / class</th><th>Distance / gravity</th><th>DSS / biological evidence</th></tr></thead><tbody>{filtered.slice(currentPage * 25, currentPage * 25 + 25).map(body => <tr key={body.bodyId}><td><button type='button' onClick={() => choose(body.bodyId)} aria-pressed={selected?.bodyId === body.bodyId}>{body.name || `Body ${body.bodyId}`}</button><p>{body.kind || 'Unknown class'}</p></td><td>{format(body.distanceLS, 'LS')}<p>{format(body.gravityG, 'g')}</p></td><td>{body.dssMapped ? 'DSS witnessed' : 'DSS not witnessed'}<p>Bio: {body.bioSignals ?? 'Unknown'} · Genus: {body.genuses?.join(', ') || 'Unknown'}</p>{body.organisms?.map((o, i) => <p key={i}>{o.species}: {o.complete ? 'Analysed' : o.samples == null ? o.stage : `${o.samples}/3`}</p>)}</td></tr>)}</tbody></table></div>
      {!filtered.length && <p>No matching local observations. Scan the system or change the filters.</p>}
      <p><button type='button' disabled={currentPage === 0} onClick={() => setPage(currentPage - 1)}>Previous bodies</button> Page {currentPage + 1} / {Math.max(1, Math.ceil(filtered.length / 25))} <button type='button' disabled={(currentPage + 1) * 25 >= filtered.length} onClick={() => setPage(currentPage + 1)}>Next bodies</button></p>
      <p>Journal-only, read-only workflow: locate a confirmed biological body → DSS/genus observations → Log → Sample → Analyse. A different organism interrupts the prior partial run. No species predictions, payouts, first-footfall assumptions, surface radar or game commands. Missing history stays unknown. Historical analysis is not unsold inventory.</p>
    </section>
  )
}
