// Deployment source for the separately managed HTTPS website. No tokens here.
export function receiverURL (search) {
  const q = new URLSearchParams(search)
  if (q.getAll('state').length !== 1) throw new Error('Missing or duplicate state')
  const match = /^erid1\.([0-9]{4,5})\.([A-Za-z0-9_-]{43})$/.exec(q.get('state'))
  if (!match || Number(match[1]) < 1024 || Number(match[1]) > 65535 || String(Number(match[1])) !== match[1]) throw new Error('Invalid receiver')
  const result = new URL(`http://127.0.0.1:${match[1]}/owner/frontier-callback`)
  result.searchParams.set('state', q.get('state'))
  const key = q.has('error') ? 'error' : 'code'
  const value = q.get(key)
  if (q.getAll(key).length !== 1 || !value || value.length > 2048 || /[\x00-\x20\x7f]/.test(value) || (key === 'error' && q.has('code'))) throw new Error('Invalid authorization response')
  result.searchParams.set(key, value)
  return result.href
}
if (typeof document !== 'undefined') {
  const status = document.querySelector('#status')
  try {
    const receiver = receiverURL(location.search)
    // Remove the code from website history before displaying the return control.
    history.replaceState(null, '', location.pathname)
    const link = document.querySelector('#return'); link.href = receiver; link.hidden = false
    document.querySelector('#copy').hidden = false
    document.querySelector('#copy').onclick = async () => { try { await navigator.clipboard.writeText(receiver); status.textContent = 'Paste in ERIDION owner settings → Frontier → Manual browser return.' } catch { status.textContent = 'Open the return link on this gaming PC.' } }
    status.textContent = 'Return to ERIDION on this computer. No account token is stored on this website.'
  } catch { status.textContent = 'Invalid or expired authorization response. Start again in ERIDION owner settings.' }
}
