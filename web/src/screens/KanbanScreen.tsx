import { useEffect, useRef, useState } from 'react'
import {
  Download, Search, Bug,
  GitBranch, RefreshCw, Clock, FileCode2, X,
  ChevronUp, ChevronDown, FileSymlink, Link2,
} from 'lucide-react'
import { useStore } from '../store'
import { api, type NodeCard } from '../api'
import { STATUS_COLOR } from '../components/util'
import { Diff } from '../components/Diff'
import { AgentAvatar } from '../components/icons'

const label = (n: NodeCard) => n.title || n.name || n.type

const KEY_PREFIX: Record<string, string> = { bug: 'BUG', qa: 'QA', map: 'MAP', import: 'IMP' }
const keyFor = (n: NodeCard) => `${KEY_PREFIX[n.type] || 'AUD'}-${n.id.slice(0, 4)}`

const SEV_COLOR: Record<string, string> = { high: '#f87171', medium: '#e0a92e', low: '#60a5fa' }

const COLS = [
  { key: 'todo', label: 'To do' },
  { key: 'active', label: 'In progress' },
  { key: 'review', label: 'Review' },
  { key: 'done', label: 'Done' },
]

const bucket = (st: string) => {
  switch (st) {
    case 'done': case 'verified': case 'closed': return 'done'
    case 'running': case 'ready': case 'in_progress': return 'active'
    case 'failed': case 'reopened': case 'blocked': case 'in_review': return 'review'
    default: return 'todo' 
  }
}
const isFailed = (st: string) => st === 'failed' || st === 'reopened'

const COL_DROP: Record<string, string> = { todo: 'open', review: 'in_review', done: 'done' }

const SEV_RANK: Record<string, number> = { high: 0, medium: 1, low: 2 }
const PRI_RANK: Record<string, number> = { P0: 0, P1: 1, P2: 2 }

const SORTERS: Record<string, (a: NodeCard, b: NodeCard) => number> = {
  default: () => 0,
  severity: (a, b) => (SEV_RANK[a.severity ?? ''] ?? 9) - (SEV_RANK[b.severity ?? ''] ?? 9),
  priority: (a, b) => (PRI_RANK[a.priority ?? ''] ?? 9) - (PRI_RANK[b.priority ?? ''] ?? 9),
  newest: (a, b) => +new Date(b.created_at) - +new Date(a.created_at),
}

function sevCounts(items: NodeCard[]) {
  const c = { high: 0, medium: 0, low: 0 }
  for (const n of items) if (n.severity && n.severity in c) c[n.severity as keyof typeof c]++
  return c
}

function progressLine(cards: NodeCard[]): { text: string; done: number; total: number } | null {
  const active = cards.filter((n) => n.status === 'ready' || n.status === 'running' || n.status === 'pending')
  if (!active.length) return null
  const imp = cards.find((n) => n.type === 'import')
  const map = cards.find((n) => n.type === 'map')
  const qas = cards.filter((n) => n.type === 'qa')
  const qaDone = qas.filter((n) => n.status === 'done').length
  const bugs = cards.filter((n) => n.type === 'bug')
  const bugDone = bugs.filter((n) => n.status === 'done').length
  let text: string
  if (imp && imp.status !== 'done') text = 'Importing the codebase…'
  else if (map && map.status !== 'done') text = 'Mapping modules…'
  else if (qas.length && qaDone < qas.length) text = `Reviewing modules — QA ${qaDone}/${qas.length}`
  else if (bugs.length) text = `Fixing — ${bugDone}/${bugs.length} done`
  else text = 'Working…'
  return { text, done: qaDone + bugDone, total: qas.length + bugs.length }
}

function TaskIcon({ type }: { type: string }) {
  const p = { size: 14 }
  if (type === 'import') return <Download {...p} />
  if (type === 'map') return <GitBranch {...p} />
  if (type === 'qa') return <Search {...p} />
  if (type === 'bug') return <Bug {...p} />
  return <FileCode2 {...p} />
}

function since(ts: string) {
  const d = Math.max(0, Date.now() - new Date(ts).getTime())
  const m = Math.floor(d / 60000), s = Math.floor((d % 60000) / 1000)
  const h = Math.floor(m / 60)
  if (h > 0) return `${h}h ${m % 60}m`
  return m > 0 ? `${m}m ${s}s` : `${s}s`
}

const TERMINAL = new Set(['done', 'closed', 'verified', 'failed', 'cancelled'])
function ageOf(n: NodeCard): string | null {
  if (TERMINAL.has(n.status)) return null
  if (n.status === 'running' && n.claimed_at) return since(n.claimed_at)
  return since(n.created_at)
}

