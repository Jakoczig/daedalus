import { useState, useEffect } from 'react'
import { useSocket, sendEvent, eventListener } from 'lib/socket'
import Layout from 'components/layout'
import Panel from 'components/panel'
import { MiningPanelNavItems } from 'lib/navigation-items'

function formatDuration (startIso) {
  if (!startIso) return null
  const ms = Date.now() - new Date(startIso).getTime()
  const h = Math.floor(ms / 3600000)
  const m = Math.floor((ms % 3600000) / 60000)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

function StatRow ({ label, value, highlight }) {
  return (
    <div className='mining-panel__stat-row' style={{ marginBottom: '.4rem' }}>
      <span className='mining-panel__stat-label' style={{ opacity: .55 }}>{label}</span>
      <span className={`mining-panel__stat-value${highlight ? ' mining-panel__stat-value--highlight' : ''}`}>
        {value}
      </span>
    </div>
  )
}

export default function MiningStatisticsPage () {
  const { connected, active, ready } = useSocket()
  const [miningStatus, setMiningStatus] = useState()

  useEffect(() => {
    ;(async () => {
      if (!connected) return
      setMiningStatus(await sendEvent('getMiningStatus'))
    })()
  }, [connected, ready])

  useEffect(() => eventListener('miningStateChange', (status) => {
    setMiningStatus(status)
  }), [])

  if (!miningStatus?.active) {
    return (
      <Layout connected={connected} active={active} ready={ready}>
        <Panel layout='full-width' scrollable navigation={MiningPanelNavItems('Statistics')}>
          <p className='text-muted'>No active mining session.</p>
        </Panel>
      </Layout>
    )
  }

  const { sessionStart, systemName, cargo, limpets, session, refined } = miningStatus

  const duration = formatDuration(sessionStart)

  return (
    <Layout connected={connected} active={active} ready={ready}>
      <Panel layout='full-width' scrollable navigation={MiningPanelNavItems('Statistics')}>
        <h2 className='text-primary' style={{ marginBottom: '1.5rem' }}>
          <span className='daedalus-terminal-trending-up-chart' style={{ marginRight: '.75rem' }} />
          Session Statistics
        </h2>

        <section style={{ marginBottom: '2rem' }}>
          <h3 className='text-primary' style={{ marginBottom: '.75rem', opacity: .7 }}>Session</h3>
          <div className='mining-panel__stats'>
            {systemName && <StatRow label='System' value={systemName} />}
            {duration && <StatRow label='Duration' value={duration} />}
            <StatRow label='Cargo' value={`${cargo?.current ?? 0} / ${cargo?.max ?? 0} t (${cargo?.fillPercent ?? 0}% full)`} />
          </div>
        </section>

        <section style={{ marginBottom: '2rem' }}>
          <h3 className='text-primary' style={{ marginBottom: '.75rem', opacity: .7 }}>Prospector Efficiency</h3>
          <div className='mining-panel__stats'>
            <StatRow
              label='Good rocks'
              value={`${session?.goodRocks ?? 0} / ${session?.totalRocks ?? 0}${session?.totalRocks > 0 ? ` (${session.goodRockPercent}%)` : ''}`}
              highlight={session?.goodRockPercent >= 30}
            />
          </div>
        </section>

        <section style={{ marginBottom: '2rem' }}>
          <h3 className='text-primary' style={{ marginBottom: '.75rem', opacity: .7 }}>Limpet Efficiency</h3>
          <div className='mining-panel__stats'>
            <StatRow label='Remaining' value={limpets?.remaining ?? 0} />
            <StatRow label='Launched' value={limpets?.used ?? 0} />
            <StatRow label='Synthed' value={limpets?.synthed ?? 0} />
            {(limpets?.abandoned ?? 0) > 0 && (
              <StatRow
                label='Abandoned'
                value={`${limpets.abandoned} (${limpets.wastedPercent}% wasted)`}
                highlight={limpets.wastedPercent > 10}
              />
            )}
          </div>
        </section>

        <section style={{ marginBottom: '2rem' }}>
          <h3 className='text-primary' style={{ marginBottom: '.75rem', opacity: .7 }}>Refined Output</h3>
          <div className='mining-panel__stats'>
            <StatRow
              label='Total'
              value={`${refined?.totalTons ?? 0} t${refined?.ratePerHour > 0 ? ` · ${refined.ratePerHour} t/hr` : ''}`}
              highlight
            />
            {(refined?.items ?? []).map((item, i) => (
              <StatRow key={i} label={item.key} value={`${item.count} t`} />
            ))}
          </div>
        </section>
      </Panel>
    </Layout>
  )
}

