import { useEffect, useState } from 'react'
import {
  Download, BookOpen, FlaskConical, CheckCircle2, Search, Bug,
  GitBranch, RefreshCw, Clock, FileCode2, Activity as ActIcon, X,
} from 'lucide-react'
import { useStore } from '../store'
import { api, type NodeCard } from '../api'
import { STATUS_COLOR } from '../components/util'

const label = (n: NodeCard) => n.title || n.name || n.type

const SEV_COLOR: Record<string, string> = { high: '#f87171', medium: '#e0a92e', low: '#60a5fa' }

const COLS = [
  { key: 'todo', label: 'To do' },
  { key: 'active', label: 'In progress' },
  { key: 'review', label: 'Review' },
  { key: 'done', label: 'Done' },
]
// Maps both audit-node statuses and bug-ticket lifecycle statuses onto columns.
// Review is the "needs a human" lane: failed regressions, unverifiable fixes,
// and no-diff tickets all land here (failed ones flagged red on the card).
const bucket = (st: string) => {
  switch (st) {
    case 'done': case 'verified': case 'closed': return 'done'
    case 'running': case 'ready': case 'in_progress': return 'active'
    case 'failed': case 'reopened': case 'blocked': case 'in_review': return 'review'
    default: return 'todo' // pending, open, …
  }
}
const isFailed = (st: string) => st === 'failed' || st === 'reopened'

// icon per task type/key
function TaskIcon({ type }: { type: string }) {
  const p = { size: 14 }
  if (type === 'import') return <Download {...p} />
  if (type === 'understand') return <BookOpen {...p} />
  if (type === 'testgen') return <FlaskConical {...p} />
  if (type === 'verify') return <CheckCircle2 {...p} />
  if (type === 'review') return <Search {...p} />
  if (type === 'bug') return <Bug {...p} />
  return <FileCode2 {...p} />
}

function since(ts: string) {
  const d = Math.max(0, Date.now() - new Date(ts).getTime())
  const m = Math.floor(d / 60000), s = Math.floor((d % 60000) / 1000)
  return m > 0 ? `${m}m ${s}s` : `${s}s`
}

