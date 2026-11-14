import { useMemo, useState } from 'react'
import reference from '../data/go-engineering-reference.json'
import { boundedRolls, engineerRows, filterBlueprints, filterMaterials, materialRows, planRecipe } from '../lib/go-engineering'

// This module is loaded on Engineering routes only. Reference contains selected
// original EDCD/Coriolis facts; hashes and source/license provenance live in JSON.
const categories = { 'raw-materials': 'raw', 'manufactured-materials': 'manufactured', 'encoded-materials': 'encoded', 'xeno-materials': 'xeno' }
const countText = count => count == null ? 'Unknown' : count.toLocaleString()
const icon = name => <i aria-hidden='true' className={`icon daedalus-terminal-${name}`} />
const gradeOptions = <>{[1, 2, 3, 4, 5].map(grade => <option key={grade} value={grade}>Grade {grade}</option>)}</>

export default function GoEngineering ({ path = '/eng/blueprints', workspace }) {
  const [query, setQuery] = useState('')
  const [grade, setGrade] = useState('')
  const [selectedId, setSelectedId] = useState('')
  const [recipeGrade, setRecipeGrade] = useState(1)
  const [rolls, setRolls] = useState(1)
  const page = path.split('/')[2] || 'blueprints'
  const materialCategory = categories[page]
  const engineersPage = page === 'engineers'
  const planner = page === 'material-planner' || page === 'wishlist'
  const materials = workspace?.parity?.materials
  const modules = workspace?.parity?.modules || []
  const engineerProgress = workspace?.activity?.commander?.engineers
  const rows = useMemo(() => materialRows(reference.materials, materials), [materials])
  const visibleMaterials = useMemo(() => filterMaterials(rows, { query, grade, category: materialCategory || '' }), [rows, query, grade, materialCategory])
  const blueprints = useMemo(() => filterBlueprints(reference.blueprints, { query, grade }), [query, grade])
  const engineers = useMemo(() => engineerRows(reference.engineers, engineerProgress).filter(e => `${e.name} ${e.description} ${e.systemName}`.toLowerCase().includes(query.trim().toLowerCase())), [query, engineerProgress])
  const candidates = materialCategory ? visibleMaterials : engineersPage ? engineers : blueprints
  const selected = candidates.find(row => (row.symbol || row.id) === selectedId) || candidates[0]
  const activeGrade = selected?.grades?.some(g => g.grade === Number(recipeGrade)) ? Number(recipeGrade) : selected?.grades?.[0]?.grade
  const plan = !materialCategory && !engineersPage ? planRecipe(selected, activeGrade, rolls, materials, reference.materials) : null
  const fitted = selected?.symbol ? modules.filter(module => module.engineering?.blueprint?.toLowerCase() === selected.symbol.toLowerCase()) : []
  const title = materialCategory ? `${materialCategory[0].toUpperCase() + materialCategory.slice(1)} materials` : engineersPage ? 'Engineers' : planner ? 'Material planner' : 'Blueprints'
  const known = materials?.state === 'known'
  const total = rows.filter(row => materialCategory ? row.category === materialCategory : true).reduce((sum, row) => sum + (row.count || 0), 0)
  return (
    <div className='go-page go-engineering' aria-label='Engineering workspace'>
      <div className='go-page__heading'><div><span className='text-primary'>ENGINEERING · LOCAL REFERENCE</span><h2>{icon(engineersPage ? 'engineer' : materialCategory ? `materials-${materialCategory}` : 'engineering')} {title}</h2></div><span className='go-badge'>{known ? 'LAST KNOWN MATERIALS' : 'MATERIAL STOCK UNKNOWN'}</span></div>
      <div className='go-metrics'>
        <div className='go-metric'><span className='go-metric__label'>Reference recipes</span><strong>{reference.blueprints.length}</strong></div>
        <div className='go-metric'><span className='go-metric__label'>Visible results</span><strong>{candidates.length}</strong></div>
        <div className='go-metric'><span className='go-metric__label'>Reported material units{materials?.truncated ? ' · partial' : ''}</span><strong>{known ? total.toLocaleString() : 'Unknown'}</strong></div>
        <div className='go-metric'><span className='go-metric__label'>Plan scope</span><strong>Per roll</strong><small>1–100 rolls · browser session</small></div>
      </div>
      <div className='go-toolbar'>
        <label>Search <input type='search' value={query} onChange={event => setQuery(event.target.value)} placeholder={engineersPage ? 'Engineer, system or specialty' : 'Material, blueprint or module'} /></label>
        {!engineersPage && <label>Reference grade <select aria-label='Reference grade' value={grade} onChange={event => setGrade(event.target.value)}><option value=''>All grades</option>{gradeOptions}</select></label>}
        {materialCategory && <a href='/eng/material-planner'>Open recipe planner</a>}
        {planner && <a href='/eng/blueprints'>Browse blueprints</a>}
      </div>
      {!known && <p className='go-warning'>Material stock unknown. Await a complete journal Materials snapshot to calculate shortages.</p>}
      {materials?.truncated && <p className='go-warning'>Material snapshot is truncated. An absent material has unknown stock.</p>}
      {page === 'wishlist' && <p className='text-muted'>Choose a recipe and roll count for this session. A saved multi-module wishlist is not available in this Go workspace.</p>}
      <div className='go-workspace go-engineering__workspace'>
        <div className='go-workspace__main go-body-list' aria-label={materialCategory ? 'Material catalog' : engineersPage ? 'Engineer reference' : 'Blueprint catalog'}>
          {candidates.map(row => {
            const id = row.symbol || row.id
            return (
              <button type='button' key={id} className={(selected?.symbol || selected?.id) === id ? 'is-selected' : ''} onClick={() => { setSelectedId(id); if (row.grades) setRecipeGrade(grade ? Number(grade) : row.grades[0].grade) }} aria-pressed={(selected?.symbol || selected?.id) === id}>
                {icon(materialCategory ? `materials-${materialCategory}` : engineersPage ? 'engineer' : 'engineering')}
                <div><strong>{row.label || row.name}</strong><small>{materialCategory ? `${row.category} · ${row.grade ? `Grade ${row.grade}` : 'Grade not reported'}` : engineersPage ? `${row.systemName} · ${row.description}` : row.modules.join(' · ')}</small></div>
                <span>{materialCategory ? countText(row.count) : engineersPage ? (row.journalProgress?.progress || 'Progress unknown') : `G${row.grades.map(g => g.grade).join('/')}`}</span>
              </button>
            )
          })}
          {!candidates.length && <div className='go-empty'><h3>No matching {materialCategory ? 'materials' : engineersPage ? 'engineers' : 'blueprints'}</h3><p>{materialCategory === 'xeno' ? 'Xeno materials appear when reported in the local journal; the bundled FDev material catalog covers raw, manufactured and encoded materials.' : 'Adjust the search or grade filter.'}</p></div>}
        </div>
        <aside className='go-inspector navigation-panel__inspector go-engineering__inspector' aria-label={materialCategory ? 'Material inspector' : engineersPage ? 'Engineer inspector' : 'Blueprint inspector'}>
          <h3>{materialCategory ? 'Material inspector' : engineersPage ? 'Engineer inspector' : 'Blueprint inspector'}</h3>
          {!selected && <p className='text-muted'>Select a reference entry to inspect it.</p>}
          {selected && materialCategory && (
            <>
              <h2>{selected.label}</h2><p className='text-primary'>{selected.category} · {selected.grade ? `Grade ${selected.grade}` : 'Grade unknown'}</p>
              <div className='go-fact-grid'><div><small>Journal stock</small><strong>{countText(selected.count)}</strong></div><div><small>Reference symbol</small><strong>{selected.symbol}</strong></div></div>
              <h4>Bundled collection reference</h4>{selected.sources.length ? <ul>{selected.sources.map(source => <li key={source}>{source}</li>)}</ul> : <p className='text-muted'>No collection hint bundled for this material.</p>}
              <h4>Used by bundled recipes</h4><ul>{reference.blueprints.filter(bp => bp.grades.some(g => g.components.some(c => c.name.toLowerCase() === selected.name.toLowerCase()))).map(bp => <li key={bp.symbol}>{bp.name} · {bp.modules.join(', ')}</li>)}</ul>
            </>
          )}
          {selected && engineersPage && (
            <>
              <h2>{selected.name}</h2><p>{selected.description}</p><p className='text-primary'>{icon('location-filled')} {selected.systemName}</p>
              {selected.journalProgress
                ? <><p className='go-badge'>Journal progress: {selected.journalProgress.progress || 'Unknown'}</p><div className='go-fact-grid'><div><small>Journal rank</small><strong>{selected.journalProgress.rank ?? 'Unknown'}</strong></div><div><small>Rank progress (reported)</small><strong>{selected.journalProgress.rankProgress ?? 'Unknown'}</strong></div></div></>
                : <p className='go-badge'>Unlock progress unknown</p>}
              <h4>Bundled unlock reference</h4><ul>{(selected.unlockRequirements || []).map((requirement, i) => <li key={i}><strong>{requirement.stage}: </strong>{requirement.description}</li>)}</ul>
              <h4>Bundled blueprint coverage</h4><ul>{reference.blueprints.filter(bp => bp.engineers.includes(selected.name)).map(bp => <li key={bp.symbol}>{bp.name} · {bp.modules.join(', ')}</li>)}</ul>
            </>
          )}
          {selected && plan && (
            <>
              <h2>{selected.name}</h2><p className='text-primary'>{selected.modules.join(' · ')}</p>
              <div className='go-engineering__recipe-controls'><label>Recipe grade <select aria-label='Recipe grade' value={activeGrade} onChange={event => setRecipeGrade(Number(event.target.value))}>{selected.grades.map(g => <option key={g.grade} value={g.grade}>Grade {g.grade}</option>)}</select></label><label>Rolls <input aria-label='Rolls' type='number' min='1' max='100' step='1' value={rolls} onChange={event => setRolls(boundedRolls(event.target.value))} /></label></div>
              <p className={plan.status === 'ready' ? 'text-success' : 'text-primary'}>{plan.status === 'ready' ? 'Materials available' : plan.status === 'missing' ? 'Missing materials' : 'Availability unknown'} · {plan.rolls} {plan.rolls === 1 ? 'roll' : 'rolls'}</p>
              <table className='go-engineering__recipe'><thead><tr><th>Material</th><th>Need</th><th>Have</th><th>Missing</th></tr></thead><tbody>{plan.components.map(c => <tr key={c.label}><td>{c.label}<small>{c.category}{c.grade ? ` · G${c.grade}` : ''}</small></td><td>{c.required}</td><td>{countText(c.available)}</td><td className={c.missing > 0 ? 'text-danger' : ''}>{countText(c.missing)}</td></tr>)}</tbody></table>
              <p className='text-muted'>Per-roll ingredients from the bundled reference. Roll count does not predict upgrade completion. Synthesis recipes are not bundled.</p>
              <h4>Reference engineers</h4><p>{selected.engineers.join(', ') || 'None bundled'}</p><small className='text-muted'>Engineer access and unlock state unknown.</small>
              <h4>Reported fitted engineering</h4>{!fitted.length && <p className='text-muted'>No matching fitted module reported.</p>}
              {fitted.map(module => <section className='go-engineering__module' key={`${module.slot}:${module.item || module.name}`}><strong>{module.name || module.item}</strong><p>{module.slot} · Grade {module.engineering.grade ?? 'unknown'} · Quality {module.engineering.quality ?? 'unknown'}</p>{module.engineering.experimental && <p>{module.engineering.experimental}</p>}<dl>{(module.engineering.modifiers || []).map((modifier, i) => <div key={i}><dt>{modifier.label}</dt><dd>{modifier.value ?? 'Unknown'}{modifier.originalValue != null ? ` (original ${modifier.originalValue})` : ''}{modifier.lessIsGood === true ? ' · lower is better' : modifier.lessIsGood === false ? ' · higher is better' : ''}</dd></div>)}</dl></section>)}
            </>
          )}
        </aside>
      </div>
      <div className='go-evidence-foot'><span>{reference.provenance.label} · EDCD FDevIDs / Coriolis / DAEDALUS</span><span>Journal counts are last known · local planning</span></div>
    </div>
  )
}
