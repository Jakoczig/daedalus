const fs = require('node:fs/promises')
const path = require('node:path')
const { execFile } = require('node:child_process')
const { promisify } = require('node:util')
const { transformSync } = require('next/dist/compiled/babel/core')
const { fixture, browser, waitFor, delay, pageTextReady } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const live = process.argv.includes('--live'); const output = path.join(root, live ? 'temp/g3c-live' : 'temp/g3c-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: live ? 'Verified public signed spatial/density assets; disposable synthetic-navigation profile, not gameplay' : 'Offline synthetic UI and exact Canvas renderer; no real-data claim', errors: [], external: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  const rpc = async (name, message = {}) => { report.lastRPC = name; const start = performance.now(); const value = await evaluate(`new Promise((resolve,reject)=>{const w=new WebSocket(location.origin.replace('http:','ws:'));const timer=setTimeout(()=>{w.close();reject(Error('fixture RPC timeout'))},10000);w.onopen=()=>w.send(JSON.stringify({requestId:'g3c',name:${JSON.stringify(name)},message:${JSON.stringify(message)}}));w.onmessage=e=>{const r=JSON.parse(e.data);if(r.requestId==='g3c'){clearTimeout(timer);w.close();r.error?reject(Error(r.error)):resolve(r.message)}}})`); (report.requests ||= []).push({ name, milliseconds: performance.now() - start }); return value }
  const click = text => evaluate(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent===${JSON.stringify(text)})?.click()`)
  const shot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  c.on(e => { if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description); if (e.method === 'Fetch.requestPaused') { const p = e.params; const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.external.push(p.request.url); c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {}) } })
  await c.call('Network.enable'); await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const setCenter = async center => { for (let i = 0; i < 3; i++) await evaluate(`(()=>{const e=document.querySelector('[aria-label="Local center ${'XYZ'[i]}"]');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(e,${JSON.stringify(String(center[i]))});e.dispatchEvent(new Event('input',{bubbles:true}))})()`) }
  const volume = center => ({ low: center.map((v, i) => v - (i === 1 ? 50 : 100)), high: center.map((v, i) => v + (i === 1 ? 50 : 100)) })
  const select = (label, value) => evaluate(`(()=>{const e=document.querySelector('[aria-label="${label}"]');e.value=${JSON.stringify(String(value))};e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
  const backendUsage = async () => {
    if (process.platform === 'win32') { const r = await promisify(execFile)('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', `(Get-Process -Id ${host.child.pid}) | Select-Object CPU,WorkingSet64 | ConvertTo-Json -Compress`], { windowsHide: true }); const v = JSON.parse(r.stdout); return { cpuSeconds: v.CPU, rssBytes: v.WorkingSet64 } }
    const text = await fs.readFile(`/proc/${host.child.pid}/status`, 'utf8'); return { rssBytes: Number(/VmRSS:\s+(\d+)/.exec(text)?.[1] || 0) * 1024 }
  }
  const measure = async () => {
    const usage = await backendUsage(); const started = performance.now(); const first = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); await delay(1500); const last = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); const after = await backendUsage()
    return { rendererHeapBytes: last.JSHeapUsedSize, mainThreadPercent: 100 * (last.TaskDuration - first.TaskDuration) / (last.Timestamp - first.Timestamp), backendRSSBytes: after.rssBytes, backendCPUPercentOneCore: after.cpuSeconds == null ? null : 100000 * (after.cpuSeconds - usage.cpuSeconds) / (performance.now() - started) }
  }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2g')); origin = host.url
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false }); await c.call('Page.navigate', { url: origin + '/nav/galaxy' }); await waitFor(() => evaluate(pageTextReady('Galaxy Map', true)))
    await click('Local Volume · actual XYZ'); await waitFor(() => evaluate('!!document.querySelector(".go-local-volume")'))
    report.layout = await evaluate('Array.from(document.querySelectorAll(".go-galaxy-plane>svg")).map(e=>({axes:e.dataset.projection,width:e.getBoundingClientRect().width,height:e.getBoundingClientRect().height}))')
    if (report.layout.length !== 3 || report.layout.find(p => p.axes === 'yz').height <= report.layout.find(p => p.axes === 'yz').width) throw Error('Three-plane layout lost')
    if (Math.abs(report.layout[0].width - report.layout[0].height) > 2) throw Error('Local XZ world scale distorted')
    const toggle = 'Array.from(document.querySelectorAll(".go-local-volume label")).find(l=>l.textContent==="Show Route").querySelector("input")'
    await evaluate(toggle + '.click()'); if (await evaluate('!!document.querySelector(".go-galaxy-route")')) throw Error('Local hide route failed')
    if (await evaluate('document.querySelectorAll("[data-marker=current]").length') !== 3) throw Error('Route visibility hid player')
    await evaluate(toggle + '.click()')
    await select('Local inspection width', 400); await select('Local inspection depth', 100); await select('Local vertical thickness', 50)
    await click('Galaxy Overview'); await waitFor(() => evaluate('!!document.querySelector(".go-galaxy-controls")')); await click('Local Volume · actual XYZ')
    await waitFor(() => evaluate('["Local inspection width","Local inspection depth","Local vertical thickness"].map(label=>document.querySelector("[aria-label=\'"+label+"\']").value).join(",")==="400,100,50"'))
    report.rectangularTransition = true; await click('Fit Local Region')
    await waitFor(() => evaluate('document.body.innerText.includes("Density fallback")')); if (await evaluate('!!document.querySelector("[data-spatial-projection]")')) throw Error('Missing spatial cache pretended to contain points')
    await shot('synthetic-local-fallback')
    await click('Galaxy Overview'); await waitFor(() => evaluate('!!document.querySelector(".go-galaxy-controls")')); await click('Local Volume · actual XYZ')
    if (live) {
      await rpc('configureGoSpatial', { downloads: true }); await rpc('installGoSpatial')
      let status; await waitFor(async () => { status = await rpc('getGoSpatialStatus'); return !status.busy }, 60000); if (!status.installed || status.error) throw Error('Signed spatial install: ' + JSON.stringify(status))
      await rpc('installGoBubble', { sha256: status.bubbleHash, bytes: status.bubbleBytes }); await waitFor(async () => { status = await rpc('getGoSpatialStatus'); return !status.busy }, 60000); if (!status.bubbleInstalled || status.error) throw Error('Bubble install: ' + JSON.stringify(status))
      report.install = status
      for (const [name, center] of [['Sol', [0, 0, 0]], ['Beagle-Point', [-1111.5625, -134.21875, 65269.75]]]) {
        await click('Fit Local Region'); await setCenter(center); await click('Retry cached view')
        const q = volume(center); let v; await waitFor(async () => { v = await rpc('getGoSpatialView', q); return v.complete && !v.pending }, 60000)
        await waitFor(() => evaluate('document.querySelectorAll("[data-spatial-projection] canvas").length===3&&Array.from(document.querySelectorAll("[data-spatial-projection] canvas")).every(c=>c.dataset.positions===' + JSON.stringify(String(v.points.length)) + ')'), 60000)
        await shot('real-' + name + '-local-volume'); report[name] = { query: q, count: v.points.length, blocks: v.blocks, decodedBytes: v.decodedBytes, source: v.source, release: v.release, resources: await measure() }
        const start = performance.now(); await rpc('getGoSpatialView', q); report[name].warmRPCMilliseconds = performance.now() - start
        if (name === 'Sol') {
          // Same XYZ frame, density is explicitly full line of sight.
          await rpc('checkGoAtlas'); await rpc('installGoAtlas'); await waitFor(async () => !(await rpc('getGoAtlasStatus')).busy, 60000); await rpc('configureGoAtlas', { details: true }); await click('Galaxy Overview'); await waitFor(() => evaluate('!!document.querySelector(".go-galaxy-controls")')); await waitFor(async () => !(await rpc('getGoAtlasView', { frame: { x: [q.low[0], q.high[0]], y: [q.low[1], q.high[1]], z: [q.low[2], q.high[2]] }, pixels: 1024 })).pending, 60000); await delay(1000); await shot('real-Sol-density-same-volume'); report.densityBaseline = await measure(); await click('Local Volume · actual XYZ')
        }
      }
      await rpc('configureGoSpatial', { downloads: true, offline: true }); const before = (await rpc('getGoSpatialStatus')).downloaded
      const cached = await rpc('getGoSpatialView', volume([0, 0, 0])); if (!cached.complete) throw Error('Offline Bubble unavailable')
      const after = await rpc('getGoSpatialStatus'); if (after.downloaded !== before) throw Error('Offline network traffic'); report.offline = { complete: true, positions: cached.points.length, status: after }
    } else {
      // Exact component in real Chromium; only coordinates are invented. No
      // replacement renderer or synthetic screenshot is claimed as live proof.
      const transform = source => transformSync(source, { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
      const code = transform(await fs.readFile(path.join(root, 'src/client/components/go-spatial.js'), 'utf8')); const projectCode = transform(await fs.readFile(path.join(root, 'src/client/lib/go-display-controller.js'), 'utf8'))
      await c.call('Emulation.setDeviceMetricsOverride', { width: 680, height: 580, deviceScaleFactor: 2, mobile: false })
      await evaluate(`(()=>{const effects=[];const projectModule={exports:{}};{const module=projectModule;const exports=module.exports;${projectCode}}const module={exports:{}};const jsx=(tag,p)=>{const e=tag==='canvas'?document.createElement(tag):document.createElementNS('http://www.w3.org/2000/svg',tag);for(const [k,v]of Object.entries(p||{})){if(k==='ref')v.current=e;else if(k==='style')Object.assign(e.style,v);else if(k==='children'){for(const child of [v].flat())if(child)e.append(child)}else if(k!=='key')e.setAttribute(k,v)}return e};const require=n=>n==='react'?{useEffect:f=>effects.push(f),useRef:()=>({current:null})}:n.includes('go-display-controller')?projectModule.exports:{jsx,jsxs:jsx};const exports=module.exports;${code};window.g3cCleanup=[];const frame={x:[-100,100],y:[-50,50],z:[-100,100]};const points=[[-0.03125,-0.0625,-0.125],[1.8125,1.25,1.71875]];for(const [i,axes]of ['xz','xy','yz'].entries()){const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.dataset.testSpatial=axes;svg.setAttribute('viewBox','0 0 100 100');Object.assign(svg.style,{position:'fixed',left:(20+i*280)+'px',top:'20px',width:'256px',height:'256px',zIndex:9999,background:'#01050d'});svg.append(module.exports.SpatialCanvas({points,axes,frame,width:100,height:100}));document.body.append(svg);window.g3cCleanup.push(effects.pop()())}})()`)
      report.canvas = await evaluate('Array.from(document.querySelectorAll("[data-test-spatial] canvas")).map(c=>({mode:c.dataset.mode,positions:Number(c.dataset.positions),width:c.width,height:c.height,pixels:Array.from(c.getContext("2d").getImageData(0,0,c.width,c.height).data).filter((v,i)=>i%4===3&&v>0).length}))')
      if (report.canvas.length !== 3 || report.canvas.some(c => c.positions !== 2 || c.pixels !== 2 || c.width > 2048)) throw Error('Exact-position Canvas regression: ' + JSON.stringify(report.canvas))
      await shot('synthetic-exact-xyz-canvases'); await evaluate('window.g3cCleanup.forEach(f=>f());document.querySelectorAll("[data-test-spatial]").forEach(e=>e.remove())')
    }
    const first = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); await delay(1500); const last = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); report.idle = { rendererHeapBytes: last.JSHeapUsedSize, mainThreadPercent: 100 * (last.TaskDuration - first.TaskDuration) / (last.Timestamp - first.Timestamp) }
    if (report.errors.length || report.external.length) throw Error('Browser error/external frontend traffic: ' + JSON.stringify(report.errors)); report.passed = true; console.log(JSON.stringify(report))
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
