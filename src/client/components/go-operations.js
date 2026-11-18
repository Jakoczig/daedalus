import { useEffect, useState } from 'react'
import { Badge, Icon } from './go-terminal-widgets'
import GoCommunityGoals from './go-community-goals'
import GoFleet from './go-fleet'
import GoProviderReference from './go-provider-reference'

const routes = {
  '/ship/fleet': ['Fleet & stored modules', 'fleet', ['fleet', 'modules', 'module-events', 'fleet-history', 'loadouts']],
  '/ship/carrier': ['Owned carrier operations', 'carrier', ['carrier', 'carrier-history']],
  '/trade/commodities': ['Station commodity observations', 'commodities', ['commodities', 'markets', 'trades']],
  '/trade/routes': ['Two-market quoted comparison', 'comparison', ['comparison', 'commodities', 'markets', 'trades']],
  '/trade/ptn': ['Pilots Trade Network', 'external', []],
  '/trade/fcoc': ['Fleet Carrier Owners Club', 'external', []],
  '/mining/statistics': ['Historical mining & sale receipts', 'mining', ['mining', 'mining-sales', 'trades']],
  '/cmdr/community-goals': ['Community Goals', 'goals', ['goals']],
  '/missions/powerplay': ['Powerplay observations', 'powerplay', ['powerplay']],
  '/cmdr/wing': ['Wing, NPC crew & squadron', 'crew', ['crew', 'wing', 'squadron']]
}
export const operationPaths = Object.keys(routes)
const labels = { 'mining-sales': 'Conservatively attributed mining receipts', 'module-events': 'Module storage / sale / transfer history', fleet: 'Owned ships · last witnessed', modules: 'Latest stored-module snapshot', 'fleet-history': 'Ship ownership / transfer events', loadouts: 'Exact historical loadouts', carrier: 'Owned carrier · latest field evidence', 'carrier-history': 'Carrier event history', trades: 'Purchases & sale receipts', markets: 'Market observations', commodities: 'Commodity quote history', comparison: 'Compare two observed markets', mining: 'Refined tonnes · historical', goals: 'Participation / progress / rewards', powerplay: 'Pledge / rank / actions', crew: 'NPC crew events', wing: 'Wing / multicrew history', squadron: 'Own squadron history' }
function Evidence ({ value, depth = 0 }) {
  if (value == null) return <span>Unknown / not witnessed</span>
  if (typeof value !== 'object') return <span>{typeof value === 'boolean' ? (value ? 'Yes' : 'No') : String(value)}</span>
  if (depth > 3) return <pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{JSON.stringify(value, null, 2)}</pre>
  return <dl>{Object.entries(value).slice(0, 128).map(([key, v]) => <div key={key}><dt>{key}</dt><dd><Evidence value={v} depth={depth + 1} /></dd></div>)}</dl>
}
export default function GoOperations (props) {
  const [title, initial, modes] = props.initialKind === 'modules' ? ['Stored modules', 'modules', ['modules', 'module-events']] : routes[props.path] || routes['/ship/fleet']
  const [kind, setKind] = useState(initial)
  const [commodity, setCommodity] = useState(''); const [market, setMarket] = useState(''); const [otherMarket, setOtherMarket] = useState(''); const [id, setID] = useState(''); const [before, setBefore] = useState('')
  const [page, setPage] = useState(null); const [notice, setNotice] = useState(''); const [revision, setRevision] = useState(0); const [ack, setAck] = useState(false); const [preview, setPreview] = useState(null)
  const query = JSON.stringify({ kind, commodity: commodity.toLowerCase(), market, otherMarket, id, before, limit: 30 })
  useEffect(() => {
    let active = true; let pending = false; let timer
    setPage(null); setPreview(null); setAck(false)
    if (props.role === 'owner' && props.connected && props.ready && kind !== 'external') {
      const load = async () => { if (pending) return; pending = true; try { const value = await props.sendEvent('getGoOperations', JSON.parse(query)); if (active) { setPage(value); setNotice('') } } catch (e) { if (active) setNotice('Operations unavailable; journal history retained.') } finally { pending = false } }
      load(); timer = setInterval(load, 3000)
    }
    return () => { active = false; clearInterval(timer) }
  }, [props.role, props.connected, props.ready, props.sendEvent, query, revision, kind])
  const change = setter => event => { setter(event.target.value); setBefore(''); setPreview(null); setAck(false) }
  if (props.role !== 'owner') return <section className='go-page'><h2>{title}</h2><Badge>LOCAL OWNER ONLY</Badge><p>Fleet, carrier finances, prices, transactions and other commanders’ names are private. Paired displays remain read-only; this private history is not transmitted.</p></section>
  if (initial === 'external') return <section className='go-page'><h2>{title}</h2><p>External community reference only. DAEDALUS has no verified consumable PTN/FCOC feed. No scraping or automatic external request.</p><a href={props.path.endsWith('/ptn') ? 'https://pilotstradenetwork.com/' : 'https://inara.cz/elite/squadron/7636/'} target='_blank' rel='noreferrer'>Open community website</a></section>
  if (!props.connected || !props.ready) return <p role='status'>Operations disconnected. Waiting for authenticated recovery.</p>
  if (initial === 'goals') return <GoCommunityGoals page={page} />
  if (initial === 'fleet' && kind === 'fleet') return <GoFleet page={page} sendEvent={props.sendEvent} />
  const exportPage = async () => {
    try { const value = await props.sendEvent('exportGoOperations', { ...JSON.parse(query), acknowledge: ack, previewHash: preview?.hash }); const url = URL.createObjectURL(new Blob([value.csv], { type: 'text/csv;charset=utf-8' })); const a = document.createElement('a'); a.href = url; a.download = 'eridion-private-' + kind + '.csv'; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000) } catch { setNotice('Private export unavailable. Preview and acknowledge first.') }
  }
  return (
    <section className='go-page go-field-ops' aria-label={title}>
      <div className='go-page__heading'><h2><Icon name='ship' /> {title}</h2><Badge>LOCAL JOURNAL · PRIVATE</Badge></div>
      <p>{page?.notice || 'Loading indexed history…'}</p>
      <div className='go-history-actions'>
        <label>View <select aria-label='Operations view' value={kind} onChange={change(setKind)}>{modes.map(k => <option key={k} value={k}>{labels[k]}</option>)}</select></label>
        <label>{kind === 'fleet' ? 'Ship model / name' : 'Commodity symbol'} <input aria-label='Commodity or ship filter' value={commodity} maxLength={128} onChange={change(setCommodity)} placeholder='Blank = all observed' /></label>
        {['commodities', 'comparison', 'trades'].includes(kind) && <label>Market ID <input value={market} maxLength={20} onChange={change(setMarket)} /></label>}
        {kind === 'comparison' && <label>Sell-market ID <input value={otherMarket} maxLength={20} onChange={change(setOtherMarket)} /></label>}
        {['loadouts', 'fleet-history'].includes(kind) && <label>Exact Ship ID <input value={id} maxLength={20} onChange={change(setID)} /></label>}
        <button onClick={() => setRevision(v => v + 1)}>Refresh local history</button>
      </div>
      {kind === 'carrier' && <p>Snapshots are generated by carrier management. Transfers are not earnings; posted orders are not completed trades. Scheduled jumps are separate from verified location. <a href='/cmdr/overview'>Commander / construction context</a></p>}
      {kind === 'trades' && <p>Witnessed expenditure and receipts are separate. Gross spread uses positive game-reported AvgPricePaid only; overhead, limpets, rebuy and unknown sources are excluded. Zero/missing basis means receipts, not profit. No FIFO assumption.</p>}
      {kind === 'mining' && <p>Each MiningRefined event records one tonne. Attributed mining receipts require a witnessed empty cargo baseline and exclusively refined input in the same session. Mixed or unknown cargo is excluded; mining profit remains unknown. All sales remain available separately. No inferred stock valuation.</p>}
      {kind === 'loadouts' && <p>Exact witnessed final engineering modifiers, never inferred from blueprint grade. Stored ship full performance remains unavailable. <a href='/ship/stats'>Current ship native statistics</a></p>}
      {kind === 'powerplay' && <p>Witnessed fields only. No inferred merit conversion, decay, post-2.0 rank rules or causal BGS influence.</p>}
      {kind === 'goals' && <p>Last witnessed participation, expiry and progress. Reward is received only for CommunityGoalReward; an offered bonus is not payment. <a href='/cmdr/field'>Local BGS activity</a></p>}
      {page?.goalMembership && <section aria-label='Subscribed-goal membership'><h3>{page.goalMembership.known ? `${page.goalMembership.ids.length} subscribed goals as last witnessed` : 'Subscribed goals unknown'}</h3><p>{page.goalMembership.observedAt || 'No complete snapshot'} · {page.goalMembership.notice}</p></section>}
      {page?.totals && <section><h3>Selected page transaction totals</h3><Evidence value={page.totals} /></section>}
      {page?.current?.length > 0 && <details><summary>Last witnessed state · not a live account</summary>{page.current.map(r => <section key={r.id}><h3>{r.title} · {r.entity || 'identity not witnessed'}</h3><small>{r.at} · {r.state.replaceAll('_', ' ')}</small><Evidence value={r.details} /></section>)}</details>}
      {!page?.rows?.length && <p role='status'>{kind === 'carrier' ? 'No owned carrier observed' : 'No matching witnessed history. Open the relevant in-game screen or recover available original journals in Data/History.'}</p>}
      {page?.rows?.map(r => <details key={r.id}><summary>{r.title || r.kind} · {r.entity || 'identity not witnessed'} · {r.at || 'time unknown'} · {r.state.replaceAll('_', ' ')}</summary><p>{r.kind} · Source: local journal / explicitly labeled cached provider · {r.at} (last known)</p><Evidence value={r.details} />{kind === 'fleet' && <div><button onClick={() => { setID(r.entity); setKind('loadouts'); setBefore('') }}>Inspect historical loadouts</button><button onClick={() => { setID(r.entity); setKind('fleet-history'); setBefore('') }}>Ownership / transfer history</button></div>}</details>)}
      <div className='go-history-actions'><button disabled={!before} onClick={() => setBefore('')}>Newest page</button><button disabled={!page?.next} onClick={() => setBefore(page.next)}>Older page</button></div>
      <details><summary>Private CSV export · selected page only</summary><p>Contains private financial details, ship locations and potentially other players’ names. Preview the exact currently loaded page before acknowledging. No provider upload occurs.</p><button disabled={!page?.rows?.length} onClick={() => { setPreview({ rows: page.rows, hash: page.previewHash }); setAck(false) }}>Preview selected private records</button>{preview && <><pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{JSON.stringify(preview.rows, null, 2)}</pre><label><input type='checkbox' checked={ack} onChange={e => setAck(e.target.checked)} /> I reviewed these private records and understand the exposure.</label><button disabled={!ack} onClick={exportPage}>Download selected private CSV</button></>}</details>
      {page?.reference?.length > 0 && <details><summary>Reconciled cached account reference · Frontier first, Inara gaps only</summary>{page.reference.filter(f => ['ships', 'ship', 'carrier', 'squadron', 'communityGoals'].includes(f.field)).map(f => <section key={f.field}><h3>{f.field} · {f.source}</h3><p>Observed {f.observedAt || 'unknown'} · fetched {f.fetchedAt} · {f.stale ? 'stale' : 'cached'}; separate source evidence, not assumed current</p><Evidence value={f.value} /></section>)}</details>}
      <GoProviderReference {...props} publicGoals={kind === 'goals'} />
      {notice && <p role='status'>{notice}</p>}
    </section>
  )
}
