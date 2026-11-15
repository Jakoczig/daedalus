// Exact accepted G2J source, isolated from the active checkout.
const fs = require('node:fs/promises')
const path = require('node:path')
const { spawnSync } = require('node:child_process')
const root = path.resolve(__dirname, '../..')
async function main () {
  const base = '2adbc86fcfc68d6a2d7822b219640a386bed738b'
  const directory = path.join(root, 'temp/g2k-exact-g2j-baseline')
  await fs.mkdir(directory, { recursive: true })
  const run = (cmd, args) => { const r = spawnSync(cmd, args, { cwd: root, encoding: 'utf8', windowsHide: true }); if (r.status !== 0) throw new Error(r.stderr || cmd + ' failed') }
  if (spawnSync('git', ['cat-file', '-e', base + '^{commit}'], { cwd: root, windowsHide: true }).status !== 0) run('git', ['fetch', '--depth=1', 'origin', base])
  run('git', ['archive', '--format=tar', '--output=temp/g2k-g2j.tar', base, 'go'])
  run('tar', ['-xf', 'temp/g2k-g2j.tar', '-C', 'temp/g2k-exact-g2j-baseline'])
  // Same assets isolate backend idle-resource comparisons from frontend changes.
  await fs.cp(path.join(root, 'go/internal/frontend/dist'), path.join(directory, 'go/internal/frontend/dist'), { recursive: true })
  console.log('Prepared exact G2J backend ' + base + '; identical 100k/1k lifecycle fixture, identical current assets for idle comparison')
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