export function KanbanScreen() {
  const s = useStore()
  const [cards, setCards] = useState<NodeCard[] | null>(null)
  const [sel, setSel] = useState<NodeCard | null>(null)
  const [tagDraft, setTagDraft] = useState('')
  const [fSev, setFSev] = useState('all')
  const [fType, setFType] = useState('all')
  const [q, setQ] = useState('')
  const [showDismissed, setShowDismissed] = useState(false)
  const [sortBy, setSortBy] = useState('default')
  const [groupBy, setGroupBy] = useState('none')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (sel && cards) { const fresh = cards.find((c) => c.id === sel.id); if (fresh) setSel(fresh) }
  }, [cards]) 

  const [fixDiff, setFixDiff] = useState('')
  const orderedRef = useRef<NodeCard[]>([]) 

  const step = (dir: 1 | -1) => {
    const list = orderedRef.current
    if (!sel || !list.length) return
    const i = list.findIndex((c) => c.id === sel.id)
    if (i < 0) return
    const n = list[(i + dir + list.length) % list.length]
    if (n) setSel(n)
  }

  useEffect(() => {
    if (!sel) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setSel(null)
      else if (e.key === 'ArrowDown') { e.preventDefault(); step(1) }
      else if (e.key === 'ArrowUp') { e.preventDefault(); step(-1) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [sel]) 

  useEffect(() => {
    setFixDiff('')
    if (!sel || !s.runId || sel.type !== 'bug' || !sel.file) return
    const path = sel.file.split(':')[0]
    let alive = true
    api.diff(s.runId, path).then((r) => { if (alive) setFixDiff(r.diff) }).catch(() => {})
    return () => { alive = false }
  }, [sel?.id, sel?.file, s.runId, s.detail]) 

  const openedFromUrl = useRef(false)
  useEffect(() => {
    if (openedFromUrl.current || !cards) return
    const cid = new URLSearchParams(location.search).get('card')
    if (cid) { const c = cards.find((x) => x.id === cid); if (c) { setSel(c); openedFromUrl.current = true } }
  }, [cards])

  useEffect(() => {
    if (!s.focusCard || !cards) return
    const c = cards.find((x) => x.id === s.focusCard)
    if (c) setSel(c)
    s.clearFocusCard()
  }, [s.focusCard, cards]) 

  const openFile = (card: NodeCard) => {
    if (!card.file) return
    s.setFile(card.file.split(':')[0])
    s.setTab('dev')
    setSel(null)
  }
  const copyLink = (card: NodeCard) => {
    const url = `${location.origin}${location.pathname}?card=${card.id}${location.hash}`
    navigator.clipboard?.writeText(url).then(() => s.toast('success', 'Link copied')).catch(() => {})
  }

  const [dragId, setDragId] = useState<string | null>(null)
  const [dragOverCol, setDragOverCol] = useState<string | null>(null)
  const onDrop = (colKey: string) => {
    const status = COL_DROP[colKey]
    const card = (cards || []).find((c) => c.id === dragId)
    setDragId(null); setDragOverCol(null)
    if (!status || !card || card.type !== 'bug' || bucket(card.status) === colKey) return
    patch(card, { status })
    s.toast('info', 'Moved', card.title || card.name)
  }

  const saveTags = async (card: NodeCard, tags: string[]) => {
    if (!s.runId) return
    setSel({ ...card, tags })
    try { await api.setNodeTags(s.runId, card.id, tags); api.board(s.runId).then(setCards) }
    catch (e) { s.toast('error', 'Tagging failed', (e as Error).message) }
  }

  const runFix = async (card: NodeCard) => {
    if (!s.runId) return
    try {
      await api.enqueue(s.runId, card.id)
      s.toast('info', 'Fix re-queued', 'The dev loop will pick it up — watch the board.')
      api.board(s.runId).then(setCards)
    } catch (e) { s.toast('error', 'Could not queue', (e as Error).message) }
  }
  const patch = async (card: NodeCard, p: { severity?: string; priority?: string; status?: string }) => {
    if (!s.runId) return
    setSel({ ...card, ...p }) 
    try { await api.patchNode(s.runId, card.id, p); api.board(s.runId).then(setCards) }
    catch (e) { s.toast('error', 'Update failed', (e as Error).message) }
  }
  const dismiss = async (card: NodeCard) => { await patch(card, { status: 'dismissed' }); s.toast('info', 'Ticket dismissed'); setSel(null) }
  const restore = async (card: NodeCard) => patch(card, { status: 'open' })

  const canRefix = (c: NodeCard) => c.type === 'bug' && ['open', 'failed', 'in_review', 'done'].includes(c.status)

  const previewsFor = (c: NodeCard) => {
    if (c.type !== 'qa') return []
    const mod = (c.tags || []).find((t) => t.startsWith('module:'))?.slice(7)
    return (s.detail?.files || []).filter((f) =>
      f.path.startsWith('.myaudit/preview/') && /\.(png|jpe?g|webp|gif)$/i.test(f.path) &&
      (!mod || f.path.includes(mod)))
  }

  useEffect(() => {
    if (!s.runId) { setCards(null); return }
    let alive = true
    const load = () => api.board(s.runId!).then((c) => { if (alive) setCards(c) }).catch(() => {})
    load()
    const h = setInterval(load, 2000)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  if (!s.runId) return <div className="empty-mid"><h3>No board</h3><p>Import a codebase to see its audit board.</p></div>
  if (cards == null) return <div className="empty-mid"><div className="spin" /><p style={{ marginTop: 12 }}>Setting up your audit…</p></div>

  const ql = q.trim().toLowerCase()
  const dismissedCount = cards.filter((n) => n.status === 'dismissed').length
  const visible = cards.filter((n) => {
    if (n.status === 'cancelled') return false 
    if (n.status === 'dismissed' && !showDismissed) return false
    if (fSev !== 'all' && n.severity !== fSev) return false
    if (fType === 'bug' && n.type !== 'bug') return false
    if (fType === 'qa' && n.type !== 'qa') return false
    if (fType === 'flow' && (n.type === 'bug' || n.type === 'qa')) return false
    if (ql) {
      const hay = (label(n) + ' ' + (n.summary || '') + ' ' + (n.tags || []).join(' ')).toLowerCase()
      if (!hay.includes(ql)) return false
    }
    return true
  })
  const activeFilter = fSev !== 'all' || fType !== 'all' || ql !== ''
  if (sortBy !== 'default') visible.sort(SORTERS[sortBy])

  const renderCard = (n: NodeCard) => {
    const stripe = SEV_COLOR[n.severity || ''] || STATUS_COLOR[n.status] || 'var(--border-subtle)'
    const moduleTag = (n.tags || []).find((t) => t.startsWith('module:'))?.slice(7)
    const otherTags = (n.tags || []).filter((t) => t !== n.severity && !t.startsWith('module:') && t !== ('from:qa'))
    return (
    <div className={`kcard jira ${isFailed(n.status) ? 'failed' : ''} ${n.status === 'dismissed' ? 'dismissed' : ''} ${n.status === 'running' ? 'running' : ''} ${dragId === n.id ? 'dragging' : ''}`}
      key={n.id} tabIndex={0} role="button" style={{ ['--stripe' as string]: stripe }}
      draggable={n.type === 'bug'}
      onDragStart={() => n.type === 'bug' && setDragId(n.id)}
      onDragEnd={() => { setDragId(null); setDragOverCol(null) }}
      onClick={() => setSel(n)}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSel(n) } }}>
      {n.type === 'bug' && (
        <div className="kcard-actions">
          <button className="ka-btn" title="Open" onClick={(e) => { e.stopPropagation(); setSel(n) }}>⤢</button>
          {canRefix(n) && <button className="ka-btn" title="Re-run fix" onClick={(e) => { e.stopPropagation(); runFix(n) }}>▶</button>}
          {n.status !== 'dismissed' && <button className="ka-btn" title="Dismiss" onClick={(e) => { e.stopPropagation(); dismiss(n) }}>✕</button>}
        </div>
      )}
      <div className="jtitle">{label(n)}</div>
      {n.status === 'running'
        ? <div className="kstep"><span className="spin-sm" />{latestStep(n.id) || 'working…'}</div>
        : n.summary && <div className="ksum">{n.summary}</div>}
      {(n.severity || moduleTag || otherTags.length > 0) && (
        <div className="ktags">
          {n.severity && <span className="jsev" style={{ background: SEV_COLOR[n.severity] || 'var(--text-muted)' }}>{n.severity}</span>}
          {moduleTag && <span className="ktag">{moduleTag}</span>}
          {otherTags.map((t) => <span key={t} className="ktag">{t}</span>)}
        </div>
      )}
      <div className="jfoot">
        <span className="jkey" title={n.type}><span className="jkey-ico"><TaskIcon type={n.type} /></span>{keyFor(n)}</span>
        {n.priority && <span className={`jprio p-${n.priority}`} title={`priority ${n.priority}`}>{n.priority}</span>}
        <span className="jfoot-meta">
          {n.attempts > 1 && <span className="kmeta"><RefreshCw size={10} /> {n.attempts}</span>}
          {n.files > 0 && <span className="kmeta"><FileCode2 size={10} /> {n.files}</span>}
          {n.cost_usd > 0 && <span className="kmeta">${n.cost_usd.toFixed(2)}</span>}
          {ageOf(n) && <span className="kmeta"><Clock size={10} /> {ageOf(n)}</span>}
        </span>
        <span className="jassignee" title="Assigned to the audit agent"><AgentAvatar size={20} radius={999} /></span>
      </div>
    </div>
    )
  }

  const laneOf = (n: NodeCard) =>
    groupBy === 'module' ? ((n.tags || []).find((t) => t.startsWith('module:'))?.slice(7) || '—')
      : groupBy === 'severity' ? (n.severity || '—')
      : 'All'
  const lanes = groupBy === 'none' ? ['All'] : [...new Set(visible.map(laneOf))].sort()

  orderedRef.current = lanes.flatMap((ln) =>
    COLS.flatMap((c) => visible.filter((n) => laneOf(n) === ln && bucket(n.status) === c.key)))

  const events = s.detail?.events || []
  const latestStep = (nodeId: string): string => {
    for (let i = events.length - 1; i >= 0; i--) {
      const e = events[i]
      if (e.node_id === nodeId && e.kind === 'agent.step') return e.msg
    }
    return ''
  }

  const prog = progressLine(cards)
  return (
    <div className="kanban-wrap">
      {prog && (
        <div className="run-banner">
          <span className="spin-sm" />
          <span className="rb-text">{prog.text}</span>
          {prog.total > 0 && (
            <span className="rb-bar"><span className="rb-fill" style={{ width: `${Math.round((prog.done / prog.total) * 100)}%` }} /></span>
          )}
        </div>
      )}
      <div className="board-filter">
        <input className="bf-search" placeholder="Filter cards…" value={q} onChange={(e) => setQ(e.target.value)} />
        <select className="bf-sel" value={fType} onChange={(e) => setFType(e.target.value)}>
          <option value="all">All types</option>
          <option value="bug">Tickets</option>
          <option value="qa">QA</option>
          <option value="flow">Pipeline</option>
        </select>
        <select className="bf-sel" value={fSev} onChange={(e) => setFSev(e.target.value)}>
          <option value="all">Any severity</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>
        <select className="bf-sel" value={sortBy} onChange={(e) => setSortBy(e.target.value)} title="Sort within column">
          <option value="default">Sort: default</option>
          <option value="severity">Sort: severity</option>
          <option value="priority">Sort: priority</option>
          <option value="newest">Sort: newest</option>
        </select>
        <select className="bf-sel" value={groupBy} onChange={(e) => setGroupBy(e.target.value)} title="Group into swimlanes">
          <option value="none">Group: none</option>
          <option value="module">Group: module</option>
          <option value="severity">Group: severity</option>
        </select>
        {activeFilter && <>
          <span className="bf-count">{visible.length} / {cards.length}</span>
          <button className="btn-sm" onClick={() => { setQ(''); setFSev('all'); setFType('all') }}>Clear</button>
        </>}
        {dismissedCount > 0 && (
          <button className={`btn-sm ${showDismissed ? 'primary' : ''}`} style={{ marginLeft: 'auto' }}
            onClick={() => setShowDismissed((v) => !v)}>{showDismissed ? 'Hide' : 'Show'} dismissed ({dismissedCount})</button>
        )}
      </div>
      {lanes.map((lane) => (
        <div key={lane} className="lane">
          {groupBy !== 'none' && <div className="swimlane-h">{groupBy === 'module' ? 'module:' : ''}{lane}</div>}
          <div className="board">
            {COLS.map((c) => {
              const items = visible.filter((n) => laneOf(n) === lane && bucket(n.status) === c.key)
              const colKey = lane + '/' + c.key
              const isCollapsed = collapsed.has(c.key)
              const droppable = !!dragId && !!COL_DROP[c.key]
              const sc = sevCounts(items)
              return (
                <div className={`kcol col-${c.key} ${dragOverCol === colKey && droppable ? 'dragover' : ''} ${isCollapsed ? 'collapsed' : ''}`} key={colKey}
                  onDragOver={(e) => { if (droppable) { e.preventDefault(); setDragOverCol(colKey) } }}
                  onDragLeave={() => setDragOverCol((cur) => (cur === colKey ? null : cur))}
                  onDrop={() => onDrop(c.key)}>
                  <div className="kcol-h" onClick={() => setCollapsed((s) => { const n = new Set(s); n.has(c.key) ? n.delete(c.key) : n.add(c.key); return n })} title={isCollapsed ? 'Expand' : 'Collapse'}>
                    <b>{c.label}</b><span className="c">{items.length}</span>
                    {(sc.high + sc.medium + sc.low) > 0 && (
                      <span className="kcol-sev">
                        {sc.high > 0 && <span style={{ color: SEV_COLOR.high }}>{sc.high}H</span>}
                        {sc.medium > 0 && <span style={{ color: SEV_COLOR.medium }}>{sc.medium}M</span>}
                        {sc.low > 0 && <span style={{ color: SEV_COLOR.low }}>{sc.low}L</span>}
                      </span>
                    )}
                  </div>
                  <div className={`kcol-acc`} />
                  {!isCollapsed && (
                    <div className="kcards">
                      {items.map((n) => renderCard(n))}
                      {!items.length && <div className="kempty">No issues</div>}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      ))}

      {sel && (
        <>
          <div className="drawer-scrim" onClick={() => setSel(null)} />
          <div className="drawer">
            <div className="drawer-h">
              <span className="kcard-ico" style={{ width: 32, height: 32 }}><TaskIcon type={sel.type} /></span>
              <h3>{label(sel)}</h3>
              <span className="drawer-nav">
                <button className="icon-btn" title="Previous card (↑)" onClick={() => step(-1)}><ChevronUp size={16} /></button>
                <button className="icon-btn" title="Next card (↓)" onClick={() => step(1)}><ChevronDown size={16} /></button>
                <button className="icon-btn" title="Copy link to this card" onClick={() => copyLink(sel)}><Link2 size={15} /></button>
                <button className="icon-btn" title="Close (Esc)" onClick={() => setSel(null)}><X size={17} /></button>
              </span>
            </div>
            <div className="drawer-body">
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                <span className="kbadge" style={{ color: STATUS_COLOR[sel.status], background: 'var(--bg-panel)' }}>
                  <span className="kdot" style={{ background: STATUS_COLOR[sel.status] }} />{sel.status}
                </span>
                {canRefix(sel) && <button className="btn-sm primary" onClick={() => runFix(sel)}>{sel.status === 'done' ? '↻ Reopen & re-fix' : '▶ Re-run fix'}</button>}
                {sel.type === 'bug' && sel.status === 'dismissed' && <button className="btn-sm" onClick={() => restore(sel)}>Restore</button>}
                {sel.type === 'bug' && sel.status !== 'dismissed' && <button className="btn-sm" onClick={() => dismiss(sel)}>Dismiss</button>}
              </div>

              {}
              {sel.type === 'bug' && (
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <select className="bf-sel" value={sel.severity || 'medium'} onChange={(e) => patch(sel, { severity: e.target.value })}>
                    <option value="high">high</option><option value="medium">medium</option><option value="low">low</option>
                  </select>
                  <select className="bf-sel" value={sel.priority || 'P1'} onChange={(e) => patch(sel, { priority: e.target.value })}>
                    <option value="P0">P0</option><option value="P1">P1</option><option value="P2">P2</option>
                  </select>
                </div>
              )}
              {}
              {sel.file && (
                <button className="file-jump" onClick={() => openFile(sel)} title="Open in editor">
                  <FileSymlink size={13} /> <code>{sel.file}</code>
                </button>
              )}
              {sel.summary && <div style={{ color: 'var(--text-secondary)', fontSize: 13, lineHeight: 1.6 }}>{sel.summary}</div>}
              {sel.detail && <div style={{ color: 'var(--text-secondary)', fontSize: 13, lineHeight: 1.6, whiteSpace: 'pre-wrap', background: 'var(--bg-panel)', border: '1px solid var(--border-dim)', borderRadius: 8, padding: 10 }}>{sel.detail}</div>}

              {}
              {sel.type === 'bug' && fixDiff.trim() && (
                <div>
                  <div style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.05em', color: 'var(--text-muted)', marginBottom: 6 }}>Fix diff</div>
                  <div style={{ maxHeight: 320, overflow: 'auto', border: '1px solid var(--border-dim)', borderRadius: 8 }}><Diff text={fixDiff} /></div>
                </div>
              )}

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

              {}
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
    </div>
  )
}
