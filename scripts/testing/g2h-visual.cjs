const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, delay } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const output = path.join(root, 'temp/g2h-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic orbital/loadout fixtures; no live providers or whole-native resource claim', pages: [], errors: [], blockedRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description || e.params.exceptionDetails.text); if (e.method === 'Fetch.requestPaused') { const p = e.params; const allowed = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!allowed) report.blockedRequests.push(p.request.url); c.call(allowed ? 'Fetch.continueRequest' : 'Fetch.failRequest', allowed ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] }); await c.call('Performance.enable')
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(`document.body.innerText.toUpperCase().includes(${JSON.stringify(text.toUpperCase())})`)); await evaluate('document.fonts.ready.then(()=>true)'); await delay(300) }
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2h'), true); origin = host.url
    for (const [viewport, width, height] of [['desktop', 1515, 992], ['laptop', 1366, 768], ['portrait', 820, 1180]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/nav/map', 'SYSTEM CARTOGRAPHY'); await waitFor(() => evaluate('document.querySelectorAll(\'.go-chart__body\').length>=18'))
      const map = await evaluate('(()=>{const m=document.querySelector(\'.go-chart\').getBoundingClientRect();const i=document.querySelector(\'.go-inspector\');return {width:m.width,height:m.height,strip:!!document.querySelector(\'.go-body-strip\'),inspectorHeight:i.clientHeight,inspectorScroll:i.scrollHeight,biology:i.innerText.includes(\'Biological observations\')}})()')
      if (map.strip || map.biology || (viewport !== 'portrait' && (map.width < map.height || map.inspectorScroll > map.inspectorHeight + 3))) throw new Error('Orbital/inspector layout: ' + JSON.stringify({ viewport, ...map }))
      await screenshot('orbital-' + viewport); report.pages.push({ page: 'orbital', viewport, ...map })
      await navigate('/nav/list', 'SYSTEM CATALOGUE'); if (await evaluate('!!document.querySelector(\'.go-body-strip\')')) throw new Error('Catalogue retained duplicate strip'); await screenshot('catalogue-' + viewport); report.pages.push({ page: 'catalogue', viewport })
      await navigate('/ship/stats', 'Maximum unladen jump range'); await waitFor(() => evaluate('Array.from(document.querySelectorAll(\'.go-stat-fact\')).find(e=>e.textContent.includes(\'Insurance rebuy\'))?.textContent.replace(/[^0-9]/g,\'\')===\'123456\'')).catch(async err => { report.failedStatistics = await evaluate('document.body.innerText'); throw err })
      if (!await evaluate('!document.body.innerText.includes(\'NaN\')&&document.body.innerText.includes(\'JOURNAL\')&&document.body.innerText.includes(\'UNAVAILABLE\')&&!document.querySelector(\'.go-flight-switches\')')) throw new Error('Stats missing evidence or duplicates Status')
      await screenshot('statistics-' + viewport); report.pages.push({ page: 'statistics', viewport })
      await navigate('/nav/galaxy', 'Galaxy Map'); await waitFor(() => evaluate('document.querySelectorAll(\'[data-marker="destination"]\').length===3'))
      if (!await evaluate('Array.from(document.querySelectorAll(\'[data-marker]\')).every(e=>e.dataset.position.split(\',\').every(v=>Number(v)>=0&&Number(v)<=1))')) throw new Error('Route not framed')
      await evaluate('Array.from(document.querySelectorAll(\'button\')).find(b=>b.textContent===\'Fit Galaxy\').click()'); await waitFor(() => evaluate('document.body.innerText.includes(\'Manual framing\')'))
      await evaluate('Array.from(document.querySelectorAll(\'button\')).find(b=>b.textContent===\'Fit Route\').click()'); await waitFor(() => evaluate('document.body.innerText.includes(\'Automatic route framing\')')); await screenshot('galaxy-' + viewport); report.pages.push({ page: 'galaxy', viewport })
    }
    await evaluate('document.querySelector(\'[aria-label="Owner settings"]\').click()'); await evaluate('document.querySelector(\'.go-data-providers\').open=true'); await waitFor(() => evaluate('document.querySelectorAll(\'.go-provider-card\').length===5'))
    if (!await evaluate('Array.from(document.querySelectorAll(\'.go-provider-card input[type="checkbox"]\')).every(e=>!e.checked)&&document.body.innerText.includes(\'application approval required\')&&document.body.innerText.includes(\'Frontier developer approval required\')')) throw new Error('Provider defaults/gates missing')
    report.providerDefaults = { newReadsOff: true, uploadsOff: true, approvalGatesVisible: true }; await screenshot('providers-defaults')
    await c.call('HeapProfiler.collectGarbage'); const metrics = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); report.renderer = { heapBytes: metrics.JSHeapUsedSize, nodes: (await c.call('Memory.getDOMCounters')).nodes }
    if (report.errors.length || report.blockedRequests.length) throw new Error('Browser error/external traffic')
    report.passed = true; console.log(JSON.stringify(report))
  } finally { await fs.writeFile(path.join(output, 'verification.json'), JSON.stringify(report, null, 2)); if (host) await host.stop(); await b.stop() }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
