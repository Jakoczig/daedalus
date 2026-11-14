// Small pure joins shared by the Go reference browser and local per-roll planner.
// Only explicit, known journal snapshots can establish zero stock.
const key = value => String(value || '').toLowerCase().replace(/[^a-z0-9]/g, '')
export function boundedRolls (value) {
  const number = Number(value)
  return Number.isFinite(number) ? Math.min(100, Math.max(1, Math.floor(number))) : 1
}
function categoryOf (value) {
  const category = key(value)
  return ['guardian', 'thargoid', 'xeno'].some(x => category.includes(x)) ? 'xeno' : category
}
function stockIndex (materials) {
  const result = new Map()
  if (materials?.state !== 'known') return result
  for (const item of materials.items || []) {
    if (!Number.isSafeInteger(item.count) || item.count < 0) continue
    for (const name of [item.name, item.label]) if (name) result.set(key(name), item.count)
  }
  return result
}
function availableCount (index, names, materials) {
  for (const name of names) if (index.has(key(name))) return index.get(key(name))
  return materials?.state === 'known' && !materials.truncated ? 0 : null
}
export function materialRows (catalog, materials) {
  const index = stockIndex(materials)
  const rows = catalog.map(material => ({ ...material, label: material.name, category: categoryOf(material.category), count: availableCount(index, [material.symbol, material.name], materials) }))
  const referenceNames = new Set(catalog.flatMap(m => [key(m.name), key(m.symbol)]))
  for (const item of materials?.items || []) {
    if (referenceNames.has(key(item.name)) || referenceNames.has(key(item.label))) continue
    rows.push({ symbol: item.name, name: item.label || item.name, label: item.label || item.name, category: categoryOf(item.category), grade: null, sources: [], count: availableCount(index, [item.name, item.label], materials) })
  }
  return rows
}
export function filterMaterials (rows, { query = '', category = '', grade = '' } = {}) {
  const text = query.trim().toLowerCase()
  return rows.filter(row => (!category || row.category === category) && (!grade || String(row.grade) === grade) && (!text || `${row.label} ${row.symbol} ${row.category}`.toLowerCase().includes(text)))
}
export function filterBlueprints (rows, { query = '', grade = '' } = {}) {
  const text = query.trim().toLowerCase()
  return rows.filter(row => (!grade || row.grades.some(g => String(g.grade) === grade)) && (!text || `${row.name} ${row.symbol} ${(row.modules || []).join(' ')} ${(row.engineers || []).join(' ')}`.toLowerCase().includes(text)))
}
export function planRecipe (blueprint, grade, amount, materials, catalog) {
  const rolls = boundedRolls(amount)
  const recipe = blueprint?.grades.find(r => r.grade === Number(grade))
  if (!recipe?.components?.length) return { rolls, components: [], status: 'unknown' }
  const stock = stockIndex(materials)
  const components = recipe.components.map(component => {
    const material = catalog.find(m => key(m.name) === key(component.name) || key(m.symbol) === key(component.name))
    const available = availableCount(stock, [component.name, material?.symbol], materials)
    const required = component.cost * rolls
    return { label: material?.name || component.name, category: material?.category || 'unknown', grade: material?.grade ?? null, required, available, missing: available == null ? null : Math.max(0, required - available) }
  })
  return { rolls, components, status: components.some(c => c.missing == null) ? 'unknown' : components.some(c => c.missing > 0) ? 'missing' : 'ready' }
}

export function engineerRows (catalog, observations = []) {
  const observed = new Map(observations.map(engineer => [String(engineer.name || '').trim().toLowerCase(), engineer]))
  return catalog.map(engineer => {
    const journal = observed.get(engineer.name.trim().toLowerCase())
    return { ...engineer, unlockRequirements: engineer.unlockRequirements?.map(requirement => ({ ...requirement })), journalProgress: journal ? { ...journal } : null }
  })
}
