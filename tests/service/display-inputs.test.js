const test = require('node:test')
const assert = require('node:assert/strict')
const Video = require('../../src/service/lib/event-handlers/video')
const Keybinds = require('../../src/service/lib/event-handlers/keybinds')
const fs = require('node:fs/promises')

test('video metadata accepts YouTube targets and rejects arbitrary hosts and option-like input before execution', async () => {
  const seen = []
  const video = new Video({
    manager: {
      getVideoInfo: target => { seen.push(target); return { id: 'abcdefghijk' } },
      getChannelVideos: target => { seen.push(target); return [] },
      search: query => { seen.push(query); return { videos: [] } }
    }
  })
  await video.videoGetInfo({ url: 'https://youtu.be/abcdefghijk?t=3' })
  await video.videoGetChannelVideos({ channelUrl: 'https://www.youtube.com/@Synthetic/videos' })
  assert.deepEqual(seen, ['https://www.youtube.com/watch?v=abcdefghijk', 'https://www.youtube.com/@Synthetic'])
  await video.videoGetInfo({ url: 'http://www.youtube.com/watch?v=abcdefghijk&list=ignored' })
  assert.equal(seen[2], 'https://www.youtube.com/watch?v=abcdefghijk')
  await video.videoGetChannelVideos({ channelUrl: 'https://www.youtube.com/@MagyarÁllomás/videos' })
  await video.videoGetChannelVideos({ channelUrl: 'https://www.youtube.com/@%E6%97%A5%E6%9C%AC%E8%AA%9E/videos' })
  assert.equal(seen[3], 'https://www.youtube.com/@Magyar%C3%81llom%C3%A1s')
  assert.equal(seen[4], 'https://www.youtube.com/@%E6%97%A5%E6%9C%AC%E8%AA%9E')
  for (const url of ['http://127.0.0.1/private', 'https://youtube.com.evil.example/watch?v=abcdefghijk', '--exec=anything', 'https://user@www.youtube.com/watch?v=abcdefghijk', 'https://www.youtube.com:444/watch?v=abcdefghijk']) {
    await assert.rejects(video.videoGetInfo({ url }))
    await assert.rejects(video.videoGetChannelVideos({ channelUrl: url }))
  }
  await assert.rejects(video.videoSearch({ query: 'x'.repeat(201) }))
  assert.equal(seen.length, 5)
  await assert.rejects(video.videoGetChannelVideos({ channelUrl: 'https://www.youtube.com/@bad%2F..%2Fredirect/videos' }))
})

test('keybinding requests reject path traversal before reading a file', async () => {
  const keybinds = new Keybinds()
  for (const preset of ['../other', '..\\other', '/tmp/other', 'C:\\other', '', null]) {
    await assert.rejects(keybinds.getKeybinds({ preset }), /Invalid preset/)
  }
})

test('ordinary preset names with spaces and version suffixes still load', async t => {
  t.mock.method(fs, 'readFile', async filename => {
    assert.ok(filename.endsWith('Synthetic Custom.4.2.binds'))
    return '<Root PresetName="Synthetic Custom">\n</Root>'
  })
  assert.deepEqual(await new Keybinds().getKeybinds({ preset: 'Synthetic Custom.4.2' }), { presetName: 'Synthetic Custom', bindings: {} })
})
