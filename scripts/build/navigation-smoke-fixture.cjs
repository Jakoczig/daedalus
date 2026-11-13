const fs = require('node:fs/promises')
const path = require('node:path')
const os = require('node:os')

async function createNavigationFixture ({ sourceDir, outputParent }) {
  const directory = await fs.mkdtemp(path.join(outputParent, 'daedalus-g2b-synthetic-navigation-'))
  for (const [source, destination] of [['replay.jsonl', 'Journal.20260101.01.log'], ['NavRoute.json', 'NavRoute.json']]) {
    await fs.writeFile(path.join(directory, destination), await fs.readFile(path.join(sourceDir, source)), { mode: 0o600 })
  }
  return directory
}
async function main () {
  const directory = await createNavigationFixture({ sourceDir: path.resolve(__dirname, process.argv.includes('--g2c') ? '../../tests/testdata/g2c' : '../../tests/testdata/g2b'), outputParent: process.env.RUNNER_TEMP || os.tmpdir() })
  if (process.env.GITHUB_OUTPUT) await fs.appendFile(process.env.GITHUB_OUTPUT, `directory=${directory}\n`)
  console.log('Prepared fabricated local route/exploration fixture; no game profile is read')
}
if (require.main === module) main().catch(() => { console.error('Synthetic navigation fixture preparation failed'); process.exitCode = 1 })
module.exports = { createNavigationFixture }
