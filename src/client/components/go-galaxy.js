import { useState, useRef, useEffect } from 'react'
import { useRouter } from 'next/router'
import manifest from '../public/images/galaxy-atlas/manifest.json'
import { project, distance } from '../lib/go-display-controller'
import { show } from './go-terminal-widgets'

const axesList = ['xz', 'xy', 'yz']
const usable = manifest.schema === 1 && manifest.status === 'ready' && manifest.run && manifest.views && axesList.every(a => manifest.views[a]?.levels?.length === manifest.maxLevel + 1)
const pointValid = p => p?.coordinates?.length === 3 && p.coordinates.every(v => typeof v === 'number' && Number.isFinite(v) && Math.abs(v) < 1e6)
const EXTENTS = manifest.extents || { x: [-65000, 65000], y: [-15000, 15000], z: [-35000, 95000] }

export function routeFrame (current, destination, full = EXTENTS) {
  if (!pointValid(current) || !pointValid(destination)) return full
  return Object.fromEntries(['x', 'y', 'z'].map((axis, i) => {
    const span = Math.max(100, Math.abs(current.coordinates[i] - destination.coordinates[i]) * 1.3)
    const center = (current.coordinates[i] + destination.coordinates[i]) / 2
    return [axis, [center - span / 2, center + span / 2]]
  }))
}

export function zoomFrame (frame, factor) {
  return Object.fromEntries(Object.entries(frame).map(([axis, [a, b]]) => {
    const half = Math.max(5, Math.min(1e6, (b - a) * factor / 2))
    return [axis, [(a + b) / 2 - half, (a + b) / 2 + half]]
  }))
}

export function panFrame (frame, axes, dx, dy) {
  const out = { ...frame }
  for (const [axis, fraction] of [[axes[0], -dx], [axes[1], dy]]) {
    const [a, b] = frame[axis]
    out[axis] = [a + (b - a) * fraction, b + (b - a) * fraction]
  }
  return out
}

export function selectLevel (meta, axes, frame, screenPixels) {
  const extent = meta.extents[axes[0]]
  const shown = frame[axes[0]][1] - frame[axes[0]][0]
  const full = extent[1] - extent[0]
  const target = Math.max(1, screenPixels * full / shown)
  // Choose the smallest level with adequate source pixels; avoid full-res loads at fit.
  for (let level = 0; level <= meta.maxLevel; level++) {
    if (meta.views[axes].levels[level].width >= target) return level
  }
  return meta.maxLevel
}

export function visibleTiles (meta, axes, frame, screenPixels, limit = 48) {
  if (!meta || !meta.views?.[axes]) return []
  const level = selectLevel(meta, axes, frame, screenPixels)
  const view = meta.views[axes].levels[level]
  const tile = meta.tileSize
  const horizontal = meta.extents[axes[0]]
  const vertical = meta.extents[axes[1]]
  const x0 = Math.max(0, Math.floor((frame[axes[0]][0] - horizontal[0]) / (horizontal[1] - horizontal[0]) * view.width / tile))
  const x1 = Math.min(view.columns - 1, Math.floor((frame[axes[0]][1] - horizontal[0]) / (horizontal[1] - horizontal[0]) * view.width / tile))
  const y0 = Math.max(0, Math.floor((vertical[1] - frame[axes[1]][1]) / (vertical[1] - vertical[0]) * view.height / tile))
  const y1 = Math.min(view.rows - 1, Math.floor((vertical[1] - frame[axes[1]][0]) / (vertical[1] - vertical[0]) * view.height / tile))
  if (x1 < x0 || y1 < y0) return []
  // Never burst-load an unbounded number of tiles on window resize/fit.
  if ((x1 - x0 + 1) * (y1 - y0 + 1) > limit && level > 0) {
    return visibleTiles({ ...meta, maxLevel: level - 1 }, axes, frame, screenPixels, limit)
  }
  const result = []
  for (let y = y0; y <= y1; y++) {
    for (let x = x0; x <= x1; x++) {
      const left = horizontal[0] + (x * tile / view.width) * (horizontal[1] - horizontal[0])
      const right = horizontal[0] + (Math.min((x + 1) * tile, view.width) / view.width) * (horizontal[1] - horizontal[0])
      const top = vertical[1] - (y * tile / view.height) * (vertical[1] - vertical[0])
      const bottom = vertical[1] - (Math.min((y + 1) * tile, view.height) / view.height) * (vertical[1] - vertical[0])
      const spanX = frame[axes[0]][1] - frame[axes[0]][0]
      const spanY = frame[axes[1]][1] - frame[axes[1]][0]
      result.push({
        key: axes + '-' + level + '-' + x + '-' + y,
        href: '/images/galaxy-atlas/tiles/' + meta.run + '/' + axes + '/' + level + '/' + x + '-' + y + '.png',
        x: (left - frame[axes[0]][0]) / spanX * 100,
        y: (frame[axes[1]][1] - top) / spanY * 100,
        width: (right - left) / spanX * 100,
        height: (top - bottom) / spanY * 100
      })
    }
  }
  return result
}

