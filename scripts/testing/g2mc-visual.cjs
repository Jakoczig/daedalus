// Actual Chromium and Go service; fabricated private profile, no live providers.
const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const output = path.join(root, 'temp/g2mc-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic startup ordering and browser interaction; not real-game acceptance', pages: [], errors: [], externalRequests: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.externalRequests.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const navigate = async (url, text) => { await c.call('Page.navigate', { url: origin + url }); await waitFor(() => evaluate(`location.pathname===${JSON.stringify(url)}&&(${pageTextReady(text, true)})`)); await evaluate('document.fonts.ready.then(()=>true)'); report.pages.push(url) }
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2mc')); origin = host.url
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await navigate('/cmdr/overview', 'Pilot ranks')
    await waitFor(() => evaluate('document.body.innerText.includes("27%")&&document.body.innerText.includes("60")'))
    await screenshot('commander-startup-snapshots')
    await navigate('/eng/engineers', 'Felicity Farseer')
    await evaluate('(()=>{const e=document.querySelector(".go-engineering input[type=search]");Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value").set.call(e,"Felicity Farseer");e.dispatchEvent(new Event("input",{bubbles:true}))})()')
    await waitFor(() => evaluate('document.querySelector(".go-engineering__inspector")?.innerText.toUpperCase().includes("JOURNAL PROGRESS: UNLOCKED")')); await screenshot('engineering-startup-snapshots')
    await navigate('/eng/raw-materials', 'Raw materials')
    await waitFor(() => evaluate('document.querySelector(".go-content").innerText.includes("91")')); await screenshot('material-startup-snapshot')
    await navigate('/ship/inventory', 'Ship locker')
    const select = await evaluate('(()=>{const e=document.querySelector(".go-content select");if(!e)return false;e.focus();return true})()')
    if (!select) throw Error('Inventory selector missing')
    await c.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'ArrowDown', code: 'ArrowDown', windowsVirtualKeyCode: 40 })
    await c.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'ArrowDown', code: 'ArrowDown', windowsVirtualKeyCode: 40 })
    if (!await evaluate('location.pathname==="/ship/inventory"&&document.activeElement.tagName==="SELECT"')) throw Error('Dropdown key navigated away or lost focus')
    await screenshot('inventory-keyboard-focus')
    await navigate('/nav/neutron-plotter', 'Spansh Plotter')
    await evaluate('document.querySelector(\'[aria-label="Spansh plotting mode"]\').focus()')
    await c.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'ArrowDown', code: 'ArrowDown', windowsVirtualKeyCode: 40 })
    await c.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'ArrowDown', code: 'ArrowDown', windowsVirtualKeyCode: 40 })
    if (!await evaluate('location.pathname==="/nav/neutron-plotter"&&document.activeElement.tagName==="SELECT"')) throw Error('Spansh selector shortcut regression')
    await screenshot('spansh-keyboard-focus')
    await navigate('/missions/local-activity', 'Private commander activity')
    if (await evaluate('document.querySelector(".go-private-activity").open')) throw Error('Private activity expanded automatically')
    await screenshot('local-activity-collapsed')
    await evaluate('document.querySelector(".go-private-activity>summary").click()')
    await waitFor(() => evaluate('!!document.querySelector(".go-context-activity")')); await screenshot('local-activity-explicit-open')
    await navigate('/nav/route', 'Not reported'); await screenshot('unknown-route-target')
    await navigate('/trade', 'Two-market quoted comparison'); await screenshot('trade-default')
    await navigate('/trade/commodities', 'Station commodity observations')
    // Use the real primary tab and its existing remembered destination.
    await evaluate('Array.from(document.querySelectorAll("#primaryNavigation button")).find(e=>e.dataset.primaryNavigation==="2").click()')
    await waitFor(() => evaluate('location.pathname.startsWith("/ship")'))
    await evaluate('Array.from(document.querySelectorAll("#primaryNavigation button")).find(e=>e.dataset.primaryNavigation==="5").click()')
    await waitFor(() => evaluate('location.pathname==="/trade/commodities"')); report.tradeRemembered = true
    if (report.errors.length || report.externalRequests.length) throw Error('Browser error or unexpected external request: ' + JSON.stringify(report))
    report.passed = true
  } finally { if (!report.passed) { report.failurePage = await evaluate('({url:location.pathname,text:document.body?.innerText})').catch(() => null); await screenshot('failure').catch(() => {}) } if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
  console.log(JSON.stringify(report))
}
if (require.main === module) main().catch(error => { console.error(error); process.exitCode = 1 })
