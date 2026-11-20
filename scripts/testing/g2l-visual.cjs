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
  const output = path.join(root, 'temp/g2l-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic Go HTTP/journal service plus real Chromium. Not whole Wails/WebView or physical gameplay. No external provider traffic.', pages: [], errors: [], outsideRequests: [], resources: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''; let eventsRead = 0
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  await c.call('Network.enable')
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Network.webSocketFrameReceived') { const count = /"eventsRead":(\d+)/.exec(e.params.response.payloadData); if (count) eventsRead = Math.max(eventsRead, Number(count[1])) }; if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.outsideRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(pageTextReady(text))); await evaluate('document.fonts.ready.then(()=>true)') }
  const measure = async name => { const a = sample(host.child.pid); const first = await c.call('Performance.getMetrics'); await delay(3200); const z = sample(host.child.pid); const last = await c.call('Performance.getMetrics'); const m = (v, n) => v.metrics.find(x => x.name === n).value; report.resources.push({ name, milliseconds: z.at - a.at, goRSSBytes: z.rss, goCPUPercentOneCore: (z.cpu - a.cpu) / ((z.at - a.at) / 1000) * 100, browserHeapBytes: m(last, 'JSHeapUsedSize'), browserMainThreadPercent: (m(last, 'TaskDuration') - m(first, 'TaskDuration')) / (m(last, 'Timestamp') - m(first, 'Timestamp')) * 100 }) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2l')); origin = host.url
    const pages = [['/ship/fleet', 'Fixture Explorer'], ['/ship/carrier', 'Owned carrier operations'], ['/trade/commodities', 'Station commodity observations'], ['/trade/routes', 'Two-market quoted comparison'], ['/mining/statistics', 'platinum'], ['/cmdr/community-goals', 'Received reward'], ['/missions/powerplay', 'PowerplayDeliver'], ['/cmdr/wing', 'CrewAssign'], ['/trade/ptn', 'External community reference'], ['/trade/fcoc', 'External community reference']]
    for (const [name, width, height] of [['desktop', 1366, 768], ['launcher', 680, 580]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      for (const [url, text] of pages) {
        await navigate(url, text)
        const overflow = await evaluate('document.querySelector(".go-page").scrollWidth-document.querySelector(".go-page").clientWidth'); if (overflow > 2) throw new Error('Operations UI overflow: ' + url + ' ' + overflow)
        if (url === '/ship/fleet') { await evaluate('Array.from(document.querySelectorAll("summary")).find(e=>e.textContent.includes("Fixture Explorer")).parentElement.open=true'); if (!await evaluate('document.body.innerText.includes("9007199254740993")')) throw new Error('Exact fleet identity lost') }
        const shot = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + url.replaceAll('/', '-') + '.png'), Buffer.from(shot.data, 'base64'))
        report.pages.push({ name, url, overflow })
      }
    }
    await navigate('/cmdr/community-goals', '2 subscribed')
    host.child.stdin.write('operations-goals-empty\n')
    await waitFor(() => evaluate(pageTextReady('0 subscribed')))
    await evaluate('Array.from(document.querySelectorAll("details")).find(e=>e.querySelector("summary")?.textContent==="Past participation & rewards").open=true')
    if (!await evaluate('/Reward\\s+1[,\\s]?000 CR/.test(document.querySelector(".go-goals").innerText)&&document.querySelector(".go-goals").innerText.includes("Reward")&&!Array.from(document.querySelectorAll(".go-goal-card .go-badge")).some(e=>e.textContent==="SUBSCRIBED")')) throw new Error('Empty complete CG snapshot lost history or left removed goals current: ' + await evaluate('document.querySelector(".go-goals").innerText'))
    await navigate('/cmdr/community-goals', '0 subscribed')
    report.completeGoalSnapshotRetiresMembershipAndKeepsHistory = true
    await navigate('/ship/fleet', 'Fixture Explorer'); await measure('G2L open Fleet')
    const priorEvents = eventsRead; host.child.stdin.write('operations-dock\n'); await waitFor(() => eventsRead >= priorEvents + 3); host.child.stdin.write('operations-market\n')
    await navigate('/trade/commodities', 'Station commodity observations'); await waitFor(() => evaluate('Array.from(document.querySelectorAll("summary")).some(e=>e.textContent.includes("gold"))'))
    await evaluate('Array.from(document.querySelectorAll("summary")).find(e=>e.textContent.includes("gold")).parentElement.open=true')
    if (!await evaluate('document.body.innerText.includes("100")&&document.body.innerText.includes("journal-sidecar")')) throw new Error('Upload-disabled local market not visible')
    report.localMarketWithoutUpload = true; await measure('G2L open Trade')
    await c.call('Page.navigate', { url: origin + '/trade/commodities' }); await waitFor(() => evaluate(pageTextReady('gold'))); report.marketSurvivesPageReload = true
    await host.stop(); host = null
    const executable = path.join(root, 'temp/g2l-exact-g2k-baseline/go/bin/g2l-baseline-fixture' + (process.platform === 'win32' ? '.exe' : ''))
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2l'), false, { executable }); origin = host.url; await navigate('/ship/status', 'SHIP'); await measure('G2K core baseline, same assets; no Fleet/Trade implementation')
    if (report.errors.length || report.outsideRequests.length) throw new Error('Operations browser error or unexpected network')
    report.passed = true
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
  console.log(JSON.stringify(report))
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
