// Default: offline synthetic Go host. --live explicitly opts into pinned atlas
// HTTPS reads in a disposable profile, never Frontier or journal uploads.
const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const live = process.argv.includes('--live'); const output = path.join(root, live ? 'temp/g2o-live-visual' : 'temp/g2o-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: live ? 'Live pinned publisher; disposable synthetic game profile, no private profile or provider authorization' : 'Offline synthetic journal, no installed atlas or external requests', pages: [], errors: [], outsideRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.outsideRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(pageTextReady(text))); await evaluate('document.fonts.ready.then(()=>true)') }
  const rpc = (name, message = {}) => evaluate(`new Promise((resolve,reject)=>{const w=new WebSocket(location.origin.replace('http:','ws:'));const timer=setTimeout(()=>{w.close();reject(Error('fixture RPC timeout'))},10000);w.onopen=()=>w.send(JSON.stringify({requestId:'atlas',name:${JSON.stringify(name)},message:${JSON.stringify(message)}}));w.onmessage=e=>{const r=JSON.parse(e.data);if(r.requestId==='atlas'){clearTimeout(timer);w.close();r.error?reject(Error(r.error)):resolve(r.message)}};w.onerror=()=>{clearTimeout(timer);reject(Error('fixture socket failed'))}})`)
  const shot = async name => { const p = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(p.data, 'base64')) }
  const rendered = async () => evaluate('Promise.all(Array.from(document.querySelectorAll(".go-galaxy-plane image")).map(async e=>{const r=await fetch(e.getAttribute("href"));if(!r.ok)throw Error("atlas image unavailable");const b=await createImageBitmap(await r.blob());const result={width:b.width,height:b.height};b.close();return result}))')
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2g')); origin = host.url
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/nav/galaxy', 'Galaxy Map'); await waitFor(() => evaluate('document.body.innerText.includes("atlas not installed")'))
    if (await evaluate('!!document.querySelector(".go-galaxy-plane image")')) throw Error('Uninstalled map shows illustrative density')
    const status = await rpc('getGoAtlasStatus'); if (status.automatic || status.details || status.installed) throw Error('Unexpected default networking or installation')
    await shot('uninstalled-overlays')
    await evaluate('document.querySelector(\'[aria-label="Owner settings"]\').click()'); await evaluate('Array.from(document.querySelectorAll(".go-companion-settings>button")).find(b=>b.textContent==="Galaxy Atlas").click()'); await waitFor(() => evaluate('document.querySelector(".go-atlas-settings").textContent.includes("Install verified overview")')); await shot('owner-atlas-settings')
    await evaluate('document.querySelector(\'[aria-label="Owner settings"]\').click()')
    if (live) {
      await rpc('checkGoAtlas'); await rpc('installGoAtlas'); await waitFor(async () => { const s = await rpc('getGoAtlasStatus'); if (!s.busy && s.error) throw Error(s.error); return !s.busy && s.installed }); report.install = await rpc('getGoAtlasStatus')
    }
    for (const [name, width, height] of [['laptop', 1366, 768], ['compact', 680, 580]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/nav/galaxy', 'Galaxy Map'); await evaluate('Array.from(document.querySelectorAll("button")).find(e=>e.textContent==="Fit Galaxy").click()')
      await waitFor(() => evaluate('document.querySelectorAll("[data-marker=current]").length===3'))
      if (live) { await waitFor(() => evaluate('document.querySelectorAll(".go-galaxy-plane image").length>=3')); report[name + 'OverviewImages'] = await rendered() }
      const geometry = await evaluate('(()=>{const r=a=>{const b=document.querySelector(".go-galaxy-"+a).getBoundingClientRect();return {x:b.x,y:b.y,w:b.width,h:b.height}};return {main:r("xz"),bottom:r("xy"),right:r("yz")}})()')
      if (Math.abs(geometry.main.w - geometry.bottom.w) > 2 || Math.abs(geometry.main.h - geometry.right.h) > 2 || geometry.bottom.h >= geometry.main.h / 2 || geometry.right.w >= geometry.main.w / 2) throw Error('Three-projection layout lost')
      await shot('overview-' + name); report.pages.push({ name, geometry })
      if (live) {
        await rpc('configureGoAtlas', { automatic: false, details: true, metered: false })
        await evaluate('Array.from(document.querySelectorAll("button")).find(e=>e.textContent==="Fit Route").click()')
        await waitFor(() => evaluate('document.querySelectorAll("[data-source-level=\\"7\\"]").length>=3'))
        await rendered(); await shot('detail-' + name); report[name + 'Detail'] = await rpc('getGoAtlasStatus')
      }
    }
    if (report.errors.length || report.outsideRequests.length) throw Error('Unexpected browser error or external browser request')
    report.passed = true; console.log(JSON.stringify(report))
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
