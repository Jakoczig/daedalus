const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const YAML = require('yaml')

test('both desktop OSes have blocking build, security and real executable gates', () => {
  const workflow = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  const job = workflow.jobs.desktop
  assert.deepEqual(job.strategy.matrix.os, ['windows-latest', 'ubuntu-24.04'])
  assert.equal(job['continue-on-error'], undefined)
  for (const step of job.steps) {
    assert.equal(step['continue-on-error'], undefined)
    assert.ok(!String(step.run).includes('|| true'))
  }
  for (const name of ['Security regression tests', 'Security lint', 'Asset fixture tests', 'Build embedded React assets', 'Go tests', 'Go vet', 'Build native desktop', 'Executable offline smoke', 'Native React smoke']) {
    const step = job.steps.find(step => step.name === name)
    assert.ok(step && step.run, `missing gate: ${name}`)
    assert.equal(step.if, undefined, `gate skips one OS: ${name}`)
  }
  assert.ok(job.steps.some(step => step.name === 'Linux native libraries' && step.run.includes('libgtk-4-dev') && step.run.includes('libwebkitgtk-6.0-dev')))
  const legacy = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/ci.yml'), 'utf8'))
  assert.ok(legacy.on.pull_request.branches.includes('security/phase-1a-foundations'))
  assert.ok(legacy.on.pull_request.branches.includes('go/g1a-desktop-foundation'), 'legacy CI must check the G1B stacked target')
  assert.ok(workflow.on.pull_request.branches.includes('go/g1a-desktop-foundation'), 'desktop CI must check the G1B stacked target')
  assert.ok(workflow.on.pull_request.branches.includes('go/g1b-secure-remote-displays'), 'G2A stacked target is not checked')
  assert.ok(legacy.on.pull_request.branches.includes('go/g1b-secure-remote-displays'), 'legacy gates must also check G2A')
  assert.ok(workflow.on.push.branches.includes('go/g2a-journal-ingestion'))
  for (const name of ['Prepare synthetic journal fixture', 'Executable journal smoke', 'Native journal smoke', 'Benchmark journal replay']) {
    const step = job.steps.find(step => step.name === name)
    assert.ok(step && step.run, `missing G2A gate: ${name}`)
    assert.equal(step.if, undefined, `G2A gate skips one OS: ${name}`)
  }
  assert.match(job.steps.find(step => step.name === 'Executable journal smoke').run, /--journal-smoke-test.*--journal-dir/)
  assert.match(job.steps.find(step => step.name === 'Native journal smoke').run, /--desktop-journal-smoke-test/)
  assert.match(job.steps.find(step => step.name === 'Go tests').run, /-count=1/, 'idle/reconnect acceptance must execute freshly on both OSes')
  const pack = job.steps.findIndex(step => step.name === 'Package experimental preview')
  const upload = job.steps.findIndex(step => String(step.uses).startsWith('actions/upload-artifact@') && String(step.with?.name).startsWith('eridion-g3c-') && String(step.with?.name).endsWith('-x64-experimental-unsigned-preview'))
  assert.ok(pack > job.steps.findIndex(step => step.name === 'Native React smoke'), 'packaging must follow all security/native gates')
  assert.ok(pack > job.steps.findIndex(step => step.name === 'Native journal smoke'), 'artifact must wait for G2A native data validation')
  assert.ok(upload > pack, 'missing gated artifact upload')
  assert.equal(job.steps[upload].if, undefined, 'artifact upload must use default success gating')
  assert.match(job.steps[upload].with.name, /g3c-.*experimental-unsigned-preview/)
  assert.equal(job.steps[upload].with['if-no-files-found'], 'error')
  assert.match(job.steps[upload].with.path, /^\$\{\{ steps\.preview\.outputs\.directory \}\}$/)
})

