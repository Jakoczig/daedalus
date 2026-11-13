import { useEffect, useState } from 'react'

export default function useGoProjection ({ connected, ready, sendEvent, eventListener }, rpc, changed) {
  const [value, setValue] = useState(null)
  const [error, setError] = useState('')
  useEffect(() => {
    if (!connected || !ready) { setValue(null); setError(''); return }
    let active = true
    let liveSeen = false
    setValue(null); setError('')
    const stop = eventListener(changed, next => { if (active) { liveSeen = true; setValue(next); setError('') } })
    sendEvent(rpc).then(next => { if (active && !liveSeen) { setValue(next); setError('') } }).catch(() => { if (active && !liveSeen) setError('Local journal view unavailable. Reconnect or check journal status.') })
    return () => { active = false; stop() }
  }, [connected, ready, sendEvent, eventListener, rpc, changed])
  return { value, error }
}
