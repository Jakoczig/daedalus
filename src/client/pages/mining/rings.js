import { useState, useEffect } from 'react'
import { useRouter } from 'next/router'
import { useSocket, sendEvent, eventListener } from 'lib/socket'
import Layout from 'components/layout'
import Panel from 'components/panel'
import { MiningPanelNavItems } from 'lib/navigation-items'

const RING_TYPE_LABELS = {
  Icy: 'Icy',
  Metallic: 'Metallic',
  MetalRich: 'Metal Rich',
  Rocky: 'Rocky'
}

export default function MiningRingsPage () {
  const { connected, active, ready } = useSocket()
  const [miningStatus, setMiningStatus] = useState()
  const router = useRouter()

  useEffect(() => {
    ;(async () => {
      if (!connected) return
      setMiningStatus(await sendEvent('getMiningStatus'))
    })()
  }, [connected, ready])

  useEffect(() => eventListener('miningStateChange', (status) => {
    setMiningStatus(status)
  }), [])

  // Autoswitch — only when already within /mining/*
  useEffect(() => eventListener('newLogEntry', (log) => {
    if (!router.pathname.startsWith('/mining')) return
    if (log.event === 'SupercruiseEntry') {
      router.push('/mining/rings')
    } else if (log.event === 'SupercruiseExit' && log.BodyType === 'PlanetaryRing') {
      router.push('/mining/status')
    }
  }), [router])

  const rings = miningStatus?.rings ?? []

  return (
    <Layout connected={connected} active={active} ready={ready}>
      <Panel layout='full-width' scrollable navigation={MiningPanelNavItems('Rings')}>
        <h2 className='text-primary' style={{ marginBottom: '1rem' }}>
          <span className='daedalus-terminal-planet-ringed' style={{ marginRight: '.75rem' }} />
          Ring Bodies
          {miningStatus?.systemName && (
            <span className='text-info' style={{ fontWeight: 'normal', marginLeft: '1rem', fontSize: '.8em', opacity: .75 }}>
              {miningStatus.systemName}
            </span>
          )}
        </h2>

        {rings.length === 0 ? (
          <p className='text-muted'>No ring bodies scanned yet. FSS and DSS scan ring bodies to populate this list.</p>
        ) : (
          <table className='table'>
            <thead>
              <tr>
                <th>Ring Body</th>
                <th>Type</th>
                <th>Hotspots</th>
              </tr>
            </thead>
            <tbody>
              {rings
                .slice()
                .sort((a, b) => {
                  // target hotspot rings first
                  if (b.hasTargetHotspots !== a.hasTargetHotspots) {
                    return b.hasTargetHotspots ? 1 : -1
                  }
                  return a.bodyName.localeCompare(b.bodyName)
                })
                .map((ring, i) => (
                  <tr key={i} className={ring.hasTargetHotspots ? 'table__row--highlighted' : ''}>
                    <td className={ring.hasTargetHotspots ? 'text-secondary' : 'text-info'}>
                      {ring.bodyName}
                    </td>
                    <td>
                      <span className={`mining-panel__ring-type-badge mining-panel__ring-type-badge--${ring.ringType ?? 'Unknown'}`}>
                        {RING_TYPE_LABELS[ring.ringType] ?? ring.ringType ?? '?'}
                      </span>
                    </td>
                    <td>
                      {ring.hotspots.length === 0 ? (
                        <span className='text-muted' style={{ opacity: .4 }}>not mapped</span>
                      ) : (
                        ring.hotspots.map((h, j) => (
                          <span key={j} className={h.isTarget ? 'text-secondary' : 'text-info'} style={{ marginRight: '.75rem' }}>
                            {h.key} ×{h.count}
                          </span>
                        ))
                      )}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        )}
      </Panel>
    </Layout>
  )
}

