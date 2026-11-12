'use strict'

function youtubeTarget (input, channel = false) {
  if (typeof input !== 'string' || input.length > 2048) throw new Error('Invalid YouTube target')
  if (!channel && /^[\w-]{11}$/.test(input)) return `https://www.youtube.com/watch?v=${input}`
  const url = new URL(input)
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.port || !['www.youtube.com', 'youtube.com', 'm.youtube.com', 'youtu.be'].includes(url.hostname)) throw new Error('Invalid YouTube target')
  if (channel) {
    const pathname = decodeURIComponent(url.pathname)
    if (url.hostname === 'youtu.be' || !/^\/(?:@[\p{L}\p{M}\p{N}_.-]+|(?:channel|c|user)\/[\p{L}\p{M}\p{N}_.-]+)(?:\/videos)?\/?$/u.test(pathname)) throw new Error('Invalid YouTube channel')
    url.pathname = pathname.replace(/\/$/, '').replace(/\/videos$/, '')
    return `https://www.youtube.com${url.pathname}`
  }
  const id = url.hostname === 'youtu.be' ? url.pathname.slice(1) : url.pathname === '/watch' ? url.searchParams.get('v') : /^\/(shorts|live)\//.test(url.pathname) ? url.pathname.split('/')[2] : ''
  if (!/^[\w-]{11}$/.test(id || '')) throw new Error('Invalid YouTube video')
  return `https://www.youtube.com/watch?v=${id}`
}

class Video {
  constructor ({ manager = require('../video-manager') } = {}) { this.manager = manager }

  async videoSearch ({ query }) {
    if (typeof query !== 'string' || !query.trim() || query.length > 200) throw new Error('Invalid search query')
    return this.manager.search(query.trim())
  }

  async videoGetInfo ({ url }) {
    if (!url || typeof url !== 'string') throw new Error('Missing video URL or ID')
    return this.manager.getVideoInfo(youtubeTarget(url.trim()))
  }

  async videoGetChannelVideos ({ channelUrl }) {
    if (!channelUrl || typeof channelUrl !== 'string') throw new Error('Missing channel URL')
    return this.manager.getChannelVideos(youtubeTarget(channelUrl.trim(), true))
  }

  async videoDownload ({ url }) {
    if (!url || typeof url !== 'string') throw new Error('Missing video URL or ID')
    return this.manager.download(url.trim())
  }

  async videoCancelDownload () {
    this.manager.cancelDownload()
    return { cancelled: true }
  }

  async videoGetCached () {
    return this.manager.listCached()
  }

  async videoCheckStatus () {
    return {
      available: this.manager.ytdlpAvailable()
    }
  }

  getHandlers () {
    return {
      videoSearch: (msg) => this.videoSearch(msg),
      videoGetInfo: (msg) => this.videoGetInfo(msg),
      videoGetChannelVideos: (msg) => this.videoGetChannelVideos(msg),
      videoDownload: (msg) => this.videoDownload(msg),
      videoCancelDownload: () => this.videoCancelDownload(),
      videoGetCached: () => this.videoGetCached(),
      videoCheckStatus: () => this.videoCheckStatus()
    }
  }
}

module.exports = Video
