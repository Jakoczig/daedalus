import { useMemo, useEffect, useState } from 'react'
import { Icon, Badge, show } from './go-terminal-widgets'
const finite = v => Number.isFinite(v) && v >= 0 ? v : null
const modifier = (m, label) => finite(m.engineering?.modifiers?.find(x => x.label === label)?.value)
export function powerStatistics (modules = []) {
  const plant = modules.find(m => m.slot === 'PowerPlant')
  const capacity = plant ? modifier(plant, 'PowerCapacity') : null
  let stowed = 0; let deployed = 0; const priorities = [0, 0, 0, 0, 0]; let unknown = modules.length ? 0 : 1
  for (const m of modules) {
    if (m === plant || m.on === false) continue
    const draw = m.on !== true ? null : m.engineering ? modifier(m, 'PowerDraw') : finite(m.reference?.powerMW)
    if (draw == null || m.engineering?.truncated) { unknown++; continue }
    deployed += draw
    if (!/^(Huge|Large|Medium|Small)Hardpoint/.test(m.slot)) stowed += draw
    if (Number.isInteger(m.priority) && m.priority >= 0 && m.priority < 5) priorities[m.priority] += draw
  }
  return { capacity, stowed: unknown ? null : stowed, deployed: unknown ? null : deployed, priorities, unknown, percent: !unknown && capacity > 0 ? deployed / capacity * 100 : null }
}
function Fact ({ label, value, unit, source = 'JOURNAL' }) { return <div className='go-stat-fact'><small>{label}</small><strong>{typeof value === 'string' ? `${value}${unit ? ` ${unit}` : ''}` : show(value, unit)}</strong><Badge>{value == null || value === '' ? 'UNAVAILABLE' : source}</Badge></div> }
export default function GoShipStatistics ({ workspace, role, sendEvent }) {
  const m = workspace?.parity || {}; const s = m.ship || {}; const status = m.status || {}; const power = useMemo(() => powerStatistics(m.modules), [m.modules])
  const [ownerName, setOwnerName] = useState('')
  useEffect(() => { let active = true; setOwnerName(''); if (role === 'owner') sendEvent('getGoShipName').then(v => { if (active) setOwnerName(v.shipName || '') }).catch(() => {}); return () => { active = false } }, [role, sendEvent, s.loadoutAt, s.shipType])
  const fuel = status.state === 'known' ? status.fuelMainT ?? s.fuelMainT : s.fuelMainT
  const cargo = status.state === 'known' ? status.cargoT ?? m.cargo?.count : m.cargo?.count
  return <section className='go-page go-ship-statistics' aria-label='Ship Statistics'><div className='go-page__heading'><h2><Icon name='ship' /> Ship Statistics</h2><span>{ownerName || s.shipType || 'Unknown ship'} · {s.shipType}</span></div><p className='go-stats-evidence'>Loadout {s.loadoutAt || 'not witnessed'} · {s.evidence || 'unknown'} · Status {status.observedAt || 'not witnessed'}. Last-known journal facts; not a live flight simulator.</p><div className='go-stats-columns'><section><h3>Jump, mass & capacity</h3><div className='go-stats-grid'><Fact label='Maximum unladen jump range' value={s.maxJumpRangeLY} unit='LY' /><Fact label='Unladen mass' value={s.unladenMassT} unit='T' /><Fact label='Main fuel / capacity' value={fuel == null ? null : `${show(fuel)} / ${show(s.fuelCapacityT)}`} unit='T' /><Fact label='Cargo carried / capacity' value={cargo == null ? null : `${show(cargo)} / ${show(s.cargoCapacityT)}`} unit='T' /><Fact label='Hull integrity' value={s.hullHealth == null ? null : s.hullHealth * 100} unit='%' /><Fact label='Insurance rebuy' value={s.rebuyCredits} unit='CR' /><Fact label='Hull value' value={s.hullValueCredits} unit='CR' /><Fact label='Modules value' value={s.modulesValueCredits} unit='CR' /></div></section><section><h3>Effective module power</h3><div className='go-stats-grid'><Fact label='Powerplant output' value={power.capacity} unit='MW' /><Fact label='Hardpoints stowed' value={power.stowed} unit='MW' source='DERIVED' /><Fact label='Hardpoints deployed' value={power.deployed} unit='MW' source='DERIVED' /><Fact label='Deployed load' value={power.percent} unit='%' source='DERIVED' /></div><p>{power.unknown ? `${power.unknown} module power values unverified; totals withheld.` : 'Enabled modules only. Weapon hardpoints contribute when deployed.'} Base reference is used only for unengineered verified modules. Engineered values require explicit journal modifiers.</p><details><summary>Priority groups & provenance</summary>{power.priorities.map((mw, i) => <p key={i}>Priority {i + 1}: {power.unknown ? 'partial known draw' : 'draw'} {show(mw, 'MW')} · REFERENCE / DERIVED</p>)}<p>Lower-priority groups may be shed when demand exceeds output. Capacity and condition are not assumed; damaged-plant emergency output is not calculated.</p></details></section></div><details className='go-stats-advanced'><summary>Additional performance — unavailable calculations</summary><p>Current/laden jump range, speed/boost, shields, armour/resists, thermal and weapon DPS require validated ship curves and effective engineered attributes. They remain UNAVAILABLE instead of substituting default stats for unknown/new modules. Use the Modules inspector for witnessed modifiers and explicit REFERENCE values.</p></details></section>
}
