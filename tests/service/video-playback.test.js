const test = require('node:test')
const assert = require('node:assert/strict')
const { selectVideo } = require('../../src/client/lib/video-playback')

test('viewer plays cached video without requesting host download and gets a clear uncached result', async () => {
  const sendEvent = async name => {
    assert.equal(name, 'videoGetCached')
    return [{ videoId: 'abcdefghijk', fileName: 'abcdefghijk.mp4' }]
  }
  assert.deepEqual(await selectVideo({ role: 'viewer', video: { id: 'abcdefghijk' }, sendEvent }), { state: 'playing', fileName: 'abcdefghijk.mp4' })
  await assert.rejects(selectVideo({ role: 'viewer', video: { id: 'lmnopqrstuv' }, sendEvent }), /Windows PC/)
})

test('owner retains downloading for an uncached video', async () => {
  let download
  const sendEvent = async (name, args) => {
    if (name === 'videoGetCached') return []
    assert.equal(name, 'videoDownload')
    download = args.url
  }
  assert.deepEqual(await selectVideo({ role: 'owner', video: { id: 'abcdefghijk' }, sendEvent }), { state: 'downloading' })
  assert.equal(download, 'abcdefghijk')
})
