const { spawn } = require('child_process')

// Metadata reads must not inherit a user's download/exec configuration or write
// video files. Limit work from paired displays; transport/downloads use other RPCs.
function createMetadataRunner ({ spawnProcess = spawn, maxBytes = 8 * 1024 * 1024 } = {}) {
  let active = 0
  return (binary, args, options) => {
    if (active >= 2) throw new Error('Metadata service busy')
    active++
    let proc
    try {
      proc = spawnProcess(binary, ['--ignore-config', '--simulate', '--no-cache-dir', ...args.filter(arg => arg !== '--no-check-certificates')], { ...options, timeout: 20000 })
    } catch (e) { active--; throw e }
    proc.once('close', () => { active-- })
    let size = 0
    let limited = false
    const count = data => {
      size += data.length
      if (size > maxBytes && !limited) {
        limited = true
        proc.kill()
        proc.emit('error', new Error('Metadata output limit exceeded'))
      }
    }
    proc.stdout.on('data', count)
    proc.stderr.on('data', count)
    return proc
  }
}

module.exports = { createMetadataRunner, spawnMetadata: createMetadataRunner() }
