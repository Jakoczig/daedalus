import { useEffect, useState } from 'react'

export async function remoteAction (action, values = {}) {
  const response = await window.fetch('/owner/remote', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...values, action })
  })
  if (!response.ok) throw new Error('Remote display operation failed. Check the address/port and private certificate storage; restart after a storage error.')
  return response.json()
}

export default function RemoteDisplays () {
  const [state, setState] = useState(null)
  const [error, setError] = useState('')
  const [pairing, setPairing] = useState(null)
  const [address, setAddress] = useState('')
  const [port, setPort] = useState('3443')
  const [pending, setPending] = useState(false)
  useEffect(() => {
    let active = true
    remoteAction('status').then(value => {
      if (!active) return
      setState(value); setAddress(value.address || value.addresses?.[0] || ''); setPort(String(value.port || 3443))
    }).catch(() => { if (active) setError('Remote settings unavailable. Restart the host after a private-storage error.') })
    return () => { active = false }
  }, [])
  const act = async (action, values = {}) => {
    setError(''); setPending(true)
    try {
      const result = await remoteAction(action, values)
      if (action === 'pair') setPairing(result)
      else { setState(result); if (action === 'disable') setPairing(null) }
    } catch (error) { setError(error.message) } finally { setPending(false) }
  }
  return (
    <details style={{ position: 'relative', zIndex: 20, background: 'var(--color-background, #100d12)', maxHeight: '70vh', overflow: 'auto', padding: '.5rem' }}>
      <summary>Remote display settings (local owner)</summary>
      <p>Remote access is disabled until you enable it. Paired devices are read-only; gameplay remains offline.</p>
      <p>Use a private IPv4 address on this host, preferably with a DHCP reservation. Allow only your private network in the host firewall. Do not forward this port to the Internet.</p>
      {error && <p role='alert'>{error}</p>}
      {state?.notice && <p role='alert'>{state.notice}</p>}
      {state && !state.enabled && (
        <form onSubmit={event => { event.preventDefault(); act('enable', { address: address.trim(), port: Number(port) }) }}>
          <label>Host IPv4 address <input required list='remote-host-addresses' value={address} onChange={event => setAddress(event.target.value)} placeholder='192.168.1.10' /></label>
          <datalist id='remote-host-addresses'>{(state.addresses || []).map(ip => <option key={ip} value={ip} />)}</datalist>
          <label>HTTPS port <input required type='number' min='1024' max='65535' value={port} onChange={event => setPort(event.target.value)} /></label>
          <button disabled={pending}>Enable HTTPS remote displays</button>
        </form>
      )}
      {state?.enabled && (
        <>
          <p>Browser address: <a href={state.url} target='_blank' rel='noreferrer'>{state.url}</a></p>
          <p>Transfer the public CA certificate securely to your device and verify its SHA256 fingerprint before trusting it. Import it into that device/browser trust store; never bypass a certificate warning. Private keys stay on this host.</p>
          <p>Certificate fingerprint: <code style={{ overflowWrap: 'anywhere' }}>{state.fingerprint}</code></p>
          <a download='daedalus-display-ca.crt' href={`data:application/x-pem-file;charset=utf-8,${encodeURIComponent(state.ca)}`}>Download public CA certificate</a>
          <button disabled={pending} onClick={() => act('pair')}>Create pairing code</button>
          <button disabled={pending} onClick={() => act('disable')}>Disable remote displays and revoke all devices</button>
          {pairing && <p>One-time pairing code: <output>{pairing.code}</output> — expires {pairing.expires}. Enter it only at the trusted HTTPS address.</p>}
          <p>Device sessions last up to 12 hours and recover after network interruptions or a clean host restart. After expiry, revocation or an unclean host exit, pair again.</p>
          <ul>{state.devices.map(device => <Device key={device.id} device={device} act={act} pending={pending} />)}</ul>
        </>
      )}
    </details>
  )
}

function Device ({ device, act, pending }) {
  const [name, setName] = useState(device.name)
  return (
    <li>
      <label>Device name <input maxLength={80} value={name} onChange={event => setName(event.target.value)} /></label>
      <button disabled={pending} onClick={() => act('rename', { deviceId: device.id, name: name.trim() })}>Rename</button>
      <button disabled={pending} onClick={() => act('revoke', { deviceId: device.id })}>Revoke</button>
      <span> Session expires {device.expires}</span>
    </li>
  )
}
