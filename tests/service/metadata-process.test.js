const test = require('node:test')
const assert = require('node:assert/strict')
const { EventEmitter } = require('node:events')
const { createMetadataRunner } = require('../../src/service/lib/metadata-process')

test('metadata runs in simulation with fixed safety options and bounded concurrency/output', () => {
  const calls = []
  const run = createMetadataRunner({
    maxBytes: 4,
    spawnProcess: (binary, args, options) => {
      const proc = new EventEmitter()
      proc.stdout = new EventEmitter()
      proc.stderr = new EventEmitter()
      proc.kill = () => { proc.killed = true }
      calls.push({ args, options, proc })
      return proc
    }
  })
  const first = run('test-fixture', ['--dump-json', 'https://www.youtube.com/watch?v=abcdefghijk'], {})
  run('test-fixture', [], {})
  assert.throws(() => run('test-fixture', [], {}), /busy/)
  assert.ok(calls[0].args.includes('--ignore-config'))
  assert.ok(calls[0].args.includes('--simulate'))
  assert.ok(calls[0].args.includes('--no-cache-dir'))
  assert.equal(calls[0].options.timeout, 20000)
  assert.equal(calls[0].args.includes('--no-check-certificates'), false)
  let error
  first.on('error', e => { error = e })
  first.stdout.emit('data', Buffer.from('12345'))
  assert.equal(first.killed, true)
  assert.match(error.message, /output limit/)
  assert.throws(() => run('test-fixture', [], {}), /busy/)
  first.emit('close', 1)
  assert.doesNotThrow(() => run('test-fixture', [], {}))
})
