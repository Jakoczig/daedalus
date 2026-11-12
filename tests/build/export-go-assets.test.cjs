const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const os = require('node:os')
const path = require('node:path')
const { exportAssets } = require('../../scripts/build/export-go-assets.cjs')

async function fixture (t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'daedalus-export-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const sourceDir = path.join(root, 'source')
  const destinationDir = path.join(root, 'destination')
  await fs.mkdir(sourceDir)
  return { root, sourceDir, destinationDir }
}

test('missing index fails without destroying an existing destination', async t => {
  const options = await fixture(t)
  await fs.mkdir(options.destinationDir)
  await fs.writeFile(path.join(options.destinationDir, 'keep'), 'keep')
  await assert.rejects(exportAssets(options), /index.html/)
  assert.equal(await fs.readFile(path.join(options.destinationDir, 'keep'), 'utf8'), 'keep')
})

test('copies bytes deterministically, preserves deep routes and removes stale destination files', async t => {
  const options = await fixture(t)
  const files = { 'index.html': '<html>fixture</html>', 'exploration/system.html': '<html>nested</html>', '_next/static/chunk.js': 'const fixture = true\n' }
  for (const [name, content] of Object.entries(files)) {
    await fs.mkdir(path.dirname(path.join(options.sourceDir, name)), { recursive: true })
    await fs.writeFile(path.join(options.sourceDir, name), content)
  }
  await fs.mkdir(options.destinationDir)
  await fs.writeFile(path.join(options.destinationDir, 'stale.html'), 'old')
  assert.equal((await exportAssets(options)).files, 3)
  for (const [name, content] of Object.entries(files)) assert.equal(await fs.readFile(path.join(options.destinationDir, name), 'utf8'), content)
  await assert.rejects(fs.access(path.join(options.destinationDir, 'stale.html')))
  const before = await fs.readFile(path.join(options.destinationDir, 'g1a-assets.json'), 'utf8')
  await exportAssets(options)
  assert.equal(await fs.readFile(path.join(options.destinationDir, 'g1a-assets.json'), 'utf8'), before)
})

test('rejects overlapping roots, stale inputs and symlink escapes before deletion', async t => {
  const options = await fixture(t)
  await fs.writeFile(path.join(options.sourceDir, 'index.html'), 'fixture')
  await assert.rejects(exportAssets({ ...options, destinationDir: options.sourceDir }), /overlap/)
  await assert.rejects(exportAssets({ ...options, destinationDir: path.join(options.sourceDir, 'child') }), /overlap/)
  await assert.rejects(exportAssets({ ...options, newestInputMs: Date.now() + 10000 }), /stale/)
  const outside = path.join(options.root, 'outside')
  await fs.mkdir(outside)
  // Windows junctions need no developer-mode privilege.
  await fs.symlink(outside, path.join(options.sourceDir, 'escape'), 'junction')
  await assert.rejects(exportAssets(options), /symbolic link/)
  await fs.rm(path.join(options.sourceDir, 'escape'))
  await fs.symlink(outside, options.destinationDir, 'junction')
  await assert.rejects(exportAssets(options), /symbolic link/)
})