function Projection ({ axes, current, destination, searched, route = [], onSelect, frame, onPan, layers = [], screenPixels, onAssetError }) {
  const drag = useRef(null)
  const position = p => project(p?.coordinates, axes, frame)
  const within = p => p && p.x >= 0 && p.x <= 1 && p.y >= 0 && p.y <= 1
  const points = route.map(position).filter(within).map(p => p.x * 100 + ',' + p.y * 100).join(' ')
  const tiles = usable ? visibleTiles(manifest, axes, frame, screenPixels) : []
  const horizontal = axes[0].toUpperCase()
  const vertical = axes[1].toUpperCase()
  return <figure className={'go-galaxy-plane go-galaxy-' + axes}>
    <figcaption>{axes === 'xz' ? 'TOP · X / Z' : axes === 'xy' ? 'EDGE · X / Y' : 'EDGE · Y / Z'} · {horizontal} RIGHT · {vertical} UP</figcaption>
    <svg viewBox='0 0 100 100' preserveAspectRatio='none' role='img' aria-label={'Galaxy ' + axes.toUpperCase() + ' projection'} onPointerDown={e => {
      drag.current = { x: e.clientX, y: e.clientY }
      e.currentTarget.setPointerCapture?.(e.pointerId)
    }} onPointerCancel={() => { drag.current = null }} onPointerUp={e => {
      if (drag.current) {
        const r = e.currentTarget.getBoundingClientRect()
        onPan(axes, (e.clientX - drag.current.x) / r.width, (e.clientY - drag.current.y) / r.height)
        drag.current = null
      }
    }}>
      {tiles.map(tile => <image key={tile.key} href={tile.href} x={tile.x} y={tile.y} width={tile.width} height={tile.height} preserveAspectRatio='none' onError={onAssetError} />)}
      <polyline points={points} fill='none' stroke='#dc9b35' strokeWidth='.25' />
      {layers.map((point, i) => {
        const p = position(point)
        return within(p) && <g key={i} role='button' tabIndex={0} aria-label={point.name} onClick={() => onSelect(point)} onKeyDown={e => { if (e.key === 'Enter') onSelect(point) }}>
          <circle cx={p.x * 100} cy={p.y * 100} r='.45' fill={point.category === 'nebula' ? '#b988ff' : '#55dfff'} /><title>{point.name} · {point.source}</title>
        </g>
      })}
      {[[ 'destination', destination, '#ffc631' ], [ 'searched', searched, '#55e5ff' ], [ 'current', current, '#39ff69' ]].map(([kind, point, color]) => {
        const p = position(point)
        return within(p) && <g key={kind} data-marker={kind} data-axes={axes} data-position={p.x + ',' + p.y} role='button' tabIndex={0} aria-label={kind + ': ' + point.name} onClick={() => onSelect(point)} onKeyDown={e => { if (e.key === 'Enter') onSelect(point) }}>
          <circle cx={p.x * 100} cy={p.y * 100} r={kind === 'current' ? 1.1 : 0.8} fill={color} stroke='#000' strokeWidth='.2' /><title>{point.name} · {point.source}</title>
        </g>
      })}
    </svg>
  </figure>
}

