const fs = require('node:fs/promises')
const path = require('node:path')
const crypto = require('node:crypto')

async function rejectLinks (target) {
  let current = path.resolve(target)
  while (true) {
    try { if ((await fs.lstat(current)).isSymbolicLink()) throw new Error('Refusing symbolic link in asset path') } catch (e) { if (e.code !== 'ENOENT') throw e }
    const parent = path.dirname(current)
    if (parent === current) break
    current = parent
  }
}

async function inventory (root, relative = '') {
  const files = []
  for (const entry of await fs.readdir(path.join(root, relative), { withFileTypes: true })) {
    if (entry.isSymbolicLink()) throw new Error('Refusing symbolic link in frontend export')
    const name = path.join(relative, entry.name)
    if (entry.isDirectory()) files.push(...await inventory(root, name))
    else if (entry.isFile()) files.push(name)
    else throw new Error('Unsupported frontend asset')
  }
  return files.sort()
}

async function exportAssets ({ sourceDir, destinationDir, newestInputMs = 0 }) {
  const source = path.resolve(sourceDir)
  const destination = path.resolve(destinationDir)
  if (destination === path.parse(destination).root || source === destination || source.startsWith(destination + path.sep) || destination.startsWith(source + path.sep)) throw new Error('Asset roots overlap or destination is a volume root')
  await rejectLinks(source)
  await rejectLinks(destination)
  let index
  try { index = await fs.stat(path.join(source, 'index.html')) } catch { throw new Error('Missing frontend index.html; run npm run build:client first') }
  if (!index.isFile()) throw new Error('Frontend index.html must be a file')
  if (index.mtimeMs < newestInputMs) throw new Error('Frontend export is stale; rebuild the client')
  const files = await inventory(source)
  const staging = `${destination}.staging-${crypto.randomBytes(8).toString('hex')}`
  await fs.mkdir(staging, { recursive: true })
  try {
    const hashes = {}
    for (const name of files) {
      const bytes = await fs.readFile(path.join(source, name))
      await fs.mkdir(path.dirname(path.join(staging, name)), { recursive: true })
      await fs.writeFile(path.join(staging, name), bytes)
      hashes[name.split(path.sep).join('/')] = crypto.createHash('sha256').update(bytes).digest('hex')
    }
    await fs.writeFile(path.join(staging, 'g1a-assets.json'), JSON.stringify({ files: hashes }, null, 2) + '\n')
    // Both absolute roots and all existing ancestors were checked above. Only
    // the explicitly supplied generated destination is replaced, never source.
    await fs.rm(destination, { recursive: true, force: true })
    await fs.rename(staging, destination)
    return { files: files.length }
  } finally { await fs.rm(staging, { recursive: true, force: true }) }
}

async function main () {
  const root = path.resolve(__dirname, '../..')
  const inputs = (await inventory(path.join(root, 'src/client'))).filter(name => !name.split(path.sep).includes('.next'))
  const timestamps = await Promise.all(inputs.map(name => fs.stat(path.join(root, 'src/client', name)).then(s => s.mtimeMs)))
  for (const name of ['package.json', 'package-lock.json']) timestamps.push((await fs.stat(path.join(root, name))).mtimeMs)
  const result = await exportAssets({ sourceDir: path.join(root, 'build/client'), destinationDir: path.join(root, 'go/internal/frontend/dist'), newestInputMs: Math.max(...timestamps) })
  console.log(`Exported ${result.files} frontend assets for Go embedding`)
}

if (require.main === module) main().catch(() => { console.error('Go asset export failed: missing, stale or unsafe frontend assets; rebuild the client'); process.exitCode = 1 })
module.exports = { exportAssets }
