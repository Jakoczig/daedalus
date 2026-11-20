// Real Chromium, disposable synthetic profile. Never authorize or contact a provider.
const fs = require('node:fs/promises')
const path = require('node:path')
const { fixture, browser, waitFor, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const output = path.join(root, 'temp/g2p-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Synthetic local Go profile and real Chromium; no live sign-in or production requests', pages: [], errors: [], external: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  const click = text => evaluate(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent===${JSON.stringify(text)}).click()`)
  c.on(e => {
    if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description)
    if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.external.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) }
  })
  await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const screenshot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2e')); origin = host.url
    for (const [name, width, height, scale] of [['native', 826, 706, 1], ['desktop', 1366, 768, 1], ['compact', 680, 580, 1], ['scaled125', 680, 580, 1.25]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: scale, mobile: false })
      await c.call('Page.navigate', { url: origin + '/launcher' }); await waitFor(() => evaluate(pageTextReady('SERVICE READY'))); await evaluate('document.fonts.ready.then(()=>true)')
      const home = await evaluate('(()=>{const e=document.querySelector(\'.go-launcher\');const buttons=Array.from(document.querySelectorAll(\'.go-launcher-actions button,.go-launcher-actions a\')).map(b=>{const r=b.getBoundingClientRect();return r.bottom<=innerHeight&&r.left>=0&&r.right<=innerWidth});return {height:e.getBoundingClientRect().height,viewport:innerHeight,overflow:e.scrollWidth-e.clientWidth,buttons,preview:document.body.textContent.includes(\'Current system — local journal preview\'),rows:Array.from(e.querySelectorAll(\'dt\')).map(e=>e.textContent)}})()')
      if (Math.abs(home.height - home.viewport) > 2 || home.overflow > 2 || home.preview || !home.buttons.every(Boolean) || home.rows.join('|') !== 'Journal|Paired / connected devices|Live Services') throw Error('Launcher home clipped or cluttered: ' + JSON.stringify(home))
      await screenshot(name + '-home'); await click('Settings')
      const names = await evaluate('Array.from(document.querySelectorAll(".go-companion-settings>button")).map(b=>b.textContent)')
      if (names.join('|') !== 'General|Journal & Commander|External Services|Galaxy Atlas|Credits') throw Error('Settings category contract')
      for (const category of names) {
        await click(category)
        await waitFor(() => evaluate('document.querySelectorAll(".go-companion-settings>button[aria-expanded=true]").length===1'))
        if (category === 'Journal & Commander') {
          await waitFor(() => evaluate('!!document.querySelector(".go-companion-settings form input")?.value'))
          await waitFor(() => evaluate('document.querySelector(".go-account-cards")?.textContent.includes("PRIVATE_SYNTHETIC")'))
          if (!await evaluate('document.body.textContent.includes("Change portrait")&&!document.body.textContent.includes("Events read:")')) throw Error('Single profile missing or verbose journal preview retained')
        }
        if (category === 'External Services') {
          await waitFor(() => evaluate('document.querySelectorAll(".go-provider-heading").length===10'))
          if (await evaluate('!!document.querySelector(".go-provider-expanded")')) throw Error('Provider expanded by default')
          await evaluate('document.querySelector(".go-provider-heading").click()')
          await waitFor(() => evaluate('document.querySelectorAll(".go-provider-expanded").length===1'))
          if (await evaluate('document.querySelector(".go-provider-settings").textContent.includes("Read profile")')) throw Error('Raw provider actions retained')
        }
        if (category === 'Galaxy Atlas') await waitFor(() => evaluate('document.body.textContent.includes("Enable Galaxy Atlas")&&document.body.textContent.includes("Clear map data")'))
        if (category === 'Credits') {
          await evaluate('document.querySelector("[aria-label=Credits] a:last-of-type")?.focus()')
          await evaluate('(()=>{const a=Array.from(document.querySelectorAll("[aria-label=Credits] a")).at(-1);a.focus()})()')
          const focus = await evaluate('(()=>{const r=document.activeElement.getBoundingClientRect();return {top:r.top,bottom:r.bottom}})()')
          if (focus.top < 0 || focus.bottom > height) throw Error('Credits keyboard focus clipped')
        }
        await screenshot(name + '-' + category.replaceAll(' ', '-').replace('&', 'and'))
      }
      await click('Credits'); if (await evaluate('document.querySelectorAll(".go-companion-settings>button[aria-expanded=true]").length')) throw Error('Active category cannot collapse')
      report.pages.push({ name, width, height, scale, home, categories: names, singleOpen: true })
    }
    if (report.errors.length || report.external.length) throw Error('Unexpected browser error or external traffic')
    report.passed = true; console.log(JSON.stringify(report))
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
