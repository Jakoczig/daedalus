import { useEffect, useState } from 'react'
import GoAccounts from './go-accounts'
import LauncherServices, { journalLabel } from './go-launcher-status'
import { AtlasSettings } from './go-atlas'
import GoCredits from './go-credits'
import dynamic from 'next/dynamic'
import useGoProjection from './go-projection'
const RemoteDisplays = dynamic(() => import('./remote-displays'), { ssr: false })
const JournalSettings = dynamic(() => import('./go-journal-status'), { ssr: false })
const Providers = dynamic(() => import('./go-provider-settings'), { ssr: false })
export function CompanionSettings (props) {
  const [category, setCategory] = useState('')
  const [value, setValue] = useState(null); const [error, setError] = useState('')
  useEffect(() => { props.sendEvent('getGoLauncher').then(setValue).catch(() => setError('Owner settings unavailable')) }, [props.sendEvent])
  const save = async config => { try { setValue(await props.sendEvent('setGoCompanion', config)); setError('') } catch { setError('Settings could not be saved') } }
  const c = value?.config
  return <section className='go-companion-settings'>{['General', 'Journal & Commander', 'External Services', 'Galaxy Atlas', 'Credits'].map(name => <button key={name} aria-expanded={category === name} aria-controls='launcher-settings-content' onClick={() => setCategory(category === name ? '' : name)}>{name}</button>)}<div id='launcher-settings-content'>{category === 'General' && <><h3>General</h3>{c && <><label>Launcher close behavior <select value={c.closeMode} onChange={e => save({ ...c, closeMode: e.target.value })}><option value='tray'>Minimize to tray (keep service running)</option><option value='quit'>Quit service and disconnect displays</option></select></label>{[['charge', 'FSD hyperdrive charge'], ['jump', 'Hyperspace jump'], ['restore', 'Restore previous screen after arrival / cancel'], ['owner', 'Affect open local dashboard'], ['allViewers', 'Affect all paired displays']].map(([key, label]) => <label key={key}><input type='checkbox' checked={c[key]} onChange={e => save({ ...c, [key]: e.target.checked })} /> {label}</label>)}<p>Choose individual paired displays; new devices are unaffected unless “all” is enabled.</p>{value.devices.map(d => <label key={d.id}><input type='checkbox' checked={c.devices?.includes(d.id) || false} onChange={e => save({ ...c, devices: e.target.checked ? [...(c.devices || []), d.id] : c.devices.filter(id => id !== d.id) })} /> {d.name}</label>)}</>}{value?.notice && <p role='status'>{value.notice}</p>}{error && <p role='alert'>{error}</p>}</>}{category === 'Journal & Commander' && <><JournalSettings {...props} settingsOnly /><GoAccounts {...props} launcher={props.launcher === true} /></>}{category === 'External Services' && <Providers {...props} onChange={() => props.onProvidersChange?.()} />}{category === 'Galaxy Atlas' && <AtlasSettings {...props} />}{category === 'Credits' && <GoCredits />}</div></section>
}
export default function GoCompanion (props) {
  useEffect(() => { if (typeof document !== 'undefined') document.title = 'ERIDION · ERID' }, [])
  const { value: journal } = useGoProjection(props, 'getJournalSummary', 'journalSummaryChanged')
  const [accounts, setAccounts] = useState(null)
  const [state, setState] = useState(null); const [panel, setPanel] = useState(''); const [error, setError] = useState('')
  useEffect(() => { let active = true; const update = () => props.sendEvent('getGoLauncher').then(v => { if (active) setState(v) }).catch(() => { if (active) setError('Service status unavailable') }); update(); const timer = setInterval(update, 5000); return () => { active = false; clearInterval(timer) } }, [props.sendEvent])
  const action = name => props.sendEvent('desktopAction', { action: name }).catch(() => setError('Native window action unavailable'))
  return <main className='go-launcher scrollable' tabIndex={0} aria-label='Native companion launcher'><h1><i className='icon daedalus-terminal-logo' /> ERIDION</h1><h2>Native Companion · experimental</h2><GoAccounts {...props} cardsOnly onState={setAccounts} /><p className={props.connected && props.ready ? 'text-success' : 'text-danger'} role='status'>{props.connected ? props.ready ? 'SERVICE READY' : 'CONNECTING' : 'DISCONNECTED'}</p><dl><div><dt>Journal</dt><dd>{journalLabel(journal)}</dd></div><div><dt>Paired / connected devices</dt><dd>{state?.devices?.length ?? '—'} / {state?.connectedDevices ?? '—'}</dd></div><div><dt>Live Services</dt><dd><LauncherServices {...props} /></dd></div></dl><div className='go-launcher-actions'><button onClick={() => setPanel(panel === 'settings' ? '' : 'settings')}>Settings</button><button onClick={() => setPanel(panel === 'pairing' ? '' : 'pairing')}>Pairing</button>{state?.native ? <button disabled={!accounts || accounts.choiceRequired} onClick={() => action('dashboard')}>Open Dashboard</button> : accounts && !accounts.choiceRequired ? <a href='/ship/status'>Open Dashboard (browser preview)</a> : <button disabled>Choose commander to open dashboard</button>}<button onClick={() => action('quit')}>Quit service</button></div>{panel === 'settings' && <CompanionSettings {...props} launcher />}{panel === 'pairing' && <RemoteDisplays />}{error && <p role='alert'>{error}</p>}</main>
}
