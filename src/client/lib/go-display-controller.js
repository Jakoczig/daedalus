// Event-driven policy: never issues game commands or opens native windows.
export function transition (state, input) {
  const { mode = 'idle', episode = '', fresh, routeKnown, policy = {}, manual, disconnected } = input
  if (manual) return { ...state, visible: false, blocked: episode || state.episode }
  if (disconnected || !fresh) return { visible: false, blocked: state.blocked || '', episode: state.episode || '' }
  if (mode === 'idle') return { visible: state.visible && policy.restore === false, blocked: '', episode: state.episode, finished: true }
  if (!policy.selected || !routeKnown || state.blocked === episode || !(mode === 'charge' ? policy.charge : policy.jump)) return { ...state, visible: false }
  return { visible: true, blocked: state.blocked, episode, finished: false }
}
export function scoopability (type) { if (!type) return 'UNKNOWN'; return /^[OBAFGKM]$/.test(type) ? 'SCOOPABLE' : /^(N|D.*|H|T|L|Y|W.*|S|C.*|MS|AeBe)$/.test(type) ? 'NOT SCOOPABLE' : 'UNKNOWN' }
export function project (coordinates, axes, extents) { if (!Array.isArray(coordinates) || coordinates.length !== 3 || !coordinates.every(Number.isFinite)) return null; const a = 'xyz'.indexOf(axes[0]); const b = 'xyz'.indexOf(axes[1]); const x = (coordinates[a] - extents[axes[0]][0]) / (extents[axes[0]][1] - extents[axes[0]][0]); const y = 1 - (coordinates[b] - extents[axes[1]][0]) / (extents[axes[1]][1] - extents[axes[1]][0]); return { x, y } }
export function distance (a, b) { return a?.length === 3 && b?.length === 3 && [...a, ...b].every(Number.isFinite) ? Math.hypot(...a.map((v, i) => v - b[i])) : null }
