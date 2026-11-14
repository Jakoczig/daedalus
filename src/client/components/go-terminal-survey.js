import { useMemo, useState } from 'react'
import useGoProjection from './go-projection'
import GoTerminalChart from './go-terminal-chart'
import { selectBodies, surveySummary } from '../lib/go-survey'
import { Icon, Metric, Badge, SampleProgress, Evidence, show } from './go-terminal-widgets'
function BodyInspector ({ body, onClose }) {
  const d = body.details || {}
  const facts = [['Radius', show(d.radiusKM, 'km')], ['Gravity', show(body.gravityG, 'g', 2)], ['Temperature', show(d.temperatureK, 'K')], ['Pressure', show(d.pressureATM, 'atm', 3)], ['Mass', show(d.isStar ? d.massSolar : d.massEarths, d.isStar ? 'M☉' : 'M⊕')], ['Orbital period', show(d.orbitalDays, 'days')], ['Rotation', show(d.rotationHours, 'hours')], ['Semi-major axis', show(d.semiMajorAU, 'AU', 3)], ['Eccentricity', show(d.eccentricity, '', 3)], ['Axial tilt', show(d.axialTiltDegrees, '°')]]
  return (
    <aside className='inspector navigation-panel__inspector go-inspector' aria-label='Body inspector'>
      <div className='inspector__title'><button type='button' aria-label='Close body inspector' onClick={onClose}><Icon name='chevron-right' /></button><h3>{d.isStar ? 'Stellar' : 'Body'} telemetry</h3></div>
      <div className='navigation-panel__inspector-heading'><Icon name={d.isStar ? 'star' : 'planet'} /><h2 className='text-info'>{body.name || `Body ${body.bodyId}`}</h2><h3 className='text-primary'>{body.kind || 'Unknown class'}</h3></div>
      <div className='go-chip-row'><Badge tone={body.landable ? 'text-success' : 'text-muted'}>{body.landable == null ? 'Landability unknown' : body.landable ? 'LANDABLE' : 'NOT LANDABLE'}</Badge><Badge tone={body.dssMapped ? 'text-success' : ''}>{body.dssMapped ? 'DSS WITNESSED' : 'DSS NOT WITNESSED'}</Badge>{body.terraformable && <Badge>TERRAFORMABLE</Badge>}</div>
      <h4>Arrival distance <strong className='text-info'>{show(body.distanceLS, 'LS')}</strong></h4><hr />
      <div className='go-fact-grid'>{facts.map(([label, value]) => <div key={label}><small>{label}</small><strong>{value}</strong></div>)}</div>
      <h4>Atmosphere & surface</h4><p>{body.atmosphere || 'Atmosphere not reported'} · {body.volcanism || 'Volcanism not reported'}</p>
      {d.composition?.length > 0 && <div className='go-composition'>{d.composition.map(c => <div key={c.name}><span>{c.name}</span><meter min='0' max='100' value={c.percent} /><b>{show(c.percent, '%')}</b></div>)}</div>}
      {d.atmosphereComposition?.length > 0 && <p className='text-muted'>{d.atmosphereComposition.map(c => `${c.name} ${show(c.percent, '%')}`).join(' · ')}</p>}
      {d.rings?.length > 0 && <><h4><Icon name='planet' /> Rings</h4>{d.rings.map((r, i) => <div className='go-ring' key={i}><strong>{r.name}</strong><small>{r.class.replace('eRingClass_', '')} · {show(r.innerKM, 'km')} – {show(r.outerKM, 'km')}</small></div>)}</>}
      <h4><Icon name='plant' /> Biological observations</h4><div className='go-chip-row'><Badge>{body.bioSignals ?? '—'} BIO SIGNALS</Badge><Badge>{body.geoSignals ?? '—'} GEO SIGNALS</Badge></div><p>{body.genuses?.join(' · ') || 'Genus not reported'}</p>
      {body.organisms?.map((o, i) => <div className='go-organism' key={i}><strong>{o.species}</strong><small>{o.genus}{o.variant && ` · ${o.variant}`}</small><SampleProgress organism={o} /></div>)}
      <details className='go-provenance'><summary>Observation provenance</summary><p>Body #{body.bodyId} · {d.parents?.map(p => `${p.kind} #${p.bodyId}`).join(' → ') || 'Parent not reported'}</p><p>Previously discovered: {body.previouslyDiscovered == null ? 'Unknown' : body.previouslyDiscovered ? 'Yes' : 'No'} · Previously mapped: {body.previouslyMapped == null ? 'Unknown' : body.previouslyMapped ? 'Yes' : 'No'} (source facts). Unknown fields remain unreported. No completeness or sale inventory is inferred.</p></details>
    </aside>
  )
}
export default function GoTerminalSurvey (props) {
  const { value, error } = useGoProjection(props, 'getGoExploration', 'goExplorationChanged')
  const [selected, setSelected] = useState(undefined); const [query, setQuery] = useState(''); const [filter, setFilter] = useState('all'); const [page, setPage] = useState(0)
  const biology = props.path === '/exploration/biologicals'; const list = props.path === '/nav/list'; const bodies = value?.bodies || []
  const filtered = useMemo(() => selectBodies(bodies, query, biology ? 'biology' : filter), [bodies, query, filter, biology]); const summary = useMemo(() => surveySummary(bodies), [bodies])
  const currentPage = Math.min(page, Math.max(0, Math.ceil(filtered.length / 24) - 1)); const visible = filtered.slice(currentPage * 24, currentPage * 24 + 24)
  // Resolve selected data from the current DTO only, never a retained body object.
  const body = selected === null ? null : filtered.find(b => b.bodyId === selected) || filtered[0]
  if (!props.connected || !props.ready) return <div className='go-empty' role='status'><Icon name='signal' /><h2>{props.connected ? 'Awaiting service readiness' : 'Service disconnected'}</h2><p>Live observations resume after reconnect.</p></div>
  return (
    <div className='go-page' aria-label='Exploration terminal'>
      <div className='go-page__heading'><div><span className='text-primary'>{biology ? 'EXOBIOLOGY' : list ? 'SYSTEM CATALOGUE' : 'SYSTEM CARTOGRAPHY'}</span><h2>{value?.systemName || 'Awaiting local journal'}</h2></div><Evidence state={value?.surveyState} /></div>
      {error && <p role='alert'>{error}</p>}
      <div className='go-metrics'><Metric label='Observed / reported bodies' value={`${bodies.length} / ${value?.reportedBodyCount ?? '—'}`} icon='system-bodies' /><Metric label='Your DSS scans' value={summary.mapped} icon='scan' /><Metric label='Biological bodies' value={summary.biologicalBodies} icon='plant' /><Metric label='Analysed organisms' value={summary.analysed} icon='exploration' /></div>
      <div className='go-toolbar'><label><Icon name='search' /><input aria-label='Find observed body or genus' value={query} placeholder='Find body, class, genus or species' onChange={e => { setQuery(e.target.value); setPage(0) }} /></label>{!biology && <select aria-label='Filter observed bodies' value={filter} onChange={e => { setFilter(e.target.value); setPage(0) }}><option value='all'>All observations</option><option value='landable'>Landable</option><option value='unmapped'>No witnessed DSS</option><option value='biology'>Biological evidence</option></select>}<span>{filtered.length} matches</span><button disabled={!currentPage} onClick={() => setPage(currentPage - 1)} aria-label='Previous bodies'>‹</button><span>{currentPage + 1}/{Math.max(1, Math.ceil(filtered.length / 24))}</span><button disabled={(currentPage + 1) * 24 >= filtered.length} onClick={() => setPage(currentPage + 1)} aria-label='Next bodies'>›</button></div>
      {value?.truncated && <p className='go-warning' role='alert'>Partial catalogue: a cache or display budget was reached.</p>}
      <div className={`go-workspace ${body ? '' : 'go-workspace--wide'}`}>
        <main className='go-workspace__main'>
          {biology ? <div className='go-bio-worklist'>{visible.map(b => <button type='button' className={`go-bio-card ${body?.bodyId === b.bodyId ? 'is-selected' : ''}`} key={b.bodyId} onClick={() => setSelected(b.bodyId)}><div><Icon name='planet' /><h3>{b.name || `Body ${b.bodyId}`}</h3><span className='text-primary'>{b.bioSignals ?? '—'} confirmed biological signals</span></div><div className='go-chip-row'>{b.genuses?.map(g => <Badge key={g}>{g}</Badge>)}</div>{b.organisms?.length ? b.organisms.map((o, i) => <div key={i}><strong>{o.species}</strong><SampleProgress organism={o} /></div>) : <p className='text-muted'>Organic sampling not yet witnessed. Genus is not a species prediction.</p>}</button>)}</div> : list ? <div className='go-body-list'>{visible.map(b => <button type='button' key={b.bodyId} onClick={() => setSelected(b.bodyId)} className={body?.bodyId === b.bodyId ? 'is-selected' : ''}><Icon name={b.details?.isStar ? 'star' : 'planet'} /><div><strong>{b.name || `Body ${b.bodyId}`}</strong><small>{b.kind || 'Class unknown'}</small></div><span>{show(b.distanceLS, 'LS')}</span><Badge>{b.dssMapped ? 'DSS' : '—'}</Badge><Badge tone='text-success'>{b.bioSignals ?? '—'} BIO</Badge></button>)}</div> : <GoTerminalChart bodies={visible} contextBodies={bodies} selected={body?.bodyId} onSelect={setSelected} />}
          {!visible.length && <div className='go-empty'><Icon name='scan' /><h3>No matching observations</h3><p>Scan in game or change the local filter.</p></div>}
          <div className='go-body-strip' aria-label='Body selection'>{visible.map(b => <button key={b.bodyId} aria-pressed={body?.bodyId === b.bodyId} onClick={() => setSelected(b.bodyId)} title={b.kind}><Icon name={b.details?.isStar ? 'star' : 'planet'} />{b.name || `#${b.bodyId}`}</button>)}</div>
        </main>
        {body && <BodyInspector body={body} onClose={() => setSelected(null)} />}
      </div>
      <footer className='go-evidence-foot'><span>{value?.allBodiesFound ? 'FSS completion witnessed' : 'Partial local survey'} · FSS {value?.fssProgress == null ? '—' : `${Math.round(value.fssProgress * 100)}%`}</span><span>{biology ? 'Sampling is last-known journal evidence; historical analysis is not sale inventory.' : 'Journal-only schematic · no provider uploads or game control'}</span></footer>
    </div>
  )
}
