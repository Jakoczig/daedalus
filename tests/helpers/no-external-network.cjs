// Child-service tests may use real loopback HTTP/WS only. Any accidental
// provider upload fails the test instead of leaving the machine.
const originalFetch = globalThis.fetch
globalThis.fetch = (input, ...args) => {
  const host = new URL(typeof input === 'string' || input instanceof URL ? input : input.url).hostname
  if (!['localhost', '127.0.0.1', '[::1]'].includes(host)) return Promise.reject(new Error('External network disabled in tests'))
  return originalFetch(input, ...args)
}
for (const protocol of ['http', 'https']) {
  const module = require(`node:${protocol}`)
  for (const method of ['request', 'get']) {
    const original = module[method]
    module[method] = function (input, ...args) {
      const host = typeof input === 'string' || input instanceof URL
        ? new URL(input).hostname
        : input.hostname || input.host || 'localhost'
      if (!['localhost', '127.0.0.1', '[::1]', '::1'].includes(host)) throw new Error('External network disabled in tests')
      return original.call(this, input, ...args)
    }
  }
}
