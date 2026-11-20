import GoOperations, { operationPaths } from './go-operations'
import { BGSTally, PrivateCommanderActivity } from './go-field-operations'
import { useState, useEffect } from 'react'
import Layout from './layout'
import Panel from './panel'
import PrimaryNavigation from './primary-navigation'
import RemoteDisplays from './remote-displays'
import GoJournalStatus from './go-journal-status'
import GoTerminalSurvey from './go-terminal-survey'
import { CompanionSettings } from './go-companion'
import GoGalaxy from './go-galaxy'
import GoSpansh from './go-spansh'
import GoColonisation from './go-colonisation'
import { transition } from '../lib/go-display-controller'
import GoTerminalRoute from './go-terminal-route'
import GoTerminalShip from './go-terminal-ship'
import GoParity, { parityPaths } from './go-parity'
import useGoProjection from './go-projection'
import * as navigation from '../lib/navigation-items'
import { Icon, Badge, Clock, show } from './go-terminal-widgets'
export default function GoTerminal (props) {
  useEffect(() => { if (typeof document !== 'undefined') document.title = 'ERIDION · ERID' }, [])
  const [settings, setSettings] = useState(false)
  useEffect(() => { let active = true; if (props.role === 'owner' && props.stage === 'G2E' && props.connected && props.ready) props.sendEvent('getGoAccounts').then(v => { if (active && v.choiceRequired) window.location.replace('/launcher') }).catch(() => {}); return () => { active = false } }, [props.role, props.stage, props.connected, props.ready, props.sendEvent])
  const [display, setDisplay] = useState({ visible: false })
  const { value: flight } = useGoProjection({ ...props, connected: props.connected && props.stage === 'G2E' }, 'getGoNavigationContext', 'goNavigationContextChanged')
  const { value: plotted } = useGoProjection({ ...props, connected: props.connected && props.stage === 'G2E' }, 'getGoNavigation', 'goNavigationChanged')
  const controllerInput = { mode: flight?.navigation?.mode, episode: flight?.navigation?.episode, fresh: flight?.navigation?.fresh, routeKnown: !!plotted?.hops?.length, policy: flight?.policy, disconnected: !props.connected || !props.ready }
  useEffect(() => { setDisplay(previous => transition(previous, controllerInput)) }, [flight, plotted, props.connected, props.ready])
  const manual = event => { if (event.target.closest('.secondary-navigation, header button') || event.target.closest('[data-primary-navigation]')) setDisplay(previous => transition(previous, { ...controllerInput, manual: true })) }
  const [communityVersion, setCommunityVersion] = useState(0)
  const path = props.path === '/cmdr/field' || props.path?.startsWith('/controls') ? '/cmdr/overview' : props.path === '/' || props.path === '/nav' ? '/nav/map' : props.path === '/ship' ? '/ship/status' : props.path === '/exploration' ? '/exploration/system' : props.path === '/eng' ? '/eng/blueprints' : props.path === '/cmdr' ? '/cmdr/overview' : props.path === '/mining' ? '/mining/status' : props.path === '/missions' ? '/missions/overview' : props.path === '/trade' ? '/trade/routes' : props.path
  const group = path.split('/')[1]; const page = path.split('/')[2] || ''
  const menus = { nav: navigation.NavPanelNavItems, exploration: navigation.ExplorationPanelNavItems, ship: navigation.ShipPanelNavItems, eng: navigation.EngineeringPanelNavItems, cmdr: navigation.CmdrPanelNavItems, missions: navigation.MissionsPanelNavItems, mining: navigation.MiningPanelNavItems, trade: navigation.TradePanelNavItems }
  const items = menus[group] ? menus[group](page, {}).filter(i => i.url).map(item => group === 'nav' ? { ...item, name: ({ Map: 'System', List: 'System List', 'Neutron Plotter': 'Spansh Plotter' })[item.name] || item.name, active: (item.url.pathname || item.url) === path } : item) : []
  if (group === 'nav' && props.stage === 'G2E') items.push({ name: 'Galaxy', icon: 'system-orbits', url: '/nav/galaxy', active: page === 'galaxy' })
  const { value: summary } = useGoProjection(props, 'getJournalSummary', 'journalSummaryChanged')
  const { value: ship } = useGoProjection(props, 'getGoShip', 'goShipChanged')
  if (group === 'cmdr' && props.stage === 'G2E') items.push({ name: 'Colonisation', icon: 'scan', url: '/cmdr/colonisation', active: page === 'colonisation' })
  let content
  if (props.stage === 'G2E' && path === '/missions/local-activity') content = <section className='go-page'><h2>Local activity</h2>{props.role === 'owner' ? <PrivateCommanderActivity {...props} /> : <p>Private commander history is available to the local owner only.</p>}<p>Mission and Powerplay observations remain in their respective pages.</p></section>
  else if (props.stage === 'G2E' && operationPaths.includes(path)) content = <GoOperations key={path} {...props} path={path} />
  else if (path === '/cmdr/colonisation' && props.stage === 'G2E') content = <GoColonisation {...props} />
  else if (path === '/nav/neutron-plotter' && props.stage === 'G2E') content = <GoSpansh {...props} />
  else if (path === '/nav/galaxy' && props.stage === 'G2E') content = <GoGalaxy context={flight?.navigation} route={plotted} {...props} />
  else if (props.stage === 'G2E' && parityPaths.includes(path)) content = <><GoParity {...props} path={path} />{path === '/missions/overview' && <BGSTally {...props} context='missions' />}</>
  else if (['/nav/map', '/nav/list', '/exploration/system', '/exploration/biologicals'].includes(path)) content = <GoTerminalSurvey {...props} communityVersion={communityVersion} path={path} />
  else if (['/ship/status', '/ship/stats'].includes(path)) content = <GoTerminalShip {...props} />
  else if (['/nav/route', '/exploration/route'].includes(path)) {
    content = (
      <>
        <GoTerminalRoute {...props} context={flight?.navigation} />
        {props.nativePreview && props.role === 'owner' && <div className='go-native-survey'><GoTerminalSurvey {...props} communityVersion={communityVersion} path='/exploration/system' />{props.stage === 'G2E' && <GoParity {...props} path='/ship/modules' />}</div>}
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
    <div className='go-terminal' data-display-role={props.role} onPointerDownCapture={manual} onKeyDownCapture={event => { if (event.key === 'Enter' || event.key === ' ') manual(event) }}>
      <header>
        <hr className='small' />
        <h1 className='text-info go-terminal__brand'><Icon name='logo' /> ERIDION <span className='hidden-small'>Terminal</span></h1>
        <div className='go-terminal__status'>
          <Clock />
          <span className={props.connected && props.ready ? 'text-success' : 'text-danger'} role='status'><Icon name='signal' /> {props.connected ? (props.ready ? 'SERVICE READY' : 'CONNECTING') : 'DISCONNECTED'}</span>
          <Badge>{props.role === 'owner' ? 'LOCAL OWNER' : 'READ-ONLY DISPLAY'}</Badge>
          <button type='button' aria-label={props.role === 'owner' ? 'Owner settings' : 'Display status'} aria-expanded={settings} onClick={() => setSettings(s => !s)}><Icon name='settings' /></button>
        </div>
        <PrimaryNavigation native />
      </header>
      <div className='go-terminal__telemetry'><span><Icon name='location-filled' /> {summary?.currentSystem || 'Awaiting local journal'}</span><span className='text-muted'>{summary?.stage ? 'G2N' : props.stage || 'G2D'} · {summary?.cached ? 'LAST KNOWN / ' : ''}{summary?.stage || summary?.sourceState || 'waiting'} · local-first · explicit provider consent</span><span><Icon name='ship' /> {ship?.shipType || 'Ship unknown'} · Fuel {show(ship?.fuelMainT, 'T')}</span></div>
      <Layout connected={props.connected} ready={props.ready} className='go-terminal__layout'>
        <Panel temporaryLabels navigation={items} className='go-terminal__panel'><div className='go-content'>{display.visible ? <section className='go-fsd' aria-label='Automatic FSD display'><div className='go-page__heading'><h2>{display.finished ? 'Jump ended' : flight?.navigation?.mode === 'charge' ? 'Hyperdrive charging' : 'Hyperspace jump'}</h2><button onClick={() => setDisplay(previous => transition(previous, { ...controllerInput, manual: true }))}>Return to previous screen</button></div><div className='go-fsd-columns'><GoTerminalRoute {...props} compact context={flight?.navigation} /><GoGalaxy context={flight?.navigation} route={plotted} {...props} compact /></div></section> : content}</div></Panel>
      </Layout>
      {settings && (
        <aside className='go-settings inspector' aria-label='Display settings'>
          <div className='go-page__heading'><h2>{props.role === 'owner' ? 'Owner settings' : 'Display status'}</h2><button aria-label='Close settings' onClick={() => setSettings(false)}>Ä‚—</button></div>
          <GoJournalStatus {...props} stage={props.stage || 'G2D'} />
          {props.role === 'owner' && props.stage === 'G2E' && <CompanionSettings {...props} onProvidersChange={() => setCommunityVersion(v => v + 1)} />}
          {props.role === 'owner' && <RemoteDisplays />}
          {props.role !== 'owner' && <p className='text-muted'>Pairing, journal directories and device permissions are controlled by the local owner.</p>}
        </aside>
      )}
    </div>
  )
}
