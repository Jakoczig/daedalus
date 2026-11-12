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
})