test('G2B stacks on G2A and gates both preview uploads on native navigation and race/stress', () => {
  const workflow = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  const legacy = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/ci.yml'), 'utf8'))
  assert.ok(workflow.on.pull_request.branches.includes('go/g2a-journal-ingestion'))
  assert.ok(legacy.on.pull_request.branches.includes('go/g2a-journal-ingestion'))
  assert.ok(workflow.on.push.branches.includes('go/g2b-local-nav-exploration'))
  const steps = workflow.jobs.desktop.steps
  for (const name of ['Prepare synthetic navigation fixture', 'Executable navigation survey smoke', 'Native navigation survey smoke', 'Benchmark projection replay', 'G2B projection stress']) {
    const step = steps.find(s => s.name === name); assert.ok(step?.run, name); assert.equal(step.if, undefined)
  }
  assert.match(steps.find(s => s.name === 'Executable navigation survey smoke').run, /--navigation-smoke-test.*--journal-dir/)
  assert.match(steps.find(s => s.name === 'Native navigation survey smoke').run, /--desktop-navigation-smoke-test/)
  assert.match(steps.find(s => s.name === 'Race detector').run, /go test -race -count=1 -timeout=4m \.\/\.\.\./)
  assert.match(steps.find(s => s.name === 'Race detector').run, /-race -count=25.*TestG2B/)
  assert.ok(steps.findIndex(s => s.name === 'Package experimental preview') > steps.findIndex(s => s.name === 'Native navigation survey smoke'))
})

test('G2C final batch includes both native OSes, original regressions and extended race stress', () => {
  const workflow = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  assert.ok(workflow.on.pull_request.branches.includes('go/g2b-local-nav-exploration'))
  assert.ok(workflow.on.push.branches.includes('go/g2c-exploration-parity'))
  const steps = workflow.jobs.desktop.steps
  assert.match(steps.find(s => s.name === 'G2C details and organic stress').run, /-count=25.*TestG2C/)
  assert.match(steps.find(s => s.name === 'Race detector').run, /-race -count=25.*TestG2C/)
  assert.equal(steps.find(s => s.name === 'G2C details and organic stress').if, undefined)
  for (const name of ['Executable G2C exploration smoke', 'Native G2C exploration smoke']) {
    const step = steps.find(s => s.name === name); assert.ok(step?.run); assert.equal(step.if, undefined)
  }
  const legacy = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/ci.yml'), 'utf8'))
  assert.ok(legacy.on.pull_request.branches.includes('go/g2b-local-nav-exploration'))
})

test('G2D preview waits for terminal, security, visual and native gates on both OSes', () => {
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  assert.ok(w.on.pull_request.branches.includes('go/g2c-exploration-parity'))
  assert.ok(w.on.push.branches.includes('go/g2d-original-ui-parity'))
  const steps = w.jobs.desktop.steps; const pack = steps.findIndex(s => s.name === 'Package experimental preview')
  for (const name of ['Executable G2D terminal smoke', 'Native G2D terminal smoke', 'Real browser visual and interaction verification', 'G2D ship and revocation stress']) {
    const index = steps.findIndex(s => s.name === name); assert.ok(index >= 0 && index < pack); assert.equal(steps[index].if, undefined)
  }
  assert.match(steps.find(s => s.name === 'Race detector').run, /-race -count=25.*TestG2D/)
  const legacy = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/ci.yml'), 'utf8'))
  assert.ok(legacy.on.pull_request.branches.includes('go/g2c-exploration-parity'))
})

test('G2E preview requires new parity native, browser and race gates', () => {
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8')); const steps = w.jobs.desktop.steps
  assert.ok(w.on.pull_request.branches.includes('go/g2d-original-ui-parity')); assert.ok(!w.on.push.branches.includes('go/g2e-feature-parity'), 'G2E uses one mandatory PR-triggered run rather than duplicate push runs')
  const pack = steps.findIndex(s => s.name === 'Package experimental preview')
  for (const name of ['Executable G2E parity smoke', 'Native G2E parity smoke', 'G2E real browser parity and performance verification', 'G2E attribution and revocation stress']) { const i = steps.findIndex(s => s.name === name); assert.ok(i >= 0 && i < pack); assert.equal(steps[i].if, undefined) }
  assert.match(steps.find(s => s.name === 'Race detector').run, /-race -count=25.*TestG2E/)
})

