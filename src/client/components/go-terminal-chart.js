import { useMemo, useRef, useState } from 'react'
import SystemMapObject from './panels/nav/system-map/system-map-object'
import { layoutBodies, legacyStarSubtype } from '../lib/go-survey'
import { Icon } from './go-terminal-widgets'
export function chartScene (bodies, contextBodies = bodies) {
  const ids = new Set(bodies.map(b => b.bodyId)); const nodes = layoutBodies(contextBodies).filter(n => ids.has(n.body.bodyId)); const roots = new Map(); const positions = new Map(); let y = 1800; let width = 14000
  for (const n of nodes) { let root = n; const seen = new Set(); while (root.parent && !root.unresolved && !seen.has(root)) { seen.add(root); root = root.parent } if (!roots.has(root))roots.set(root, []); roots.get(root).push(n) }
  for (const group of roots.values()) {
    let x = 1600; let column = 0; let bottom = y + 1800; let columnBottom = y
    for (const n of group) {
      if (n.depth <= 1) {
        if (n.depth === 1) column++
        x = 1600 + column * 3600
        columnBottom = y
      } else {
        x = positions.get(n.parent?.body.bodyId)?.x || x
      }
      const parent = positions.get(n.parent?.body.bodyId)
      // Nodes are in depth-first order. Reserve a fresh row after the entire
      // previous subtree so siblings cannot share glyphs, labels or hit targets.
      const py = n.depth > 1 ? Math.max((parent?.y ?? y) + 2600, columnBottom + 2600) : y
      const radius = n.body.details?.isStar ? 650 : Math.max(220, Math.min(500, Math.sqrt(n.body.details?.radiusKM || 1200) * 8))
      positions.set(n.body.bodyId, { ...n, x, y: py, radius })
      columnBottom = py
      bottom = Math.max(bottom, py + 1800)
      width = Math.max(width, x + 4700)
    }
    y = bottom + 2000
  }
  return { nodes: nodes.map(n => positions.get(n.body.bodyId)), width, height: Math.max(6000, y) }
}
export default function GoTerminalChart ({ bodies, contextBodies, selected, onSelect }) {
  const [zoom, setZoom] = useState(1); const [fit, setFit] = useState(false); const viewport = useRef(null)
  const scene = useMemo(() => chartScene(bodies, contextBodies), [bodies, contextBodies])
  return (
    <section className='go-chart navigation-panel__map' aria-label='System visual map'>
      <div className='navigation-panel__map-frame'><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-top-left' /><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-top-right' /><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-bottom-left' /><div className='navigation-panel__map-frame-border navigation-panel__map-frame-border-bottom-right' /></div>
      <div className='go-chart__tools'><button type='button' onClick={() => { setFit(false); setZoom(z => Math.min(3, z + 0.25)) }} aria-label='Zoom in'><Icon name='search' /> +</button><button type='button' onClick={() => { setFit(false); setZoom(z => Math.max(0.4, z - 0.25)) }} aria-label='Zoom out'>−</button><button type='button' onClick={() => { setFit(true); setZoom(1) }}>Fit</button><span>Schematic · {bodies.length} observations</span></div>
      <div className='go-chart__viewport scrollable' ref={viewport}>
        <svg aria-label='Selectable star and body map' role='group' viewBox={`0 0 ${scene.width} ${scene.height}`} style={{ width: fit ? '100%' : `${Math.max(900, scene.width / 12) * zoom}px`, height: fit ? '100%' : `${Math.max(430, scene.height / 12) * zoom}px` }}>
          <defs><filter id='svg-filter__star-glow--light'><feGaussianBlur stdDeviation='18' /><feMerge><feMergeNode /><feMergeNode in='SourceGraphic' /></feMerge></filter></defs>
          {scene.nodes.map(n => {
            const b = n.body; const p = scene.nodes.find(n => n.body.bodyId === b.details?.parents?.[0]?.bodyId); const object = { id: `go-${b.bodyId}`, name: b.name || `Body ${b.bodyId}`, label: b.name || `Body ${b.bodyId}`, type: b.details?.isStar ? 'Star' : 'Planet', subType: b.details?.isStar ? legacyStarSubtype(b.kind) : b.kind, atmosphereType: b.atmosphere, isLandable: b.landable === true, distanceToArrival: b.distanceLS ?? 0, orbitsStar: false, rings: !!b.details?.rings?.length, _x: n.x, _y: n.y, _r: n.radius }; const choose = () => onSelect(b.bodyId)
            return (
              <g key={b.bodyId} className='go-chart__body' role='button' tabIndex={0} aria-label={`Inspect ${object.name}`} aria-pressed={selected === b.bodyId} onClick={choose} onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); choose() } }}>
                {p && !n.unresolved && <path d={`M ${p.x} ${p.y} L ${n.x} ${p.y} L ${n.x} ${n.y}`} fill='none' stroke='currentColor' strokeWidth='12' opacity='.3' />}
                {selected === b.bodyId && <circle cx={n.x} cy={n.y} r={n.radius + 230} className='go-chart__selection' />}
                <SystemMapObject systemObject={object} setSystemObject={choose} labels={false} />
                <text x={n.x} y={n.y - n.radius - 450} textAnchor='middle' className='go-chart__name'>{object.name}</text>
                <text x={n.x} y={n.y + n.radius + 540} textAnchor='middle' className='go-chart__caption'>{n.unresolved ? 'Parent unobserved' : b.details?.isStar ? `Class ${b.kind}` : b.distanceLS == null ? 'Distance unknown' : `${b.distanceLS.toFixed(1)} LS`}</text>
                {b.bioSignals > 0 && <text x={n.x} y={n.y + n.radius + 880} textAnchor='middle' className='go-chart__bio'>BIO {b.bioSignals}</text>}
              </g>
            )
          })}
        </svg>
      </div>
      <p className='go-chart__legend'><span className='text-primary'>●</span> Selected <span className='text-success'>●</span> Biological evidence · Position and sizes are schematic</p>
    </section>
  )
}
