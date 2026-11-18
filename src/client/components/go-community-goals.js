import { Metric } from './go-terminal-widgets'
const details = r => r?.details?.fields || r?.details || {}
function GoalCard ({ id, records }) {
  const progress = records.find(r => r.title === 'progress_snapshot'); const g = details(progress); const reward = records.find(r => r.title === 'reward_received'); const r = details(reward)
  return <article key={id} className='go-goal-card'><h3>{g.Title || 'Subscribed goal'}</h3><p className='text-primary'>{g.SystemName || 'System unknown'} · {g.MarketName || 'Station unknown'}</p>{g.Objective && <p>{g.Objective}</p>}<div className='go-fact-grid'><Metric label='Global total' value={g.CurrentTotal} /><Metric label='Tier reached' value={g.TierReached} /><Metric label='Your contribution' value={g.PlayerContribution} /><Metric label='Your percentile band' value={g.PlayerPercentileBand} unit='%' /><Metric label='Contributors' value={g.NumContributors} /><Metric label='Offered bonus' value={g.Bonus} unit='CR' /><Metric label='Received reward' value={r.Reward} unit='CR' /></div><p>{g.IsComplete == null ? 'Completion unknown' : g.IsComplete ? 'Complete' : 'In progress'} · {g.PlayerInTopRank === true ? 'In top rank' : g.PlayerInTopRank === false ? 'Outside top rank' : 'Rank unknown'}</p><small>Deadline {g.Expiry || 'not reported'} · Updated {progress?.at || 'not witnessed'}</small></article>
}
export default function GoCommunityGoals ({ page }) {
  const membership = page?.goalMembership
  const ids = membership?.ids || []
  const rows = page?.current || []
  const history = [...rows, ...(page?.rows || [])].filter(r => r.entity && (!ids.includes(r.entity) || r.kind === 'CommunityGoalReward' || r.title === 'reward_received'))
  const seen = new Set()
  const past = history.filter(r => { const d = details(r); const key = [r.entity, r.at, d.Reward, d.PlayerContribution].join('/'); if (seen.has(key)) return false; seen.add(key); return true })
  return (
    <section className='go-page' aria-label='Community Goals'><div className='go-page__heading'><h2>Community Goals</h2><small>{membership?.observedAt || 'No complete snapshot'}</small></div><p role='status'>{membership?.known ? `${ids.length} subscribed goals as last witnessed` : 'Subscribed goals unknown'}</p><div className='go-goal-cards'>{ids.map(id => <GoalCard key={id} id={id} records={rows.filter(r => r.entity === id)} />)}</div>{membership?.known && !ids.length && <p>No currently subscribed goals. Historical contributions and rewards are retained.</p>}<details><summary>Past participation & rewards</summary>{past.map(r => { const d = details(r); return <p key={r.id + r.kind}>{d.Title || d.Name || 'Goal ' + r.entity} · Historical participation · Contribution {d.PlayerContribution ?? '—'} · Received reward {d.Reward ?? '—'} CR · {r.at}</p> })}</details>
    </section>
  )
}