test('G2F stacks on G2E with required offline provider and visual coverage', () => {
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  assert.ok(w.on.pull_request.branches.includes('go/g2e-feature-parity'))
  assert.ok(!w.on.push.branches.includes('go/g2f-usability-edsm'))
  const visual = fs.readFileSync(path.join(__dirname, '../../scripts/testing/g2e-visual.cjs'), 'utf8')
  assert.match(visual, /timeoutHides: true/)
  assert.match(visual, /community-map-/)
  assert.match(visual, /ownerDisableClears: true/)
})

test('G2G final companion and visual gates remain mandatory before packaging', () => {
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  assert.ok(w.on.pull_request.branches.includes('go/g2f-usability-edsm'))
  const steps = w.jobs.desktop.steps; const pack = steps.findIndex(s => s.name === 'Package experimental preview')
  for (const name of ['Native G2G companion lifecycle smoke', 'G2G launcher navigation galaxy and display-controller visuals']) { const i = steps.findIndex(s => s.name === name); assert.ok(i > 0 && i < pack); assert.equal(steps[i].if, undefined) }
})

test('G2H keeps cross-platform native/security gates and new provider/visual checks before previews', () => {
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8')); assert.ok(w.on.pull_request.branches.includes('go/g2g-navigation-launcher')); assert.ok(!w.on.push.branches.includes('go/g2h-unified-navigation-integrations')); const steps = w.jobs.desktop.steps; const pack = steps.findIndex(s => s.name === 'Package experimental preview'); for (const name of ['G2H orbital statistics galaxy and provider privacy visuals', 'G2H provider observation benchmark']) { const i = steps.findIndex(s => s.name === name); assert.ok(i > 0 && i < pack); assert.equal(steps[i].if, undefined) }
})

test('G2J stacks on G2H with native SQLite, database performance and privacy gates', () => {
  for (const file of ['go-desktop.yml', 'ci.yml']) {
    const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows', file), 'utf8'))
    assert.ok(w.on.pull_request.branches.includes('go/g2h-unified-navigation-integrations'), 'G2J target must trigger CI')
  }
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  assert.equal(w.jobs.desktop.env.CGO_ENABLED, '1')
  const steps = w.jobs.desktop.steps; const pack = steps.findIndex(s => s.name === 'Package experimental preview')
  for (const name of ['Verify native SQLite compiler', 'G2J database lifecycle performance', 'G2J database history and privacy visuals']) { const i = steps.findIndex(s => s.name === name); assert.ok(i >= 0 && i < pack); assert.equal(steps[i].if, undefined) }
  assert.match(steps.find(s => s.name === 'G2J database lifecycle performance').run, /BenchmarkG2HExactLifecycle/)
  assert.match(steps.find(s => s.name === 'Go tests').run, /\.\/\.\.\./)
})

