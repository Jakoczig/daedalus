import { useEffect, useState } from 'react'

import { Icon, Badge, show } from './go-terminal-widgets'

// Projection only: all power calculations and unknown handling belong to Go.

export function powerStatistics (native) {
  return { capacity: native?.capacityMW ?? null, stowed: native?.stowedMW ?? null, deployed: native?.deployedMW ?? null, priorities: native?.knownPriorityMW || [0, 0, 0, 0, 0], unknown: native?.unknownPower ?? 1, percent: native?.deployedPercent ?? null }
}

function Fact ({ label, value, unit, source = 'JOURNAL' }) { return <div className='go-stat-fact'><small>{label}</small><strong>{typeof value === 'string' ? `${value}${unit ? ` ${unit}` : ''}` : show(value, unit)}</strong><Badge>{value == null || value === '' ? 'UNAVAILABLE' : source}</Badge></div> }

export default function GoShipStatistics ({ workspace, role, sendEvent }) {
  const m = workspace?.parity || {}; const s = m.ship || {}; const status = m.status || {}

  const [ownerName, setOwnerName] = useState('')

  const [effective, setEffective] = useState(null)

  const power = powerStatistics(effective)
  const moduleEvidence = JSON.stringify(m.modules || {})
  const deployment = status.state === 'known' ? status.flags?.hardpointsDeployed : null

  useEffect(() => { let active = true; setEffective(null); sendEvent('getGoEngineeredStatistics').then(v => { if (active) setEffective(v) }).catch(() => {}); return () => { active = false } }, [sendEvent, s.loadoutAt, s.shipType, moduleEvidence, deployment])

  useEffect(() => { let active = true; setOwnerName(''); if (role === 'owner') sendEvent('getGoShipName').then(v => { if (active) setOwnerName(v.shipName || '') }).catch(() => {}); return () => { active = false } }, [role, sendEvent, s.loadoutAt, s.shipType])

  const fuel = status.state === 'known' ? status.fuelMainT ?? s.fuelMainT : s.fuelMainT

  const cargo = status.state === 'known' ? status.cargoT ?? m.cargo?.count : m.cargo?.count

  return <section className='go-page go-ship-statistics' aria-label='Ship Statistics'><div className='go-page__heading'><h2><Icon name='ship' /> Ship Statistics</h2><span>{ownerName || s.shipType || 'Unknown ship'} · {s.shipType}</span></div><p className='go-stats-evidence'>Loadout {s.loadoutAt || 'not witnessed'} · {s.evidence || 'unknown'} · Status {status.observedAt || 'not witnessed'}. Last-known journal facts; not a live flight simulator.</p><div className='go-stats-columns'><section><h3>Jump, mass & capacity</h3><div className='go-stats-grid'><Fact label='Maximum unladen jump range' value={s.maxJumpRangeLY} unit='LY' /><Fact label='Unladen mass' value={s.unladenMassT} unit='T' /><Fact label='Main fuel / capacity' value={fuel == null ? null : `${show(fuel)} / ${show(s.fuelCapacityT)}`} unit='T' /><Fact label='Cargo carried / capacity' value={cargo == null ? null : `${show(cargo)} / ${show(s.cargoCapacityT)}`} unit='T' /><Fact label='Hull integrity' value={s.hullHealth == null ? null : s.hullHealth * 100} unit='%' /><Fact label='Insurance rebuy' value={s.rebuyCredits} unit='CR' /><Fact label='Hull value' value={s.hullValueCredits} unit='CR' /><Fact label='Modules value' value={s.modulesValueCredits} unit='CR' /></div></section><section><h3>Effective module power</h3><div className='go-stats-grid'><Fact label='Rated powerplant capacity' value={power.capacity} unit='MW' source='FINAL MODIFIER' /><Fact label='Hardpoints stowed' value={power.stowed} unit='MW' source='DERIVED' /><Fact label='Hardpoints deployed' value={power.deployed} unit='MW' source='DERIVED' /><Fact label='Deployed load' value={power.percent} unit='%' source='DERIVED' /></div><p>{power.unknown ? `${power.unknown} module power values unverified; totals withheld.` : 'Enabled modules only. Weapon hardpoints contribute when deployed.'} Base reference is used only for unengineered verified modules. Engineered values require explicit journal modifiers.</p><details><summary>Priority groups & provenance</summary>{power.priorities.map((mw, i) => <p key={i}>Priority {i + 1}: {power.unknown ? 'partial known draw' : 'draw'} {show(mw, 'MW')} · NATIVE GO / DERIVED</p>)}<p>Lower-priority groups may be shed when demand exceeds output. Capacity and condition are not assumed; damaged-plant emergency output is not calculated.</p></details></section></div><section><h3>Witnessed final engineering · native Go</h3><p>Each modifier below is the actual final value reported in Loadout. No blueprint percentage is applied again. Attributes keep their journal labels and units; they are not substituted for total ship performance.</p>{effective && <><div className='go-stats-grid'><Fact label='Witnessed plant output' value={effective.capacityMW} unit='MW' source='GAME-REPORTED' /><Fact label='Rated draw of enabled fitted modules' value={effective.ratedPowerMW} unit='MW' source={'DERIVED · ' + effective.formula} /></div><p>Observed hardpoints: {effective.observedHardpointsDeployed == null ? 'unknown' : effective.observedHardpointsDeployed ? 'deployed' : 'stowed'} · rated scenario {show(effective.observedDeploymentRatedMW, 'MW')} (derived, not measured draw). Known rated draw {show(effective.knownRatedMW, 'MW')} · Formula {effective.formula} · {effective.unknownPower} enabled module power inputs unknown. Known rated inputs are shown separately; damaged-plant/emergency output remains unavailable. Stowed/deployed values are derived scenarios, not observed hardpoint state.</p>{effective.modules?.map(module => <details key={module.slot}><summary>{module.slot} · {module.item} · {module.attributes.length} witnessed attributes</summary>{module.notice && <p>{module.notice}</p>}<dl>{module.attributes.map(a => <div key={a.label}><dt>{a.label}</dt><dd>{a.value} {a.original != null && <small>(original {a.original})</small>} · GAME-REPORTED EFFECTIVE</dd></div>)}</dl></details>)}<p>{effective.unavailable?.join(' · ')}</p></>}</section><details className='go-stats-advanced'><summary>Additional performance — unavailable calculations</summary><p>Current/laden jump range, speed/boost, shields, armour/resists, thermal and weapon DPS require validated ship curves and effective engineered attributes. They remain UNAVAILABLE instead of substituting default stats for unknown/new modules. Use the Modules inspector for witnessed modifiers and explicit REFERENCE values.</p></details></section>
}
