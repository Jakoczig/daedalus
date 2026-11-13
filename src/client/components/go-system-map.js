import SystemMapObject from './panels/nav/system-map/system-map-object'
// Reuses the original SVG textures, rings, landability and body selection renderer.
// Layout uses witnessed Parents only; this is a schematic, not a live orrery.
export default function GoSystemMap ({ nodes, selected, onSelect }) {
  const rows = nodes.slice(0, 40)
  const width = Math.max(14000, ...rows.map(n => 6500 + n.depth * 2500))
  return (
    <div aria-label='Observed system map' style={{ overflow: 'auto', maxHeight: '28rem', border: '1px solid currentColor' }}>
      <svg role='group' aria-label='Selectable journal system map' viewBox={`0 0 ${width} ${Math.max(1800, rows.length * 1800)}`} width={Math.max(850, width / 16)} height={Math.max(120, rows.length * 112)}>
        {rows.map((node, index) => {
          const body = node.body; const x = 1200 + node.depth * 2500; const y = index * 1800 + 900
          const object = { id: `go-${body.bodyId}`, name: body.name || `Body ${body.bodyId}`, label: body.name || `Body ${body.bodyId}`, type: body.details?.isStar ? 'Star' : 'Planet', subType: body.kind, atmosphereType: body.atmosphere, isLandable: body.landable === true, distanceToArrival: body.distanceLS ?? 0, orbitsStar: false, rings: body.details?.rings?.length > 0, _x: x, _y: y, _r: body.details?.isStar ? 550 : 330 }
          const choose = () => onSelect(body.bodyId)
          return (
            <g key={body.bodyId} role='button' aria-label={`Inspect ${object.name}${node.unresolved ? ', parent unknown' : ''}`} aria-pressed={selected === body.bodyId} tabIndex={0} onClick={choose} onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); choose() } }}>
              {node.depth > 0 && <path d={`M ${x - 1900} ${y - 500} L ${x - 1900} ${y} L ${x - 500} ${y}`} stroke='currentColor' fill='none' opacity='.4' />}
              {selected === body.bodyId && <rect x={x - 700} y={y - 800} width={Math.max(5500, width - x)} height={1650} fill='none' stroke='currentColor' strokeWidth={30} />}
              <SystemMapObject systemObject={object} setSystemObject={choose} labels={false} />
              <text x={x + 950} y={y - 100} fill='currentColor' fontSize={260}>{object.name}</text>
              <text x={x + 950} y={y + 300} fill='currentColor' fontSize={210}>{body.kind || 'Class unknown'} · {body.distanceLS == null ? 'Arrival distance unknown' : `${body.distanceLS.toFixed(1)} LS`}{node.unresolved ? ' · Parent unobserved / ambiguous' : ''}</text>
            </g>
          )
        })}
      </svg>
    </div>
  )
}
