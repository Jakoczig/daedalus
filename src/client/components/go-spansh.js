import { useState } from 'react'
import { useRouter } from 'next/router'
import { ImportedItinerary } from './go-field-operations'

// Public navigation routes verified on spansh.co.uk, 2026-11-18. These are
// browser destinations, not a claimed generation API or form-prefill contract.
const common = [['from', 'Starting system'], ['to', 'Destination system'], ['range', 'Jump range (LY)']]
const exploration = [...common, ['radius', 'Search radius (LY)'], ['minimum', 'Minimum reference value (CR)'], ['distance', 'Maximum arrival distance (LS)']]
export const plotters = [
  ['plotter', 'Neutron Router', [...common, ['efficiency', 'Efficiency (%)']]],
  ['exact-plotter', 'Galaxy / Exact Plotter', [...common, ['ship', 'Ship / fitted range notes']]],
  ['fleet-carrier', 'Fleet Carrier', [['from', 'Starting system'], ['destinations', 'Destination systems (one per line)'], ['tritium', 'Tritium fuel (T)'], ['cargo', 'Stored tritium (T)']]],
  ['trade', 'Trade Planner', [['from', 'Starting system / station'], ['cargo', 'Cargo capacity (T)'], ['capital', 'Starting capital (CR)'], ['hops', 'Maximum hops'], ['range', 'Maximum hop distance (LY)'], ['age', 'Maximum price age (hours)']]],
  ['tourist', 'Tourist Planner', [['from', 'Starting system'], ['destinations', 'Destination systems (one per line)'], ['range', 'Jump range (LY)']]],
  ['riches', 'Road to Riches', exploration],
  ['ammonia', 'Ammonia Worlds', exploration],
  ['earth', 'Earth-like Worlds', exploration],
  ['rocky-metal', 'Rocky / Metal Worlds', exploration],
  ['exobiology', 'Exobiology', exploration],
  ['colonisation', 'Colonisation', [['from', 'Starting system'], ['to', 'Destination system']]],
  ['history', 'Route History', []]
].map(([id, label, fields]) => ({ id, label, fields, url: id === 'history' ? null : `https://spansh.co.uk/${id}` }))
export function plannerText (plotter, values) {
  return [plotter.label, plotter.url, ...plotter.fields.filter(([id]) => values[id]?.trim()).map(([id, label]) => `${label}: ${values[id].trim()}`)].join('\n')
}
export default function GoSpansh (props) {
  const router = useRouter()
  const [mode, setMode] = useState('plotter'); const [values, setValues] = useState({}); const [notice, setNotice] = useState('')
  const plotter = plotters.find(p => p.id === mode)
  const copy = async () => { try { await navigator.clipboard.writeText(plannerText(plotter, values)); setNotice('Settings copied on this device. Enter them on the official website.') } catch { setNotice('Copy the settings below manually on this device.') } }
  return <section className='go-page go-spansh' aria-label='Spansh Plotter'><div className='go-page__heading'><h2>Spansh Plotter</h2><span>Official website · manual web workflow</span></div><p>ERIDION does not generate routes or send these settings. Open Spansh only when you choose; its own terms and privacy policy apply. Plotter filters, advanced ship inputs and results remain on the official website.</p><label>Plotting mode <select aria-label='Spansh plotting mode' value={mode} onChange={e => { setMode(e.target.value); setNotice('') }}>{plotters.map(p => <option key={p.id} value={p.id}>{p.label}</option>)}</select></label>{mode !== 'history' && <><div className='go-spansh-options'>{plotter.fields.map(([id, label]) => <label key={id}>{label}{id === 'destinations' ? <textarea maxLength={2048} value={values[id] || ''} onChange={e => setValues({ ...values, [id]: e.target.value })} /> : <input maxLength={128} value={values[id] || ''} onChange={e => setValues({ ...values, [id]: e.target.value })} />}</label>)}</div><p>These are local planning notes, not validated or automatically applied website options. No locations or settings are encoded in the link.</p><div className='go-history-actions'><button onClick={copy}>Copy settings on this device</button><a href={plotter.url} target='_blank' rel='noreferrer noopener'>Open official {plotter.label}</a></div><details><summary>Planning notes to copy</summary><pre>{plannerText(plotter, values)}</pre></details><p>After plotting, export CSV on Spansh and choose it below. Import accepts a system-name column and unique systems, not every body/trade/loop export. Edit unsupported exports into a unique-system itinerary; prices, body targets and generated safety predictions are not imported. No automatic result retrieval or background web requests.</p></>}{props.role === 'owner' ? <ImportedItinerary {...props} historyOnly={mode === 'history'} onImported={() => router.push('/nav/route')} /> : <p>Saved route history and import controls are private to the local owner. The Route page shares only allowlisted progress with paired displays.</p>}{notice && <p role='status'>{notice}</p>}</section>
}
