import { useEffect, useState } from 'react'
import { Icon } from './go-terminal-widgets'
export default function GoAccounts ({ sendEvent, cardsOnly = false }) {
  const [value, setValue] = useState(null); const [error, setError] = useState('')
  useEffect(() => { let active = true; sendEvent('getGoAccounts').then(v => { if (active) setValue(v) }).catch(() => setError('Account settings unavailable')); return () => { active = false } }, [sendEvent])
  const save = async (multiple, selected) => { try { setValue(await sendEvent('setGoAccounts', { multiple, selected })); setError(''); window.dispatchEvent(new Event('eridion-account-changed')); window.location.reload() } catch (e) { setError(e.message || 'Account selection failed') } }
  if (cardsOnly && !value?.settings.multiple) return null
  return <section className='go-account-settings' aria-label='Commander profiles'>{!cardsOnly && <><h3>Commander profiles</h3><label><input type='checkbox' checked={value?.settings.multiple || false} onChange={e => save(e.target.checked, value?.cards.find(c => c.active)?.fid || value?.cards[0]?.fid || '')} /> Multiple accounts</label><p>Off: follow the verified game commander. On: choose one profile here before opening the terminal.</p></>}{value?.settings.multiple && <div className='go-account-cards'>{value.cards.map(c => <button key={c.fid} aria-pressed={c.active} onClick={() => save(true, c.fid)}>{c.avatar ? <img src={c.avatar} alt='Commander portrait' width='32' height='32' /> : <Icon name='shield' />}<strong>{c.name || 'Verified commander'}</strong><span>{c.shipName || c.shipType || 'Ship unknown'}</span><small>{c.system || 'System unknown'}</small></button>)}</div>}{error && <p role='alert'>{error}</p>}</section>
}
