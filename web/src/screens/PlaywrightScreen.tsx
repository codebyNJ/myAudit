import { useStore } from '../store'
import { IcCircleCheck, IcCircleX } from '../components/icons'

export function PlaywrightScreen() {
  const s = useStore()
  if (!s.runId) return <div className="empty-mid"><h3>No run</h3><p>Import a codebase to see verification results.</p></div>
  const checks = (s.detail?.events || []).filter((e) => /verify|regression|checkpoint|node\.fail|qa\.install/.test(e.kind))
  return (
    <div className="pane">
      <h2>Verification</h2>
      <p className="sub">Test runs and regression checks behind each autonomous fix — passes, failures, and anything needing your review.</p>
      {checks.length ? (
        <div className="evt-list">
          {checks.map((e, i) => (
            <div className="evt-row" key={i}>
              {/fail|checkpoint/.test(e.kind) ? <IcCircleX /> : <IcCircleCheck />}
              <span className="msg" style={{ flex: 1 }}>{e.msg}</span>
              <span className="kind" style={{ color: 'var(--text-muted)' }}>{e.kind}</span>
            </div>
          ))}
        </div>
      ) : <div className="empty-mid" style={{ position: 'static', paddingTop: 40 }}><h3>No verification events yet</h3><p>As QA runs suites and the dev loop checks its fixes, the results appear here.</p></div>}
    </div>
  )
}
