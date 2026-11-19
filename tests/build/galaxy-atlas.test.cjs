const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const os = require('node:os')
const path = require('node:path')
const { generate, jsonObjects, dimensions, projectBin } = require('../../scripts/build/galaxy-atlas.cjs')
const { Readable } = require('node:stream')
const zlib = require('node:zlib')

test('JSON object streaming handles arbitrary splits, escapes and commas', async () => {
  const input = JSON.stringify([{ name: 'A, B', coords: { x: 0, y: 0, z: 0 } }, { name: 'C', coords: { x: 20, y: 30, z: 40 } }])
  const chunks = []
  for (let i = 0; i < input.length; i += 3) chunks.push(input.slice(i, i + 3))
  const got = []
  for await (const obj of jsonObjects(Readable.from(chunks))) got.push(obj)
  assert.equal(got.length, 2)
  assert.equal(got[0].name, 'A, B')
  assert.equal(got[1].coords.y, 30)
  await assert.rejects(async () => {
    for await (const obj of jsonObjects(Readable.from(['[{"coords":{}}']))) assert.ok(obj)
  }, /Truncated/)
})

test('projection has consistent X, Y, Z orientations', () => {
  const p = { x: 0, y: 0, z: 25900 }
  const top = projectBin(p, 'xz', 512, 512)
  const bottom = projectBin(p, 'xy', 512, 118)
  const right = projectBin(p, 'yz', 256, 1109)
  assert.equal(top.x, bottom.x)
  assert.equal(bottom.y, 59)
  assert.equal(right.x, 128)
  assert.equal(top.y, Math.floor((95000 - 25900) / 130000 * 512))
  assert.equal(right.y, Math.floor((95000 - 25900) / 130000 * 1109))
  assert.ok(dimensions('xy', 1).height < dimensions('xz', 1).height)
  assert.ok(dimensions('yz', 1).height > dimensions('yz', 1).width)
})

test('offline EDSM gzip generates an atomic real tile manifest', async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'eridion-galaxy-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const input = path.join(root, 'systems.json.gz')
  const rows = [
    { name: 'Sol', coords: { x: 0, y: 0, z: 0 } },
    { name: 'Near center', coords: { x: 0, y: 0, z: 25900 } },
    { name: 'Unknown' },
    { name: 'Outside', coords: { x: 900000, y: 0, z: 0 } }
  ]
  await fs.writeFile(input, zlib.gzipSync(JSON.stringify(rows)))
  const out = path.join(root, 'out')
  const m = await generate({ input, directory: out, sourceUrl: 'EDSM fixture', sourceDate: '2026-11-19', maxLevel: 1 })
  assert.deepEqual(m.stats, { read: 4, accepted: 2, invalid: 1, outside: 1 })
  assert.equal(m.views.xy.levels.length, 2)
  assert.equal(m.source.inputSha256.length, 64)
  assert.match(m.source.redistribution, /permission/)
  const tile = await fs.readFile(path.join(out, 'tiles', m.run, 'xz', '0', '0-0.png'))
  assert.equal(tile.subarray(0, 8).toString('hex'), '89504e470d0a1a0a')
  assert.equal(JSON.parse(await fs.readFile(path.join(out, 'manifest.json'))).run, m.run)
  await assert.rejects(generate({ input: path.join(root, 'missing'), directory: out }), /ENOENT/)
  assert.equal(JSON.parse(await fs.readFile(path.join(out, 'manifest.json'))).run, m.run)
})

test('not-installed manifest never claims illustrative density is real', () => {
  const fallback = require('../../src/client/public/images/galaxy-atlas/manifest.json')
  assert.equal(fallback.status, 'not-installed')
  assert.equal(fallback.views.xz, undefined)
})
