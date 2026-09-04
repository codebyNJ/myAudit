import { useEffect, useState } from 'react'
import {
  Boxes, Cog, Hammer, CheckCircle2,
  GitBranch, RefreshCw, Clock, FileCode2, Activity as ActIcon, X,
} from 'lucide-react'
import { useStore } from '../store'
import { api, type NodeCard } from '../api'
import { STATUS_COLOR } from '../components/util'

// A feature node is labelled by its resource name; others by type.
const label = (n: { type: string; name: string }) => (n.type === 'feature' && n.name ? n.name : n.type)

const COLS = [
  { key: 'pending', label: 'To do' },
  { key: 'active', label: 'In progress' },
  { key: 'review', label: 'Review' },
  { key: 'done', label: 'Done' },
]
const bucket = (st: string) => st === 'done' ? 'done' : st === 'running' || st === 'ready' ? 'active' : st === 'blocked' ? 'review' : 'pending'

// icon per task type/key
function TaskIcon({ type }: { type: string }) {
  const p = { size: 14 }
  if (type === 'scaffold') return <Hammer {...p} />
  if (type === 'config') return <Cog {...p} />
  if (type === 'finalize') return <CheckCircle2 {...p} />
  return <Boxes {...p} /> // feature
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

  useEffect(() => {
    if (!s.runId) { setCards(null); return }
    let alive = true
    const load = () => api.board(s.runId!).then((c) => { if (alive) setCards(c) }).catch(() => {})
    load()
    const h = setInterval(load, 2000)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  if (!s.runId) return <div className="empty-mid"><h3>No board</h3><p>Create a project to see its task board.</p></div>
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
                <div className="kcard" key={n.id} onClick={() => setSel(n)}>
                  <div className="kcard-top">
                    <span className="kcard-ico"><TaskIcon type={n.type} /></span>
                    <span className="kt">{label(n)}</span>
                    <span className="kid">{n.id.slice(0, 6)}</span>
                  </div>
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
              <span className="kbadge" style={{ color: STATUS_COLOR[sel.status], background: 'var(--bg-panel)', alignSelf: 'flex-start' }}>
                <span className="kdot" style={{ background: STATUS_COLOR[sel.status] }} />{sel.status}
              </span>
              {sel.summary && <div style={{ color: 'var(--text-secondary)', fontSize: 13, lineHeight: 1.6 }}>{sel.summary}</div>}
              <dl className="drawer-kv">
                <dt>id</dt><dd>{sel.id}</dd>
                <dt>type</dt><dd>{sel.type}</dd>
                {sel.name && <><dt>resource</dt><dd>{sel.name}</dd></>}
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
