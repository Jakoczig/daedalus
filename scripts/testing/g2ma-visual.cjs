const fs = require('node:fs/promises')
const path = require('node:path')
const sync = require('node:fs')
const { execFileSync } = require('node:child_process')
const { fixture, browser, waitFor, delay, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
function sample (pid) {
  if (process.platform === 'win32') { const v = JSON.parse(execFileSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', `Get-Process -Id ${pid} | Select-Object CPU,WorkingSet64 | ConvertTo-Json -Compress`], { windowsHide: true }).toString()); return { cpu: v.CPU, rss: v.WorkingSet64, at: performance.now() } }
  const stat = sync.readFileSync(`/proc/${pid}/stat`, 'utf8'); const fields = stat.slice(stat.lastIndexOf(')') + 2).split(' '); const hz = Number(execFileSync('getconf', ['CLK_TCK']).toString()); const rss = Number(/^VmRSS:\s+(\d+)/m.exec(sync.readFileSync(`/proc/${pid}/status`, 'utf8'))?.[1]) * 1024
  return { cpu: (Number(fields[11]) + Number(fields[12])) / hz, rss, at: performance.now() }
}
async function main () {
  const output = path.join(root, 'temp/g2ma-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Fabricated local journal and real Chromium; no live account/provider authorization', pages: [], errors: [], outsideRequests: [], resources: [], baselineCommit: '5f0bee9fb5df8ea243c54dadcc7edcc02581efef', head: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root }).toString().trim() }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.outsideRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(`location.origin === ${JSON.stringify(origin)} && ${pageTextReady(text, true)}`)); await evaluate('document.fonts.ready.then(()=>true)') }
  const measure = async name => { await c.call('HeapProfiler.collectGarbage'); const a = sample(host.child.pid); const first = await c.call('Performance.getMetrics'); await delay(3200); const z = sample(host.child.pid); const last = await c.call('Performance.getMetrics'); const m = (v, n) => v.metrics.find(x => x.name === n).value; report.resources.push({ name, milliseconds: z.at - a.at, goRSSBytes: z.rss, goCPUPercentOneCore: (z.cpu - a.cpu) / ((z.at - a.at) / 1000) * 100, browserHeapBytes: m(last, 'JSHeapUsedSize'), browserMainThreadPercent: (m(last, 'TaskDuration') - m(first, 'TaskDuration')) / (m(last, 'Timestamp') - m(first, 'Timestamp')) * 100 }) }
  const rpc = (name, message = {}) => evaluate(`new Promise((resolve,reject)=>{const w=new WebSocket(location.origin.replace('http:','ws:'));const timer=setTimeout(()=>{w.close();reject(new Error('owner RPC timed out'))},5000);w.onopen=()=>w.send(JSON.stringify({requestId:'g2ma',name:${JSON.stringify(name)},message:${JSON.stringify(message)}}));w.onmessage=e=>{const r=JSON.parse(e.data);if(r.requestId==='g2ma'){clearTimeout(timer);w.close();r.error?reject(new Error(r.error)):resolve(r.message)}};w.onerror=()=>{clearTimeout(timer);reject(new Error('owner socket failed'))}})`)
  try {
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2l')); origin = host.url
    await navigate('/ship/status', 'SHIP')
    await waitFor(async () => !(await rpc('getJournalSummary')).importInProgress)
    await measure('G2M-A fresh common Ship Status')
    const pages = [['/cmdr/overview', 'PRIVATE_SYNTHETIC'], ['/cmdr/community-goals', 'Fixture Community Goal'], ['/ship/fleet', 'Fixture Explorer'], ['/ship/modules', 'Ship internals'], ['/ship/stats', 'Ship Statistics']]
    for (const [name, width, height] of [['desktop', 1366, 768], ['compact', 680, 580]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      for (const [url, text] of pages) {
        await navigate(url, text)
        if (url === '/ship/fleet') { await evaluate('document.querySelector(".go-fleet-card").open=true'); await waitFor(() => evaluate('document.body.textContent.includes("9007199254740993")')) }
        const overflow = await evaluate('document.querySelector(".go-page").scrollWidth-document.querySelector(".go-page").clientWidth'); if (overflow > 2) throw new Error('UI overflow ' + url + ': ' + overflow)
        if (await evaluate('document.body.innerText.includes("NaN")')) throw new Error('Invalid displayed calculation')
        const shot = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + url.replaceAll('/', '-') + '.png'), Buffer.from(shot.data, 'base64')); report.pages.push({ name, url, overflow })
      }
    }
    const accounts = await rpc('getGoAccounts'); if (accounts.settings.multiple) throw new Error('Multiple accounts enabled by default')
    await rpc('setGoAccounts', { multiple: true, selected: 'SYNTHETIC-G2B' })
    host.child.stdin.write('accounts-other-session\n')
    await waitFor(async () => (await rpc('getGoAccounts')).cards.some(c => c.fid === 'SYNTHETIC-OTHER'))
    const selected = await rpc('getGoCommanderOverview'); if (selected.name !== 'PRIVATE_SYNTHETIC' || !selected.journalMismatch) throw new Error('Journal silently changed chosen commander')
    try { await rpc('getGoOperations', { kind: 'fleet', fid: 'SYNTHETIC-OTHER' }); throw new Error('Foreign account query accepted') } catch (e) { if (e.message === 'Foreign account query accepted') throw e }
    try { await rpc('getGoBGSTally', { fid: 'SYNTHETIC-OTHER' }); throw new Error('Foreign tally query accepted') } catch (e) { if (e.message === 'Foreign tally query accepted') throw e }
    await rpc('setGoAccounts', { multiple: true, selected: 'SYNTHETIC-OTHER' }); const other = await rpc('getGoCommanderOverview'); if (other.name !== 'Other Fixture Commander') throw new Error('Explicit profile switch failed')
    await navigate('/cmdr/overview', 'Other Fixture Commander'); if (await evaluate('document.body.innerText.includes("PRIVATE_SYNTHETIC")')) throw new Error('Previous identity remained visible')
    report.globalAccountIsolation = true
    await host.stop(); host = null
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2l'), false, { frontier: true }); origin = host.url
    await navigate('/ship/status', 'SHIP'); await waitFor(async () => !(await rpc('getJournalSummary')).importInProgress)
    const tick = () => new Promise(resolve => { const onData = data => { if (data.toString().includes('FRONTIER_TICKED')) { host.child.stdout.off('data', onData); resolve() } }; host.child.stdout.on('data', onData); host.child.stdin.write('frontier-tick\n') })
    await rpc('configureGoProvider', { id: 'frontier', config: { read: true } })
    await rpc('readGoProvider', { id: 'frontier', kind: 'profile' })
    const membershipBefore = JSON.stringify((await rpc('getGoOperations', { kind: 'goals' })).goalMembership)
    const fleetBeforeStock = (await rpc('getGoOperations', { kind: 'fleet' })).rows.map(r => r.entity).sort().join(',')
    for (const kind of ['communitygoals', 'market', 'shipyard', 'fleetcarrier']) { await tick(); await rpc('readGoProvider', { id: 'frontier', kind }) }
    if ((await rpc('getGoOperations', { kind: 'fleet' })).rows.map(r => r.entity).sort().join(',') !== fleetBeforeStock) throw new Error('Station stock became owned fleet')
    if (JSON.stringify((await rpc('getGoOperations', { kind: 'goals' })).goalMembership) !== membershipBefore) throw new Error('Global CG response changed journal subscription')
    report.frontierPages = []
    for (const [name, width, height] of [['desktop', 1366, 768], ['compact', 680, 580]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      for (const [url, text] of [['/ship/fleet', 'Frontier owned-fleet reference'], ['/cmdr/community-goals', 'Frontier · cached'], ['/trade/commodities', 'Frontier market reference']]) {
        await navigate(url, text)
        if (url === '/ship/fleet') await evaluate('document.querySelectorAll(".go-fleet-card").forEach(e=>{e.open=true})')
        const overflow = await evaluate('document.querySelector(".go-page").scrollWidth-document.querySelector(".go-page").clientWidth'); if (overflow > 2) throw new Error('Frontier reference overflow ' + url + ': ' + overflow)
        if (await evaluate('document.body.innerText.includes("SYNTHETIC-ACCESS")||document.body.innerText.includes("SYNTHETIC-REFRESH")')) throw new Error('Credential reached reference UI')
        const shot = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, 'frontier-' + name + '-' + url.replaceAll('/', '-') + '.png'), Buffer.from(shot.data, 'base64'))
        report.frontierPages.push({ name, url, overflow })
      }
    }
    report.frontierReferenceIsolation = true
    await host.stop(); host = null
    const executable = path.join(root, 'temp/g2ma-exact-g2l-baseline/go/bin/g2ma-baseline-fixture' + (process.platform === 'win32' ? '.exe' : ''))
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2l'), false, { executable }); origin = host.url
    await navigate('/ship/status', 'SHIP'); await waitFor(async () => !(await rpc('getJournalSummary')).importInProgress); await measure('Exact G2L fresh common Ship Status, identical current assets')
    await delay(300); report.measurementMethod = 'Fresh fixture per backend at 1366×768; identical Ship Status assets and fabricated journals; no prior private-page queries; explicit renderer GC; one 3.2-second idle sample each. Not whole Wails or a statistical performance claim.'
    if (report.errors.length || report.outsideRequests.length) throw new Error('Runtime error or unexpected network')
    report.passed = true
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
  console.log(JSON.stringify(report))
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
