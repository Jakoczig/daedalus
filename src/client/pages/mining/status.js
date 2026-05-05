import { useState, useEffect, useRef } from 'react'
import { useRouter } from 'next/router'
import { useSocket, sendEvent, eventListener } from 'lib/socket'
import Layout from 'components/layout'
import Panel from 'components/panel'
import { MiningPanelNavItems } from 'lib/navigation-items'
import MiningDashboard from 'components/panels/mining/mining-status'

const applyScaling = (wrapper, content) => {
  try {
    content.style.transform = 'scale(1, 1)'
    const { width: cw, height: ch } = content.getBoundingClientRect()
    const { width: ww, height: wh } = wrapper.getBoundingClientRect()
    const scale = Math.min(ww / cw, wh / ch)
    content.style.transform = `scale(${scale}, ${scale})`
    content.style.opacity = 1
  } catch (e) {
    console.log(e)
  }
}

export default function MiningStatusPage () {
  const { connected, active, ready } = useSocket()
  const [miningStatus, setMiningStatus] = useState()
  const scaledWrapper = useRef()
  const scaledContent = useRef()
  const router = useRouter()

  useEffect(() => {
    ;(async () => {
      if (!connected) return
      setMiningStatus(await sendEvent('getMiningStatus'))
    })()
  }, [connected, ready])

  // applyScaling on resize and on data change
  useEffect(() => {
    const onResize = () => {
      if (scaledWrapper.current && scaledContent.current) {
        applyScaling(scaledWrapper.current, scaledContent.current)
      }
    }
    window.addEventListener('resize', onResize)
    onResize()
    return () => window.removeEventListener('resize', onResize)
  }, [])

  useEffect(() => {
    if (scaledWrapper.current && scaledContent.current) {
      applyScaling(scaledWrapper.current, scaledContent.current)
    }
  }, [miningStatus, scaledWrapper.current, scaledContent.current])

  // Live updates from service
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

  return (
    <Layout connected={connected} active={active} ready={ready}>
      <Panel layout='full-width' navigation={MiningPanelNavItems('Status')}>
        <div className='mining-panel'>
          <MiningDashboard
            miningStatus={miningStatus}
            scaledWrapper={scaledWrapper}
            scaledContent={scaledContent}
          />
        </div>
      </Panel>
    </Layout>
  )
}

