const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, delay, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const output = path.join(root, 'temp/g2j-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic private SQLite, owner controls and engineered evidence; no live API traffic', errors: [], blockedRequests: [], pages: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description || e.params.exceptionDetails.text); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.blockedRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] }); await c.call('Performance.enable')
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(pageTextReady(text))); await evaluate('document.fonts.ready.then(()=>true)'); await delay(200) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2e')); origin = host.url
    for (const [name, width, height] of [['desktop', 1366, 768], ['launcher', 680, 580]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/ship/status', 'POWER DISTRIBUTION')
      await evaluate('document.querySelector(\'[aria-label="Owner settings"]\').click()')
      await evaluate('Array.from(document.querySelectorAll("summary")).find(s=>s.textContent==="Advanced local tools").parentElement.open=true')
      await waitFor(() => evaluate('!!document.querySelector(".go-data-history")'))
      await evaluate('document.querySelector(".go-data-history").open=true')
      await waitFor(() => evaluate('document.querySelector(".go-history-health")?.innerText.includes("verified")&&!!document.querySelector(".go-history-rows details")'))
      await waitFor(() => evaluate('document.querySelector(".go-history-workspace").textContent.includes("WAITING_FOR_GAME")&&document.querySelector(".go-history-workspace").textContent.includes("Last-known cached data")')).catch(async err => { report.failedSource = await evaluate('document.querySelector(".go-history-workspace").textContent'); throw err })
      await evaluate('document.querySelector(".go-data-history").scrollIntoView({block:"start"})')
      const overflow = await evaluate('document.querySelector(".go-history-workspace").scrollWidth-document.querySelector(".go-history-workspace").clientWidth')
      if (overflow > 2) throw new Error('History workspace overflows horizontally')
      await screenshot('history-' + name)
      await evaluate('Array.from(document.querySelectorAll(".go-history-workspace details")).find(d=>d.querySelector("summary")?.textContent.includes("Exploration publication policy")).open=true')
      if (!await evaluate('document.querySelector(".go-history-workspace").innerText.includes("No automatic sale-triggered uploads")&&Array.from(document.querySelectorAll("option")).some(o=>o.value==="After sale"&&o.textContent.includes("hold + manual review only"))')) throw new Error('After-sale manual-only disclosure missing')
      await evaluate('document.querySelector(\'select[aria-label="Exploration publication timing"]\').scrollIntoView({block:"center"})')
      await screenshot('publication-policy-' + name)
      await evaluate('Array.from(document.querySelectorAll(".go-history-workspace details")).find(d=>d.querySelector("summary")?.textContent.includes("Historical publication")).open=true')
      await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Preview eligibility and exact fields").click()')
      await waitFor(() => evaluate('document.querySelector(".go-history-workspace").innerText.includes("At most 32 committed records per review")'))
      if (await evaluate('Array.from(document.querySelectorAll("button")).some(b=>b.textContent==="Authorize reviewed eligible records once"&&!b.disabled)')) throw new Error('Unconsented historical upload became available')
      await screenshot('private-review-' + name)
      report.pages.push({ name, width, height, overflow, historyVisible: true, cachedStatus: true, publicationDisabled: true })
    }
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/ship/stats', 'Witnessed final engineering')
    await waitFor(() => evaluate('document.body.textContent.includes("effective-fitted-power-sum-v2")')).catch(async err => { report.failedStatistics = await evaluate('document.body.textContent'); throw err })
    if (await evaluate('document.body.innerText.includes("NaN")')) throw new Error('Native statistics contains NaN')
    await screenshot('native-effective-statistics')
    await c.call('HeapProfiler.collectGarbage')
    const metrics = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); report.rendererHeapBytes = metrics.JSHeapUsedSize
    if (report.errors.length || report.blockedRequests.length) throw new Error('Runtime errors or unexpected network attempted')
    report.passed = true
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
  console.log(JSON.stringify(report))
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
