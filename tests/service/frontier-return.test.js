const { test } = require('node:test')
const assert = require('node:assert/strict')
test('Frontier website only returns to fixed numeric loopback receiver', async () => {
  const { receiverURL } = await import('../../scripts/frontier-return/return.mjs')
  const nonce = 'A'.repeat(43); const q = 'state=erid1.12345.' + nonce + '&code=fixture-code'
  const u = new URL(receiverURL(q + '&host=evil.invalid&redirect_uri=https://evil.invalid'))
  assert.equal(u.origin, 'http://127.0.0.1:12345'); assert.equal(u.pathname, '/owner/frontier-callback'); assert.equal(u.searchParams.get('code'), 'fixture-code'); assert.equal([...u.searchParams].length, 2)
  for (const query of ['state=erid1.80.' + nonce + '&code=x', 'state=erid1.99999.' + nonce + '&code=x', q + '&state=evil', q + '&code=other', q + '&error=denied', 'state=erid1.01234.' + nonce + '&code=x', 'state=https://evil.invalid&code=x']) assert.throws(() => receiverURL(query))
  assert.equal(new URL(receiverURL('state=erid1.12345.' + nonce + '&error=access_denied')).searchParams.get('error'), 'access_denied')
})
