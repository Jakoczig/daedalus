import { useState, useEffect } from 'react'
import { useSocket, sendEvent, eventListener } from 'lib/socket'
import Layout from 'components/layout'
import Panel from 'components/panel'
import PowerplayPanel from 'components/panels/missions/powerplay-panel'
import { MissionsPanelNavItems } from 'lib/navigation-items'

const PP_LOG_EVENTS = [
  'Powerplay', 'PowerplayMerits', 'PowerplayRank',
  'PowerplayJoin', 'PowerplayLeave', 'PowerplayDefect',
  'FSDJump', 'Location'
]

async function fetchNearbySources (power, currentSystem, setNearbySources) {
  setNearbySources(undefined) // loading
  const result = await sendEvent('getPowerplayNearbySources', { power, currentSystem })
  setNearbySources(result ?? null)
}

export default function MissionsPowerplayPage () {
  const { connected, active, ready } = useSocket()
  const [powerplay, setPowerplay] = useState()
  const [nearbySources, setNearbySources] = useState(null)

  async function refreshPowerplay () {
    const data = await sendEvent('getPowerplay')
    setPowerplay(data)
    if (data?.pledged && data.power && data.currentSystem) {
      fetchNearbySources(data.power, data.currentSystem, setNearbySources)
    } else {
      setNearbySources(null)
    }
  }

  useEffect(() => {
    if (!connected) return
    refreshPowerplay()
  }, [connected, ready])

  useEffect(() => eventListener('newLogEntry', async (log) => {
    if (PP_LOG_EVENTS.includes(log.event)) refreshPowerplay()
  }), [])

  useEffect(() => eventListener('gameStateChange', async (event) => {
    if (event?._changedFile === 'Status') return
    refreshPowerplay()
  }), [])

  return (
    <Layout connected={connected} active={active} ready={ready}>
      <Panel layout='full-width' scrollable navigation={MissionsPanelNavItems('Powerplay')}>
        <PowerplayPanel powerplay={powerplay} nearbySources={nearbySources} />
      </Panel>
    </Layout>
  )
}
