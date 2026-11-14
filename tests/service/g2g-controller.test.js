const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const Module = require('node:module')
const { transformSync } = require('next/dist/compiled/babel/core')
const mod = new Module(__filename, module)
mod._compile(transformSync(fs.readFileSync('src/client/lib/go-display-controller.js', 'utf8'), { babelrc: false, configFile: false, plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code, __filename)
const { transition, project, distance, scoopability } = mod.exports
const input = { mode: 'charge', episode: 'one', fresh: true, routeKnown: true, policy: { selected: true, charge: true, jump: true, restore: true } }
test('automatic policy targets only selected displays, restores and yields through the whole manually overridden episode', () => {
  assert.equal(transition({}, { ...input, policy: { ...input.policy, selected: false } }).visible, false)
  let state = transition({}, input); assert.equal(state.visible, true)
  state = transition(state, { ...input, manual: true }); assert.equal(state.visible, false)
  state = transition(state, { ...input, disconnected: true }); assert.equal(state.visible, false)
  state = transition(state, { ...input, mode: 'jump' }); assert.equal(state.visible, false)
  state = transition(state, { ...input, mode: 'idle' }); assert.equal(state.visible, false)
  state = transition(state, { ...input, episode: 'two' }); assert.equal(state.visible, true)
  assert.equal(transition(state, { ...input, mode: 'idle' }).visible, false)
  assert.equal(transition(state, { ...input, fresh: false }).visible, false)
  assert.equal(transition({}, { ...input, routeKnown: false }).visible, false)
})
test('independent mode switches and optional restore', () => {
  const policy = { ...input.policy, charge: false, restore: false }
  assert.equal(transition({}, { ...input, policy }).visible, false)
  const state = transition({}, { ...input, mode: 'jump', policy }); assert.equal(state.visible, true)
  assert.equal(transition(state, { ...input, mode: 'idle', policy }).visible, true)
  assert.equal(transition(state, { ...input, disconnected: true, policy }).visible, false)
})
test('three galaxy projections preserve Elite Y vertical and real 3D distance', () => {
  const extents = { x: [-10, 10], y: [-10, 10], z: [-10, 10] }
  assert.deepEqual(project([2, 4, 6], 'xz', extents), { x: 0.6, y: 0.19999999999999996 })
  assert.deepEqual(project([2, 4, 6], 'xy', extents), { x: 0.6, y: 0.30000000000000004 })
  assert.deepEqual(project([2, 4, 6], 'zy', extents), { x: 0.8, y: 0.30000000000000004 })
  assert.equal(distance([0, 0, 0], [3, 4, 0]), 5)
  assert.equal(distance(null, [3, 4, 0]), null)
  for (const t of ['O', 'B', 'A', 'F', 'G', 'K', 'M']) assert.equal(scoopability(t), 'SCOOPABLE')
  for (const t of ['N', 'D', 'L', 'T', 'Y']) assert.equal(scoopability(t), 'NOT SCOOPABLE')
  assert.equal(scoopability('Mystery'), 'UNKNOWN')
})
