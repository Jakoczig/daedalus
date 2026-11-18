const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, delay, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const output = path.join(root, 'temp/g2h-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic orbital/loadout fixtures; no live providers or whole-native resource claim', pages: [], errors: [], blockedRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description || e.params.exceptionDetails.text); if (e.method === 'Fetch.requestPaused') { const p = e.params; const allowed = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!allowed) report.blockedRequests.push(p.request.url); c.call(allowed ? 'Fetch.continueRequest' : 'Fetch.failRequest', allowed ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] }); await c.call('Performance.enable')
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(pageTextReady(text))); await evaluate('document.fonts.ready.then(()=>true)'); await delay(300) }
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2h'), false, { deferJournal: true }); origin = host.url
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/nav/map', 'Current system not available yet')
    if (await evaluate('!!document.querySelector(".go-chart,.go-inspector,.go-toolbar")')) throw new Error('Startup rendered empty chart or filters')
    await screenshot('journal-waiting')
    host.child.stdin.write('empty-system\n')
    await waitFor(() => evaluate('document.body.textContent.includes("Fixture Empty")&&document.body.textContent.includes("No body observations available")')).catch(async err => { report.failedStartup = await evaluate('document.body.innerText'); throw err })
    if (await evaluate('!!document.querySelector(".go-chart__body,.go-inspector")||document.body.innerText.includes("No matching observations")')) throw new Error('Valid empty system confused with filters')
    await screenshot('known-empty-system')
    host.child.stdin.write('load\n')
    await waitFor(() => evaluate('document.querySelectorAll(".go-chart__body").length>=18'))
    report.startup = { waiting: true, knownEmpty: true, automaticLiveTransition: true }
    await host.stop(); host = null
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2h'), true); origin = host.url
    for (const [viewport, width, height] of [['desktop', 1515, 992], ['laptop', 1366, 768], ['portrait', 820, 1180]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/nav/map', 'SYSTEM CARTOGRAPHY'); await waitFor(() => evaluate('document.querySelectorAll(\'.go-chart__body\').length>=18'))
      const map = await evaluate('(()=>{const m=document.querySelector(\'.go-chart\').getBoundingClientRect();const i=document.querySelector(\'.go-inspector\');return {width:m.width,height:m.height,strip:!!document.querySelector(\'.go-body-strip\'),inspectorHeight:i.clientHeight,inspectorScroll:i.scrollHeight,inspectorWidth:i.clientWidth,inspectorScrollWidth:i.scrollWidth,biology:i.innerText.includes(\'Biological observations\')}})()')
      if (map.strip || map.biology || map.inspectorScrollWidth > map.inspectorWidth + 2 || (viewport !== 'portrait' && (map.width < map.height || map.inspectorScroll > map.inspectorHeight + 3))) throw new Error('Orbital/inspector layout: ' + JSON.stringify({ viewport, ...map }))
      await screenshot('orbital-' + viewport); report.pages.push({ page: 'orbital', viewport, ...map })
      await evaluate("document.querySelector('[aria-label=\"Zoom in\"]').click()")
      const pan = await evaluate('(()=>{const v=document.querySelector(".go-chart__viewport");v.scrollLeft=100;v.scrollTop=100;return {x:v.scrollLeft,y:v.scrollTop,overflow:getComputedStyle(v).overflowX}})()')
      if (pan.x < 50 || pan.y < 50 || pan.overflow !== 'auto') throw new Error('Zoomed orbital viewport cannot pan: ' + JSON.stringify(pan))
      await evaluate('Array.from(document.querySelectorAll(".go-chart__tools button")).find(b=>b.textContent==="Fit").click()')

      await waitFor(() => evaluate('document.querySelectorAll(".go-chart__station-marker").length===6'))
      const stationDock = await evaluate('(()=>{const d=document.querySelector(".go-chart__stations").getBoundingClientRect(),v=document.querySelector(".go-chart__viewport").getBoundingClientRect();return {top:d.top,viewportBottom:v.bottom,height:d.height}})()')
      if (stationDock.top < stationDock.viewportBottom) throw new Error('Station dock obscures orbital viewport')
      await evaluate('document.querySelector(".go-chart__station-more").click()')
      await waitFor(() => evaluate('document.querySelectorAll(".go-chart__station-marker").length===64'))
      await evaluate('(()=>{const b=document.querySelectorAll(".go-chart__station-marker");b[b.length-1].focus()})()')
      await c.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 })
      await c.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 })
      await waitFor(() => evaluate('document.querySelector(".go-station-inspector")?.textContent.includes("Fixture Station 63")')).catch(async err => { report.failedStation = await evaluate('({focus:document.activeElement.outerHTML,inspector:document.querySelector(".go-inspector")?.textContent,pressed:Array.from(document.querySelectorAll(".go-chart__station-marker[aria-pressed=true]")).map(e=>e.textContent)})'); await screenshot('failed-station'); throw err })
      if (!await evaluate('(()=>{const t=document.querySelector(".go-station-inspector").innerText;return t.includes("EDSM community reported")&&t.includes("Stations fetched")&&t.includes("Cache expires")&&t.includes("—")})()')) throw new Error('Station inspector lacks source/freshness/unknown data')
      await screenshot('stations-' + viewport); report.pages.push({ page: 'stations', viewport, count: 64, keyboardSelection: true, ...stationDock })
      await evaluate('document.querySelector(\'[aria-label="Inspect Fixture Alpha"]\').dispatchEvent(new MouseEvent(\'click\',{bubbles:true}))')
      await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Body inspector"]\')&&!document.querySelector(\'.go-station-inspector\')'))

      await navigate('/nav/list', 'SYSTEM CATALOGUE'); if (await evaluate('!!document.querySelector(\'.go-body-strip\')')) throw new Error('Catalogue retained duplicate strip'); await screenshot('catalogue-' + viewport); report.pages.push({ page: 'catalogue', viewport })
      await navigate('/ship/stats', 'Maximum unladen jump range'); await waitFor(() => evaluate('Array.from(document.querySelectorAll(\'.go-stat-fact\')).find(e=>e.textContent.includes(\'Insurance rebuy\'))?.textContent.replace(/[^0-9]/g,\'\')===\'123456\'')).catch(async err => { report.failedStatistics = await evaluate('document.body?.innerText || \'\''); throw err })
      if (!await evaluate('!document.body.innerText.includes(\'NaN\')&&document.body.innerText.includes(\'JOURNAL\')&&document.body.innerText.includes(\'UNAVAILABLE\')&&!document.querySelector(\'.go-flight-switches\')')) throw new Error('Stats missing evidence or duplicates Status')
      await screenshot('statistics-' + viewport); report.pages.push({ page: 'statistics', viewport })
      await navigate('/nav/galaxy', 'Galaxy Map'); await waitFor(() => evaluate('document.querySelectorAll(\'[data-marker="destination"]\').length===3'))
      if (!await evaluate('Array.from(document.querySelectorAll(\'[data-marker]\')).every(e=>e.dataset.position.split(\',\').every(v=>Number(v)>=0&&Number(v)<=1))')) throw new Error('Route not framed')
      await evaluate('Array.from(document.querySelectorAll(\'button\')).find(b=>b.textContent===\'Fit Galaxy\').click()'); await waitFor(() => evaluate('document.body.innerText.includes(\'Manual framing\')'))
      await evaluate('Array.from(document.querySelectorAll(\'button\')).find(b=>b.textContent===\'Fit Route\').click()'); await waitFor(() => evaluate('document.body.innerText.includes(\'Automatic route framing\')')); await screenshot('galaxy-' + viewport); report.pages.push({ page: 'galaxy', viewport })
    }
    await evaluate('document.querySelector(\'[aria-label="Owner settings"]\').click()'); await evaluate('document.querySelector(\'.go-data-providers\').open=true'); await waitFor(() => evaluate('document.querySelectorAll(\'.go-provider-card\').length===10'))
    if (!await evaluate('Array.from(document.querySelectorAll(\'.go-provider-card input[type="checkbox"]\')).every(e=>!e.checked)&&document.body.innerText.includes(\'application approval required\')&&(()=>{const card=Array.from(document.querySelectorAll(\'.go-provider-card\')).find(e=>e.querySelector(\'h3\')?.textContent.includes(\'FRONTIER\'));return !!card&&!card.textContent.includes(\'Frontier developer approval required\')&&Array.from(card.querySelectorAll(\'button\')).find(e=>e.textContent.includes(\'Connect in system browser\'))?.disabled})()')) throw new Error('Provider defaults/gates missing')
    report.providerDefaults = { newReadsOff: true, uploadsOff: true, approvalGatesVisible: true }; await screenshot('providers-defaults')
    await c.call('HeapProfiler.collectGarbage'); const metrics = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); report.renderer = { heapBytes: metrics.JSHeapUsedSize, nodes: (await c.call('Memory.getDOMCounters')).nodes }
    if (report.errors.length || report.blockedRequests.length) throw new Error('Browser error/external traffic')
    report.passed = true; console.log(JSON.stringify(report))
  } finally { await fs.writeFile(path.join(output, 'verification.json'), JSON.stringify(report, null, 2)); if (host) await host.stop(); await b.stop() }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
