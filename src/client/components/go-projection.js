import { useEffect, useState } from 'react'

export default function useGoProjection ({ connected, ready, sendEvent, eventListener }, rpc, changed, delta = null) {
  const [value, setValue] = useState(null)
  const [error, setError] = useState('')
  useEffect(() => {
    if (!connected || !ready) { setValue(null); setError(''); return }
    let active = true
    let liveSeen = false
    let latestDelta = null
    setValue(null); setError('')
    const stop = eventListener(changed, next => { if (active) { liveSeen = true; latestDelta = null; setValue(next); setError('') } })
    const stopDelta = delta ? eventListener(delta.name, next => { if (active) { latestDelta = next; setValue(previous => previous ? delta.merge(previous, next) : previous) } }) : () => {}
    sendEvent(rpc).then(next => { if (active && !liveSeen) { setValue(latestDelta && delta ? delta.merge(next, latestDelta) : next); setError('') } }).catch(() => { if (active && !liveSeen) setError('Local journal view unavailable. Reconnect or check journal status.') })
    return () => { active = false; stop(); stopDelta() }
  }, [connected, ready, sendEvent, eventListener, rpc, changed, delta])
  return { value, error }
}
