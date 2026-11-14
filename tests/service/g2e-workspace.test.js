const test = require('node:test'); const assert = require('node:assert/strict'); const { harness } = require('../helpers/go-panel-harness.cjs')
test('flight deltas merge after delayed initial RPC and clear on disconnect', async () => {
  const h = harness('go-parity'); h.props.path = '/ship/status'; let finish; h.props.sendEvent = () => new Promise(resolve => { finish = resolve }); h.html(); await h.flush()
  h.broadcast({ state: 'known', flags: { inMainShip: true }, fuelMainT: 17.125 }, 'goFlightChanged')
  finish({ parity: { ship: { shipType: 'asp', fuelMainT: 16, fuelCapacityT: 32 }, status: { state: 'known', flags: { inMainShip: true }, fuelMainT: 16 }, cargo: { state: 'unknown', items: [] } }, activity: {} })
  const html = await h.flush(); assert.match(html, /17[,.]1/); assert.doesNotMatch(html, /Main fuel.*16 T/)
  h.props.connected = false; h.html(); const disconnected = await h.flush(); assert.match(disconnected, /disconnected/); assert.doesNotMatch(disconnected, /17[,.]1/); assert.equal(h.unsubscribed, 2)
})
