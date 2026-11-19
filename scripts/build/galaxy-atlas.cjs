// ERIDION observed-system density atlas. Offline only; never opens network connections.
// Input: EDSM systemsWithCoordinates JSON (optionally gzip), or user-supplied x,y,z CSV.
const fs = require('node:fs')
const fsp = require('node:fs/promises')
const path = require('node:path')
const crypto = require('node:crypto')
const zlib = require('node:zlib')
const readline = require('node:readline')

const TILE = 256
const EXTENTS = Object.freeze({ x: [-65000, 65000], y: [-15000, 15000], z: [-35000, 95000] })
const AXES = ['xz', 'xy', 'yz']

function validateCoordinates (coords) {
  return coords && ['x', 'y', 'z'].every(a => typeof coords[a] === 'number' && Number.isFinite(coords[a]))
}

function dimensions (axes, level) {
  const a = EXTENTS[axes[0]]
  const b = EXTENTS[axes[1]]
  const width = TILE * 2 ** level
  return { width, height: Math.max(1, Math.round(width * (b[1] - b[0]) / (a[1] - a[0]))) }
}

function projectBin (coords, axes, width, height) {
  if (!validateCoordinates(coords)) return null
  const [a, b] = axes
  const horizontal = EXTENTS[a]
  const vertical = EXTENTS[b]
  const x = Math.floor((coords[a] - horizontal[0]) * width / (horizontal[1] - horizontal[0]))
  const y = Math.floor((vertical[1] - coords[b]) * height / (vertical[1] - vertical[0]))
  if (x < 0 || x >= width || y < 0 || y >= height) return null
  return { x, y }
}

// Unlike line-based parsing, this accepts an EDSM JSON array regardless of
// whitespace or chunk boundaries. Nesting, quoted braces and escapes are handled.
async function * jsonObjects (stream) {
  stream.setEncoding('utf8')
  let opened = false
  let closed = false
  let expectingValue = true
  let afterComma = false
  let depth = 0
  let quoted = false
  let escaped = false
  let object = ''
  for await (const part of stream) {
    for (const char of part) {
      if (closed) {
        if (!/\s/.test(char)) throw new Error('Unexpected trailing JSON data')
        continue
      }
      if (!opened) {
        if (/\s/.test(char)) continue
        if (char !== '[') throw new Error('Expected top-level JSON array')
        opened = true
        continue
      }
      if (depth === 0) {
        if (/\s/.test(char)) continue
        if (expectingValue) {
          if (char === ']' && !afterComma) { closed = true; continue }
          if (char !== '{') throw new Error('Expected system object')
          depth = 1
          object = '{'
          afterComma = false
          continue
        }
        if (char === ',') { expectingValue = true; afterComma = true; continue }
        if (char === ']') { closed = true; continue }
        throw new Error('Expected comma or end of array')
      }
      object += char
      if (object.length > 1024 * 1024) throw new Error('Overlarge system record')
      if (quoted) {
        if (escaped) escaped = false
        else if (char === '\\') escaped = true
        else if (char === '"') quoted = false
      } else if (char === '"') quoted = true
      else if (char === '{') depth++
      else if (char === '}') depth--
      if (depth === 0) {
        yield JSON.parse(object)
        object = ''
        expectingValue = false
      }
    }
  }
  if (!opened || !closed || depth) throw new Error('Truncated JSON array')
}

async function * coordinatesFrom (stream, format) {
  if (format === 'csv') {
    stream.setEncoding('utf8')
    const lines = readline.createInterface({ input: stream, crlfDelay: Infinity })
    let header = false
    for await (const line of lines) {
      if (!header) {
        if (line.trim().replace(/^\uFEFF/, '').toLowerCase() !== 'x,y,z') throw new Error('CSV requires x,y,z header')
        header = true
        continue
      }
      if (!line.trim()) continue
      const columns = line.split(',')
      const numbers = columns.map(Number)
      yield columns.length === 3 && columns.every(s => s.trim() !== '') && numbers.every(Number.isFinite)
        ? { x: numbers[0], y: numbers[1], z: numbers[2] }
        : null
    }
    if (!header) throw new Error('Missing CSV header')
    return
  }
  for await (const record of jsonObjects(stream)) {
    // EDSM API defines coords as {x,y,z}, absent when unknown.
    yield record && record.coords
  }
}

