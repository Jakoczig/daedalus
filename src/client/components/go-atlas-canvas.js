import { useEffect, useRef } from 'react'

// Close-zoom occupied-bin presentation adapted from Galaxy Painter AtlasRenderer
// (Jakoczig, atlas.py blob 75551b7ef96f3d8d5b09950688206c6def35d3b5, GPL-3.0).
// Client adaptation 2026-11-21; see LICENSE and the G2Q handoff for provenance.
// These are density-bin centres, never individual stellar coordinates.
export const maxBinSymbols = 12000
export function safeTileURL (url) { return typeof url === 'string' && /^\/atlas\/assets\/[a-f0-9]{24}\/(tiles\/)?(xz|xy|yz)\/[0-7]\/(0|[1-9][0-9]{0,2})-(0|[1-9][0-9]{0,2})\.png$/.test(url) }
export function useBinSymbols (tile, pixelsPerBin, verticalPixelsPerBin = pixelsPerBin) { return tile.level === 7 && tile.sourceLevel === 7 && Math.min(pixelsPerBin, verticalPixelsPerBin) > 1 / 0.65 }
export function tileGeometry (t, extents, frame, width, height) {
  const axes = t.axes; const size = 256 * 2 ** t.level; const pw = axes === 'yz' ? size / 4 : size; const ph = axes === 'xy' ? size / 4 : size
  const a = extents[axes[0]]; const b = extents[axes[1]]; const fx = frame[axes[0]]; const fy = frame[axes[1]]
  const tileW = Math.min(256, pw - t.x * 256); const tileH = Math.min(256, ph - t.y * 256)
  const x = width * (a[0] + (a[1] - a[0]) * t.x * 256 / pw - fx[0]) / (fx[1] - fx[0]); const y = height * (fy[1] - (b[1] - (b[1] - b[0]) * t.y * 256 / ph)) / (fy[1] - fy[0])
  const w = width * (a[1] - a[0]) * tileW / pw / (fx[1] - fx[0]); const h = height * (b[1] - b[0]) * tileH / ph / (fy[1] - fy[0])
  const scale = 2 ** (t.level - t.sourceLevel); const sx = (t.x / scale - t.sourceX) * 256; const sy = (t.y / scale - t.sourceY) * 256
  const sourceSize = 256 * 2 ** t.sourceLevel; const iw = Math.min(256, (axes === 'yz' ? sourceSize / 4 : sourceSize) - t.sourceX * 256); const ih = Math.min(256, (axes === 'xy' ? sourceSize / 4 : sourceSize) - t.sourceY * 256)
  return { x, y, w, h, sx, sy, sw: tileW / scale, sh: tileH / scale, iw, ih }
}
export function occupiedBins (rgba, iw, ih, crop, limit = maxBinSymbols) {
  const bins = []; const { sx, sy, sw, sh, width, height } = crop
  for (let y = Math.max(0, Math.floor(sy)); y < Math.min(ih, Math.ceil(sy + sh)); y++) {
    for (let x = Math.max(0, Math.floor(sx)); x < Math.min(iw, Math.ceil(sx + sw)); x++) {
      const i = (y * iw + x) * 4
      if (!rgba[i + 3]) continue
      if (bins.length >= limit) return null
      bins.push({ x: (x + 0.5 - sx) * width / sw, y: (y + 0.5 - sy) * height / sh, color: Array.from(rgba.slice(i, i + 4)) })
    }
  }
  return bins
}
// At most 48 decoded 256px source images (~12 MiB RGBA). No full-galaxy canvas,
// raw coordinate dump, rendering loop, remote fetch or persistent pixel duplication.
const images = new Map()
function sourceImage (url) {
  if (!safeTileURL(url)) return Promise.reject(Error('Invalid local tile reference'))
  if (images.has(url)) { const value = images.get(url); images.delete(url); images.set(url, value); return value }
  const value = new Promise((resolve, reject) => { const image = new window.Image(); image.onload = () => image.width <= 256 && image.height <= 256 ? resolve(image) : reject(Error('Tile dimensions')); image.onerror = () => reject(Error('Cached tile unavailable')); image.src = url })
  images.set(url, value); while (images.size > 48) images.delete(images.keys().next().value)
  value.catch(() => { if (images.get(url) === value) images.delete(url) }); return value
}
export default function AtlasBinCanvas ({ tiles, extents, frame, width, height, viewKey, onReady }) {
  const ref = useRef(null)
  useEffect(() => {
    let active = true; const canvas = ref.current; if (!canvas) return
    const rect = canvas.getBoundingClientRect(); const cssW = Math.max(1, rect.width); const cssH = Math.max(1, rect.height); const dpr = Math.min(2, window.devicePixelRatio || 1)
    canvas.width = Math.min(2048, Math.ceil(cssW * dpr)); canvas.height = Math.min(2048, Math.ceil(cssH * dpr))
    const ctx = canvas.getContext('2d'); if (!ctx) return
    ctx.scale(canvas.width / cssW, canvas.height / cssH)
    const draw = async () => {
      const prepared = []; let count = 0; let dense = false
      const scratch = document.createElement('canvas'); scratch.width = scratch.height = 256; const pixels = scratch.getContext('2d', { willReadFrequently: true }); if (!pixels) return
      for (const t of tiles) {
        if (!active) return
        const image = await sourceImage(t.url); if (!active) return
        const g = tileGeometry(t, extents, frame, cssW, cssH)
        const left = Math.max(0, g.x); const top = Math.max(0, g.y); const right = Math.min(cssW, g.x + g.w); const bottom = Math.min(cssH, g.y + g.h)
        if (right <= left || bottom <= top) continue
        const crop = { sx: g.sx + (left - g.x) * g.sw / g.w, sy: g.sy + (top - g.y) * g.sh / g.h, sw: (right - left) * g.sw / g.w, sh: (bottom - top) * g.sh / g.h, width: right - left, height: bottom - top }
        pixels.clearRect(0, 0, 256, 256); pixels.drawImage(image, 0, 0)
        const bins = dense ? null : occupiedBins(pixels.getImageData(0, 0, image.width, image.height).data, image.width, image.height, crop, maxBinSymbols - count)
        if (bins === null) dense = true; else count += bins.length
        prepared.push({ image, g, crop, bins, left, top })
      }
      if (!active) return
      ctx.clearRect(0, 0, cssW, cssH)
      for (const p of prepared) {
        if (dense) { ctx.imageSmoothingEnabled = false; ctx.drawImage(p.image, p.crop.sx, p.crop.sy, p.crop.sw, p.crop.sh, p.left, p.top, p.crop.width, p.crop.height); continue }
        const radius = Math.min(3, Math.max(1.3, 0.32 * p.g.w / p.g.sw))
        for (const bin of p.bins) { ctx.fillStyle = `rgba(${bin.color[0]},${bin.color[1]},${bin.color[2]},${bin.color[3] / 255})`; ctx.beginPath(); ctx.arc(p.left + bin.x, p.top + bin.y, radius, 0, 2 * Math.PI); ctx.fill() }
      }
      canvas.dataset.mode = dense ? 'bounded-raster' : 'occupied-bins'; canvas.dataset.symbols = String(dense ? 0 : count); onReady(viewKey)
    }
    draw().catch(() => {}) // Retain the verified SVG raster if decoding fails.
    return () => { active = false }
  }, [viewKey])
  return <foreignObject data-atlas-level='7' data-source-level='7' x='0' y='0' width={width} height={height} pointerEvents='none'><canvas ref={ref} style={{ width: '100%', height: '100%', display: 'block', background: 'transparent' }} aria-label='Occupied density bins, not individual stars' /></foreignObject>
}
