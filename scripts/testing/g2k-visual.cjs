const fs = require('node:fs/promises')
const sync = require('node:fs')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const { fixture, browser, waitFor, delay, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
function sample (pid) {
  if (process.platform === 'win32') { const v = JSON.parse(execFileSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', `Get-Process -Id ${pid} | Select-Object CPU,WorkingSet64 | ConvertTo-Json -Compress`], { windowsHide: true }).toString()); return { cpu: v.CPU, rss: v.WorkingSet64, at: performance.now() } }
  const stat = sync.readFileSync(`/proc/${pid}/stat`, 'utf8'); const fields = stat.slice(stat.lastIndexOf(')') + 2).split(' '); const hz = Number(execFileSync('getconf', ['CLK_TCK']).toString()); const rss = Number(/^VmRSS:\s+(\d+)/m.exec(sync.readFileSync(`/proc/${pid}/status`, 'utf8'))?.[1]) * 1024
  return { cpu: (Number(fields[11]) + Number(fields[12])) / hz, rss, at: performance.now() }
}
async function main () {
  const output = path.join(root, 'temp/g2k-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic field operations; no live providers/game. Idle comparison is Go fixture only with identical current UI assets, not the full Wails/WebView process tree.', pages: [], errors: [], outsideRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.outsideRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  let statsRequests = 0
  await c.call('Network.enable')
  c.on(e => { if (e.method === 'Network.webSocketFrameSent' && e.params.response.payloadData.includes('getGoEngineeredStatistics')) statsRequests++ })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(pageTextReady(text))); await evaluate('document.fonts.ready.then(()=>true)'); await delay(200) }
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2k')); origin = host.url
    for (const [name, width, height] of [['desktop', 1366, 768], ['launcher', 680, 580]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/nav/route', 'IMPORTED ITINERARY')
      await evaluate('document.querySelector(".go-field-ops details").open=true')
      const csv = path.join(output, 'fixture-' + name + '.csv'); await fs.writeFile(csv, 'System Name,Neutron,Refuel\nFixture Alpha,false,false\nFixture Beta,true,true\n')
      const doc = await c.call('DOM.getDocument'); const node = await c.call('DOM.querySelector', { nodeId: doc.root.nodeId, selector: '.go-field-ops input[type="file"]' }); await c.call('DOM.setFileInputFiles', { nodeId: node.nodeId, files: [csv] })
      await waitFor(() => evaluate('Array.from(document.querySelectorAll("button")).some(b=>b.textContent==="Import selected CSV"&&!b.disabled)'))
      await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Import selected CSV").click()')
      await waitFor(() => evaluate('document.querySelector(".go-field-metrics")?.innerText.includes("Fixture Beta")'))
      const overflow = await evaluate('document.querySelector(".go-field-ops").scrollWidth-document.querySelector(".go-field-ops").clientWidth'); if (overflow > 2) throw new Error('Field UI overflow')
      await evaluate('document.querySelector(".go-field-ops").scrollIntoView({block:"start"})'); await screenshot('itinerary-' + name)
      await navigate('/cmdr/field', 'BGS Tally'); await waitFor(() => evaluate('Array.from(document.querySelectorAll("button")).some(b=>b.textContent==="Load tally"&&!b.disabled)')); await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Load tally").click()'); await waitFor(() => evaluate('document.body.innerText.includes("UTC day (not BGS tick)")')); await screenshot('tally-' + name)
      report.pages.push({ name, width, height, overflow, csvImported: true, utcNotTick: true })
    }
    await navigate('/exploration/biologicals', 'Fixture Alpha 1'); await evaluate('document.querySelector(".go-bio-card").click()'); await waitFor(() => evaluate('document.body.innerText.includes("bioscan-fc6b85c")')); await evaluate('document.querySelector(".go-inspector .go-field-ops").scrollIntoView({block:"start"})'); await screenshot('bioscan')
    await navigate('/ship/stats', 'Ship Statistics'); await evaluate('Array.from(document.querySelectorAll(\'.go-ship-statistics details\')).find(e=>e.querySelector(\'summary\')?.textContent.includes(\'Witnessed final engineering\')).open=true'); await waitFor(() => evaluate(pageTextReady('effective-fitted-power-sum-v2'))); await screenshot('canonical-native-statistics')
    // Keep Ship Statistics open while the real sidecar monitor publishes fresh
    // Status snapshots. Wait for each exact timestamp instead of sleeping races.
    let written = ''; host.child.stdout.on('data', data => { const all = data.toString().match(/STATUS_WRITTEN=([^\r\n]+)/g); if (all) written = all[all.length - 1].slice(15) })
    const status = async deployed => {
      const previous = written; host.child.stdin.write(deployed ? 'status-deployed\n' : 'status-stowed\n')
      await waitFor(() => written && written !== previous)
      await waitFor(() => evaluate(`document.querySelector('.go-stats-evidence')?.textContent.includes(${JSON.stringify(written)})`))
      await waitFor(() => evaluate(`document.body.innerText.includes('Observed hardpoints: ${deployed ? 'deployed' : 'stowed'}')`))
    }
    await status(false)
    const requestsBefore = statsRequests; const start = sample(host.child.pid)
    const browserBefore = await c.call('Performance.getMetrics')
    for (let i = 0; i < 12; i++) await status(false)
    const finish = sample(host.child.pid); const browserAfter = await c.call('Performance.getMetrics')
    if (statsRequests !== requestsBefore) throw new Error('Volatile Status snapshots refetched static statistics')
    await status(true)
    if (statsRequests !== requestsBefore + 1) throw new Error('Deployment did not refresh observed statistics exactly once')
    const metric = (v, name) => v.metrics.find(x => x.name === name).value
    report.liveStatistics = { statusUpdates: 12, volatileStatisticsRPCs: 0, deploymentStatisticsRPCs: 1, milliseconds: finish.at - start.at, goRSSBytes: finish.rss, goCPUPercentOneCore: (finish.cpu - start.cpu) / ((finish.at - start.at) / 1000) * 100, browserHeapBytes: metric(browserAfter, 'JSHeapUsedSize'), browserMainThreadPercent: (metric(browserAfter, 'TaskDuration') - metric(browserBefore, 'TaskDuration')) / (metric(browserAfter, 'Timestamp') - metric(browserBefore, 'Timestamp')) * 100 }
    await screenshot('live-statistics-deployed')

    const idle = async () => { await delay(500); const a = sample(host.child.pid); await delay(2000); const z = sample(host.child.pid); return { rssBytes: z.rss, cpuPercentOneCore: (z.cpu - a.cpu) / ((z.at - a.at) / 1000) * 100 } }
    await host.stop(); host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2k')); origin = host.url; await navigate('/ship/status', 'SHIP')
    report.g2kIdle = await idle(); await host.stop(); host = null
    const executable = path.join(root, 'temp/g2k-exact-g2j-baseline/go/bin/g2k-baseline-fixture' + (process.platform === 'win32' ? '.exe' : ''))
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2k'), false, { executable }); origin = host.url; await navigate('/ship/status', 'SHIP'); report.g2jIdleSameAssets = await idle()
    if (report.errors.length || report.outsideRequests.length) throw new Error('Field UI error or unexpected network')
    report.passed = true
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
  console.log(JSON.stringify(report))
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
