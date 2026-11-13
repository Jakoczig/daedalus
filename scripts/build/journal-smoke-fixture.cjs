const fs = require('node:fs/promises')
const path = require('node:path')
const os = require('node:os')

async function createJournalFixture ({ sourceDir, outputParent }) {
  const directory = await fs.mkdtemp(path.join(outputParent, 'daedalus-g2a-synthetic-journals-'))
  for (const [source, destination] of [['replay.jsonl', 'Journal.20260101.01.log'], ...['Status', 'Cargo', 'Materials', 'NavRoute'].map(kind => [`${kind}.json`, `${kind}.json`])]) {
    await fs.writeFile(path.join(directory, destination), await fs.readFile(path.join(sourceDir, source)), { mode: 0o600 })
  }
  return directory
}
async function main () {
  const directory = await createJournalFixture({ sourceDir: path.resolve(__dirname, '../../tests/testdata/g2a'), outputParent: process.env.RUNNER_TEMP || os.tmpdir() })
  if (process.env.GITHUB_OUTPUT) await fs.appendFile(process.env.GITHUB_OUTPUT, `directory=${directory}\n`)
  console.log('Prepared committed synthetic journal fixture; no game profile is read')
}
if (require.main === module) main().catch(() => { console.error('Synthetic journal fixture preparation failed'); process.exitCode = 1 })
module.exports = { createJournalFixture }
