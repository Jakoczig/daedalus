const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const os = require('node:os')
const path = require('node:path')
const { generate } = require('../../scripts/build/galaxy-density.cjs')
test('offline generator streams coordinate CSV and records license, axes and incomplete-coverage provenance', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'daedalus-density-test-'))
  try { const csv = path.join(dir, 'input.csv'); await fs.writeFile(csv, 'x,y,z\n0,0,0\n3,4,5\n'); await assert.rejects(generate({ directory: path.join(dir, 'bad'), csv })); const meta = await generate({ directory: path.join(dir, 'out'), csv, source: 'fabricated fixture', license: 'test fixture' }); assert.equal(meta.illustrative, false); assert.equal(meta.records, 2); assert.match(meta.label, /not total/); assert.deepEqual(meta.views.map(v => v.axes), ['xz', 'xy', 'zy', 'yz']); for (const v of meta.views) { const raw = await fs.readFile(path.join(dir, 'out', `galaxy-${v.axes}.png`)); assert.equal(raw.subarray(0, 8).toString('hex'), '89504e470d0a1a0a') } } finally { await fs.rm(dir, { recursive: true, force: true }) }
})

test('right edge texture has Y horizontal and Z vertical rather than a rotated face-on copy', async t => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'eridion-yz-'))
  t.after(() => fs.rm(dir, { recursive: true, force: true }))
  const m = await generate({ directory: dir })
  const v = m.views.find(v => v.axes === 'yz')
  assert.ok(v, 'Missing real Y/Z projection')
  assert.equal(v.width, 120)
  assert.equal(v.height, 512)
  assert.match(m.coordinates, /Y\/Z/)
})
