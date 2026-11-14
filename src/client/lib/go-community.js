// Community data never changes the personal journal survey or sampling state.
export function mergeCommunity (local, community) {
  const bodies = (local?.bodies || []).map(body => ({ ...body, source: 'local' }))
  if (!community?.enabled || !['cached', 'stale', 'unavailable'].includes(community.status) || community.systemName?.toLowerCase() !== local?.systemName?.toLowerCase()) return bodies
  const byId = new Map(bodies.map(body => [body.bodyId, body]))
  for (const external of community.bodies || []) {
    const witnessed = byId.get(external.bodyId)
    // Exact body identity only; names or provider database IDs are not join keys.
    if (witnessed) { witnessed.communityReference = external; continue }
    const body = { ...external, source: 'community', dssMapped: false, bioSignals: null, geoSignals: null, previouslyMapped: null, previouslyDiscovered: null, genuses: [], organisms: [] }
    bodies.push(body); byId.set(body.bodyId, body)
  }
  return bodies.slice(0, 512)
}
