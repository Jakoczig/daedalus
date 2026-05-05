import { useState, useEffect } from 'react'
import { useSocket, sendEvent, eventListener } from 'lib/socket'
import Layout from 'components/layout'
import Panel from 'components/panel'
import { MiningPanelNavItems } from 'lib/navigation-items'

const CONTENT_COLORS = {
  High: 'text-secondary',
  Medium: 'text-primary',
  Low: 'text-info',
  Unknown: 'text-muted'
}

function formatTime (iso) {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } catch { return '—' }
}

export default function MiningLogPage () {
  const { connected, active, ready } = useSocket()
  const [miningStatus, setMiningStatus] = useState()
  const [filter, setFilter] = useState('all') // 'all' | 'good'

  useEffect(() => {
    ;(async () => {
      if (!connected) return
      setMiningStatus(await sendEvent('getMiningStatus'))
    })()
  }, [connected, ready])

  useEffect(() => eventListener('miningStateChange', (status) => {
    setMiningStatus(status)
  }), [])

  // Full results list lives inside session.recentResults (capped at 50 server-side)
  // but we expose all 50 here; the last 10 are on other pages
  const allResults = miningStatus?.session?.recentResults ?? []
  const filtered = filter === 'good' ? allResults.filter(r => r.isGood) : allResults

  return (
    <Layout connected={connected} active={active} ready={ready}>
      <Panel layout='full-width' scrollable navigation={MiningPanelNavItems('Log')}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '1rem' }}>
          <h2 className='text-primary' style={{ margin: 0, flex: 1 }}>
            <span className='daedalus-terminal-table-inspector' style={{ marginRight: '.75rem' }} />
            Asteroid Log
          </h2>
          <div className='button-group'>
            <button
              className={`btn btn--sm ${filter === 'all' ? 'btn--active' : ''}`}
              onClick={() => setFilter('all')}
            >
              All
            </button>
            <button
              className={`btn btn--sm ${filter === 'good' ? 'btn--active' : ''}`}
              onClick={() => setFilter('good')}
            >
              Good only
            </button>
          </div>
        </div>

        {filtered.length === 0 ? (
          <p className='text-muted'>
            {allResults.length === 0
              ? 'No asteroids prospected this session.'
              : 'No good rocks found yet. Adjust your target materials or minimum threshold.'}
          </p>
        ) : (
          <table className='table'>
            <thead>
              <tr>
                <th>Time</th>
                <th>Content</th>
                <th>Materials</th>
                <th>%</th>
                <th>Left</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {filtered.map((r, i) => (
                <tr key={i} className={r.isGood ? 'table__row--highlighted' : ''}>
                  <td className='text-muted' style={{ whiteSpace: 'nowrap', opacity: .6 }}>
                    {formatTime(r.timestamp)}
                  </td>
                  <td>
                    <span className={`mining-panel__content-badge mining-panel__content-badge--${r.contentLevel}`}>
                      {r.contentLevel}
                    </span>
                  </td>
                  <td>
                    {r.materials.map((m, j) => (
                      <span
                        key={j}
                        className={m.isTarget ? 'text-secondary' : 'text-muted'}
                        style={{ marginRight: '.5rem', opacity: m.isTarget ? 1 : .5 }}
                      >
                        {m.name}
                      </span>
                    ))}
                    {r.motherlode && (
                      <span className='text-secondary'> ★ {r.motherlode}</span>
                    )}
                  </td>
                  <td className='text-info'>
                    {r.materials
                      .filter(m => m.isTarget)
                      .map((m, j) => (
                        <span key={j} style={{ marginRight: '.5rem' }}>
                          {m.pct.toFixed(1)}%
                        </span>
                      ))
                    }
                  </td>
                  <td className='text-muted' style={{ opacity: .6 }}>
                    {r.remaining < 100 ? `${Math.round(r.remaining)}%` : '—'}
                  </td>
                  <td>
                    {r.isGood && <span className='text-secondary'>✓</span>}
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

