const fs = require('node:fs/promises')
const path = require('node:path')
const os = require('node:os')
const crypto = require('node:crypto')

async function regularBytes (filename) {
  const info = await fs.lstat(filename)
  if (!info.isFile() || info.isSymbolicLink() || info.size === 0) throw new Error('Preview inputs must be nonempty regular files')
  return fs.readFile(filename)
}

async function packagePreview ({ binaryPath, readmePath, outputParent, platform, milestone = 'g2b' }) {
  if (!['g2b', 'g2c', 'g2d', 'g2e', 'g2f', 'g2g', 'g2h', 'g2j', 'g2k', 'g2l', 'g2m-a', 'g2m-b', 'g2m-c', 'g2n', 'g2o'].includes(milestone)) throw new Error('Unsupported preview milestone')
  if (!['Windows', 'Linux'].includes(platform)) throw new Error('Unsupported preview platform')
  // Read explicit inputs only. Never enumerate/copy a build directory or profile.
  const binary = await regularBytes(binaryPath)
  const readme = await regularBytes(readmePath)
  const descriptor = ['g2h', 'g2j', 'g2k', 'g2l', 'g2m-a', 'g2m-b', 'g2m-c', 'g2n', 'g2o'].includes(milestone) ? 'preview' : 'local-route-survey-no-uploads'
  const directory = await fs.mkdtemp(path.join(outputParent, `${['g2m-a', 'g2m-b', 'g2m-c', 'g2n', 'g2o'].includes(milestone) ? 'eridion' : 'daedalus'}-${milestone}-experimental-unsigned-${descriptor}-`))
  const executable = ['g2m-a', 'g2m-b', 'g2m-c', 'g2n', 'g2o'].includes(milestone) ? 'eridion' : 'daedalus'
  const files = { [executable + (platform === 'Windows' ? '.exe' : '')]: binary, 'README.md': readme }
  const hashes = []
  for (const [name, bytes] of Object.entries(files)) {
    await fs.writeFile(path.join(directory, name), bytes)
    hashes.push(`${crypto.createHash('sha256').update(bytes).digest('hex')}  ${name}\n`)
  }
  await fs.writeFile(path.join(directory, 'SHA256SUMS'), hashes.join(''))
  return directory
}

async function main () {
  const root = path.resolve(__dirname, '../..')
  const platform = process.env.RUNNER_OS || (process.platform === 'win32' ? 'Windows' : 'Linux')
  const directory = await packagePreview({
    binaryPath: path.join(root, 'go/bin', platform === 'Windows' ? 'daedalus.exe' : 'daedalus'),
    readmePath: path.join(root, 'docs/testing/g2o-signed-atlas-preview.md'),
    outputParent: process.env.RUNNER_TEMP || os.tmpdir(),
    platform,
    milestone: 'g2o'
  })
  if (process.env.GITHUB_OUTPUT) await fs.appendFile(process.env.GITHUB_OUTPUT, `directory=${directory}\n`)
  console.log(`Packaged experimental, unsigned, local-first preview (optional providers; uploads require explicit consent) (${platform}); executable, README and hashes only`)
}
if (require.main === module) main().catch(() => { console.error('Experimental preview packaging failed'); process.exitCode = 1 })
module.exports = { packagePreview }
