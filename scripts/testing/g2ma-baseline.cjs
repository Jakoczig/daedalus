// Exact accepted G2L source, isolated from the active checkout.
const fs = require('node:fs/promises')
const path = require('node:path')
const { spawnSync } = require('node:child_process')
const root = path.resolve(__dirname, '../..')
async function main () {
  const base = '5f0bee9fb5df8ea243c54dadcc7edcc02581efef'
  const directory = path.join(root, 'temp/g2ma-exact-g2l-baseline')
  await fs.mkdir(directory, { recursive: true })
  const run = (cmd, args) => { const r = spawnSync(cmd, args, { cwd: root, encoding: 'utf8', windowsHide: true }); if (r.status !== 0) throw new Error(r.stderr || cmd + ' failed') }
  if (spawnSync('git', ['cat-file', '-e', base + '^{commit}'], { cwd: root, windowsHide: true }).status !== 0) run('git', ['fetch', '--depth=1', 'origin', base])
  run('git', ['archive', '--format=tar', '--output=temp/g2ma-g2l.tar', base, 'go'])
  run('tar', ['-xf', 'temp/g2ma-g2l.tar', '-C', 'temp/g2ma-exact-g2l-baseline'])
  // Same assets isolate backend idle-resource comparisons from frontend changes.
  await fs.cp(path.join(root, 'go/internal/frontend/dist'), path.join(directory, 'go/internal/frontend/dist'), { recursive: true })
  console.log('Prepared exact G2L backend ' + base + '; identical 100k/1k lifecycle fixture, identical current assets for idle comparison')
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