export function KanbanScreen() {
  const s = useStore()
  const [cards, setCards] = useState<NodeCard[] | null>(null)
  const [sel, setSel] = useState<NodeCard | null>(null)
  const [tagDraft, setTagDraft] = useState('')

  // Keep the open drawer in sync with polled board data (tags/status updates).
  useEffect(() => {
    if (sel && cards) { const fresh = cards.find((c) => c.id === sel.id); if (fresh) setSel(fresh) }
  }, [cards]) // eslint-disable-line react-hooks/exhaustive-deps

  const saveTags = async (card: NodeCard, tags: string[]) => {
    if (!s.runId) return
    setSel({ ...card, tags })
    try { await api.setNodeTags(s.runId, card.id, tags); api.board(s.runId).then(setCards) }
    catch (e) { s.toast('error', 'Tagging failed', (e as Error).message) }
  }

  // Re-run the autonomous fix for a ticket that failed or is parked in review.
  const runFix = async (card: NodeCard) => {
    if (!s.runId) return
    try {
      await api.enqueue(s.runId, card.id)
      s.toast('info', 'Fix re-queued', 'The dev loop will pick it up — watch the board.')
      api.board(s.runId).then(setCards)
    } catch (e) { s.toast('error', 'Could not queue', (e as Error).message) }
  }
  const canRefix = (c: NodeCard) => c.type === 'bug' && ['open', 'failed', 'in_review'].includes(c.status)
  const previewsFor = (c: NodeCard) =>
    c.type === 'qa' ? (s.detail?.files || []).filter((f) => f.path.startsWith('.myaudit/preview/') && /\.(png|jpe?g|webp|gif)$/i.test(f.path)) : []

  useEffect(() => {
    if (!s.runId) { setCards(null); return }
    let alive = true
    const load = () => api.board(s.runId!).then((c) => { if (alive) setCards(c) }).catch(() => {})
    load()
    const h = setInterval(load, 2000)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  if (!s.runId) return <div className="empty-mid"><h3>No board</h3><p>Import a codebase to see its audit board.</p></div>
  if (cards == null) return <div className="empty-mid"><div className="spin" /></div>

  return (
    <>
      <div className="board">
        {COLS.map((c) => {
          const items = cards.filter((n) => bucket(n.status) === c.key)
          return (
            <div className="kcol" key={c.key}>
              <div className="kcol-h"><b>{c.label}</b><span className="c">{items.length}</span></div>
              {items.map((n) => (
                <div className={`kcard ${isFailed(n.status) ? 'failed' : ''}`} key={n.id} onClick={() => setSel(n)}>
                  <div className="kcard-top">
                    <span className="kcard-ico"><TaskIcon type={n.type} /></span>
                    <span className="kt">{label(n)}</span>
                    <span className="kid">{n.id.slice(0, 6)}</span>
                  </div>
                  {(n.severity || (n.tags && n.tags.length > 0)) && (
                    <div className="ktags">
                      {n.severity && <span className="ktag" style={{ color: SEV_COLOR[n.severity] || 'var(--text-secondary)', borderColor: SEV_COLOR[n.severity] || 'var(--border-subtle)' }}>{n.severity}</span>}
                      {n.priority && <span className="ktag">{n.priority}</span>}
                      {(n.tags || []).filter((t) => t !== n.severity).map((t) => <span key={t} className="ktag">{t}</span>)}
                    </div>
                  )}
                  {n.summary && <div className="ksum">{n.summary}</div>}
                  <div className="kcard-meta">
                    <span className="kbadge" style={{ color: STATUS_COLOR[n.status], background: 'var(--bg-panel)' }}>
                      <span className="kdot" style={{ background: STATUS_COLOR[n.status] }} />{n.status}
                    </span>
                    {n.attempts > 1 && <span className="kmeta"><RefreshCw size={10} /> {n.attempts}</span>}
                    {n.files > 0 && <span className="kmeta"><FileCode2 size={10} /> {n.files}</span>}
                    {n.cost_usd > 0 && <span className="kmeta">${n.cost_usd.toFixed(3)}</span>}
                    {n.events > 0 && <span className="kmeta"><ActIcon size={10} /> {n.events}</span>}
                    {n.deps > 0 && <span className="kmeta"><GitBranch size={10} /> {n.deps}</span>}
                    <span className="kmeta"><Clock size={10} /> {since(n.created_at)}</span>
                  </div>
                </div>
              ))}
              {!items.length && <div className="kempty">Empty</div>}
            </div>
          )
        })}
      </div>

      {sel && (
        <>
          <div className="drawer-scrim" onClick={() => setSel(null)} />
          <div className="drawer">
            <div className="drawer-h">
              <span className="kcard-ico" style={{ width: 32, height: 32 }}><TaskIcon type={sel.type} /></span>
              <h3>{label(sel)}</h3>
              <span className="x" onClick={() => setSel(null)}><X size={18} /></span>
            </div>
            <div className="drawer-body">
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                <span className="kbadge" style={{ color: STATUS_COLOR[sel.status], background: 'var(--bg-panel)' }}>
                  <span className="kdot" style={{ background: STATUS_COLOR[sel.status] }} />{sel.status}
                </span>
                {canRefix(sel) && <button className="btn-sm primary" onClick={() => runFix(sel)}>▶ Re-run fix</button>}
              </div>
              {sel.summary && <div style={{ color: 'var(--text-secondary)', fontSize: 13, lineHeight: 1.6 }}>{sel.summary}</div>}
              {sel.detail && <div style={{ color: 'var(--text-secondary)', fontSize: 13, lineHeight: 1.6, whiteSpace: 'pre-wrap', background: 'var(--bg-panel)', border: '1px solid var(--border-dim)', borderRadius: 8, padding: 10 }}>{sel.detail}</div>}

              {previewsFor(sel).length > 0 && (
                <div>
                  <div style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.05em', color: 'var(--text-muted)', marginBottom: 6 }}>QA preview</div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {previewsFor(sel).map((f) => (
                      <img key={f.path} src={api.rawUrl(s.runId!, f.path)} alt={f.path}
                        style={{ maxWidth: '100%', borderRadius: 8, border: '1px solid var(--border-dim)' }} />
                    ))}
                  </div>
                </div>
              )}

              {/* Tags — add/remove (manual triage) */}
              <div>
                <div style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.05em', color: 'var(--text-muted)', marginBottom: 6 }}>Tags</div>
                <div className="ktags">
                  {(sel.tags || []).map((t) => (
                    <span key={t} className="ktag" style={{ cursor: 'pointer' }} onClick={() => saveTags(sel, (sel.tags || []).filter((x) => x !== t))} title="Remove">{t} ×</span>
                  ))}
                  <input className="tag-input" value={tagDraft} onChange={(e) => setTagDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && tagDraft.trim()) {
                        const nt = Array.from(new Set([...(sel.tags || []), tagDraft.trim()]))
                        saveTags(sel, nt); setTagDraft('')
                      }
                    }}
                    placeholder="+ tag" />
                </div>
              </div>

              <dl className="drawer-kv">
                <dt>id</dt><dd>{sel.id}</dd>
                <dt>type</dt><dd>{sel.type}</dd>
                {sel.severity && <><dt>severity</dt><dd style={{ color: SEV_COLOR[sel.severity] }}>{sel.severity}</dd></>}
                {sel.priority && <><dt>priority</dt><dd>{sel.priority}</dd></>}
                <dt>attempts</dt><dd>{sel.attempts}</dd>
                <dt>dependencies</dt><dd>{sel.deps}</dd>
                <dt>files changed</dt><dd>{sel.files}</dd>
                {sel.cost_usd > 0 && <><dt>cost</dt><dd>${sel.cost_usd.toFixed(4)}</dd></>}
                <dt>events</dt><dd>{sel.events}</dd>
                <dt>created</dt><dd>{new Date(sel.created_at).toLocaleString()}</dd>
                {sel.claimed_at && <><dt>claimed</dt><dd>{new Date(sel.claimed_at).toLocaleString()}</dd></>}
              </dl>
              <div style={{ borderTop: '1px solid var(--border-dim)', paddingTop: 14 }}>
                <div style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.05em', color: 'var(--text-muted)', marginBottom: 8 }}>Activity for this task</div>
                {(s.detail?.events || []).filter((e) => e.node_id === sel.id).slice(-12).map((e, i) => (
                  <div key={i} style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, lineHeight: 1.7, color: 'var(--text-secondary)' }}>
                    <span style={{ color: 'var(--text-muted)' }}>{new Date(e.ts).toLocaleTimeString()}</span> {e.kind} {e.msg}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </>
      )}
    </>
  )
}
