import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { api, type NodeCard } from '../api'
import { IcCircleCheck, IcCircleX } from '../components/icons'

// Verification dashboard: how each module's QA went, whether each autonomous fix
// was verified (green regression) / failed / needs review, and any UI previews.
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

  if (!s.runId) return <div className="empty-mid"><h3>No run</h3><p>Import a codebase to see verification results.</p></div>
  if (cards == null) return <div className="empty-mid"><div className="spin" /></div>

  const qa = cards.filter((c) => c.type === 'qa')
  const bugs = cards.filter((c) => c.type === 'bug')
  const findingsFor = (mod?: string) => bugs.filter((b) => (b.tags || []).includes('module:' + mod)).length
  const previews = (s.detail?.files || []).filter((f) => f.path.startsWith('.myaudit/preview/') && /\.(png|jpe?g|webp|gif)$/i.test(f.path))

  const fixIcon = (st: string) => (st === 'done' ? <IcCircleCheck /> : st === 'failed' ? <IcCircleX /> : <span className="v-dot" />)
  const fixWord = (st: string) => (st === 'done' ? 'verified' : st === 'failed' ? 'failed regression' : st === 'in_review' ? 'needs review' : st)

  return (
    <div className="pane">
      <h2>Verification</h2>
      <p className="sub">How each module's QA ran, whether each fix passed regression, and any captured UI previews.</p>

      <div className="set-card">
        <div className="set-h">QA by module</div>
        {qa.length ? qa.map((m) => (
          <div className="set-row" key={m.id}>
            <span>{fixIcon(m.status === 'done' ? 'done' : m.status)} {m.title || m.name}</span>
            <span style={{ color: 'var(--text-muted)' }}>{m.status === 'done' ? `${findingsFor((m.tags || []).find((t) => t.startsWith('module:'))?.slice(7))} finding(s)` : m.status}</span>
          </div>
        )) : <div style={{ color: 'var(--text-muted)', fontSize: 13 }}>No QA has run yet.</div>}
      </div>

      <div className="set-card">
        <div className="set-h">Fix verification</div>
        {bugs.length ? bugs.map((b) => (
          <div className="v-row" key={b.id}>
            {fixIcon(b.status)}
            <span className="v-title">{b.title || b.name}</span>
            <span className={`v-word v-${b.status}`}>{fixWord(b.status)}</span>
          </div>
        )) : <div style={{ color: 'var(--text-muted)', fontSize: 13 }}>No fixes yet.</div>}
      </div>

      {previews.length > 0 && (
        <div className="set-card">
          <div className="set-h">UI previews</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
            {previews.map((f) => (
              <img key={f.path} src={api.rawUrl(s.runId!, f.path)} alt={f.path}
                style={{ maxWidth: 320, borderRadius: 8, border: '1px solid var(--border-dim)' }} />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
