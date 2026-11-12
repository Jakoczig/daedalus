const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const os = require('node:os')
const path = require('node:path')
const net = require('node:net')
const { spawn } = require('node:child_process')
const { once } = require('node:events')
const WebSocket = require('ws')

test('actual service starts with isolated synthetic data and supports native-style owner sessions', { timeout: 15000 }, async t => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'daedalus-startup-'))
  const logs = path.join(dir, 'journals')
  await fs.mkdir(logs)
  const journal = path.join(logs, 'Journal.synthetic.log')
  await fs.writeFile(journal, JSON.stringify({ timestamp: '2026-11-12T00:00:00Z', event: 'Fileheader', gameversion: '4.0', build: 'synthetic' }) + '\n')
  const prefsDir = path.join(dir, 'AppData', 'Local', 'DAEDALUS Terminal')
  await fs.mkdir(prefsDir, { recursive: true })
  await fs.writeFile(path.join(prefsDir, 'Preferences.json'), JSON.stringify({ integrations: { inaraApiKey: 'synthetic-startup-key', capiRefreshToken: 'synthetic-startup-refresh' } }))
  const probe = net.createServer()
  probe.listen(0, '127.0.0.1')
  await once(probe, 'listening')
  const port = probe.address().port
  await new Promise(resolve => probe.close(resolve))
  const child = spawn(process.execPath, ['--require', path.resolve(__dirname, '../helpers/no-external-network.cjs'), 'src/service/main.js', '--port', String(port), '--save-game-dir', logs.replace(/\\/g, '/')], {
    cwd: path.resolve(__dirname, '../..'),
    windowsHide: true,
    env: { ...process.env, USERPROFILE: dir, HOME: dir, DAEDALUS_REMOTE_PORT: '' },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  let output = ''
  child.stdout.on('data', data => { output += data })
  child.stderr.on('data', data => { output += data })
  t.after(async () => {
    if (child.exitCode === null) { child.kill(); await once(child, 'exit') }
    await fs.rm(dir, { recursive: true, force: true })
  })
  const origin = `http://localhost:${port}`
  let response
  const deadline = Date.now() + 10000
  while (Date.now() < deadline) {
    if (child.exitCode !== null) assert.fail(`Service exited: ${output}`)
    try { response = await fetch(`${origin}/auth/session`, { method: 'POST', headers: { Origin: origin } }); break } catch {}
    await new Promise(resolve => setTimeout(resolve, 25))
  }
  assert.equal(response?.status, 200, output)
  const cookie = response.headers.get('set-cookie').split(';')[0]
  const ws = new WebSocket(`ws://localhost:${port}`, { headers: { Origin: origin, Cookie: cookie } })
  await once(ws, 'open')
  t.after(() => ws.terminate())
  let sequence = 0
  const rpc = (name, message) => new Promise(resolve => {
    const requestId = String(++sequence)
    function receive (raw) {
      const data = JSON.parse(raw)
      if (data.requestId !== requestId) return
      ws.off('message', receive)
      resolve(data)
    }
    ws.on('message', receive)
    ws.send(JSON.stringify({ requestId, name, message }))
  })
  assert.equal(JSON.stringify((await rpc('getPreferences')).message).includes('synthetic-startup'), false)
  let loaded = false
  while (Date.now() < deadline) {
    loaded = (await rpc('getLoadingStatus')).message?.loadingComplete
    if (loaded) break
    await new Promise(resolve => setTimeout(resolve, 25))
  }
  assert.equal(loaded, true, output)
  assert.equal((await rpc('getMiningStatus')).error, undefined)
  assert.equal((await rpc('getMaterials')).error, undefined)
  assert.equal((await rpc('getNavRoute')).error, undefined)
  const streamed = new Promise(resolve => {
    function receive (raw) {
      const data = JSON.parse(raw)
      if (data.name !== 'newLogEntry' || data.message?.event !== 'LoadGame') return
      ws.off('message', receive)
      resolve(data.message)
    }
    ws.on('message', receive)
  })
  await fs.appendFile(journal, JSON.stringify({ timestamp: '2026-11-12T00:00:01Z', event: 'LoadGame', Commander: 'Synthetic Commander', Credits: 1000 }) + '\n')
  assert.equal((await streamed).Commander, 'Synthetic Commander')
  await rpc('setPreferences', { covasExtendedEnabled: false })
  const saved = JSON.parse(await fs.readFile(path.join(prefsDir, 'Preferences.json'), 'utf8'))
  assert.equal(saved.integrations.capiRefreshToken, 'synthetic-startup-refresh')
  assert.equal(output.includes('synthetic-startup'), false)
})
