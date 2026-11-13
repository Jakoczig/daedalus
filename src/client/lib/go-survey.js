// Pure, bounded adapters over typed Go DTOs. IDs stay strings throughout.
const order = (a, b) => a.bodyId.length - b.bodyId.length || a.bodyId.localeCompare(b.bodyId)
export function layoutBodies (bodies = []) {
  const nodes = new Map(bodies.map(body => [body.bodyId, { body, depth: 0, parent: null, unresolved: false, state: 0 }]))
  for (const node of nodes.values()) {
    const parents = node.body.details?.parents || []
    if (parents.length) {
      const first = parents[0]
      node.parent = first.kind !== 'Null' && first.bodyId !== node.body.bodyId ? nodes.get(first.bodyId) || null : null
      node.unresolved = !node.parent
    }
  }
  const visit = node => {
    if (node.state === 2) return
    if (node.state === 1) { node.unresolved = true; node.parent = null; return }
    node.state = 1
    if (node.parent) {
      visit(node.parent)
      node.unresolved ||= node.parent.unresolved
      node.depth = node.unresolved ? 0 : Math.min(12, node.parent.depth + 1)
    }
    node.state = 2
  }
  for (const node of nodes.values()) visit(node)
  const result = [...nodes.values()]
  const children = new Map()
  for (const node of result) {
    const parent = node.unresolved ? null : node.parent
    if (!children.has(parent)) children.set(parent, [])
    children.get(parent).push(node)
  }
  const ordered = []
  const walk = parent => { for (const node of (children.get(parent) || []).sort((a, b) => order(a.body, b.body))) { ordered.push(node); walk(node) } }
  walk(null)
  return ordered
}
export function selectBodies (bodies = [], query = '', filter = 'all') {
  const needle = query.toLocaleLowerCase().trim()
  return bodies.filter(body => (!needle || `${body.name} ${body.kind} ${body.bodyId} ${body.genuses?.join(' ')} ${body.organisms?.map(o => `${o.genus} ${o.species}`).join(' ')}`.toLocaleLowerCase().includes(needle)) &&
    (filter !== 'landable' || body.landable === true) &&
    (filter !== 'unmapped' || body.dssMapped !== true) &&
    (filter !== 'biology' || body.bioSignals > 0 || body.genuses?.length > 0 || body.organisms?.length > 0))
}
export function surveySummary (bodies = []) {
  return bodies.reduce((s, b) => ({ mapped: s.mapped + (b.dssMapped ? 1 : 0), landable: s.landable + (b.landable ? 1 : 0), biologicalBodies: s.biologicalBodies + (b.bioSignals > 0 || b.organisms?.length > 0 ? 1 : 0), analysed: s.analysed + (b.organisms || []).filter(o => o.complete).length }), { mapped: 0, landable: 0, biologicalBodies: 0, analysed: 0 })
}
