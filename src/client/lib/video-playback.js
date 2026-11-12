async function selectVideo ({ role, video, sendEvent }) {
  const cached = await sendEvent('videoGetCached')
  const match = cached.find(entry => entry.videoId === video.id)
  if (match) return { state: 'playing', fileName: match.fileName }
  if (role !== 'owner') throw new Error('Download this video on the Windows PC first, then select it here.')
  sendEvent('videoDownload', { url: video.url || video.id }, 300000).catch(() => {})
  return { state: 'downloading' }
}

module.exports = { selectVideo }
