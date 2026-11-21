const fs = require('node:fs/promises')
const path = require('node:path')
const { transformSync } = require('next/dist/compiled/babel/core')
const { fixture, browser, waitFor, pageTextReady, delay } = require('./g2d-visual.cjs')
const root = path.resolve(__dirname, '../..')
async function main () {
  const live = process.argv.includes('--live'); const output = path.join(root, live ? 'temp/g2q-live' : 'temp/g2q-visual'); await fs.mkdir(output, { recursive: true })
  const report = { scope: live ? 'Real pinned public atlas; synthetic navigation, read-only downloads' : 'Synthetic published-format PNG and real Chromium, offline provider isolation', errors: [], external: [], pages: [] }
  const b = await browser(output); const c = b.client; let host; let origin = ''; let localTileRequests = 0; let viewRequest
  const fixtureData = JSON.parse(await fs.readFile(path.join(root, 'go/internal/atlas/testdata/publisher-fixture.json'), 'utf8'))
  const tileURL = '/atlas/assets/aaaaaaaaaaaaaaaaaaaaaaaa/tiles/xz/7/0-0.png'
  const evaluate = async expression => { const r = await c.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  c.on(e => {
    if (e.method === 'Runtime.exceptionThrown') report.errors.push(e.params.exceptionDetails.exception?.description)
    if (e.method === 'Network.webSocketFrameSent') { try { const q = JSON.parse(e.params.response.payloadData); if (q.name === 'getGoAtlasView') viewRequest = q.message } catch {} }
    if (e.method === 'Fetch.requestPaused') {
      const p = e.params
      if (p.request.url === origin + tileURL) { localTileRequests++; c.call('Fetch.fulfillRequest', { requestId: p.requestId, responseCode: 200, responseHeaders: [{ name: 'Content-Type', value: 'image/png' }], body: fixtureData.tile }).catch(() => {}); return }
      const ok = p.request.url.startsWith(origin + '/') || p.request.url.startsWith('data:'); if (!ok) report.external.push(p.request.url)
      c.call(ok ? 'Fetch.continueRequest' : 'Fetch.failRequest', ok ? { requestId: p.requestId } : { requestId: p.requestId, errorReason: 'BlockedByClient' }).catch(() => {})
    }
  })
  await c.call('Network.enable'); await c.call('Fetch.enable', { patterns: [{ urlPattern: '*' }] })
  const rpc = (name, message = {}) => evaluate(`new Promise((resolve,reject)=>{const w=new WebSocket(location.origin.replace('http:','ws:'));const timer=setTimeout(()=>{w.close();reject(Error('fixture RPC timeout'))},30000);w.onopen=()=>w.send(JSON.stringify({requestId:'g2q',name:${JSON.stringify(name)},message:${JSON.stringify(message)}}));w.onmessage=e=>{const r=JSON.parse(e.data);if(r.requestId==='g2q'){clearTimeout(timer);w.close();r.error?reject(Error(r.error)):resolve(r.message)}}})`)
  const shot = async name => { const r = await c.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await fs.writeFile(path.join(output, name + '.png'), Buffer.from(r.data, 'base64')) }
  try {
    host = await fixture('G2E', '', path.join(root, 'tests/testdata/g2g')); origin = host.url
    for (const [name, width, height, dpr] of [['laptop', 1366, 768, 1], ['compact', 680, 580, 2]]) {
      await c.call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: dpr, mobile: false }); await c.call('Page.navigate', { url: origin + '/nav/galaxy' }); await waitFor(() => evaluate(pageTextReady('Galaxy Map', true))); await evaluate('document.fonts.ready.then(()=>true)')
      const geometry = await evaluate('(()=>{const outer=document.querySelector(\'.go-galaxy-xz>svg\');const tile=document.createElementNS(\'http://www.w3.org/2000/svg\',\'svg\');tile.setAttribute(\'width\',\'20\');tile.setAttribute(\'height\',\'30\');const fill=document.createElementNS(\'http://www.w3.org/2000/svg\',\'rect\');fill.setAttribute(\'width\',\'100%\');fill.setAttribute(\'height\',\'100%\');tile.append(fill);outer.append(tile);const a=outer.getBoundingClientRect(),b=tile.getBoundingClientRect(),style=getComputedStyle(tile);const info={outer:a.width,tile:b.width,ratio:b.width/a.width,cssWidth:style.width,cssHeight:style.height,display:style.display,viewBox:outer.getAttribute("viewBox")};tile.remove();return info})()')
      if (Math.abs(geometry.ratio - 0.2) > 0.015) throw Error('Tile viewport stretched by outer projection CSS: ' + JSON.stringify(geometry))
      const input = 'Array.from(document.querySelectorAll(".go-galaxy-controls label")).find(l=>l.textContent==="Show Route").querySelector("input")'
      if (!await evaluate('!!document.querySelector(".go-galaxy-route")')) throw Error('Default route missing')
      await evaluate(input + '.click()'); if (await evaluate('!!document.querySelector(".go-galaxy-route")')) throw Error('Hide route failed')
      if (await evaluate('document.querySelectorAll("[data-marker=current]").length') !== 3) throw Error('Route toggle hid current-system markers')
      await evaluate(input + '.click()'); await shot('overlays-' + name); report.pages.push({ name, dpr, geometry, routeToggle: true })
    }
    // Mount the exact canvas module in a tiny test-only hook host. Real DOM,
    // decoding, alpha and canvas are exercised; the PNG/coordinates are synthetic.
    const source = await fs.readFile(path.join(root, 'src/client/components/go-atlas-canvas.js'), 'utf8')
    const code = transformSync(source, { babelrc: false, configFile: false, presets: [[require('next/dist/compiled/babel/preset-react'), { runtime: 'automatic' }]], plugins: [require('next/dist/compiled/babel/plugin-transform-modules-commonjs')] }).code
    await c.call('Emulation.setDeviceMetricsOverride', { width: 1366, height: 768, deviceScaleFactor: 1, mobile: false })
    await evaluate(`(()=>{const effects=[];const module={exports:{}};const jsx=(tag,p)=>{const e=tag==='canvas'?document.createElement(tag):document.createElementNS('http://www.w3.org/2000/svg',tag);for(const [k,v]of Object.entries(p||{})){if(k==='ref')v.current=e;else if(k==='style')Object.assign(e.style,v);else if(k==='children'){for(const child of [v].flat())if(child)e.append(child)}else if(k!=='key')e.setAttribute(k,v)}return e};const require=n=>n==='react'?{useEffect:f=>effects.push(f),useRef:()=>({current:null})}:{jsx,jsxs:jsx};const exports=module.exports;${code};window.g2qModule=module.exports;window.g2qEffects=effects;window.g2qMount=frame=>{window.g2qCleanup?.();document.getElementById('g2q-canvas-test')?.remove();const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.id='g2q-canvas-test';svg.setAttribute('viewBox','0 0 100 100');Object.assign(svg.style,{position:'fixed',inset:'20px',width:'512px',height:'512px',zIndex:9999,background:'#01050d'});const extents={x:[-65000,65000],y:[-15000,15000],z:[-35000,95000]};const node=module.exports.default({tiles:[{axes:'xz',level:7,sourceLevel:7,x:0,y:0,sourceX:0,sourceY:0,url:${JSON.stringify(tileURL)}}],extents,frame,width:100,height:100,viewKey:JSON.stringify(frame),onReady:()=>{window.g2qReady=true}});svg.append(node);document.body.append(svg);window.g2qReady=false;window.g2qCleanup=effects.pop()()};window.g2qMount({x:[-65000,-64900],y:[-50,50],z:[94900,95000]})})()`)
    await waitFor(() => evaluate('window.g2qReady'))
    report.canvas = await evaluate('(()=>{const c=document.querySelector("#g2q-canvas-test canvas");return {mode:c.dataset.mode,symbols:Number(c.dataset.symbols),width:c.width,height:c.height}})()')
    if (report.canvas.mode !== 'occupied-bins' || report.canvas.symbols > 12000) throw Error('Close zoom renderer did not use bounded occupied bins')
    await shot('synthetic-occupied-bins'); const before = localTileRequests
    await evaluate('window.g2qMount({x:[-64999,-64899],y:[-50,50],z:[94900,95000]})'); await waitFor(() => evaluate('window.g2qReady'))
    if (localTileRequests !== before) throw Error('Panning refetched an unchanged decoded source')
    await evaluate('window.g2qCleanup();document.getElementById("g2q-canvas-test").remove()')
    if (live) {
      await rpc('checkGoAtlas'); await rpc('installGoAtlas'); await waitFor(async () => { const s = await rpc('getGoAtlasStatus'); if (!s.busy && s.error) throw Error(s.error); return !s.busy && s.installed }, 60000)
      await c.call('Page.navigate', { url: origin + '/nav/galaxy' }); await waitFor(() => evaluate(pageTextReady('Galaxy Map', true))); await waitFor(() => evaluate('document.body.textContent.includes("fallback")'))
      await shot('real-L4-fallback-route'); await evaluate('Array.from(document.querySelectorAll(".go-galaxy-controls label")).find(l=>l.textContent==="Show Route").querySelector("input").click()'); await shot('real-L4-fallback-no-route')
      await rpc('configureGoAtlas', { automatic: false, details: true, metered: false }); await evaluate('window.dispatchEvent(new Event("eridion-atlas-changed"))')
      await waitFor(() => evaluate('!!document.querySelector(".go-galaxy-xz canvas[data-mode=occupied-bins]")&&Array.from(document.querySelectorAll("[data-source-level]")).filter(e=>e.getAttribute("data-source-level")==="7").length>=3'), 60000)
      await waitFor(() => evaluate('document.querySelector(".go-galaxy-source").textContent.includes("Displayed L7 · 0 unavailable")'), 60000)
      await shot('real-L7-occupied-bins-no-route'); report.liveStatus = await rpc('getGoAtlasStatus'); report.liveView = await rpc('getGoAtlasView', viewRequest)
      const densityPNG = await evaluate('(()=>{const source=document.querySelector(".go-galaxy-xz canvas");const out=document.createElement("canvas");out.width=source.width;out.height=source.height;const ctx=out.getContext("2d");ctx.fillStyle="#01050d";ctx.fillRect(0,0,out.width,out.height);ctx.drawImage(source,0,0);return out.toDataURL("image/png").split(",")[1]})()')
      await fs.writeFile(path.join(output, 'eridion-density-only.png'), Buffer.from(densityPNG, 'base64'))
      const painter = path.join(output, 'painter-reference'); await fs.mkdir(painter, { recursive: true })
      for (const tile of report.liveView.tiles.filter(t => t.sourceLevel === 7)) {
        const data = await evaluate(`fetch(${JSON.stringify(tile.url)}).then(r=>{if(!r.ok)throw Error('verified tile missing');return r.arrayBuffer()}).then(b=>Array.from(new Uint8Array(b)))`)
        const file = path.join(painter, tile.axes, '7', `${tile.sourceX}-${tile.sourceY}.png`); await fs.mkdir(path.dirname(file), { recursive: true }); await fs.writeFile(file, Buffer.from(data))
      }
      await fs.writeFile(path.join(output, 'view-request.json'), JSON.stringify(viewRequest, null, 2))
      report.liveCanvas = await evaluate('Array.from(document.querySelectorAll(".go-galaxy-plane canvas")).map(c=>({mode:c.dataset.mode,symbols:Number(c.dataset.symbols),width:c.width,height:c.height}))')
    }
    const first = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value])); await delay(1500); const last = Object.fromEntries((await c.call('Performance.getMetrics')).metrics.map(m => [m.name, m.value]))
    report.idle = { rendererHeapBytes: last.JSHeapUsedSize, mainThreadPercent: 100 * (last.TaskDuration - first.TaskDuration) / (last.Timestamp - first.Timestamp), localSyntheticTileRequests: localTileRequests }
    if (report.errors.length || report.external.length) throw Error('Unexpected browser errors or external frontend requests')
    report.passed = true; console.log(JSON.stringify(report))
  } finally { if (host) await host.stop(); await b.stop(); await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2)) }
}
if (require.main === module) main().catch(e => { console.error(e); process.exitCode = 1 })
