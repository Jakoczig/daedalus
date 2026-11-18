// Test-only real browser, real Go pipeline, fabricated journals. No game profiles.
const fs = require('node:fs/promises')
const sync = require('node:fs')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const { fixture, browser, waitFor, delay, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')

function backendSample (pid) {
  if (!Number.isSafeInteger(pid) || pid <= 0) throw new Error('Invalid fixture process')
  if (process.platform === 'win32') {
    const value = JSON.parse(execFileSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', `Get-Process -Id ${pid} | Select-Object CPU,WorkingSet64 | ConvertTo-Json -Compress`], { windowsHide: true }).toString())
    return { cpuSeconds: value.CPU, rssBytes: value.WorkingSet64, at: performance.now() }
  }
  const status = sync.readFileSync(`/proc/${pid}/status`, 'utf8')
  const rssKB = /^VmRSS:\s+(\d+)\s+kB/m.exec(status)?.[1]
  return { cpuSeconds: null, rssBytes: rssKB == null ? null : Number(rssKB) * 1024, at: performance.now() }
}

async function main () {
  const focused = process.env.G2F_FOCUSED_VISUAL === '1'
  const output = path.join(root, 'temp/g2e-visual')
  await fs.mkdir(output, { recursive: true })
  const report = { scope: 'Sandboxed Chromium and separate synthetic Go fixture process; not whole Wails CPU/RAM or gameplay', pages: [], errors: [], blockedRequests: [] }
  const b = await browser(output); const client = b.client
  let host; let origin = ''
  const evaluate = async expression => {
    const r = await client.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
    if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text)
    return r.result.value
  }
  client.on(event => {
    if (event.method === 'Runtime.exceptionThrown') report.errors.push(event.params.exceptionDetails.exception?.description || event.params.exceptionDetails.text)
    if (event.method === 'Fetch.requestPaused') {
      const p = event.params; const allowed = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:')
      if (!allowed) report.blockedRequests.push(p.request.url)
      client.call(allowed ? 'Fetch.continueRequest' : 'Fetch.failRequest', allowed ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {})
    }
  })
  await client.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const metrics = async () => Object.fromEntries((await client.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value]))
  const screenshot = async name => { const value = await client.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(value.data, 'base64')) }
  const navigate = async (pathname, expected) => {
    await client.call('Page.navigate', { url: origin + pathname })
    try { await waitFor(() => evaluate(pageTextReady(expected, true))) } catch (error) { report.failedPage = { pathname, expected, text: await evaluate('document.body?.innerText || \'\'') }; await screenshot('failed-page'); throw error }
    await evaluate('document.fonts.ready.then(()=>true)'); await delay(400)
    if (pathname === '/ship/status' && await evaluate("!!document.querySelector('.go-parity-ship')")) await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Ship lights: On"]\')'))
  }
  const sample = async () => {
    await client.call('HeapProfiler.collectGarbage')
    const first = await metrics(); const goFirst = backendSample(host.child.pid)
    await delay(2000)
    const second = await metrics(); const goSecond = backendSample(host.child.pid)
    return { rendererHeapBytes: second.JSHeapUsedSize, rendererBusyPercent: (second.TaskDuration - first.TaskDuration) / (second.Timestamp - first.Timestamp) * 100, backendRSSBytes: goSecond.rssBytes, backendCPUPercentOneCore: goFirst.cpuSeconds == null ? null : (goSecond.cpuSeconds - goFirst.cpuSeconds) / ((goSecond.at - goFirst.at) / 1000) * 100 }
  }
  try {
    await client.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    const source = path.join(root, 'tests/testdata/g2f')
    if (!focused) {
      host = await fixture('G2D', '', source); origin = host.url
      await navigate('/ship/status', 'ASP'); await screenshot('g2d-ship-baseline-laptop'); report.g2dBaseline = await sample()
      await host.stop(); host = null
    }
    host = await fixture('G2E', '', source); origin = host.url
    await navigate('/ship/status', 'POWER DISTRIBUTION'); if (!focused) report.g2eComparison = await sample()
    report.measurementMethod = '1366×768 Ship Status, same synthetic replay, explicit renderer GC, two-second idle sample. Backend process is the test fixture at the production 500ms polling interval, without a Wails window. Windows CPU is Get-Process CPU seconds / elapsed one-core time; Linux backend CPU unmeasured. RSS is working set/VmRSS and includes resident static assets/reference caches.'
    for (const [viewport, width, height] of (focused ? [['laptop', 1366, 768]] : [['desktop', 1920, 1080], ['laptop', 1366, 768], ['tablet', 820, 1180]])) {
      await client.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      for (const [page, pathname, text] of [['ship', '/ship/status', 'POWER DISTRIBUTION'], ['modules', '/ship/modules', 'FRAME SHIFT DRIVE'], ['cargo', '/ship/cargo', 'LIMPET'], ['inventory', '/ship/inventory', 'MEDKIT'], ['blueprints', '/eng/blueprints', 'REFERENCE RECIPES'], ['planner', '/eng/material-planner', 'MATERIAL PLANNER'], ['materials', '/eng/raw-materials', 'IRON'], ['commander', '/cmdr/overview', 'PILOT RANKS'], ['missions', '/missions/overview', 'FIXTURE DELIVERY'], ['mining', '/mining/status', 'PLATINUM'], ['rings', '/mining/rings', 'FIXTURE ALPHA 1 RING']]) {
        if (focused && !['ship', 'modules'].includes(page)) continue
        await navigate(pathname, text)
        const layout = await evaluate('({documentWidth:document.documentElement.scrollWidth,width:innerWidth,headers:document.querySelectorAll("header").length})')
        if (layout.documentWidth > width + 2 || layout.headers !== 1) throw new Error(`Layout error ${page}/${viewport}`)
        if (page === 'modules') {
          await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Inspect Fighter Hangar Mk II"]\')'))
          await evaluate('document.querySelector(\'[aria-label="Inspect Fighter Hangar Mk II"]\').click()')
          await waitFor(() => evaluate('document.querySelector(\'.go-module-inspector\')?.textContent.includes(\'Module class has not been verified\')'))
          if (!await evaluate('document.querySelector(\'[aria-label="Inspect Fighter Hangar Mk II"] .go-module-class\').textContent.startsWith(\'?\')')) throw new Error('Unverified module class must remain explicitly unknown')
          await screenshot('fighter-module-' + viewport)

          const selected = await evaluate('new Promise(resolve=>{const t=performance.now();const b=Array.from(document.querySelectorAll(".go-module-card")).find(b=>b.textContent.includes("Frame Shift Drive"));if(!b)return resolve(null);b.click();requestAnimationFrame(()=>requestAnimationFrame(()=>resolve({ms:performance.now()-t,text:document.querySelector(".go-module-inspector")?.innerText})))})')
          if (!selected?.text.includes('Mass Manager')) throw new Error('Engineering inspector selection failed')
          const inspector = await evaluate('(()=>{const i=document.querySelector(".go-module-inspector");return {display:getComputedStyle(i).display,width:i.clientWidth,scrollWidth:i.scrollWidth}})()')
          if (inspector.display !== 'block' || inspector.width < 200 || inspector.scrollWidth > inspector.width + 2) { await screenshot('module-inspector-failed-' + viewport); report.failedInspector = { viewport, ...inspector }; throw new Error('Module inspector unreadable: ' + JSON.stringify(inspector)) }
          report.pages.push({ viewport, page, selectionMs: selected.ms, inspector })
        } else {
          if (page === 'ship') {
            const switches = await evaluate('Array.from(document.querySelectorAll(\'.go-flight-switches .checkbox__control\')).map(e=>{const r=e.getBoundingClientRect();const t=getComputedStyle(e,\'::after\');return {width:r.width,height:r.height,thumb:parseFloat(t.width)*r.width/e.offsetWidth,left:parseFloat(t.left)*r.width/e.offsetWidth,label:getComputedStyle(e.parentElement.querySelector(\'small\')).fontSize}})')
            if (switches.length !== 5 || switches.some(s => s.width < 55 || s.height < 25 || s.thumb < 15 || s.left + s.thumb > s.width)) throw new Error('Observed switch track/thumb is not legible: ' + JSON.stringify(switches))
            report.switches = switches
            // Pointer reveal must expire even while hover/focus remains, keyboard
            // focus must still reveal its own label. This uses real browser input.
            const point = await evaluate('(()=>{const b=document.querySelector(\'.go-temporary-labels .secondary-navigation__has-label\');const r=b.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()')
            await client.call('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
            await client.call('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1, ...point })
            await client.call('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...point })
            await delay(150)
            if (!await evaluate('(()=>{const b=document.querySelector(\'.go-temporary-labels .secondary-navigation__has-label\');return getComputedStyle(b.querySelector(\'.secondary-navigation__label\')).opacity!==\'0\'})()')) throw new Error('Pointer label did not reveal')
            await delay(2400)
            if (!await evaluate('getComputedStyle(document.querySelector(\'.go-temporary-labels .secondary-navigation__label\')).opacity===\'0\'')) throw new Error('Pointer label stuck after timeout')
            await client.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 })
            await client.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 })
            await delay(250)
            if (!await evaluate('(()=>{const b=document.activeElement;return b.matches(\'.go-temporary-labels .secondary-navigation__has-label\')&&getComputedStyle(b.querySelector(\'.secondary-navigation__label\')).opacity===\'1\'})()')) throw new Error('Keyboard navigation label not visible')
            report.navigationLabels = { pointerReveals: true, timeoutHides: true, keyboardVisible: true }
            await evaluate('document.activeElement.blur()'); await delay(250)

            const control = await evaluate('({count:document.querySelectorAll(".go-flight-switches input[disabled]").length,flags:document.querySelectorAll(".go-flight-indicators>div").length})')
            if (control.count !== 5 || control.flags !== 15) throw new Error('Ship instrumentation incomplete')
          }
          if (page === 'planner') {
            const counts = await evaluate('({inputs:Array.from(document.querySelectorAll("input")).map(i=>({label:i.getAttribute("aria-label"),type:i.type})),inspector:!!document.querySelector(".go-engineering__inspector")})')
            if (!counts.inspector || !counts.inputs.some(i => i.type === 'number')) throw new Error('Recipe planning controls missing')
          }
          report.pages.push({ viewport, page, ...layout })
        }
        await screenshot(`${page}-${viewport}`)
      }
    }
    await host.stop(); host = null
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2f'), true); origin = host.url
    for (const [viewport, width, height] of (focused ? [['laptop', 1366, 768]] : [['desktop', 1920, 1080], ['laptop', 1366, 768], ['tablet', 820, 1180]])) {
      await client.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
      await navigate('/nav/map', 'SYSTEM CARTOGRAPHY')
      await waitFor(() => evaluate('!!document.querySelector(\'[aria-label="Inspect Fixture Community Planet"]\')'))
      await evaluate('Array.from(document.querySelectorAll(\'.go-chart__body\')).find(b=>b.getAttribute(\'aria-label\')===\'Inspect Fixture Community Planet\').dispatchEvent(new MouseEvent(\'click\',{bubbles:true}))')
      await waitFor(() => evaluate('document.querySelector(\'.go-inspector\')?.innerText.includes(\'personal scan state unknown\')'))
      const mergeCheck = await evaluate('({inspector:document.querySelector(\'.go-inspector\').innerText,privateLeak:document.body.innerText.includes(\'PRIVATE_PROVIDER\'),bodies:document.querySelectorAll(\'.go-chart__body\').length})')
      if (!mergeCheck.inspector.toUpperCase().includes('FIXTURE COMMUNITY PLANET') || !mergeCheck.inspector.includes('YOUR DSS UNKNOWN') || mergeCheck.privateLeak || mergeCheck.bodies !== 4) { await screenshot('community-failed-' + viewport); throw new Error('Community provenance or merge failed: ' + JSON.stringify(mergeCheck)) }
      await evaluate('Array.from(document.querySelectorAll(\'.go-community-stations summary\')).find(e=>e.innerText.includes(\'Community stations\')).click()')
      const stations = await evaluate('(()=>{const d=document.querySelector(".go-community-stations");return {open:d.open,text:d.textContent}})()')
      if (!stations.open || !stations.text.includes('Fixture Community Station') || !stations.text.includes('2026-01-01')) throw new Error('Station source freshness missing')
      await evaluate('document.querySelector(".go-community-stations button").click()')
      await waitFor(() => evaluate('(()=>{const t=document.querySelector(".go-station-inspector")?.textContent||"";return t.includes("Fixture Community Station")&&t.includes("EDSM community reported")&&t.includes("2026-01-01")&&t.includes("Stations fetched")})()'))
      await screenshot('community-map-' + viewport)
      report.pages.push({ viewport, page: 'community-map', provenance: true, stations: true })
    }
    await evaluate('document.querySelector(\'[aria-label="Owner settings"]\').click()')
    await evaluate('document.querySelector(\'.go-data-providers\').open=true')
    await waitFor(() => evaluate('document.querySelector(\'[aria-label="Optional EDSM enrichment"]\')?.textContent.includes(\'Disable EDSM lookups\')'))
    await evaluate('Array.from(document.querySelectorAll(\'[aria-label="Optional EDSM enrichment"] button\')).find(b=>b.textContent===\'Disable EDSM lookups\').click()')
    await waitFor(() => evaluate("!document.querySelector('[aria-label=\"Inspect Fixture Community Planet\"]')&&!document.querySelector('.go-station-inspector,.go-chart__stations')"))
    report.community = { mockOnly: true, ownerDisableClears: true }
    if (report.errors.length || report.blockedRequests.length) throw new Error('Runtime errors or unexpected external requests')
    report.passed = true
    await fs.writeFile(path.join(output, 'verification.json'), JSON.stringify(report, null, 2))
    console.log(JSON.stringify(report))
  } finally {
    if (host) await host.stop()
    await b.stop()
    if (!report.passed) await fs.writeFile(path.join(output, 'verification.json'), JSON.stringify(report, null, 2))
  }
}
if (require.main === module) main().catch(error => { console.error(error); process.exitCode = 1 })
