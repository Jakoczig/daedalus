import dynamic from 'next/dynamic'
import useGoProjection from './go-projection'
import GoProviderReference from './go-provider-reference'
import GoShipStatistics from './go-ship-statistics'
import GoParityShip from './go-parity-ship'
import GoParityActivity, { GoMiningRings } from './go-parity-activity'

const GoEngineering = dynamic(() => import('./go-engineering'), { loading: () => <p>Loading bundled engineering reference…</p> })
export const parityPaths = ['/ship/status', '/ship/stats', '/ship/modules', '/ship/cargo', '/ship/inventory', '/cmdr/overview', '/cmdr/session', '/missions/overview', '/mining/status', '/mining/statistics', '/mining/log', '/mining/rings', '/eng/blueprints', '/eng/engineers', '/eng/raw-materials', '/eng/manufactured-materials', '/eng/encoded-materials', '/eng/xeno-materials', '/eng/material-planner', '/eng/wishlist']
const flightDelta = { name: 'goFlightChanged', merge: (workspace, status) => ({ ...workspace, parity: { ...workspace.parity, status } }) }
export default function GoParity (props) {
  const { value, error } = useGoProjection(props, 'getGoWorkspace', 'goWorkspaceChanged', flightDelta)
  if (!props.connected || !props.ready) return <div className='go-empty' role='status'>Journal workspace disconnected. Waiting for authenticated recovery.</div>
  if (error) return <p role='alert'>{error}</p>
  if (props.path === '/mining/rings') return <GoMiningRings {...props} />
  if (props.path.startsWith('/eng/')) return <GoEngineering key={props.path} path={props.path} workspace={value} />
  if (props.path === '/ship/stats') return <><GoShipStatistics {...props} workspace={value} /><GoProviderReference {...props} /></>
  if (props.path.startsWith('/ship/')) return <GoParityShip key={props.path} path={props.path} workspace={value} />
  return <><GoParityActivity key={props.path} path={props.path} workspace={value} />{props.path.startsWith('/cmdr/') && <GoProviderReference {...props} />}</>
}
