import { useState } from 'react'
import Layout from './layout'
import Panel from './panel'
import PrimaryNavigation from './primary-navigation'
import RemoteDisplays from './remote-displays'
import GoJournalStatus from './go-journal-status'
import GoTerminalSurvey from './go-terminal-survey'
import GoTerminalRoute from './go-terminal-route'
import GoTerminalShip from './go-terminal-ship'
import useGoProjection from './go-projection'
import * as navigation from '../lib/navigation-items'
import { Icon, Badge, Clock, show } from './go-terminal-widgets'
export default function GoTerminal (props) {
  const [settings, setSettings] = useState(false)
  const path = props.path === '/' || props.path === '/nav' ? '/nav/map' : props.path === '/ship' ? '/ship/status' : props.path === '/exploration' ? '/exploration/system' : props.path
  const group = path.split('/')[1]; const page = path.split('/')[2] || ''
  const menus = { nav: navigation.NavPanelNavItems, exploration: navigation.ExplorationPanelNavItems, ship: navigation.ShipPanelNavItems }
  const items = menus[group] ? menus[group](page, {}).filter(i => i.url) : []
  const { value: summary } = useGoProjection(props, 'getJournalSummary', 'journalSummaryChanged')
  const { value: ship } = useGoProjection(props, 'getGoShip', 'goShipChanged')
  let content
  if (['/nav/map', '/nav/list', '/exploration/system', '/exploration/biologicals'].includes(path)) content = <GoTerminalSurvey {...props} path={path} />
  else if (['/ship/status', '/ship/stats'].includes(path)) content = <GoTerminalShip {...props} />
  else if (['/nav/route', '/exploration/route'].includes(path)) {
    content = (
      <>
        <GoTerminalRoute {...props} />
        {props.nativePreview && props.role === 'owner' && <div className='go-native-survey'><GoTerminalSurvey {...props} path='/exploration/system' /></div>}
      </>
    )
  } else {
    content = (
      <div className='go-empty go-unported'>
        <Icon name='settings' /><h2>{group === 'cmdr' ? 'Commander' : group.charAt(0).toUpperCase() + group.slice(1)} workspace</h2>
        <p>This workspace has not been migrated. Keep the original DAEDALUS application for these capabilities.</p>
        <div className='go-shortcuts'>
          <a href='/nav/map'><Icon name='system-orbits' /> System map</a>
          <a href='/nav/route'><Icon name='route' /> Route plan</a>
          <a href='/exploration/biologicals'><Icon name='plant' /> Biology</a>
          <a href='/ship/status'><Icon name='ship' /> Ship journal</a>
        </div>
        <Badge>UNPORTED — NO HOST ACTIONS</Badge>
      </div>
    )
  }
  return (
    <div className='go-terminal' data-display-role={props.role}>
      <header>
        <hr className='small' />
        <h1 className='text-info go-terminal__brand'><Icon name='logo' /> DAEDALUS <span className='hidden-small'>Terminal</span></h1>
        <div className='go-terminal__status'>
          <Clock />
          <span className={props.connected && props.ready ? 'text-success' : 'text-danger'} role='status'><Icon name='signal' /> {props.connected ? (props.ready ? 'SERVICE READY' : 'CONNECTING') : 'DISCONNECTED'}</span>
          <Badge>{props.role === 'owner' ? 'LOCAL OWNER' : 'READ-ONLY DISPLAY'}</Badge>
          <button type='button' aria-label={props.role === 'owner' ? 'Owner settings' : 'Display status'} aria-expanded={settings} onClick={() => setSettings(s => !s)}><Icon name='settings' /></button>
        </div>
        <PrimaryNavigation />
      </header>
      <div className='go-terminal__telemetry'><span><Icon name='location-filled' /> {summary?.currentSystem || 'Awaiting local journal'}</span><span className='text-muted'>G2D · {summary?.sourceState || 'waiting'} · no provider uploads</span><span><Icon name='ship' /> {ship?.shipType || 'Ship unknown'} · Fuel {show(ship?.fuelMainT, 'T')}</span></div>
      <Layout connected={props.connected} ready={props.ready} className='go-terminal__layout'>
        <Panel navigation={items} className='go-terminal__panel'><div className='go-content'>{content}</div></Panel>
      </Layout>
      {settings && (
        <aside className='go-settings inspector' aria-label='Display settings'>
          <div className='go-page__heading'><h2>{props.role === 'owner' ? 'Owner settings' : 'Display status'}</h2><button aria-label='Close settings' onClick={() => setSettings(false)}>×</button></div>
          <GoJournalStatus {...props} stage='G2D' />
          {props.role === 'owner' && <RemoteDisplays />}
          {props.role !== 'owner' && <p className='text-muted'>Pairing, journal directories and device permissions are controlled by the local owner.</p>}
        </aside>
      )}
    </div>
  )
}
