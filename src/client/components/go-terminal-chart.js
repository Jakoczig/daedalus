/* global ResizeObserver */
import { useMemo, useRef, useState, useEffect } from 'react'
import SystemMapObject from './panels/nav/system-map/system-map-object'
import { legacyStarSubtype } from '../lib/go-survey'
import { Icon } from './go-terminal-widgets'
export function chartScene (bodies, contextBodies = bodies) {
  // Legacy orbital convention: stellar branches run horizontally; moons below
  // their actual planet. Reserve the whole subtree before placing a sibling.
  const nodes = new Map(contextBodies.map(body => [body.bodyId, { body, parent: null, children: [], unresolved: false, depth: 0 }]))
  for (const n of [...nodes.values()]) {
    const p = n.body.details?.parents?.[0]
    if (p?.kind === 'Null' && !nodes.has(p.bodyId)) nodes.set(p.bodyId, { body: { bodyId: p.bodyId, name: `Barycentre #${p.bodyId}`, virtual: true, details: {} }, parent: null, children: [], depth: 0 })
  }
  for (const n of nodes.values()) {
    const p = n.body.details?.parents?.[0]
    n.parent = p && p.bodyId !== n.body.bodyId ? nodes.get(p.bodyId) || null : null
    n.unresolved = !n.body.virtual && !n.body.details?.isStar && !n.parent
    if (p && !n.parent) n.unresolved = true
  }
  for (const n of nodes.values()) {
    let p = n.parent; const seen = new Set([n]); let depth = 0
    while (p) { if (seen.has(p) || ++depth > 32) { n.parent = null; n.unresolved = true; break } seen.add(p); p = p.parent }
  }
  for (const n of nodes.values()) if (n.parent) n.parent.children.push(n)
  const order = (a, b) => a.body.bodyId.length - b.body.bodyId.length || a.body.bodyId.localeCompare(b.body.bodyId)
  const size = n => {
    n.children.sort(order); n.children.forEach(size)
    const horizontal = n.body.virtual || n.body.details?.isStar
    n.w = horizontal ? 3600 + n.children.reduce((v, c) => v + c.w, 0) : Math.max(3600, ...n.children.map(c => c.w))
    n.h = horizontal ? Math.max(2600, ...n.children.map(c => c.h)) : 2600 + n.children.reduce((v, c) => v + c.h, 0)
  }
  const place = (n, x, y, depth) => {
    Object.assign(n, { x, y, depth, radius: n.body.details?.isStar ? 650 : 450 })
    let dx = 3600; let dy = 2600
    for (const c of n.children) { const horizontal = n.body.virtual || n.body.details?.isStar; place(c, horizontal ? x + dx : x, horizontal ? y : y + dy, depth + 1); dx += c.w; dy += c.h }
  }
  const roots = [...nodes.values()].filter(n => !n.parent).sort(order)
  let y = 1800; let width = 10000
  for (const root of roots) { size(root); place(root, 1600, y, 0); y += root.h + 3000; width = Math.max(width, root.w + 3200) }
  const ids = new Set(bodies.map(b => b.bodyId))
  return { nodes: [...nodes.values()].filter(n => ids.has(n.body.bodyId) || n.body.virtual), width, height: Math.max(6000, y) }
}
export default function GoTerminalChart ({ bodies, contextBodies, selected, onSelect, systemKey, stations = [], selectedStation = null, onSelectStation = () => {} }) {
  const [showAllStations, setShowAllStations] = useState(false)
  const [zoom, setZoom] = useState(1); const [fit, setFit] = useState(true); const viewport = useRef(null); const [bounds, setBounds] = useState({ width: 800, height: 500 })
  useEffect(() => { setFit(true); setZoom(1); setShowAllStations(false); if (viewport.current) { viewport.current.scrollTop = 0; viewport.current.scrollLeft = 0 } }, [systemKey])
  useEffect(() => { const element = viewport.current; if (!element || typeof ResizeObserver === 'undefined') return; const measure = () => { const r = element.getBoundingClientRect(); if (r.width && r.height) setBounds({ width: r.width, height: r.height }) }; measure(); const observer = new ResizeObserver(measure); observer.observe(element); return () => observer.disconnect() }, [])
  const scene = useMemo(() => chartScene(bodies, contextBodies), [bodies, contextBodies])
  const scale = Math.max(0.001, Math.min(bounds.width / scene.width, bounds.height / scene.height)); const labelSize = fit ? Math.max(270, 10 / scale) : 270; const shortName = name => { if (!fit) return name; if (name === systemKey) return 'Primary'; return name.startsWith(systemKey + ' ') ? name.slice(systemKey.length + 1) : name.length > 16 ? name.slice(0, 15) + '…' : name }
  return (
    <section className='go-chart navigation-panel__map' aria-label='System visual map'>
      <div className='navigation-panel__map-frame'><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-top-left' /><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-top-right' /><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-bottom-left' /><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-bottom-right' /></div>
      <div className='go-chart__tools'><button type='button' onClick={() => { setFit(false); setZoom(z => Math.min(3, z + 0.25)) }} aria-label='Zoom in'><Icon name='search' /> +</button><button type='button' onClick={() => { setFit(false); setZoom(z => Math.max(0.4, z - 0.25)) }} aria-label='Zoom out'>−</button><button type='button' onClick={() => { setFit(true); setZoom(1) }}>Fit</button><span>Schematic · {bodies.length} observations</span></div>
      <div className='go-chart__viewport scrollable' ref={viewport}>
        <svg aria-label='Selectable star and body map' role='group' viewBox={`0 0 ${scene.width} ${scene.height}`} style={{ width: fit ? '100%' : `${Math.max(900, scene.width / 12) * zoom}px`, height: fit ? '100%' : `${Math.max(430, scene.height / 12) * zoom}px` }}>
          <defs><filter id='svg-filter__star-glow--light'><feGaussianBlur stdDeviation='18' /><feMerge><feMergeNode /><feMergeNode in='SourceGraphic' /></feMerge></filter></defs>
          {scene.nodes.map(n => {
            if (n.body.virtual) return <g key={'bary-' + n.body.bodyId} aria-label={n.body.name}><circle cx={n.x} cy={n.y} r='180' fill='none' stroke='currentColor' strokeWidth='30' /><text className='go-chart__name' style={{ fontSize: labelSize }} x={n.x} y={n.y - 500} textAnchor='middle'>{n.body.name}</text></g>
            const b = n.body; const p = n.parent; const object = { id: `go-${b.bodyId}`, name: b.name || `Body ${b.bodyId}`, label: b.name || `Body ${b.bodyId}`, type: b.details?.isStar ? 'Star' : 'Planet', subType: b.details?.isStar ? legacyStarSubtype(b.kind) : b.kind, atmosphereType: b.atmosphere, isLandable: b.landable === true, distanceToArrival: b.distanceLS ?? 0, orbitsStar: false, rings: !!b.details?.rings?.length, _x: n.x, _y: n.y, _r: n.radius }; const choose = () => onSelect(b.bodyId)
            return (
              <g key={b.bodyId} className='go-chart__body' role='button' tabIndex={0} aria-label={`Inspect ${object.name}`} aria-pressed={selected === b.bodyId} onClick={choose} onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); choose() } }}>
                <circle cx={n.x} cy={n.y} r={fit ? Math.min(1100, Math.max(n.radius, 10 / scale)) : n.radius} fill='transparent' />
                {p && !n.unresolved && <path d={`M ${p.x} ${p.y} L ${n.x} ${p.y} L ${n.x} ${n.y}`} fill='none' stroke='currentColor' strokeWidth='12' opacity='.3' />}
                {selected === b.bodyId && <circle cx={n.x} cy={n.y} r={n.radius + 230} className='go-chart__selection' />}
                <SystemMapObject systemObject={object} setSystemObject={choose} labels={false} />
                {(scene.nodes.length < 36 || !fit || selected === b.bodyId) && <text x={n.x} y={n.y - n.radius - 450} textAnchor='middle' className='go-chart__name' style={{ fontSize: labelSize }}>{shortName(object.name)}</text>}
                {(scene.nodes.length < 36 || !fit || selected === b.bodyId) && <text x={n.x} y={n.y + n.radius + 540} textAnchor='middle' className='go-chart__caption' style={{ fontSize: fit ? labelSize * 0.8 : 190 }}>{n.unresolved ? 'Parent unobserved' : b.details?.isStar ? `Class ${b.kind}` : b.distanceLS == null ? 'Distance unknown' : `${b.distanceLS.toFixed(1)} LS`}</text>}
                {b.bioSignals > 0 && <text x={n.x} y={n.y + n.radius + 880} textAnchor='middle' className='go-chart__bio'>BIO {b.bioSignals}</text>}
              </g>
            )
          })}
        </svg>
      </div>
      {stations.length > 0 && (
        <div className='go-chart__stations' role='group' aria-label='Community station markers on system map'>
          <div className='go-chart__station-heading'><strong>◆ STATIONS · {stations.length} EDSM</strong><small>System-level markers; orbital positions not available</small></div>
          <div className='go-chart__station-scroll'>
            {stations.slice(0, showAllStations ? 64 : 6).map((station, index) => (
              <button key={station.name + '-' + index} type='button' className='go-chart__station-marker' aria-pressed={selectedStation === index} title={[station.name, station.type, station.distanceLS == null ? 'Arrival distance unknown' : station.distanceLS + ' LS'].filter(Boolean).join(' · ')} onClick={() => onSelectStation(index)}>
                <span aria-hidden='true'>◆</span><span>{station.name}</span>
              </button>
            ))}
          </div>
          {stations.length > 6 && <button type='button' className='go-chart__station-more' aria-expanded={showAllStations} onClick={() => setShowAllStations(v => !v)}>{showAllStations ? 'Fewer' : '+' + (stations.length - 6) + ' more'}</button>}
        </div>
      )}
      <p className='go-chart__legend'><span className='text-primary'>●</span> Selected <span className='text-success'>●</span> Biological evidence · Orbital topology schematic, not to scale</p>
    </section>
  )
}