test('G2K stacked base and mandatory field-operation evidence are gated before packaging', () => {
  for (const file of ['go-desktop.yml', 'ci.yml']) { const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows', file), 'utf8')); assert.ok(w.on.pull_request.branches.includes('go/g2j-persistent-history-integrations')) }
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8')); const steps = w.jobs.desktop.steps; const pack = steps.findIndex(s => s.name === 'Package experimental preview')
  for (const name of ['G2K exact G2J lifecycle comparison and field query performance', 'G2K field operations browser verification']) { const i = steps.findIndex(s => s.name === name); assert.ok(i >= 0 && i < pack); assert.equal(steps[i].if, undefined) }
})

test('G2L stacked base, operations evidence and preview remain mandatory', () => {
  for (const file of ['go-desktop.yml', 'ci.yml']) { const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows', file), 'utf8')); assert.ok(w.on.pull_request.branches.includes('go/g2k-native-field-operations')) }
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8')); const steps = w.jobs.desktop.steps; const pack = steps.findIndex(s => s.name === 'Package experimental preview')
  for (const name of ['G2L exact G2K lifecycle baseline and operations queries', 'G2L owner operations and upload-disabled market browser smoke']) { const i = steps.findIndex(s => s.name === name); assert.ok(i >= 0 && i < pack); assert.equal(steps[i].if, undefined) }
})

test('G2M-B stacked base, complete regression gates and new exact geometry evidence precede preview', () => {
  const workflow = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  const legacy = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/ci.yml'), 'utf8'))
  for (const w of [workflow, legacy])assert.ok(w.on.pull_request.branches.includes('go/g2m-a-eridion-identity-frontier'))
  const steps = workflow.jobs.desktop.steps
  const i = steps.findIndex(s => s.name === 'G2M-B navigation, Spansh, activity and exact galaxy geometry')
  assert.ok(i > steps.findIndex(s => s.name === 'G2M-A commander, fleet, goals and account browser smoke'))
  assert.ok(i < steps.findIndex(s => s.name === 'Package experimental preview'))
  assert.equal(steps[i].if, undefined)
  assert.equal(steps[i]['continue-on-error'], undefined)
})

test('G2M-C stacked target runs both workflows and keyboard/startup browser gates before previews', () => {
  for (const file of ['go-desktop.yml', 'ci.yml']) {
    const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows', file), 'utf8'))
    assert.ok(w.on.pull_request.branches.includes('go/g2m-b-navigation-field-context'))
  }
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  const i = w.jobs.desktop.steps.findIndex(s => s.name === 'G2M-C verified startup and keyboard navigation browser regressions')
  assert.ok(i > 0 && i < w.jobs.desktop.steps.findIndex(s => s.name === 'Package experimental preview'))
  assert.equal(w.jobs.desktop.steps[i].if, undefined)
})

test('G2N stacks on G2M-C and gates previews on real account/fleet/goal browser checks', () => {
  for (const file of ['go-desktop.yml', 'ci.yml']) {
    const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows', file), 'utf8'))
    assert.ok(w.on.pull_request.branches.includes('go/g2m-c-live-audit-functional-corrections'))
  }
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  const i = w.jobs.desktop.steps.findIndex(s => s.name === 'G2N account choice goals fleet and activity browser verification')
  assert.ok(i > 0 && i < w.jobs.desktop.steps.findIndex(s => s.name === 'Package experimental preview'))
  assert.equal(w.jobs.desktop.steps[i].if, undefined)
})

test('G2O stacked target, offline visual gate and cache benchmark are mandatory', () => {
  const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/go-desktop.yml'), 'utf8'))
  const legacy = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows/ci.yml'), 'utf8'))
  assert.ok(w.on.pull_request.branches.includes('go/g2n-commander-fleet-completion'))
  assert.ok(legacy.on.pull_request.branches.includes('go/g2n-commander-fleet-completion'))
  const visual = w.jobs.desktop.steps.find(s => s.name === 'G2O offline atlas controls and projection browser verification')
  assert.ok(visual); assert.equal(visual.if, undefined); assert.doesNotMatch(visual.run, /--live/)
})

test('G3C stacks on G2Q and keeps spatial browser verification offline and mandatory', () => {
  for (const name of ['go-desktop.yml', 'ci.yml']) {
    const w = YAML.parse(fs.readFileSync(path.join(__dirname, '../../.github/workflows', name), 'utf8'))
    assert.ok(w.on.pull_request.branches.includes('go/g2q-galaxy-map-fidelity'))
    if (name === 'go-desktop.yml') {
      const index = w.jobs.desktop.steps.findIndex(s => s.name === 'G3C hybrid spatial map geometry and exact-coordinate Canvas')
      assert.ok(index > 0 && index < w.jobs.desktop.steps.findIndex(s => s.name === 'Package experimental preview'))
      assert.equal(w.jobs.desktop.steps[index].if, undefined)
      assert.doesNotMatch(w.jobs.desktop.steps[index].run, /--live/)
    }
  }
})
