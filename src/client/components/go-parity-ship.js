import { useMemo, useState } from 'react'
import { Icon, Metric, Evidence, show } from './go-terminal-widgets'

const switches = [['lightsOn', 'Ship lights'], ['nightVision', 'Night vision'], ['cargoScoopDeployed', 'Cargo hatch'], ['landingGearDown', 'Landing gear'], ['hardpointsDeployed', 'Hard points']]
const indicators = [['overheating', 'Overheating'], ['beingInterdicted', 'Interdiction'], ['inDanger', 'Hazard'], ['lowFuel', 'Low fuel'], ['shieldsUp', 'Shields up'], ['fsdMassLocked', 'Mass locked'], ['fsdCooldown', 'FSD cooldown'], ['supercruise', 'Supercruise'], ['fsdCharging', 'FSD charging'], ['fsdJump', 'FSD jumping'], ['docked', 'Docked'], ['landed', 'Landed'], ['flightAssistOff', 'Flight assist off'], ['silentRunning', 'Silent running'], ['scoopingFuel', 'Fuel scooping']]
const slotLabel = slot => slot.replaceAll('_', ' ').replace(/([a-z])([A-Z0-9])/g, '$1 $2')
const onOff = value => value == null ? 'Unknown' : value ? 'On' : 'Off'
function Meter ({ label, value, max, unit = 'T' }) {
  return <div className='go-flight-meter'><span>{label} <b>{show(value, unit)} / {show(max, unit)}</b></span>{value != null && max > 0 ? <progress value={Math.max(0, Math.min(max, value))} max={max} /> : <span className='text-muted'>Unknown / not reported</span>}</div>
}
function Status ({ model }) {
  const ship = model.ship || {}; const status = model.status || {}; const flags = status.state === 'known' && status.flags?.inMainShip === true ? status.flags : null
  const mainFuel = flags?.inMainShip ? status.fuelMainT ?? ship.fuelMainT : ship.fuelMainT
  const cargo = flags?.inMainShip ? status.cargoT ?? model.cargo?.count : model.cargo?.count
  return (
    <>
      <div className='go-flight-switches'>{switches.map(([key, label]) => <label className='checkbox' key={key}><span>{label}</span><input type='checkbox' disabled readOnly checked={flags?.[key] === true} aria-label={`${label}: ${onOff(flags?.[key])}`} /><span className='checkbox__control' /><small>{onOff(flags?.[key])}</small></label>)}</div>
      <div className='go-flight-deck'>
        <div className='go-flight-dial'><div className='dial-background'><Icon name='ship' /><h3>Hull integrity</h3><strong>{show(ship.hullHealth == null ? null : ship.hullHealth * 100, '%')}</strong><p>Shields {onOff(flags?.shieldsUp)}</p></div><small>Surface coordinates remain private.<br />No game control is available.</small></div>
        <div className='go-flight-facts'><Metric label='Observed max jump range' value={ship.maxJumpRangeLY} unit='LY' /><Metric label='Fuel reservoir' value={status.state === 'known' ? status.fuelReservoirT : null} unit='T' /><Metric label='Unladen mass' value={ship.unladenMassT} unit='T' /><Metric label='Targeting mode' value={flags?.analysisMode == null ? null : flags.analysisMode ? 'Analysis' : 'Combat'} /><Meter label='Main fuel' value={mainFuel} max={ship.fuelCapacityT} /><Meter label='Cargo' value={cargo} max={ship.cargoCapacityT} /></div>
        <div className='go-flight-power'><h3>Power distribution</h3><div>{['sys', 'eng', 'wep'].map(key => <div key={key}><progress max='4' value={status.state === 'known' ? status.pips?.[key] ?? 0 : 0} /><strong>{status.state === 'known' ? show(status.pips?.[key], '', 1) : '—'}</strong><span>{key.toUpperCase()}</span></div>)}</div><small>{status.state === 'known' ? 'Observed Status.json' : 'Awaiting attributable Status.json'}</small></div>
      </div>
      <div className='go-flight-indicators'>{indicators.map(([key, label]) => <div key={key} className={flags?.[key] == null ? 'is-unknown' : flags[key] ? 'is-on' : 'is-off'}><i /><span>{label}</span><small>{onOff(flags?.[key])}</small></div>)}</div>
      <p className='go-evidence-foot'>Status: {status.state || 'unknown'} · Journal facts are last known. Unknown and out-of-vehicle telemetry is not shown as current ship state.</p>
    </>
  )
}
export function moduleGroup (slot) {
  if (/^(Huge|Large|Medium|Small)Hardpoint/i.test(slot)) return 'Hardpoints'
  if (/^TinyHardpoint/i.test(slot)) return 'Utility mounts'
  if (/^(Armour|PowerPlant|MainEngines|FrameShiftDrive|LifeSupport|PowerDistributor|Radar|FuelTank)$/i.test(slot)) return 'Core internals'
  return 'Optional internals'
}
export function Modules ({ model }) {
  const [query, setQuery] = useState(''); const [selection, setSelection] = useState('')
  const modules = model.modules || []
  const visible = useMemo(() => modules.filter(m => `${m.name} ${m.slot} ${m.item} ${m.engineering?.blueprint || ''}`.toLowerCase().includes(query.toLowerCase())).slice(0, 128), [modules, query])
  // Store a key, never a cached module object that could survive a new loadout.
  const selected = modules.find(m => `${m.slot}:${m.item}` === selection)
  return (
    <>
      <div className='go-toolbar'><label><Icon name='search' /><input aria-label='Find installed module' placeholder='Module, slot or blueprint' value={query} onChange={e => setQuery(e.target.value)} /></label><span>{modules.length} observed modules · {model.modulesState || 'unknown'}</span></div>
      {!modules.length && <p className='go-empty'>Module loadout not yet witnessed. Missing modules are not an empty loadout.</p>}
      <div className={`go-module-workspace ${selected ? 'has-inspector' : ''}`}>
        <div className='go-module-groups'>
          {['Hardpoints', 'Optional internals', 'Core internals', 'Utility mounts'].map(group => {
            const items = visible.filter(m => moduleGroup(m.slot) === group)
            return items.length > 0 && <section key={group}><h3 className='section-heading text-primary'>{group}</h3><div className='go-module-cards'>{items.map(m => <button type='button' aria-label={`Inspect ${m.name || m.item}`} aria-pressed={selected === m} key={`${m.slot}:${m.item}`} className={`go-module-card ${selected === m ? 'is-selected' : ''}`} onClick={() => setSelection(`${m.slot}:${m.item}`)}><span className='go-module-class'>{m.reference?.class ?? 0}{m.reference?.rating || ''}<small>{slotLabel(m.slot)}</small>{m.reference?.class == null && <small>Class not verified</small>}</span><div><strong>{m.name || m.item}</strong><small>Health {show(m.health == null ? null : m.health * 100, '%')} · {onOff(m.on)}</small>{m.engineering && <span className='text-secondary'><Icon name='engineering' /> Grade {m.engineering.grade ?? '—'}</span>}</div></button>)}</div></section>
          })}
        </div>
        {selected && <aside className='go-module-inspector inspector' aria-label='Module engineering inspector'><div className='go-page__heading'><h3>Module inspector</h3><button type='button' aria-label='Close module inspector' onClick={() => setSelection('')}>×</button></div><h2 className='text-info'>{selected.name || selected.item}</h2><p className='text-primary'>{slotLabel(selected.slot)}</p>{selected.reference?.class == null && <p>Class 0 is a display placeholder · actual class unknown / not verified.</p>}<div className='go-fact-grid'><Metric label='Health' value={selected.health == null ? null : selected.health * 100} unit='%' /><Metric label='Priority' value={selected.priority} /><Metric label='Ammo / clip' value={selected.ammoInClip} /><Metric label='Ammo / hopper' value={selected.ammoInHopper} /></div>{selected.reference && <details><summary>Bundled module reference</summary><p>{selected.reference.description}</p><p>Class {selected.reference.class ?? '—'}{selected.reference.rating} · Base mass {show(selected.reference.massT, 'T')} · Base power {show(selected.reference.powerMW, 'MW')}</p><small>Bundled EDCD/Coriolis reference. Base values are not observed engineered performance.</small></details>}{selected.engineering ? <><h3 className='text-primary'>Engineering</h3><h4>{selected.engineering.blueprint}</h4><p>Grade {selected.engineering.grade ?? '—'} · Quality {show(selected.engineering.quality == null ? null : selected.engineering.quality * 100, '%')}</p><p className='text-secondary'>{selected.engineering.experimental || 'Experimental effect not reported'}</p><table><thead><tr><th>Modifier</th><th>Original</th><th>Observed</th></tr></thead><tbody>{(selected.engineering.modifiers || []).map((m, i) => <tr key={i}><td>{m.label}</td><td>{show(m.originalValue, '', 3)}</td><td>{show(m.value, '', 3)}</td></tr>)}</tbody></table><p className='text-muted'>Explicit journal modifier values; units are not inferred.</p></> : <p className='text-muted'>Engineering not reported for this module.</p>}</aside>}
      </div>
    </>
  )
}
export function Inventory ({ model, cargo }) {
  const [query, setQuery] = useState(''); const [location, setLocation] = useState('shiplocker'); const [page, setPage] = useState(0); const snapshot = cargo ? model.cargo : model.inventory; const items = snapshot?.items || []
  const filtered = useMemo(() => items.filter(i => (cargo || location === 'all' || i.location?.toLowerCase() === location) && `${i.label} ${i.name} ${i.category || ''} ${i.location || ''}`.toLowerCase().includes(query.toLowerCase())).slice(0, 512), [items, query, cargo, location])
  const pages = Math.max(1, Math.ceil(filtered.length / 50)); const currentPage = Math.min(page, pages - 1)
  const known = snapshot?.state === 'known' || (!cargo && (location === 'shiplocker' ? snapshot?.shipLockerState === 'known' : location === 'backpack' && snapshot?.backpackState === 'known'))
  return <><div className='go-toolbar'><label><Icon name='search' /><input aria-label={cargo ? 'Find cargo' : 'Find locker item'} placeholder='Search manifest' value={query} onChange={e => { setQuery(e.target.value); setPage(0) }} /></label>{!cargo && <select aria-label='Inventory location' value={location} onChange={e => { setLocation(e.target.value); setPage(0) }}><option value='shiplocker'>Ship locker</option><option value='backpack'>Backpack</option><option value='all'>All locations</option></select>}<button disabled={currentPage === 0} onClick={() => setPage(currentPage - 1)} aria-label='Previous manifest page'>‹</button><span>{currentPage + 1}/{pages}</span><button disabled={currentPage >= pages - 1} onClick={() => setPage(currentPage + 1)} aria-label='Next manifest page'>›</button><Evidence state={known ? 'verified' : snapshot?.state} /></div>{cargo && <Meter label='Cargo' value={snapshot?.count} max={model.ship?.cargoCapacityT} />}{!known && <p role='status'>Complete {cargo ? 'cargo' : 'locker'} snapshot not yet witnessed. Displayed items may be partial; missing items remain unknown.</p>}{snapshot?.truncated && <p role='status'>Snapshot exceeds display bounds; list is truncated.</p>}<div className='go-table-scroll'><table className='go-manifest'><thead><tr><th>#</th><th>{cargo ? 'Cargo manifest' : 'Item storage'}</th><th>{cargo ? 'Description' : 'Category / location'}</th></tr></thead><tbody>{filtered.slice(currentPage * 50, (currentPage + 1) * 50).map((i, n) => <tr key={`${i.name}:${i.location}:${n}`}><td>{i.count}</td><td>{i.label || i.name}</td><td>{cargo ? <>{i.mission && <span className='text-secondary'>Mission critical · </span>}{i.stolen > 0 && <span className='text-danger'>{i.stolen} stolen</span>}</> : `${i.category} · ${i.location}`}</td></tr>)}</tbody></table>{!filtered.length && known && <p className='go-empty'>Observed {cargo ? 'cargo hold' : location === 'shiplocker' ? 'ship locker' : location === 'backpack' ? 'backpack' : 'inventory'} is empty.</p>}</div></>
}
export default function GoParityShip ({ workspace, path }) {
  const model = workspace?.parity || {}; const isModules = path === '/ship/modules'; const isCargo = path === '/ship/cargo'; const isInventory = path === '/ship/inventory'
  const title = isModules ? 'Ship internals' : isCargo ? 'Cargo manifest' : isInventory ? 'Item storage' : model.ship?.shipType?.replaceAll('_', ' ').toUpperCase() || 'Ship status'
  return <section className='go-page go-parity-ship' aria-label='Native ship workspace'><div className='go-page__heading'><div><span className='text-primary'>{isModules ? 'MODULES & ENGINEERING UPGRADES' : 'SHIP · READ-ONLY JOURNAL TELEMETRY'}</span><h2>{title}</h2></div><Evidence state={model.evidence} /></div>{isModules ? <Modules model={model} /> : isCargo || isInventory ? <Inventory model={model} cargo={isCargo} /> : <Status model={model} />}</section>
}
