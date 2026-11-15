// Compare exactly the same synthetic lifecycle fixture against the accepted G2H
// source. The archive is disposable and never changes the active checkout.
const fs = require('node:fs/promises')
const path = require('node:path')
const { spawnSync } = require('node:child_process')
const root = path.resolve(__dirname, '../..')
const base = 'd7c5cd93e16ee0480fa11ef53d2ea9d9d0d4cb58'
async function main () {
  const directory = path.join(root, 'temp/g2j-exact-g2h-baseline')
  // Reuse an existing archive directory rather than deleting arbitrary paths.
  await fs.mkdir(directory, { recursive: true })
  const archive = path.join(root, 'temp/g2j-exact-g2h-baseline.tar')
  const run = (cmd, args) => { const result = spawnSync(cmd, args, { cwd: root, encoding: 'utf8', windowsHide: true }); if (result.status !== 0) throw new Error(result.stderr || `${cmd} failed`) }
  const known = spawnSync('git', ['cat-file', '-e', base + '^{commit}'], { cwd: root, windowsHide: true })
  if (known.status !== 0) run('git', ['fetch', '--depth=1', 'origin', base])
  run('git', ['archive', '--format=tar', '--output=' + archive, base, 'go'])
  // GNU tar treats a Windows drive prefix as a remote host. Relative POSIX
  // paths work with both Git Bash GNU tar and Windows/Linux native tar.
  run('tar', ['-xf', path.relative(root, archive).split(path.sep).join('/'), '-C', path.relative(root, directory).split(path.sep).join('/')])
  // Expose G2H's existing ReaderStats byte counter only; retain its reader and
  // reducer algorithms unchanged. No persistent storage is added to the baseline.
  const servicePath = path.join(directory, 'go/internal/gamestate/service.go')
  const service = (await fs.readFile(servicePath, 'utf8')).replace('type SourceDetails struct {', 'type SourceDetails struct {\nReaderBytes uint64').replace('stats := reader.Stats()', 'stats := reader.Stats(); s.details.ReaderBytes=stats.BytesRead')
  await fs.writeFile(servicePath, service)
  const source = await fs.readFile(path.join(root, 'go/internal/gamestate/persistence_test.go'), 'utf8')
  let benchmark = source.slice(source.indexOf('func BenchmarkG2JLifecycle'))
  benchmark = benchmark.replace('BenchmarkG2JLifecycle', 'BenchmarkG2HExactLifecycle').replace(/\s*profile := b\.TempDir\(\)\s*if os\.Getenv\("G2J_BASELINE"\) == "1" \{\s*profile = ""\s*\}/, '').replaceAll('StorageDir: profile, ', '').replaceAll('v.Stage == "DB_ERROR"', 'false').replaceAll('v.Cached', 'false')
  const header = 'package gamestate\nimport("context";"fmt";"os";"path/filepath";"runtime";"testing";"time")\n'
  await fs.writeFile(path.join(directory, 'go/internal/gamestate/g2j_baseline_test.go'), header + benchmark)
  for (const name of ['memory_windows_test.go', 'memory_linux_test.go']) await fs.copyFile(path.join(root, 'go/internal/gamestate', name), path.join(directory, 'go/internal/gamestate', name))
  console.log('Prepared exact G2H baseline ' + base + ' in temp/g2j-exact-g2h-baseline/go; same 100k/1k fixture, no persistent cache')
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
