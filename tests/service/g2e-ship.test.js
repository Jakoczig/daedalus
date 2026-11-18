const test = require('node:test')
const assert = require('node:assert/strict')
const { harness } = require('../helpers/go-panel-harness.cjs')
const workspace = { parity: { evidence: 'journal_last_known', ship: { shipType: 'asp', hullHealth: 0.8, fuelMainT: 16, fuelCapacityT: 32, cargoCapacityT: 64, maxJumpRangeLY: 45 }, modulesState: 'known', modules: [{ slot: 'FrameShiftDrive', item: 'int_hyperdrive_size5_class5', name: 'Frame Shift Drive', health: 0.9, on: true, priority: 1, reference: { class: 5, rating: 'A', massT: 20, powerMW: 0.6, source: 'bundled_edcd_coriolis_reference' }, engineering: { blueprint: 'FSD_LongRange', grade: 5, quality: 0.8, experimental: 'Mass Manager', modifiers: [{ label: 'FSDOptimalMass', value: 1750, originalValue: 1000, lessIsGood: false }] } }], status: { state: 'known', flags: { inMainShip: true, lightsOn: true, hardpointsDeployed: false, shieldsUp: true, supercruise: true }, pips: { sys: 2, eng: 3, wep: 1 }, fuelReservoirT: 0.4 }, cargo: { state: 'known', count: 6, items: [{ name: 'drones', label: 'Limpet', count: 6, mission: false, stolen: 0 }] }, inventory: { state: 'known', items: [{ name: 'medkit', label: 'Medkit', category: 'Consumables', location: 'ShipLocker', count: 4 }] } } }
for (const [path, expected] of [['/ship/status', /Power distribution/], ['/ship/modules', /Frame Shift Drive/], ['/ship/cargo', /Limpet/], ['/ship/inventory', /Medkit/]]) {
  test('G2E actual read-only Ship page ' + path, () => { const h = harness('go-parity-ship'); h.props.workspace = workspace; h.props.path = path; const html = h.html(); assert.match(html, expected); assert.doesNotMatch(html, /PRIVATE_|Latitude|Longitude|owner\/|toggleSwitch/); assert.equal(h.calls.length, 0) })
}
test('G2E unknown cargo is not an empty hold and module inspector resolves fresh facts', () => {
  const h = harness('go-parity-ship'); h.props.workspace = { parity: { ship: {}, cargo: { state: 'unknown', items: [] }, modules: [], modulesState: 'unknown', inventory: { state: 'unknown', items: [] } } }; h.props.path = '/ship/cargo'; assert.match(h.html(), /not.*witnessed|Unknown|Awaiting/i); assert.doesNotMatch(h.html(), /hold is empty/i)
  const m = harness('go-parity-ship', undefined, 'Modules'); m.props.model = workspace.parity; m.html(); const row = m.elements('button').find(b => String(b.props['aria-label']).includes('Inspect Frame Shift Drive')); assert.ok(row); row.props.onClick(); assert.match(m.html(), /Mass Manager/); assert.match(m.html(), /1750|1,750|1 750/)
  m.props.model = { ...workspace.parity, modules: [], modulesState: 'unknown' }; assert.doesNotMatch(m.html(), /Mass Manager/)
})

test('locker completeness does not imply backpack completeness and manifests page at fifty', () => {
  const h = harness('go-parity-ship', undefined, 'Inventory'); h.props.model = { ship: {}, inventory: { state: 'partial', shipLockerState: 'known', backpackState: 'unknown', items: [] } }; h.props.cargo = false
  assert.match(h.html(), /Observed ship locker is empty/); assert.doesNotMatch(h.html(), /snapshot not yet witnessed/)
  const location = h.elements('select')[0]; location.props.onChange({ target: { value: 'backpack' } }); assert.match(h.html(), /snapshot not yet witnessed/); assert.doesNotMatch(h.html(), /is empty/)
  h.props.cargo = true; h.props.model = { ship: {}, cargo: { state: 'known', count: 51, items: Array.from({ length: 51 }, (_, i) => ({ name: 'Fixture ' + i, label: 'Fixture ' + i, count: 1 })) } }; let html = h.html(); assert.equal((html.match(/<tbody>.*?<tr/g) || []).length, 1); assert.equal((html.match(/<tr>/g) || []).length, 51)
  h.elements('button').find(b => b.props['aria-label'] === 'Next manifest page').props.onClick(); html = h.html(); assert.equal((html.match(/<tr>/g) || []).length, 2)
})

test('ship switches do not claim current main-ship state while outside the main ship', () => {
  const h = harness('go-parity-ship')
  h.props.path = '/ship/status'
  h.props.workspace = { parity: { ship: {}, status: { state: 'known', flags: { inMainShip: false, lightsOn: true, shieldsUp: true } } } }
  const html = h.html()
  assert.match(html, /Ship lights: Unknown/)
  assert.doesNotMatch(html, /Ship lights: On/)
})

test('unknown module class remains explicitly unverified, never Class 0', () => {
  const h = harness('go-parity-ship', undefined, 'Modules')
  h.props.model = { modules: [{ slot: 'Slot05_Size6', item: 'INT_FIGHTERBAYMK2_S', name: 'Fighter Hangar Mk II' }] }
  assert.match(h.html(), /go-module-class">\?/)
  h.elements('button').find(b => b.props['aria-label'] === 'Inspect Fighter Hangar Mk II').props.onClick()
  assert.match(h.html(), /Module class has not been verified/)
})
