const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { transformSync } = require('next/dist/compiled/babel/core')
const cache = new Map()
function load (relative, react = React) {
  const filename = path.resolve(__dirname, '../../src/client', relative)
  const code = transformSync(fs.readFileSync(filename, 'utf8'), { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
  const mod = new Module(filename, module)
  mod.filename = filename
  mod.paths = module.paths
  mod.require = name => {
    if (name === 'react') return react
    if (name.startsWith('.') && name.endsWith('.json')) return require(path.resolve(path.dirname(filename), name))
    if (name.startsWith('.')) {
      const target = path.relative(path.resolve(__dirname, '../../src/client'), path.resolve(path.dirname(filename), name + '.js'))
      if (!cache.has(target)) cache.set(target, load(target))
      return cache.get(target)
    }
    return require(name)
  }
  mod._compile(code, filename)
  return mod.exports
}
const api = () => load('lib/go-engineering.js')
const iron = { id: '128672318', symbol: 'Iron', name: 'Iron', category: 'raw', grade: 1, sources: ['Surface reference'] }
const nickel = { id: '128672319', symbol: 'Nickel', name: 'Nickel', category: 'raw', grade: 1, sources: [] }
const recipe = { symbol: 'Fixture', name: 'Fixture blueprint', modules: ['Armour'], grades: [{ grade: 1, components: [{ name: 'Iron', cost: 2 }, { name: 'Nickel', cost: 1 }] }] }

test('recipe planning joins journal symbols and labels, multiplies and reports exact shortfalls', () => {
  const result = api().planRecipe(recipe, 1, 3, { state: 'known', items: [{ name: 'iron', label: 'Iron', count: 5 }, { name: 'unusual_symbol', label: 'Nickel', count: 9 }] }, [iron, nickel])
  assert.equal(result.rolls, 3)
  assert.deepEqual(result.components.map(x => [x.label, x.required, x.available, x.missing]), [['Iron', 6, 5, 1], ['Nickel', 3, 9, 0]])
  assert.equal(result.status, 'missing')
})

test('unknown and truncated snapshots never turn absent materials into zero', () => {
  for (const materials of [undefined, { state: 'unknown', items: [] }, { state: 'known', truncated: true, items: [] }]) {
    const result = api().planRecipe(recipe, 1, 1, materials, [iron, nickel])
    assert.equal(result.status, 'unknown')
    assert.equal(result.components[0].available, null)
    assert.equal(result.components[0].missing, null)
  }
  assert.equal(api().planRecipe(recipe, 1, 1, { state: 'known', items: [] }, [iron, nickel]).components[0].missing, 2)
})

test('roll input is bounded to 1 through 100 and unrecognized grades do not claim readiness', () => {
  for (const [input, expected] of [[-9, 1], [0, 1], ['2.7', 2], [999, 100], [NaN, 1], [Infinity, 1]]) assert.equal(api().boundedRolls(input), expected)
  assert.equal(api().planRecipe(recipe, 5, 1, { state: 'known', items: [] }, [iron]).status, 'unknown')
})

test('material catalog merges counts without inventing stock and filters category grade and text', () => {
  const rows = api().materialRows([iron, nickel], { state: 'known', items: [{ name: 'IRON', label: 'Iron', category: 'Raw', count: 7 }, { name: 'guardianpowercell', label: 'Guardian Power Cell', category: 'Guardian', count: 4 }] })
  assert.equal(rows.find(x => x.name === 'Iron').count, 7)
  assert.equal(rows.find(x => x.name === 'Nickel').count, 0)
  assert.equal(rows.find(x => x.label === 'Guardian Power Cell').category, 'xeno')
  assert.deepEqual(api().filterMaterials(rows, { query: 'IRON', category: 'raw', grade: '1' }).map(x => x.label), ['Iron'])
  assert.equal(api().materialRows([iron], { state: 'unknown', items: [] })[0].count, null)
})

test('blueprint filters search module and engineer names together with grade', () => {
  const rows = [{ ...recipe, engineers: ['Engineer Fixture'] }, { symbol: 'Other', name: 'Other', modules: ['FSD'], grades: [{ grade: 5 }] }]
  assert.deepEqual(api().filterBlueprints(rows, { query: 'engineer fixture', grade: '1' }).map(x => x.symbol), ['Fixture'])
  assert.equal(api().filterBlueprints(rows, { query: 'armour', grade: '5' }).length, 0)
})

function harness (props) {
  const states = []; let cursor = 0; let tree
  const component = load('components/go-engineering.js', { ...React, useMemo: (fn, deps) => { const i = cursor++; if (!(i in states) || deps.some((value, index) => value !== states[i].deps[index])) states[i] = { deps, value: fn() }; return states[i].value }, useState: initial => { const i = cursor++; if (!(i in states)) states[i] = typeof initial === 'function' ? initial() : initial; return [states[i], value => { states[i] = typeof value === 'function' ? value(states[i]) : value }] } }).default
  return {
    html () { cursor = 0; tree = component(props); return renderToStaticMarkup(tree) },
    elements (type) { const found = []; const visit = node => { if (!React.isValidElement(node)) return; if (node.type === type) found.push(node); React.Children.forEach(node.props.children, visit) }; visit(tree); return found }
  }
}

test('real blueprint inspector changes recipe, rolls and known material requirements through controls', () => {
  const h = harness({ path: '/eng/material-planner', workspace: { parity: { materials: { state: 'known', items: [] }, modules: [] } } })
  assert.match(h.html(), /Blueprint inspector/)
  assert.match(h.html(), /Worn Shield Emitters/)
  assert.match(h.html(), /Missing/)
  const rolls = h.elements('input').find(x => x.props.type === 'number')
  rolls.props.onChange({ target: { value: '3' } })
  assert.match(h.html(), /3 rolls/)
  h.elements('select').find(x => x.props['aria-label'] === 'Recipe grade').props.onChange({ target: { value: '2' } })
  assert.match(h.html(), /Carbon/)
  assert.match(h.html(), /Shield Emitters/)
})

test('real material inspector shows bundled sources and unknown stock rather than missing', () => {
  const h = harness({ path: '/eng/raw-materials', workspace: { parity: { materials: { state: 'unknown', items: [] } } } })
  assert.match(h.html(), /Material inspector/)
  assert.match(h.html(), /Unknown/)
  assert.match(h.html(), /Bundled collection reference/)
})

test('engineer reference states progress unknown and module inspector only shows reported modifiers', () => {
  const engineers = harness({ path: '/eng/engineers', workspace: {} })
  assert.match(engineers.html(), /Unlock progress unknown/)
  assert.match(engineers.html(), /Didi Vatermann/)
  const h = harness({ path: '/eng/blueprints', workspace: { parity: { modules: [{ slot: 'Fixture slot', name: 'Fixture module', engineering: { blueprint: 'AFM_Shielded', grade: 2, quality: 0.4, experimental: 'Fixture experimental', modifiers: [{ label: 'Fixture integrity', value: 42, originalValue: 20, lessIsGood: false }] } }] } } })
  assert.match(h.html(), /Fixture integrity/)
  assert.match(h.html(), /42/)
  assert.match(h.html(), /Fixture experimental/)
  assert.doesNotMatch(h.html(), /Game connected|Apply blueprint|Synthesize now/)
})

test('known sufficient stock reports ready without consuming journal counts', () => {
  const materials = { state: 'known', items: [{ name: 'Iron', count: 4 }, { name: 'Nickel', count: 2 }] }
  const result = api().planRecipe(recipe, 1, 2, materials, [iron, nickel])
  assert.equal(result.status, 'ready')
  assert.deepEqual(result.components.map(c => c.missing), [0, 0])
  assert.deepEqual(materials.items.map(c => c.count), [4, 2])
})

test('selected reference retains string identities and every bundled recipe joins a material', () => {
  const reference = require('../../src/client/data/go-engineering-reference.json')
  assert.equal(reference.blueprints.length, 80)
  assert.equal(reference.materials.length, 137)
  assert.equal(reference.engineers.length, 38)
  assert.ok(reference.materials.every(m => typeof m.id === 'string'))
  assert.ok(reference.engineers.every(e => typeof e.id === 'string'))
  const names = new Set(reference.materials.map(m => m.name.toLowerCase()))
  for (const blueprint of reference.blueprints) for (const grade of blueprint.grades) for (const component of grade.components) assert.ok(names.has(component.name.toLowerCase()), component.name)
  assert.match(reference.provenance.label, /may be outdated/)
  assert.ok(reference.provenance.sources.every(source => /^[a-f0-9]{64}$/.test(source.sha256)))
})

test('engineer progress joins case-insensitive names into detached rows without mutating reference or journal', () => {
  const reference = [{ id: '9007199254740993', name: 'Didi Vatermann', unlockRequirements: [{ stage: 'unlock', description: 'Static requirement' }] }, { id: '2', name: 'Bill Turner' }]
  const journal = [{ name: 'dIDI vATERMANN', progress: 'Unlocked', rank: 3, rankProgress: 0.25 }]
  const original = JSON.stringify({ reference, journal })
  const rows = api().engineerRows(reference, journal)
  assert.equal(rows[0].journalProgress.progress, 'Unlocked')
  assert.equal(rows[0].journalProgress.rank, 3)
  assert.equal(rows[0].journalProgress.rankProgress, 0.25)
  assert.equal(rows[0].id, '9007199254740993')
  assert.equal(rows[1].journalProgress, null)
  rows[0].journalProgress.rank = 5
  rows[0].name = 'Output changed'
  rows[0].unlockRequirements[0].description = 'Output requirement changed'
  assert.equal(JSON.stringify({ reference, journal }), original)
})

test('real engineer inspector shows explicit journal progress and clears it on source replacement', () => {
  const props = { path: '/eng/engineers', workspace: { activity: { commander: { engineers: [{ name: 'DIDI VATerMANN', progress: 'Unlocked', rank: 0, rankProgress: 0.25 }] } } } }
  const before = JSON.stringify(props.workspace)
  const h = harness(props)
  const html = h.html()
  assert.match(html, /Journal progress: Unlocked/)
  assert.match(html, /Journal rank<\/small><strong>0/)
  assert.match(html, /Rank progress \(reported\)<\/small><strong>0.25/)
  assert.match(html, /Bundled unlock reference/)
  assert.doesNotMatch(html, /Unlock progress unknown/)
  assert.equal(JSON.stringify(props.workspace), before)
  props.workspace = { activity: { commander: { engineers: [] } } }
  const cleared = h.html()
  assert.match(cleared, /Unlock progress unknown/)
  assert.doesNotMatch(cleared, /Journal progress: Unlocked/)
})