export default function GoGalaxy ({ context, route, sendEvent, role, compact = false }) {
  const [manualFrame, setManualFrame] = useState(null)
  const [layerData, setLayerData] = useState(null)
  const [layerSelection, setLayerSelection] = useState({ poi: false, nebula: false })
  const [assetFailure, setAssetFailure] = useState(false)
  useEffect(() => {
    let active = true
    sendEvent('getGoGalaxyLayers').then(v => { if (active) setLayerData(v) }).catch(() => {})
    return () => { active = false }
  }, [sendEvent])
  const area = useRef(null)
  const [chartSize, setChartSize] = useState(320)
  useEffect(() => {
    if (compact || !area.current || typeof window.ResizeObserver === 'undefined') return
    const observer = new window.ResizeObserver(([entry]) => setChartSize(Math.max(180, Math.min(entry.contentRect.width, entry.contentRect.height))))
    observer.observe(area.current)
    return () => observer.disconnect()
  }, [compact])
  const router = useRouter()
  const [query, setQuery] = useState('')
  const [searched, setSearched] = useState(null)
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const current = context?.current
  const known = context?.points || []
  const destination = known.find(p => p.name === route?.destinationSystem)
  const path = (route?.hops || []).map(h => known.find(p => p.name === h.name)).filter(Boolean)
  const frame = manualFrame || routeFrame(current, destination)
  const onPan = (axes, dx, dy) => { if (Math.abs(dx) + Math.abs(dy) > 0.01) setManualFrame(panFrame(frame, axes, dx, dy)) }
  const layers = (layerData?.points || []).filter(p => layerSelection[p.category])
  const projectionProps = { frame, onPan, layers, onAssetError: () => setAssetFailure(true), screenPixels: compact ? 350 : chartSize }
  const search = async external => {
    setBusy(true)
    try {
      const result = await sendEvent(external ? 'lookupGoSystem' : 'getGoSystem', { name: query.trim() })
      setSearched(result.point?.name ? result.point : null)
      setNotice(result.availability + ' · ' + (result.community?.notice || 'No scan completion is inferred.'))
    } catch {
      setNotice('Lookup unavailable; only the local owner can request enabled EDSM.')
    } finally { setBusy(false) }
  }
  return <section className={'go-page go-galaxy ' + (compact ? 'is-compact' : '')} aria-label='Galaxy map'>
    <div className='go-page__heading'><h2>Galaxy Map</h2><span>Y is vertical · no game control</span></div>
    {!compact && <form className='go-toolbar' onSubmit={e => { e.preventDefault(); search(false) }}>
      <label>System <input aria-label='Galaxy system search' maxLength={128} value={query} onChange={e => setQuery(e.target.value)} /></label>
      <button disabled={busy || !query.trim()}>Search local / cached</button>
      {role === 'owner' && <button type='button' disabled={busy || !query.trim()} onClick={() => search(true)}>Lookup with enabled EDSM</button>}
    </form>}
    {!compact && <div className='go-galaxy-controls'>
      <button onClick={() => setManualFrame(null)}>Fit Route</button>
      <button onClick={() => setManualFrame(EXTENTS)}>Fit Galaxy</button>
      <button aria-label='Galaxy zoom in' onClick={() => setManualFrame(zoomFrame(frame, 0.5))}>+</button>
      <button aria-label='Galaxy zoom out' onClick={() => setManualFrame(zoomFrame(frame, 2))}>−</button>
      <span>Drag to pan · {manualFrame ? 'Manual framing' : 'Automatic route framing'}</span>
      {['poi', 'nebula'].map(category => <label key={category}><input type='checkbox' checked={layerSelection[category]} onChange={e => setLayerSelection({ ...layerSelection, [category]: e.target.checked })} />{category === 'poi' ? 'Rare GEC POIs' : 'Nebulae'}</label>)}
      <small>{layerData?.points?.length ? 'EDAstro GEC rare subset · ' + layerData.fetchedAt + ' · ' + layerData.points.length + ' cached points' : 'Layers not cached. Optional community points require explicit owner consent.'}</small>
    </div>}
    <div ref={area} className='go-galaxy-area'>
      <div className='go-galaxy-projections' style={compact ? undefined : { width: chartSize, height: chartSize }}>
        <Projection {...projectionProps} axes='xz' current={current} destination={destination} searched={searched} route={path} onSelect={setSearched} />
        {!compact && <>
          <Projection {...projectionProps} axes='xy' current={current} destination={destination} searched={searched} route={path} onSelect={setSearched} />
          <Projection {...projectionProps} axes='yz' current={current} destination={destination} searched={searched} route={path} onSelect={setSearched} />
        </>}
      </div>
    </div>
    <p className='go-galaxy-source' role='status'>
      {usable ? manifest.label + ' · ' + manifest.stats.accepted.toLocaleString() + ' coordinates · ' + manifest.source.name + (assetFailure ? ' · Some atlas tiles unavailable; overlays remain usable.' : '') : 'Real-data galaxy atlas not installed. No simulated density shown; route and system coordinate markers remain usable.'}
      {' '}Green: player · cyan: searched · amber: destination.
    </p>
    {!compact && <div className='go-galaxy-detail'>
      <p>{searched?.name || current?.name || 'Current galactic position unknown'} · {searched?.source || current?.source || 'No coordinates reported'} · Distance {show(distance(current?.coordinates, searched?.coordinates), 'LY', 1)}</p>
      {searched?.coordinates && <p>X {searched.coordinates[0]} · Y {searched.coordinates[1]} · Z {searched.coordinates[2]} LY</p>}
      {searched?.name && <button onClick={() => router.push({ pathname: '/nav/map', query: { system: searched.systemName || searched.name } })}>View System</button>}
      <button disabled>Navigate (coming later)</button><p role='status'>{notice}</p>
    </div>}
  </section>
}
