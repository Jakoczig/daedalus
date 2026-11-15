// One browser batch for launcher, map/catalogue, route, galaxy and FSD policy.
const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, delay, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const focused = process.env.G2G_FOCUS === '1'
  const output = path.join(root, focused ? 'temp/g2g-focused' : 'temp/g2g-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic journal and mock EDSM; renderer operation evidence, not whole-native gameplay CPU/RAM', pages: [], errors: [], blockedRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''; const rpcNames = []
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(event => { if (event.method === 'Runtime.exceptionThrown') report.errors.push(event.params.exceptionDetails.exception?.description || event.params.exceptionDetails.text); if (event.method === 'Fetch.requestPaused') { const p = event.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.blockedRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) }; if (event.method === 'Network.webSocketFrameSent') { try { rpcNames.push(JSON.parse(event.params.response.payloadData).name) } catch {} } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] }); await c.call('Network.enable'); await c.call('Performance.enable')
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(pageTextReady(text))); await evaluate('document.fonts.ready.then(()=>true)'); await delay(300) }
  const sample = async () => { await c.call('HeapProfiler.collectGarbage'); const first = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); if (!focused) await delay(1500); const metrics = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); return { heapBytes: metrics.JSHeapUsedSize, nodes: (await c.call('Memory.getDOMCounters')).nodes, sampledMainThreadPercent: focused ? null : 100 * (metrics.TaskDuration - first.TaskDuration) / (metrics.Timestamp - first.Timestamp) } }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2g'), true); origin = host.url
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/launcher', 'NATIVE COMPANION'); await waitFor(() => evaluate('document.body.innerText.includes(\'Paired / connected devices\')'))
    if (await evaluate('!!document.querySelector(\'.go-terminal\')') || rpcNames.some(n => ['getGoWorkspace', 'getGoExploration', 'getGoNavigation'].includes(n))) throw new Error('Launcher instantiated the dashboard pipeline')
    report.launcher = await sample(); report.launcher.noDashboardRPC = true; await screenshot('launcher-laptop')
    report.launcher.settingsScroll = []
    for (const [label, width, height, factor] of [['native', 680, 580, 1], ['scaled150', 680, 580, 1.5], ['reduced125', 544, 464, 1.25]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: factor, mobile: false })
      await navigate('/launcher', 'NATIVE COMPANION')
      await evaluate('Array.from(document.querySelectorAll(".go-launcher-actions button")).find(b=>b.textContent==="Settings").click()')
      await waitFor(() => evaluate('!!document.querySelector(".go-companion-settings .go-data-providers summary")'))
      await evaluate('document.querySelector(".go-companion-settings .go-data-providers summary").focus()')
      await c.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, text: '\r' })
      await c.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 })
      await waitFor(() => evaluate('document.querySelector(".go-companion-settings .go-data-providers").open&&document.querySelectorAll(".go-provider-card").length===5'))
      await evaluate('(()=>{const links=document.querySelectorAll(".go-provider-card a");links[links.length-1].focus()})()')
      const focus = await evaluate('(()=>{const e=document.querySelector(".go-launcher"),a=document.activeElement,r=a.getBoundingClientRect();return {label:a.textContent,top:r.top,bottom:r.bottom,height:innerHeight,overflow:e.scrollWidth-e.clientWidth,extra:e.scrollHeight-e.clientHeight,scrollTop:e.scrollTop}})()')
      if (focus.label !== 'Provider documentation' || focus.extra < 100 || focus.scrollTop < 100 || focus.top < 0 || focus.bottom > height + 2 || focus.overflow > 2) throw new Error('Launcher keyboard focus/settings clipped: ' + JSON.stringify(focus))
      await screenshot('launcher-settings-' + label)
      await evaluate('(()=>{const e=document.querySelector(".go-launcher");e.focus();e.scrollTop=0})()')
      await c.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'PageDown', code: 'PageDown', windowsVirtualKeyCode: 34 })
      await c.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'PageDown', code: 'PageDown', windowsVirtualKeyCode: 34 })
      await waitFor(() => evaluate('document.querySelector(".go-launcher").scrollTop>20'))
      report.launcher.settingsScroll.push({ case: label, width, height, factor, ...focus, keyboardScroll: true })
    }
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/ship/status', 'POWER DISTRIBUTION'); await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Ship lights: On"]\')')); report.dashboard = await sample()
    for (const [viewport, width, height] of (focused ? [] : [['desktop', 1920, 1080], ['laptop', 1366, 768], ['portrait', 820, 1180]])) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/launcher', 'NATIVE COMPANION'); await screenshot('launcher-' + viewport); report.pages.push({ viewport, page: 'launcher' })
      await navigate('/ship/status', 'POWER DISTRIBUTION'); await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Ship lights: On"]\')'))
      const fit = await evaluate('(()=>{const e=document.querySelector(\'.go-content\');return {height:e.clientHeight,scroll:e.scrollHeight,width:e.clientWidth,scrollWidth:e.scrollWidth}})()')
      if (fit.scroll > fit.height + 3 || fit.scrollWidth > fit.width + 2) throw new Error('Ship instrumentation does not fit: ' + JSON.stringify({ viewport, ...fit }))
      await screenshot('ship-' + viewport); report.pages.push({ viewport, page: 'ship', ...fit })
      await navigate('/nav/map', 'SYSTEM CARTOGRAPHY'); await waitFor(() => evaluate('document.querySelectorAll(\'.go-chart__body\').length>=63'))
      const map = await evaluate('(()=>{const svg=document.querySelector(\'.go-chart svg\');return {bodies:document.querySelectorAll(\'.go-chart__body\').length,width:svg.getBoundingClientRect().width,height:svg.getBoundingClientRect().height,banner:!!document.querySelector(\'.go-cartography>.go-community\'),metrics:!!document.querySelector(\'.go-cartography>.go-metrics\')}})()')
      if (map.bodies < 63 || map.banner || map.metrics) throw new Error('Cartography clipped/paginated or noisy: ' + JSON.stringify(map))
      await screenshot('cartography-' + viewport); report.pages.push({ viewport, page: 'cartography', ...map })
      await navigate('/nav/list', 'SYSTEM CATALOGUE'); await waitFor(() => evaluate('document.querySelectorAll(\'.go-body-list>button\').length>=63'))
      if (!await evaluate('(()=>{const e=document.querySelector(\'.go-body-list\');return e.scrollHeight>e.clientHeight})()')) throw new Error('Catalogue not scrollable')
      await screenshot('catalogue-' + viewport); report.pages.push({ viewport, page: 'catalogue' })
      await navigate('/nav/route', 'ROUTE PLAN'); await waitFor(() => evaluate('document.body.innerText.includes(\'SCOOPABLE\')'))
      if (await evaluate('document.body.innerText.includes(\'SCOOP CLASS\')')) throw new Error('Generic scoop label retained')
      await screenshot('route-' + viewport); report.pages.push({ viewport, page: 'route' })
      await navigate('/nav/galaxy', 'GALAXY MAP'); await waitFor(() => evaluate('document.querySelectorAll(\'[data-marker="current"]\').length===3'))
      if (viewport !== 'portrait' && !await evaluate('Array.from(document.querySelectorAll(\'.go-galaxy-plane svg\')).every(e=>{const r=e.getBoundingClientRect();return r.height>30&&r.top>=0&&r.bottom<=innerHeight&&r.left>=0&&r.right<=innerWidth})')) throw new Error('Galaxy projections exceed the visible dashboard')
      const projections = await evaluate('Array.from(document.querySelectorAll(\'[data-marker="current"]\')).map(e=>({axes:e.dataset.axes,position:e.dataset.position}))')
      if (new Set(projections.map(p => p.axes)).size !== 3 || !await evaluate('document.body.innerText.includes(\'Illustrative spiral-arm concept\')&&!!Array.from(document.querySelectorAll(\'button\')).find(b=>b.disabled&&b.textContent===\'Navigate (coming later)\')')) throw new Error('Galaxy projection/provenance missing')
      await screenshot('galaxy-' + viewport); report.pages.push({ viewport, page: 'galaxy', projections })
    }
    // Use the ordinary owner-authenticated WebSocket, with no Wails bindings.
    await evaluate('window.g2gRPC=async(name,message)=>{const session=await fetch(\'/auth/session\',{method:\'POST\'});if(!session.ok)throw Error(\'owner session\');return new Promise((resolve,reject)=>{const ws=new WebSocket(location.origin.replace(\'http\',\'ws\'));const id=\'g2g-check\';ws.onopen=()=>ws.send(JSON.stringify({name,message,requestId:id}));ws.onmessage=e=>{const r=JSON.parse(e.data);if(r.requestId===id){ws.close();r.error?reject(Error(r.error)):resolve(r.message)}};ws.onerror=reject})}')
    await evaluate('g2gRPC(\'setGoCompanion\',{charge:true,jump:true,restore:true,owner:true,allViewers:false,devices:[],closeMode:\'tray\'})')
    await navigate('/nav/galaxy', 'GALAXY MAP'); host.child.stdin.write('jump\n')
    await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Automatic FSD display"]\')'))
    await screenshot('automatic-fsd'); report.fsd = { selectedOwner: true }
    await evaluate('document.querySelector(\'[data-primary-navigation="2"]\').dispatchEvent(new PointerEvent(\'pointerdown\',{bubbles:true}))')
    await evaluate('document.querySelector(\'[data-primary-navigation="2"]\').click()')
    await waitFor(() => evaluate('!!document.querySelector(\'.go-flight-switches\')&&!document.querySelector(\'[aria-label="Automatic FSD display"]\')'))
    await delay(1200); if (await evaluate('!!document.querySelector(\'[aria-label="Automatic FSD display"]\')')) throw new Error('Manual override was lost')
    report.fsd.manualOverride = true; host.child.stdin.write('arrival\n'); await delay(1200)
    await navigate('/nav/galaxy', 'GALAXY MAP'); host.child.stdin.write('jump\n'); await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Automatic FSD display"]\')')); host.child.stdin.write('arrival\n'); await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Galaxy map"]\')&&!document.querySelector(\'[aria-label="Automatic FSD display"]\')')); report.fsd.restoredPriorScreen = true
    await evaluate('(()=>{const i=document.querySelector(\'[aria-label="Galaxy system search"]\');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,\'value\').set.call(i,\'Fixture Beta\');i.dispatchEvent(new Event(\'input\',{bubbles:true}))})()')
    await evaluate('Array.from(document.querySelectorAll(\'button\')).find(b=>b.textContent===\'Lookup with enabled EDSM\').click()')
    await waitFor(() => evaluate('document.querySelector(\'.go-galaxy-detail\')?.textContent.includes(\'Fixture Beta\')'))
    await evaluate('Array.from(document.querySelectorAll(\'button\')).find(b=>b.textContent===\'View System\').click()')
    await waitFor(() => evaluate('document.querySelector(\'.go-cartography\')?.textContent.includes(\'Fixture Beta\')')); report.viewSystem = true
    if (report.errors.length || report.blockedRequests.length) throw new Error('Runtime/external request errors')
    report.passed = true; await fs.writeFile(path.join(output, 'verification.json'), JSON.stringify(report, null, 2)); console.log(JSON.stringify(report))
  } finally { if (host) await host.stop(); await b.stop(); if (!report.passed) await fs.writeFile(path.join(output, 'verification.json'), JSON.stringify(report, null, 2)) }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
