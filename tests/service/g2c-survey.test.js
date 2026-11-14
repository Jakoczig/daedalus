const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const { transformSync } = require('next/dist/compiled/babel/core')
function load (filename) {
  const code = transformSync(fs.readFileSync(filename, 'utf8'), { babelrc: false, configFile: false, plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
  const mod = new Module(filename, module); mod.paths = module.paths; mod._compile(code, filename); return mod.exports
}
function survey () { return load(path.resolve(__dirname, '../../src/client/lib/go-survey.js')) }
test('G2C map uses exact parent IDs, stable order and bounded cycle-safe layout', () => {
  const { layoutBodies } = survey()
  const bodies = [{ bodyId: '9007199254740993', name: 'Star', details: { isStar: true } }, { bodyId: '9007199254740994', name: 'Moon', details: { parents: [{ kind: 'Planet', bodyId: '9007199254740995' }] } }, { bodyId: '9007199254740995', name: 'Planet', details: { parents: [{ kind: 'Star', bodyId: '9007199254740993' }] } }, { bodyId: '8', name: 'Unknown', details: { parents: [{ kind: 'Null', bodyId: '0' }] } }]
  const copy = JSON.stringify(bodies)
  const layout = layoutBodies(bodies)
  assert.equal(layout.find(n => n.body.bodyId === '9007199254740994').depth, 2)
  assert.equal(layout.find(n => n.body.bodyId === '9007199254740995').depth, 1)
  assert.equal(JSON.stringify(bodies), copy)
  assert.equal(layout.find(n => n.body.bodyId === '8').unresolved, true)
  const cycle = layoutBodies([{ bodyId: '1', details: { parents: [{ kind: 'Planet', bodyId: '2' }] } }, { bodyId: '2', details: { parents: [{ kind: 'Planet', bodyId: '1' }] } }])
  assert.equal(cycle.length, 2); assert.ok(cycle.every(n => n.unresolved))
})
test('G2C survey filters distinguish witnessed mapping and confirmed biology', () => {
  const { selectBodies, surveySummary } = survey()
  const bodies = [{ bodyId: '1', name: 'Alpha', landable: true, dssMapped: false, previouslyMapped: true, bioSignals: 2 }, { bodyId: '2', name: 'Beta', dssMapped: true, organisms: [{ species: 'Confirmed', complete: true }] }]
  assert.deepEqual(selectBodies(bodies, '', 'unmapped').map(b => b.bodyId), ['1'])
  assert.deepEqual(selectBodies(bodies, 'beta', 'biology').map(b => b.bodyId), ['2'])
  assert.equal(surveySummary(bodies).analysed, 1); assert.equal(surveySummary(bodies).mapped, 1)
})

test('G2C genus-only observations agree between worklist and summary', () => {
  const { selectBodies, surveySummary } = survey(); const bodies = [{ bodyId: '1', genuses: ['Bacterium'] }]
  assert.equal(selectBodies(bodies, '', 'biology').length, 1); assert.equal(surveySummary(bodies).biologicalBodies, 1)
})