function halve (grid) {
  const width = Math.ceil(grid.width / 2)
  const height = Math.ceil(grid.height / 2)
  const counts = new Uint32Array(width * height)
  for (let y = 0; y < grid.height; y++) {
    const source = y * grid.width
    const target = Math.floor(y / 2) * width
    for (let x = 0; x < grid.width; x++) counts[target + (x >> 1)] += grid.counts[source + x]
  }
  return { width, height, counts }
}

let crcTable
function crc32 (buffer) {
  if (!crcTable) {
    crcTable = Array.from({ length: 256 }, (_, n) => {
      let c = n
      for (let j = 0; j < 8; j++) c = (c & 1) ? (0xedb88320 ^ (c >>> 1)) : (c >>> 1)
      return c >>> 0
    })
  }
  let crc = 0xffffffff
  for (const byte of buffer) crc = crcTable[(crc ^ byte) & 255] ^ (crc >>> 8)
  return (crc ^ 0xffffffff) >>> 0
}
function pngChunk (name, bytes) {
  const type = Buffer.from(name)
  const length = Buffer.allocUnsafe(4)
  length.writeUInt32BE(bytes.length)
  const crc = Buffer.allocUnsafe(4)
  crc.writeUInt32BE(crc32(Buffer.concat([type, bytes])))
  return Buffer.concat([length, type, bytes, crc])
}
function densityPng (grid, x0, y0, width, height, maximum) {
  const raw = Buffer.alloc((width * 4 + 1) * height)
  const divisor = Math.log1p(Math.max(1, maximum))
  for (let y = 0; y < height; y++) {
    const offset = y * (width * 4 + 1)
    for (let x = 0; x < width; x++) {
      const value = grid.counts[(y0 + y) * grid.width + x0 + x]
      if (!value) continue
      const intensity = Math.min(1, Math.log1p(value) / divisor)
      const i = offset + 1 + 4 * x
      raw[i] = Math.round(18 + 236 * intensity ** 1.3)
      raw[i + 1] = Math.round(104 + 125 * intensity)
      raw[i + 2] = Math.round(153 - 90 * intensity)
      raw[i + 3] = Math.round(100 + 155 * Math.sqrt(intensity))
    }
  }
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(width)
  ihdr.writeUInt32BE(height, 4)
  ihdr[8] = 8
  ihdr[9] = 6 // RGBA
  return Buffer.concat([
    Buffer.from('89504e470d0a1a0a', 'hex'),
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', zlib.deflateSync(raw, { level: 6 })),
    pngChunk('IEND', Buffer.alloc(0))
  ])
}

