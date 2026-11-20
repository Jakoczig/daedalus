// Fabricated profile and fixed Frontier transport only; never contacts a provider.
const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const output = path.join(root, 'temp/g2n-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic Chromium + Go, fixed Frontier responses; not live gameplay acceptance', pages: [], errors: [], externalRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.externalRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(`location.pathname===${JSON.stringify(url)}&&(${pageTextReady(text, true)})`)); await evaluate('document.fonts.ready.then(()=>true)'); report.pages.push(url) }
  const rpc = (name, message = {}) => evaluate(`new Promise((resolve,reject)=>{const w=new WebSocket(location.origin.replace('http:','ws:'));const timer=setTimeout(()=>{w.close();reject(Error('fixture RPC timeout'))},5000);w.onopen=()=>w.send(JSON.stringify({requestId:'g2n',name:${JSON.stringify(name)},message:${JSON.stringify(message)}}));w.onmessage=e=>{const r=JSON.parse(e.data);if(r.requestId==='g2n'){clearTimeout(timer);w.close();r.error?reject(Error(r.error)):resolve(r.message)}};w.onerror=()=>{clearTimeout(timer);reject(Error('fixture socket failed'))}})`)
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2l'), false, { frontier: true }); origin = host.url
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/ship/status', 'SHIP')
    await rpc('configureGoProvider', { id: 'frontier', config: { read: true } }); await rpc('readGoProvider', { id: 'frontier', kind: 'profile' })
    const before = JSON.stringify((await rpc('getGoOperations', { kind: 'goals' })).goalMembership)
    await new Promise(resolve => { const ready = data => { if (data.toString().includes('FRONTIER_TICKED')) { host.child.stdout.off('data', ready); resolve() } }; host.child.stdout.on('data', ready); host.child.stdin.write('frontier-tick\n') })
    await navigate('/cmdr/community-goals', 'Refresh Frontier goals')
    await waitFor(() => evaluate('Array.from(document.querySelectorAll("button")).some(e=>e.textContent==="Refresh Frontier goals"&&!e.disabled)'))
    await evaluate('Array.from(document.querySelectorAll("button")).find(e=>e.textContent==="Refresh Frontier goals").click()')
    await waitFor(() => evaluate('document.querySelector(".go-goals")?.innerText.includes("Frontier · cached")'))
    if (before !== JSON.stringify((await rpc('getGoOperations', { kind: 'goals' })).goalMembership)) throw Error('Frontier global goals changed personal membership')
    for (const [name, width, height] of [['laptop', 1366, 768], ['compact', 680, 580]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/cmdr/community-goals', 'Frontier · cached')
      if (await evaluate('document.querySelector(".go-goals").scrollWidth-document.querySelector(".go-goals").clientWidth>2')) throw Error('Goal cards overflow')
      await screenshot('goals-' + name)
      await navigate('/ship/fleet', 'Fleet view'); await evaluate('Array.from(document.querySelectorAll(".go-fleet-card")).find(e=>e.querySelector("summary strong").textContent==="Fixture Explorer").open=true')
      await waitFor(() => evaluate('Array.from(document.querySelectorAll(".go-fleet-card")).some(e=>e.open&&e.textContent.includes("9007199254740993"))')); await screenshot('fleet-' + name)
    }
    await evaluate('Array.from(document.querySelectorAll("button")).find(e=>e.textContent==="Stored modules").click()')
    await waitFor(() => evaluate('document.querySelector("[aria-label=\\"Stored modules\\"]")?.textContent.includes("Fixture Beta")')); await screenshot('stored-modules')
    await navigate('/ship/stats', 'Ship Statistics'); await screenshot('ship-statistics')
    await navigate('/missions/overview', 'Activity in this page'); await screenshot('context-activity')
    await rpc('setGoAccounts', { multiple: true, selected: 'SYNTHETIC-G2B' })
    await c.call('Page.navigate', { url: origin + '/cmdr/overview' })
    await waitFor(() => evaluate('location.pathname==="/launcher"&&document.body.innerText.toUpperCase().includes("CHOOSE COMMANDER")'))
    if (await evaluate('!!document.querySelector("a[href=\\"/ship/status\\"]")')) throw Error('Launcher bypassed explicit profile choice')
    await screenshot('launcher-choice-required')
    await evaluate('document.querySelector(".go-account-cards button").click()')
    await waitFor(() => evaluate('!!document.querySelector("a[href=\\"/ship/status\\"]")'))
    if ((await rpc('getGoAccounts')).choiceRequired) throw Error('Explicit choice did not confirm current launch')
    host.child.stdin.write('accounts-other-session\n')
    await waitFor(async () => (await rpc('getGoAccounts')).cards.length === 2)
    const selected = await rpc('getGoCommanderOverview'); if (selected.name !== 'PRIVATE_SYNTHETIC' || !selected.journalMismatch) throw Error('Selected commander followed another journal account')
    try { await rpc('getGoOperations', { kind: 'fleet', fid: 'SYNTHETIC-OTHER' }); throw Error('Foreign fleet allowed') } catch (e) { if (e.message === 'Foreign fleet allowed') throw e }
    report.isolation = true; report.goalRefresh = 'explicit fixed-transport GET; personal membership unchanged'; report.launcherChoice = true
    if (report.errors.length || report.externalRequests.length) throw Error(JSON.stringify(report))
    report.passed = true
  } finally { if (!report.passed) { report.failurePage = await evaluate('({path:location.pathname,text:document.body?.innerText})').catch(() => null); await screenshot('failure').catch(() => {}) } if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
  console.log(JSON.stringify(report))
}
if (require.main === module) main().catch(error => { console.error(error); process.exitCode = 1 })
