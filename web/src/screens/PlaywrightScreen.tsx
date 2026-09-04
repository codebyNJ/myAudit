import { useStore } from '../store'
import { IcCircleCheck, IcCircleX } from '../components/icons'

export function PlaywrightScreen() {
  const s = useStore()
  if (!s.runId) return <div className="empty-mid"><h3>No run</h3><p>Create a project to see verification results.</p></div>
  const checks = (s.detail?.events || []).filter((e) => /verify|checkpoint|node\.fail/.test(e.kind))
  return (
    <div className="pane">
      <h2>Verification</h2>
      <p className="sub">Each generated feature must pass a real check (syntax + build/boot) before it lands.</p>
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
      ) : <div className="empty-mid" style={{ position: 'static', paddingTop: 40 }}><h3>All clear</h3><p>Verification results (and any failures that need your review) appear here as features are generated.</p></div>}
    </div>
  )
}