async function generate ({ directory, input, format = 'edsm-json-gz', sourceUrl, sourceDate, maxLevel = 4 }) {
  if (!input || !directory) throw new Error('Both --input and --out are required; no illustrative fallback exists')
  if (!['edsm-json-gz', 'edsm-json', 'csv'].includes(format)) throw new Error('Unsupported input format')
  if (!Number.isInteger(maxLevel) || maxLevel < 1 || maxLevel > 5) throw new Error('maxLevel must be 1..5')
  const output = path.resolve(directory)
  const source = path.resolve(input)
  if (output === source || output.startsWith(source + path.sep) || source.startsWith(output + path.sep)) throw new Error('Input and output paths overlap')
  if (!(await fsp.stat(source)).isFile()) throw new Error('Input must be a file')
  const grids = Object.fromEntries(AXES.map(axes => {
    const { width, height } = dimensions(axes, maxLevel)
    return [axes, { width, height, counts: new Uint32Array(width * height) }]
  }))
  const sha = crypto.createHash('sha256')
  let compressedBytes = 0
  const raw = fs.createReadStream(source, { highWaterMark: 256 * 1024 })
  raw.on('data', data => { sha.update(data); compressedBytes += data.length })
  const stream = format === 'edsm-json-gz' ? raw.pipe(zlib.createGunzip()) : raw
  const stats = { read: 0, accepted: 0, invalid: 0, outside: 0 }
  for await (const coords of coordinatesFrom(stream, format)) {
    stats.read++
    if (!validateCoordinates(coords)) { stats.invalid++; continue }
    const targets = AXES.map(axes => projectBin(coords, axes, grids[axes].width, grids[axes].height))
    if (targets.some(p => !p)) { stats.outside++; continue }
    stats.accepted++
    AXES.forEach((axes, i) => {
      const p = targets[i]
      const grid = grids[axes]
      grid.counts[p.y * grid.width + p.x]++
    })
  }
  if (stats.accepted === 0) throw new Error('No valid in-bounds coordinates; refusing to publish an empty atlas')
  const digest = sha.digest('hex')
  const run = digest.slice(0, 16) + '-' + Date.now().toString(36)
  await fsp.mkdir(output, { recursive: true })
  const staging = path.join(output, '.pending-' + crypto.randomBytes(8).toString('hex'))
  const root = path.join(output, 'tiles')
  const views = {}
  let files = 0
  await fsp.mkdir(staging, { recursive: true })
  try {
    const levels = {}
    for (const axes of AXES) {
      let grid = grids[axes]
      levels[axes] = []
      for (let level = maxLevel; level >= 0; level--) {
        levels[axes][level] = grid
        if (level) grid = halve(grid)
      }
      views[axes] = {
        axes: axes.split(''),
        width: grids[axes].width,
        height: grids[axes].height,
        levels: levels[axes].map((g, level) => ({
          level,
          width: g.width,
          height: g.height,
          columns: Math.ceil(g.width / TILE),
          rows: Math.ceil(g.height / TILE)
        }))
      }
    }
    for (let level = 0; level <= maxLevel; level++) {
      let maximum = 1
      for (const axes of AXES) {
        for (const count of levels[axes][level].counts) maximum = Math.max(maximum, count)
      }
      for (const axes of AXES) {
        const grid = levels[axes][level]
        const dir = path.join(staging, axes, String(level))
        await fsp.mkdir(dir, { recursive: true })
        for (let y = 0; y < Math.ceil(grid.height / TILE); y++) {
          for (let x = 0; x < Math.ceil(grid.width / TILE); x++) {
            const width = Math.min(TILE, grid.width - x * TILE)
            const height = Math.min(TILE, grid.height - y * TILE)
            await fsp.writeFile(path.join(dir, x + '-' + y + '.png'), densityPng(grid, x * TILE, y * TILE, width, height, maximum))
            files++
          }
        }
      }
    }
    await fsp.mkdir(root, { recursive: true })
    await fsp.rename(staging, path.join(root, run))
    const manifest = {
      schema: 1,
      status: 'ready',
      label: 'Community-reported systems (observed distribution; not total galaxy density)',
      generatedAt: new Date().toISOString(),
      source: {
        name: format.startsWith('edsm-') ? 'EDSM systemsWithCoordinates' : 'Operator-supplied coordinates CSV',
        url: sourceUrl || null,
        publishedAt: sourceDate || null,
        inputSha256: digest,
        inputBytes: compressedBytes,
        redistribution: 'Not assessed: obtain dataset usage permission before distributing generated assets'
      },
      extents: EXTENTS,
      tileSize: TILE,
      maxLevel,
      run,
      stats,
      files,
      views
    }
    const pendingManifest = path.join(output, '.manifest-' + run + '.tmp')
    await fsp.writeFile(pendingManifest, JSON.stringify(manifest, null, 2) + '\n')
    await fsp.rename(pendingManifest, path.join(output, 'manifest.json'))
    return manifest
  } finally {
    await fsp.rm(staging, { recursive: true, force: true })
  }
}

function cli () {
  const args = process.argv.slice(2)
  const arg = name => { const i = args.indexOf(name); return i < 0 ? undefined : args[i + 1] }
  return generate({
    input: arg('--input'),
    directory: arg('--out'),
    format: arg('--format') || 'edsm-json-gz',
    sourceUrl: arg('--source-url'),
    sourceDate: arg('--source-date'),
    maxLevel: arg('--max-level') == null ? 4 : Number(arg('--max-level'))
  })
}

if (require.main === module) cli().then(m => console.log(JSON.stringify({ stats: m.stats, files: m.files, sha256: m.source.inputSha256, run: m.run }))).catch(e => {
  console.error('Galaxy atlas generation failed:', e.message)
  process.exitCode = 1
})
module.exports = { generate, dimensions, projectBin, jsonObjects, densityPng, EXTENTS, AXES }
