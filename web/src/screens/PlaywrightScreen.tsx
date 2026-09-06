import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { api, type NodeCard } from '../api'
import { IcCircleCheck, IcCircleX } from '../components/icons'
import { evColor, fmtTime } from '../components/util'

// Overview: the run dashboard — headline stat tiles, per-module QA, fix
// verification and UI previews, with a live Activity feed in the rail. Merges
// the old Summary + Activity tabs into one Vercel-style dashboard.
export function PlaywrightScreen() {
  const s = useStore()
  const [cards, setCards] = useState<NodeCard[] | null>(null)

  useEffect(() => {
    if (!s.runId) { setCards(null); return }
    let alive = true
    const load = () => api.board(s.runId!).then((c) => { if (alive) setCards(c) }).catch(() => {})
    load()
    const h = setInterval(load, 2000)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  if (!s.runId) return <div className="empty-mid"><h3>No run</h3><p>Import a codebase to see the overview.</p></div>
  if (cards == null) return <div className="empty-mid"><div className="spin" /></div>

  const qa = cards.filter((c) => c.type === 'qa')
  const bugs = cards.filter((c) => c.type === 'bug')
  const findingsFor = (mod?: string) => bugs.filter((b) => (b.tags || []).includes('module:' + mod)).length
  const previews = (s.detail?.files || []).filter((f) => f.path.startsWith('.myaudit/preview/') && /\.(png|jpe?g|webp|gif)$/i.test(f.path))
  const events = s.detail?.events || []

  const sev = { high: 0, medium: 0, low: 0 }
  for (const b of bugs) if (b.severity && b.severity in sev) sev[b.severity as keyof typeof sev]++
  const verified = bugs.filter((b) => b.status === 'done').length
  const needsReview = bugs.filter((b) => b.status === 'in_review').length
  const failed = bugs.filter((b) => b.status === 'failed').length
  const runStatus = s.detail?.run?.status || '—'
  const cost = s.detail?.cost_usd || 0

  const fixIcon = (st: string) => (st === 'done' ? <IcCircleCheck /> : st === 'failed' ? <IcCircleX /> : <span className="v-dot" />)
  const fixWord = (st: string) => (st === 'done' ? 'verified' : st === 'failed' ? 'failed regression' : st === 'in_review' ? 'needs review' : st)

  return (
    <div className="dash">
      <div className="dash-inner">
        <div className="dash-head"><h2>Overview</h2><p className="sub">Findings, fixes, and live activity for this audit.</p></div>

        {/* Headline tiles */}
        <div className="dash-grid tiles" style={{ marginBottom: 20 }}>
          <div className="tile"><p className="tile-k">Modules</p><div className="tile-v">{qa.length}</div><div className="tile-sub">reviewed by QA</div></div>
          <div className="tile"><p className="tile-k">Findings</p><div className="tile-v">{bugs.length}</div>
            <div className="tile-sub"><span style={{ color: '#f85149' }}>{sev.high}H</span> · <span style={{ color: '#e0a92e' }}>{sev.medium}M</span> · <span style={{ color: '#3fb950' }}>{sev.low}L</span></div></div>
          <div className="tile"><p className="tile-k">Fixes verified</p><div className="tile-v">{verified}<span style={{ fontSize: 15, color: 'var(--text-muted)' }}>/{bugs.length}</span></div>
            <div className="tile-sub">{needsReview ? `${needsReview} need review` : failed ? `${failed} failed` : 'all green'}</div></div>
          <div className="tile"><p className="tile-k">Cost</p><div className="tile-v">${cost.toFixed(2)}</div><div className="tile-sub">model spend</div></div>
          <div className="tile"><p className="tile-k">Status</p><div className="tile-v" style={{ fontSize: 20, textTransform: 'capitalize' }}>{runStatus}</div><div className="tile-sub">{events.length} events</div></div>
        </div>

        <div className="rail-split">
          {/* Main column */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div className="card">
              <div className="card-h"><span className="card-t">QA by module</span></div>
              <div className="card-b" style={{ padding: 0 }}>
                {qa.length ? qa.map((m) => (
                  <div className="v-row" key={m.id}>
                    {fixIcon(m.status === 'done' ? 'done' : m.status)}
                    <span className="v-title">{m.title || m.name}</span>
                    <span className="v-word" style={{ color: 'var(--text-muted)' }}>{m.status === 'done' ? `${findingsFor((m.tags || []).find((t) => t.startsWith('module:'))?.slice(7))} finding(s)` : m.status}</span>
                  </div>
                )) : <div className="v-row" style={{ color: 'var(--text-muted)' }}>No QA has run yet.</div>}
              </div>
            </div>

            <div className="card">
              <div className="card-h"><span className="card-t">Fix verification</span></div>
              <div className="card-b" style={{ padding: 0 }}>
                {bugs.length ? bugs.map((b) => (
                  <div className="v-row v-click" key={b.id} title="Open on the board" onClick={() => s.openCard(b.id)}>
                    {fixIcon(b.status)}
                    <span className="v-title">{b.title || b.name}</span>
                    <span className={`v-word v-${b.status}`}>{fixWord(b.status)}</span>
                  </div>
                )) : <div className="v-row" style={{ color: 'var(--text-muted)' }}>No fixes yet.</div>}
              </div>
            </div>

            {previews.length > 0 && (
              <div className="card">
                <div className="card-h"><span className="card-t">UI previews</span></div>
                <div className="card-b" style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
                  {previews.map((f) => (
                    <img key={f.path} src={api.rawUrl(s.runId!, f.path)} alt={f.path}
                      style={{ maxWidth: 320, borderRadius: 8, border: '1px solid var(--border-dim)' }} />
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Activity rail */}
          <div className="card" style={{ position: 'sticky', top: 0 }}>
            <div className="card-h"><span className="card-t">Activity</span><span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{events.length}</span></div>
            <div className="ov-activity">
              {events.length ? events.slice().reverse().slice(0, 200).map((e, i) => (
                <div className="ov-evt" key={i}>
                  <span className="dot" style={{ background: evColor(e.kind) }} />
                  <span className="ts">{fmtTime(e.ts)}</span>
                  <span className="kind" style={{ color: evColor(e.kind) }}>{e.kind}</span>
                  <span className="msg">{e.msg}</span>
                </div>
              )) : <div style={{ padding: 16, color: 'var(--text-muted)', fontSize: 12 }}>No events yet.</div>}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
