import { useStore } from '../store'
import { evColor, fmtTime } from '../components/util'

export function ActivityScreen() {
  const s = useStore()
  if (!s.runId) return <div className="empty-mid"><h3>No activity</h3><p>Import a codebase to see its live activity.</p></div>
  const events = s.detail?.events || []
  return (
    <div className="pane">
      <h2>Activity</h2>
      <p className="sub">Live event log for {(s.runs || []).find((r) => r.id === s.runId)?.project}</p>
      {events.length ? (
        <div className="evt-list">
          {events.slice().reverse().map((e, i) => (
            <div className="evt-row" key={i}>
              <span className="dot" style={{ background: evColor(e.kind) }} />
              <span className="ts">{fmtTime(e.ts)}</span>
              <span className="kind" style={{ color: evColor(e.kind) }}>{e.kind}</span>
              <span className="msg">{e.msg}</span>
            </div>
          ))}
        </div>
      ) : <div className="empty-mid" style={{ position: 'static', paddingTop: 40 }}><h3>No events yet</h3><p>Activity streams here as the build runs.</p></div>}
    </div>
  )
}
