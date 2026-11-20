// One synthetic browser batch. No live providers, OAuth, journals or game access.
const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const output = path.join(root, 'temp/g2mb-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic G2M-B navigation and contextual activity; no live provider requests or game access', geometry: [], pages: [], errors: [], outsideRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.outsideRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] }); await c.call('Performance.enable')
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(`location.origin===${JSON.stringify(origin)}&&location.pathname===${JSON.stringify(url)}&&(${pageTextReady(text)})`)); await evaluate('document.fonts.ready.then(()=>true)') }
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  const geometry = async () => evaluate(`(()=>{
    const rect=axes=>{const r=document.querySelector('.go-galaxy-'+axes+' svg').getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom}};
    const pixel=axes=>{const e=document.querySelector('[data-marker="current"][data-axes="'+axes+'"]'),r=rect(axes),p=e.dataset.position.split(',').map(Number);return {x:r.x+p[0]*r.width,y:r.y+p[1]*r.height}};
    return {main:rect('xz'),bottom:rect('xy'),right:rect('yz'),current:{main:pixel('xz'),bottom:pixel('xy'),right:pixel('yz')},width:innerWidth,height:innerHeight}
  })()`)
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2g'), true); origin = host.url
    const requests = []
    await c.call('Network.enable')
    c.on(e => { if (e.method === 'Network.webSocketFrameSent') { try { requests.push(JSON.parse(e.params.response.payloadData).name) } catch {} } })
    for (const url of ['/cmdr/overview', '/cmdr/session']) {
      requests.length = 0
      await navigate(url, 'SERVICE READY')
      await waitFor(() => evaluate('!!document.querySelector(".go-commander-overview")||document.body.innerText.toUpperCase().includes("SESSION SUMMARY")'))
      if (await evaluate('!!document.querySelector(".go-context-activity")') || requests.includes('getGoBGSTally')) throw Error('Commander layout mounted the full activity ledger: ' + url)
    }
    report.commanderUncluttered = true
    await navigate('/cmdr/overview', 'Pilot ranks')
    requests.length = 0
    await evaluate('document.querySelector(\'[aria-label="Owner settings"]\').click()')
    await waitFor(() => evaluate('!!document.querySelector(".go-private-activity")'))
    if (await evaluate('document.querySelector(".go-private-activity").open') || requests.includes('getGoBGSTally')) throw Error('Private activity queried before explicit open')
    await evaluate('document.querySelector(".go-private-activity>summary").click()')
    await waitFor(() => evaluate('!!document.querySelector(".go-private-activity .go-context-activity")'))
    await waitFor(() => requests.includes('getGoBGSTally'))
    await evaluate('document.querySelector(\'[aria-label="Close settings"]\').click()')
    report.ownerActivityExplicitOnly = true

    for (const [name, width, height] of [['laptop', 1366, 768], ['desktop', 1920, 1080], ['compact', 680, 580], ['portrait', 820, 1180]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/nav/galaxy', 'GALAXY MAP')
      await waitFor(() => evaluate('document.querySelectorAll("[data-marker=current]").length===3'))
      // Wait for the observed container size to settle across two animation frames.
      await evaluate('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
      const g = await geometry()
      const near = (a, b) => Math.abs(a - b) <= 2
      if (!near(g.main.x, g.bottom.x) || !near(g.main.width, g.bottom.width) || !near(g.main.y, g.right.y) || !near(g.main.height, g.right.height) || g.bottom.height > g.main.height * 0.22 || g.right.width > g.main.width * 0.22 || g.bottom.y < g.main.bottom || g.right.x < g.main.right) throw Error('Incorrect thin projection geometry: ' + JSON.stringify(g))
      if (!near(g.current.main.x, g.current.bottom.x) || !near(g.current.main.y, g.current.right.y)) throw Error('Shared X/Z marker alignment is broken: ' + JSON.stringify(g))
      if (g.right.right > width + 2 || g.bottom.bottom > height + 2) throw Error('Galaxy projections leave the visible dashboard: ' + JSON.stringify(g))
      report.geometry.push({ name, ...g }); await screenshot('galaxy-' + name)
      if (name === 'laptop') {
        await evaluate('document.querySelector("[aria-label=\\"Galaxy zoom in\\"]").click()')
        const zoomed = await geometry(); if (!near(zoomed.current.main.x, zoomed.current.bottom.x) || !near(zoomed.current.main.y, zoomed.current.right.y)) throw Error('Zoom broke shared marker alignment')
      }
    }
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/nav/neutron-plotter', 'Spansh Plotter')
    const labels = (await evaluate('document.querySelector(".secondary-navigation").innerText')).toUpperCase()
    if (!labels.includes('SYSTEM LIST') || labels.includes('NEUTRON PLOTTER') || !labels.includes('SPANSH PLOTTER') || await evaluate('document.querySelector("#primaryNavigation").innerText.toUpperCase().includes("CONTROLS")')) throw Error('Native navigation labels not migrated')
    const modes = await evaluate('Array.from(document.querySelector("[aria-label=\\"Spansh plotting mode\\"]").options).map(o=>o.value)')
    if (modes.length !== 12) throw Error('Public plotter catalogue/history missing')
    for (const id of modes.filter(id => id !== 'history')) {
      await evaluate(`(()=>{const e=document.querySelector('[aria-label="Spansh plotting mode"]');Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,'value').set.call(e,${JSON.stringify(id)});e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
      await waitFor(() => evaluate(`document.querySelector('.go-spansh a')?.getAttribute('href')===${JSON.stringify('https://spansh.co.uk/' + id)}`))
    }
    await screenshot('spansh-chooser')
    await evaluate('document.querySelector(".go-spansh .go-field-ops details").open=true')
    const csv = path.join(output, 'synthetic-route.csv'); await fs.writeFile(csv, 'System Name,Neutron,Refuel\nFixture Alpha,false,false\nFixture Beta,true,true\n')
    const doc = await c.call('DOM.getDocument'); const node = await c.call('DOM.querySelector', { nodeId: doc.root.nodeId, selector: '.go-spansh input[type=file]' }); await c.call('DOM.setFileInputFiles', { nodeId: node.nodeId, files: [csv] })
    await waitFor(() => evaluate('Array.from(document.querySelectorAll("button")).some(b=>b.textContent==="Import selected CSV"&&!b.disabled)'))
    await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Import selected CSV").click()')
    await waitFor(() => evaluate('location.pathname==="/nav/route"&&document.querySelector(".go-field-metrics")?.textContent.includes("Fixture Beta")'))
    const order = await evaluate('!!(document.querySelector(".go-route__endpoints").compareDocumentPosition(document.querySelector(".go-route>.go-field-ops"))&Node.DOCUMENT_POSITION_FOLLOWING)')
    if (!order) throw Error('Manual import controls were not placed below the game route')
    report.route = { navigatedAfterImport: true, manualControlsBelowGameRoute: true }
    await c.call('Page.reload'); await waitFor(() => evaluate('document.querySelector(".go-field-metrics")?.textContent.includes("Fixture Beta")')); report.route.survivedReload = true
    await navigate('/nav/neutron-plotter', 'Spansh Plotter')
    await evaluate('(()=>{const e=document.querySelector("[aria-label=\\"Spansh plotting mode\\"]");Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,"value").set.call(e,"history");e.dispatchEvent(new Event("change",{bubbles:true}))})()')
    await waitFor(() => evaluate('document.querySelector(".go-field-ops select")?.textContent.includes("synthetic-route")')); await screenshot('route-history')
    report.route.savedHistory = true
    await evaluate('window.g2mbRPC=async(name,message={})=>{const session=await fetch(\'/auth/session\',{method:\'POST\'});if(!session.ok)throw Error(\'owner session\');return new Promise((resolve,reject)=>{const ws=new WebSocket(location.origin.replace(\'http\',\'ws\'));const requestId=\'g2mb-capacity\';ws.onopen=()=>ws.send(JSON.stringify({name,message,requestId}));ws.onmessage=e=>{const r=JSON.parse(e.data);if(r.requestId===requestId){ws.close();r.error?reject(Error(r.error)):resolve(r.message)}};ws.onerror=reject})}')
    await evaluate('(async()=>{for(let i=0;i<15;i++)await g2mbRPC(\'importGoItinerary\',{name:\'Capacity \'+i,csv:\'system\\nAlpha\\nBeta\\n\'});window.capacityActive=(await g2mbRPC(\'getGoFieldView\')).plan.id})()')
    await c.call('Page.reload'); await waitFor(() => evaluate('document.querySelector(".go-field-ops select")?.options.length===16')); await evaluate('document.querySelector(".go-field-ops details").open=true')
    const nextCSV = path.join(output, 'seventeenth-route.csv'); await fs.writeFile(nextCSV, 'system\nGamma\nDelta\n')
    const d = await c.call('DOM.getDocument'); const n = await c.call('DOM.querySelector', { nodeId: d.root.nodeId, selector: '.go-spansh input[type=file]' }); await c.call('DOM.setFileInputFiles', { nodeId: n.nodeId, files: [nextCSV] })
    await waitFor(() => evaluate('Array.from(document.querySelectorAll("button")).some(b=>b.textContent==="Import selected CSV"&&!b.disabled)'))
    await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Import selected CSV").click()')
    await waitFor(() => evaluate('document.querySelector(".go-field-ops")?.textContent.includes("16 saved plans:")'))
    // Reload discards the JS helper, so compare the active label before changing selection.
    if (!await evaluate('document.querySelector(".go-field-ops select").selectedOptions[0].textContent.includes("Capacity 14")')) throw Error('Failed import disturbed active route')
    await evaluate('(()=>{const e=document.querySelector(\'.go-field-ops select\'),p=Array.from(e.options).find(o=>o.textContent.includes(\'synthetic-route\'));Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,\'value\').set.call(e,p.value);e.dispatchEvent(new Event(\'change\',{bubbles:true}))})()')
    await waitFor(() => evaluate('!!document.querySelector(".go-route-delete input")&&!document.querySelector(".go-route-delete input").disabled'))
    if (!await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Delete selected inactive route").disabled')) throw Error('Delete skipped confirmation')
    await evaluate('document.querySelector(".go-route-delete").open=true;document.querySelector(".go-route-delete input").click()')
    await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Delete selected inactive route").click()')
    await waitFor(() => evaluate('document.querySelector(".go-field-ops select")?.options.length===15'))
    if (!await evaluate('document.querySelector(".go-field-ops select").selectedOptions[0].textContent.includes("Capacity 14")')) throw Error('Inactive deletion changed the active route')
    await evaluate('Array.from(document.querySelectorAll("button")).find(b=>b.textContent==="Import selected CSV").click()')
    await waitFor(() => evaluate('location.pathname==="/nav/route"&&document.querySelector(".go-field-metrics")?.textContent.includes("Gamma")'))
    report.route.capacityRecoveredByConfirmedOwnerDelete = true

    for (const [url, title] of [['/missions/overview', 'Mission & faction activity'], ['/missions/powerplay', 'Powerplay activity'], ['/cmdr/colonisation', 'Colonisation contributions'], ['/ship/carrier', 'Owned carrier operations']]) {
      await navigate(url, title)
      const overflow = await evaluate('document.querySelector(".go-content").scrollWidth-document.querySelector(".go-content").clientWidth')
      if (overflow > 2) throw Error('Context page overflows: ' + url)
      report.pages.push({ url, overflow }); await screenshot(url.split('/').filter(Boolean).join('-'))
    }
    await navigate('/cmdr/field', 'Pilot ranks'); if (await evaluate('document.querySelector(".secondary-navigation").innerText.toUpperCase().includes("FIELD / BGS")')) throw Error('Standalone generic BGS entry retained')
    await navigate('/controls', 'Pilot ranks'); if (await evaluate('!!document.querySelector("[aria-label=\\"Key bindings\\"]")')) throw Error('Native Controls page still active')
    await navigate('/nav/galaxy', 'GALAXY MAP'); await c.call('HeapProfiler.collectGarbage')
    const metrics = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value]))
    report.renderer = { heapBytes: metrics.JSHeapUsedSize, svgCount: await evaluate('document.querySelectorAll(".go-galaxy-plane svg").length'), artworkBytes: await evaluate('performance.getEntriesByType("resource").filter(r=>r.name.includes("/atlas/assets/")).reduce((n,r)=>n+(r.encodedBodySize||0),0)'), method: 'one synthetic renderer snapshot after explicit GC; no claim about full Wails process memory or real gameplay' }
    if (report.errors.length || report.outsideRequests.length) throw Error('Runtime or external request errors')
    report.passed = true; console.log(JSON.stringify(report))
  } finally { if (!report.passed) report.diagnostic = await evaluate('document.body.innerText').catch(() => 'unavailable'); if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
