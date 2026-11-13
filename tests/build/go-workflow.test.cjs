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
  const upload = job.steps.findIndex(step => String(step.uses).startsWith('actions/upload-artifact@'))
  assert.ok(pack > job.steps.findIndex(step => step.name === 'Native React smoke'), 'packaging must follow all security/native gates')
  assert.ok(pack > job.steps.findIndex(step => step.name === 'Native journal smoke'), 'artifact must wait for G2A native data validation')
  assert.ok(upload > pack, 'missing gated artifact upload')
  assert.equal(job.steps[upload].if, undefined, 'artifact upload must use default success gating')
  assert.match(job.steps[upload].with.name, /g2c-.*experimental-unsigned-local-exploration-biology-no-uploads/)
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
