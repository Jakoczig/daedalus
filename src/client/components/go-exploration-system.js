import useGoProjection from './go-projection'

const labels = {
  unknown: 'Survey unknown. No scanner count or body scans witnessed yet.',
  unverified: 'Survey unverified. Waiting for a commander session and confirmed system arrival.',
  observed: 'Partial journal-only survey observations; not a complete body list.',
  scanner_complete: 'FSS all-bodies completion witnessed. Listed scan rows can still be incomplete.',
  partial: 'Partial survey: invalid data or a display limit prevented a complete projection.',
  in_transit_last_known: 'Jump pending. Showing last-known arrived-system observations.',
  stale_source: 'Journal source unavailable. These are last-known observations.',
  replaying: 'Replaying local journal history…'
}
const truth = value => value == null ? 'Unknown' : value ? 'Yes' : 'No'
const number = (value, unit) => value == null ? 'Unknown' : `${value.toFixed(2)} ${unit}`
export default function GoExplorationSystem (props) {
  const { value, error } = useGoProjection(props, 'getGoExploration', 'goExplorationChanged')
  if (!props.connected) return <section aria-label='Current-system survey'><h2>Current-system exploration</h2><p role='status'>Service disconnected. Survey unavailable until reconnect.</p></section>
  if (!props.ready) return <p role='status'>Waiting for the Go service to become ready.</p>
  return (
    <section aria-label='Current-system survey' style={{ minWidth: 0 }}>
      <h2>Current-system exploration — {value?.systemName || 'Unknown system'}</h2>
      <p>Journal-only confirmed observations. Unreported fields remain unknown.</p>
      <p role='status'>{labels[value?.surveyState] || 'Loading local survey evidence…'}</p>
      {error && <p role='alert'>{error}</p>}
      {value && (
        <>
          <p>Scanner-reported bodies: {value.reportedBodyCount ?? 'Unknown'}. FSS progress: {value.fssProgress == null ? 'Unknown' : `${Math.round(value.fssProgress * 100)}%`}. Observed scan rows: {value.bodies?.length || 0}.</p>
          <p>All-bodies completion: {value.allBodiesFound ? `Witnessed (${value.scannerBodyCount ?? 'unknown'} reported)` : 'Not witnessed in this journal'}.</p>
          {value.truncated && <p role='alert'>Body list exceeds a cache or display budget. This table is incomplete.</p>}
          {value.bodies?.length > 0 && (
            <div style={{ overflowX: 'auto' }}>
              <table aria-label='Observed body scans' style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead><tr><th>Body / class</th><th>Physical observations</th><th>Scan evidence</th><th>Confirmed signals</th></tr></thead>
                <tbody>{value.bodies.map(body => (
                  <tr key={body.bodyId} style={{ verticalAlign: 'top', borderBottom: '1px solid currentColor' }}>
                    <td style={{ padding: '.6rem', overflowWrap: 'anywhere' }}><strong>{body.name || 'Unknown'}</strong><p>{body.kind || 'Unknown class'}</p></td>
                    <td style={{ padding: '.6rem' }}><p>Landable: {truth(body.landable)}</p><p>Terraformable: {truth(body.terraformable)}</p><p>Gravity: {number(body.gravityG, 'g')}</p><p>Arrival distance: {number(body.distanceLS, 'LS')}</p><p>Atmosphere: {body.atmosphere || 'Unknown'}</p><p>Volcanism: {body.volcanism || 'Unknown'}</p></td>
                    <td style={{ padding: '.6rem' }}><p>Previously discovered: {truth(body.previouslyDiscovered)} (source fact)</p><p>Previously mapped: {truth(body.previouslyMapped)} (source fact)</p><p>Your DSS completion: {body.dssMapped ? 'Witnessed SAAScanComplete' : 'Not witnessed'}</p></td>
                    <td style={{ padding: '.6rem' }}><p>Biological signals: {body.bioSignals ?? 'Unknown'}</p><p>Geological signals: {body.geoSignals ?? 'Unknown'}</p><p>Observed genus: {body.genuses?.length ? body.genuses.join(', ') : 'Unknown'}</p></td>
                  </tr>
                ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
      <p>Signal counts and genus observations do not identify a species. Gravity uses 9.80665 m/s² per Earth g. Other exploration features remain unfinished.</p>
    </section>
  )
}
