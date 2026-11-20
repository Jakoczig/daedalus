import { useEffect, useState } from 'react'
import { Metric } from './go-terminal-widgets'
const fields = r => r?.details?.fields || r?.details || {}
const amount = v => { if (v == null || v === '') return null; try { return BigInt(v).toLocaleString() } catch { return String(v) } }
const date = v => { const d = new Date(v); return Number.isNaN(d.getTime()) ? 'Not reported' : d.toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) }
export function goalCards (page) {
  const source = (page?.reference || []).filter(f => f.field === 'communityGoals').flatMap(f => [f, ...(f.alternatives || [])]).find(f => f.source === 'frontier')
  const global = source?.value?.global || []
  const ids = new Set((page?.goalMembership?.ids || []).map(String))
  for (const g of global.slice(0, 256)) if (typeof g.id === 'string') ids.add(g.id)
  return [...ids].slice(0, 256).map(id => {
    const records = (page?.current || []).filter(r => r.entity === id)
    const progress = records.find(r => r.title === 'progress_snapshot'); const local = fields(progress)
    const remote = global.find(g => g.id === id); const fresh = remote && !source.stale
    const prefer = (key, fallback) => fresh && remote[key] != null && remote[key] !== '' ? remote[key] : fallback ?? remote?.[key]
    return { id, title: prefer('title', local.Title) || 'Community Goal', system: prefer('system', local.SystemName), station: prefer('station', local.MarketName), objective: prefer('objective', local.Objective), total: prefer('globalTotal', local.CurrentTotal), target: remote?.targetTotal, expiry: prefer('expiry', local.Expiry), contribution: local.PlayerContribution ?? null, band: local.PlayerPercentileBand ?? null, tier: local.TierReached ?? null, contributors: local.NumContributors ?? null, complete: local.IsComplete, subscribed: page?.goalMembership?.ids?.includes(id) === true, bonus: local.Bonus, reward: fields(records.find(r => r.title === 'reward_received')).Reward, remote: !!remote, stale: source?.stale, fetched: source?.fetchedAt, observed: progress?.at }
  })
}
export default function GoCommunityGoals ({ page, role, sendEvent, onRefresh }) {
  const [provider, setProvider] = useState(null); const [busy, setBusy] = useState(false); const [notice, setNotice] = useState('')
  useEffect(() => { let active = true; if (role === 'owner' && sendEvent) sendEvent('getGoProviders').then(v => { if (active) setProvider(v.find(p => p.capability.id === 'frontier')) }).catch(() => {}); return () => { active = false } }, [role, sendEvent])
  const refresh = async () => { setBusy(true); setNotice(''); try { await sendEvent('refreshGoCommunityGoals'); onRefresh?.() } catch (e) { setNotice(e.message || 'Frontier goals unavailable; saved progress retained.') } finally { setBusy(false) } }
  const cards = goalCards(page)
  const past = [...(page?.current || []), ...(page?.rows || [])].filter((r, i, all) => r.entity && (r.title === 'reward_received' || !page?.goalMembership?.ids?.includes(r.entity)) && all.findIndex(x => x.id === r.id && x.kind === r.kind) === i)
  return (
    <section className='go-page go-goals' aria-label='Community Goals'>
      <div className='go-page__heading'><div><h2>Community Goals</h2><small>{page?.goalMembership?.known ? `${page.goalMembership.ids.length} subscribed` : 'Subscriptions not reported'}</small></div>{role === 'owner' && sendEvent && <button disabled={busy || !provider?.config.read || provider?.config.paused || !provider?.credentialStored || !!provider?.capability.gate} onClick={refresh}>{busy ? 'Refreshing…' : 'Refresh Frontier goals'}</button>}</div>
      {role === 'owner' && provider && (!provider.config.read || !provider.credentialStored || provider.capability.gate) && <p>Connect and allow Frontier reads in Settings to refresh global goals. Saved participation remains available offline.</p>}
      <div className='go-goal-cards'>{cards.map(g => <article className='go-goal-card' key={g.id}><div className='go-page__heading'><h3>{g.title}</h3><span className='go-badge'>{g.subscribed ? 'SUBSCRIBED' : 'GLOBAL GOAL'}</span></div><p className='text-primary'>{g.system || 'System not reported'} · {g.station || 'Station not reported'}</p>{g.objective && <p>{g.objective}</p>}<div className='go-fact-grid'><Metric label='Global total' value={amount(g.total)} />{g.target != null && g.target !== '' && <Metric label='Global target' value={amount(g.target)} />}{g.subscribed && <><Metric label='Your contribution' value={amount(g.contribution)} />{g.band != null && <Metric label='Your percentile band' value={g.band} unit='%' />}</>}{g.tier != null && <Metric label='Tier reached' value={g.tier} />}{g.contributors != null && <Metric label='Contributors' value={amount(g.contributors)} />}{g.subscribed && <>{g.bonus != null && <Metric label='Offered bonus' value={amount(g.bonus)} unit='CR' />}{g.reward != null && <Metric label='Received reward' value={amount(g.reward)} unit='CR' />}</>}</div><p>{g.complete == null ? 'Completion not reported' : g.complete ? 'Last reported: complete' : 'Last reported: in progress'} · Deadline {date(g.expiry)}</p><small>{g.remote ? `Frontier · ${g.stale ? 'expired cache' : 'cached'} · fetched ${date(g.fetched)}` : `Progress updated ${date(g.observed)}`}</small></article>)}</div>
      {!cards.length && <p>No goals in the saved snapshots.</p>}<details><summary>Source details</summary><p>Global progress does not establish your subscription or contribution. Personal participation and received rewards come from verified journal snapshots.</p></details>
      {past.length > 0 && <details><summary>Past participation & rewards</summary>{past.map(r => { const d = fields(r); return <p key={r.id + r.kind}>{d.Title || d.Name || 'Goal ' + r.entity} · Contribution {amount(d.PlayerContribution) ?? '—'} · Reward {amount(d.Reward) ?? '—'} CR · {date(r.at)}</p> })}</details>}
      {notice && <p role='status'>{notice}</p>}
    </section>
  )
}
