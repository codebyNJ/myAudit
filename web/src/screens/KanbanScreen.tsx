import { useEffect, useRef, useState, type DragEvent } from 'react'
import { useScrollAnchor } from '../useScrollAnchor'
import {
  Download, Search, Bug,
  GitBranch, RefreshCw, Clock, FileCode2, X, Workflow,
  ChevronUp, ChevronDown, FileSymlink, Link2,
} from 'lucide-react'
import { useAppStore } from '../store/slices'
import { api, type NodeCard } from '../api'
import type { PatchNodeBody } from '../schemas'
import { STATUS_COLOR } from '../components/util'
import { Diff } from '../components/Diff'
import { Markdown } from '../components/Markdown'
import { AgentAvatar } from '../components/icons'

const label = (n: NodeCard) => n.title || n.name || n.type

const KEY_PREFIX: Record<string, string> = { bug: 'BUG', qa: 'QA', map: 'MAP', import: 'IMP', flows: 'FLW' }
const keyFor = (n: NodeCard) => `${KEY_PREFIX[n.type] || 'AUD'}-${n.id.slice(0, 4)}`

const SEV_COLOR: Record<string, string> = { high: '#f87171', medium: '#e0a92e', low: '#60a5fa' }

// Non-defects are muted on purpose: an improvement or a style note should be
// visible without competing with a real bug for attention.
const CLASS_COLOR: Record<string, string> = {
  improvement: '#7c9cbf',
  style: '#8a8a9e',
  question: '#b08bd4',
}
const isDefect = (n: NodeCard) => !n.class || n.class === 'bug'

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

const COL_DROP: Record<string, NonNullable<PatchNodeBody['status']>> = { todo: 'open', review: 'in_review', done: 'done' }

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
  if (type === 'flows') return <Workflow {...p} />
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
  const runId = useAppStore((st) => st.runId)
  const loadBoard = useAppStore((st) => st.loadBoard)
  const detail = useAppStore((st) => st.detail)
  const focusCard = useAppStore((st) => st.focusCard)
  const clearFocusCard = useAppStore((st) => st.clearFocusCard)
  const setFile = useAppStore((st) => st.setFile)
  const setTab = useAppStore((st) => st.setTab)
  const toast = useAppStore((st) => st.toast)
  // One board poll lives in the store; Overview reads the same cards.
  const cards = useAppStore((st) => st.board)
  const [sel, setSel] = useState<NodeCard | null>(null)
  const drawerBody = useScrollAnchor<HTMLDivElement>(sel?.id)
  const [tagDraft, setTagDraft] = useState('')
  const [fSev, setFSev] = useState('all')
  const [fType, setFType] = useState('all')
  const [fClass, setFClass] = useState('defects')
  const [q, setQ] = useState('')
  const [showDismissed, setShowDismissed] = useState(false)
  const [sortBy, setSortBy] = useState('default')
  const [groupBy, setGroupBy] = useState('none')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!sel || !cards) return
    const fresh = cards.find((c) => c.id === sel.id)
    // Every poll produces new card objects. Replacing `sel` with an
    // equal-but-new one re-rendered the whole drawer twice a second for no
    // change; only swap when the contents actually differ.
    if (fresh && JSON.stringify(fresh) !== JSON.stringify(sel)) setSel(fresh)
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
    if (!sel || !runId || sel.type !== 'bug' || !sel.file) return
    const path = sel.file.split(':')[0]
    let alive = true
    api.diff(runId, path).then((r) => { if (alive) setFixDiff(r.diff) }).catch(() => {})
    return () => { alive = false }
    // Keyed on what can actually change this diff, not on `detail`. `detail`
    // is replaced by the 2s poll, so listing it here re-cleared and refetched
    // the diff ten times per 20s for a file nobody had touched.
  }, [sel?.id, sel?.file, sel?.commit_sha, sel?.status, sel?.files, runId])

  const openedFromUrl = useRef(false)
  useEffect(() => {
    if (openedFromUrl.current || !cards) return
    const cid = new URLSearchParams(location.search).get('card')
    if (cid) { const c = cards.find((x) => x.id === cid); if (c) { setSel(c); openedFromUrl.current = true } }
  }, [cards])

  useEffect(() => {
    if (!focusCard || !cards) return
    const c = cards.find((x) => x.id === focusCard)
    if (c) setSel(c)
    clearFocusCard()
  }, [focusCard, cards])

  const openFile = (card: NodeCard) => {
    if (!card.file) return
    setFile(card.file.split(':')[0])
    setTab('dev')
    setSel(null)
  }
  const copyLink = (card: NodeCard) => {
    const url = `${location.origin}${location.pathname}?card=${card.id}${location.hash}`
    navigator.clipboard?.writeText(url).then(() => toast('success', 'Link copied')).catch(() => {})
  }

  const [dragId, setDragId] = useState<string | null>(null)
  const dragIdRef = useRef<string | null>(null)
  const [dragOverCol, setDragOverCol] = useState<string | null>(null)
  const [prBusy, setPrBusy] = useState(false)
  const clearDrag = () => { dragIdRef.current = null; setDragId(null); setDragOverCol(null) }
  const onDragCol = (e: DragEvent, laneColKey: string, col: string) => {
    if (!dragIdRef.current || !COL_DROP[col]) return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setDragOverCol(laneColKey)
  }
  const onDropCol = (e: DragEvent, col: string) => {
    e.preventDefault()
    const status = COL_DROP[col]
    const id = dragIdRef.current || e.dataTransfer.getData('text/plain')
    const card = (cards || []).find((c) => c.id === id)
    clearDrag()
    if (!status || !card || card.type !== 'bug' || bucket(card.status) === col) return
    patch(card, { status })
    toast('info', 'Moved', card.title || card.name)
  }

  const saveTags = async (card: NodeCard, tags: string[]) => {
    if (!runId) return
    setSel({ ...card, tags })
    try { await api.setNodeTags(runId, card.id, tags); void loadBoard(runId) }
    catch (e) { toast('error', 'Tagging failed', (e as Error).message) }
  }

  const runFix = async (card: NodeCard) => {
    if (!runId) return
    try {
      await api.enqueue(runId, card.id)
      toast('info', 'Fix re-queued', 'The dev loop will pick it up — watch the board.')
      void loadBoard(runId)
    } catch (e) { toast('error', 'Could not queue', (e as Error).message) }
  }
  const patch = async (card: NodeCard, p: PatchNodeBody) => {
    if (!runId) return
    setSel({ ...card, ...p })
    try { await api.patchNode(runId, card.id, p); void loadBoard(runId) }
    catch (e) { toast('error', 'Update failed', (e as Error).message) }
  }
  const dismiss = async (card: NodeCard) => { await patch(card, { status: 'dismissed' }); toast('info', 'Ticket dismissed'); setSel(null) }
  const restore = async (card: NodeCard) => patch(card, { status: 'open' })
  const canMarkDone = (c: NodeCard) => c.type === 'bug' && c.status === 'in_review'
  const markDone = async (card: NodeCard) => {
    await patch(card, { status: 'done' })
    toast('success', 'Marked done', card.title || card.name)
  }

  const canRefix = (c: NodeCard) => c.type === 'bug' && ['open', 'failed', 'in_review', 'done'].includes(c.status)
  const hasGit = !!detail?.git?.has_git && !!detail?.git?.remote_url
  const canPushPR = (c: NodeCard) =>
    c.type === 'bug' && hasGit && !!c.commit_sha && !c.pr_url &&
    ['done', 'in_review'].includes(c.status)

  const pushPR = async (card: NodeCard) => {
    if (!runId || prBusy) return
    setPrBusy(true)
    try {
      const res = await api.pushPR(runId, card.id)
      toast('success', 'PR created', res.pr_url)
      setSel({ ...card, pr_url: res.pr_url, pr_status: 'pushed' })
      void loadBoard(runId)
    } catch (e) {
      toast('error', 'Push PR failed', (e as Error).message)
    } finally {
      setPrBusy(false)
    }
  }

  const previewsFor = (c: NodeCard) => {
    if (c.type !== 'qa') return []
    const mod = c.tags.find((t) => t.startsWith('module:'))?.slice(7)
    return (detail?.files || []).filter((f) =>
      f.path.startsWith('.myaudit/preview/') && /\.(png|jpe?g|webp|gif)$/i.test(f.path) &&
      (!mod || f.path.includes(mod)))
  }



  if (!runId) return <div className="empty-mid"><h3>No board</h3><p>Import a codebase to see its audit board.</p></div>
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
    // Defects-only by default. #25: the agent is asked for improvements and
    // style notes too, and before they were classed they arrived as tickets
    // competing with real bugs. Non-bug cards are one dropdown away.
    if (n.type === 'bug' && fClass !== 'all' && !(fClass === 'defects' ? isDefect(n) : n.class === fClass)) return false
    if (ql) {
      const hay = (label(n) + ' ' + (n.summary || '') + ' ' + n.tags.join(' ')).toLowerCase()
      if (!hay.includes(ql)) return false
    }
    return true
  })
  const activeFilter = fSev !== 'all' || fType !== 'all' || fClass !== 'defects' || ql !== ''
  if (sortBy !== 'default') visible.sort(SORTERS[sortBy])

  const renderCard = (n: NodeCard) => {
    const stripe = (isDefect(n) ? SEV_COLOR[n.severity || ''] : CLASS_COLOR[n.class!]) || STATUS_COLOR[n.status] || 'var(--border-subtle)'
    const moduleTag = n.tags.find((t) => t.startsWith('module:'))?.slice(7)
    const otherTags = n.tags.filter((t) => t !== n.severity && !t.startsWith('module:') && !t.startsWith('class:') && !t.startsWith('scope:') && t !== ('from:qa'))
    return (
    <div className={`kcard jira ${isFailed(n.status) ? 'failed' : ''} ${n.status === 'dismissed' ? 'dismissed' : ''} ${n.status === 'running' ? 'running' : ''} ${dragId === n.id ? 'dragging' : ''}`}
      key={n.id} tabIndex={0} role="button" style={{ ['--stripe' as string]: stripe }}
      draggable={n.type === 'bug'}
      onDragStart={(e) => {
        if (n.type !== 'bug') return
        dragIdRef.current = n.id
        setDragId(n.id)
        e.dataTransfer.setData('text/plain', n.id)
        e.dataTransfer.effectAllowed = 'move'
        if (e.dataTransfer.setDragImage && e.currentTarget instanceof HTMLElement) {
          e.dataTransfer.setDragImage(e.currentTarget, 16, 16)
        }
      }}
      onDragEnd={clearDrag}
      onClick={() => setSel(n)}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSel(n) } }}>
      {n.type === 'bug' && (
        <div className="kcard-actions" draggable={false} onDragStart={(e) => e.preventDefault()}>
          <button className="ka-btn" draggable={false} title="Open" onClick={(e) => { e.stopPropagation(); setSel(n) }}>⤢</button>
          {canRefix(n) && <button className="ka-btn" draggable={false} title="Re-run fix" onClick={(e) => { e.stopPropagation(); runFix(n) }}>▶</button>}
          {canMarkDone(n) && <button className="ka-btn" draggable={false} title="Mark done" onClick={(e) => { e.stopPropagation(); markDone(n) }}>✓</button>}
          {n.status !== 'dismissed' && <button className="ka-btn" draggable={false} title="Dismiss" onClick={(e) => { e.stopPropagation(); dismiss(n) }}>✕</button>}
        </div>
      )}
      <div className="jtitle">{label(n)}</div>
      {n.status === 'running'
        ? <div className="kstep"><span className="spin-sm" />{latestStep(n.id) || 'working…'}</div>
        : n.summary && <div className="ksum">{n.summary}</div>}
      {(n.severity || moduleTag || otherTags.length > 0) && (
        <div className="ktags">
          {!isDefect(n) && <span className="jsev" style={{ background: CLASS_COLOR[n.class!] || 'var(--text-muted)' }}>{n.class}</span>}
          {n.severity && isDefect(n) && <span className="jsev" style={{ background: SEV_COLOR[n.severity] || 'var(--text-muted)' }}>{n.severity}</span>}
          {n.tags.some((t) => t === 'scope:adjacent') && <span className="ktag" title="This finding is outside the module that was audited">outside module</span>}
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
    groupBy === 'module' ? (n.tags.find((t) => t.startsWith('module:'))?.slice(7) || '—')
      : groupBy === 'severity' ? (n.severity || '—')
      : 'All'
  const lanes = groupBy === 'none' ? ['All'] : [...new Set(visible.map(laneOf))].sort()

  orderedRef.current = lanes.flatMap((ln) =>
    COLS.flatMap((c) => visible.filter((n) => laneOf(n) === ln && bucket(n.status) === c.key)))

  const events = detail?.events || []
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
        <select className="bf-sel" value={fClass} onChange={(e) => setFClass(e.target.value)} title="Defects, or everything the agent reported">
          <option value="defects">Defects</option>
          <option value="all">All findings</option>
          <option value="improvement">Improvements</option>
          <option value="style">Style</option>
          <option value="question">Questions</option>
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
          <button className="btn-sm" onClick={() => { setQ(''); setFSev('all'); setFType('all'); setFClass('defects') }}>Clear</button>
        </>}
        {dismissedCount > 0 && (
          <button className={`btn-sm ${showDismissed ? 'primary' : ''}`} style={{ marginLeft: 'auto' }}
            onClick={() => setShowDismissed((v) => !v)}>{showDismissed ? 'Hide' : 'Show'} dismissed ({dismissedCount})</button>
        )}
        {runId && (
          <a className="btn-sm" href={api.patchUrl(runId)} download="fixes.patch" title="Download all code changes as a unified diff"
            style={{ marginLeft: dismissedCount > 0 ? 0 : 'auto' }}>
            <Download size={12} /> Patch
          </a>
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
              const canDropHere = !!COL_DROP[c.key]
              const dragOver = dragOverCol === colKey && canDropHere && !!dragId
              const sc = sevCounts(items)
              return (
                <div className={`kcol col-${c.key} ${dragOver ? 'dragover' : ''} ${isCollapsed ? 'collapsed' : ''}`} key={colKey}
                  onDragOver={(e) => onDragCol(e, colKey, c.key)}
                  onDragLeave={() => setDragOverCol((cur) => (cur === colKey ? null : cur))}
                  onDrop={(e) => onDropCol(e, c.key)}>
                  <div className="kcol-h" onClick={() => setCollapsed((s) => { const n = new Set(s); n.has(c.key) ? n.delete(c.key) : n.add(c.key); return n })} title={isCollapsed ? 'Expand' : 'Collapse'}
                    onDragOver={(e) => onDragCol(e, colKey, c.key)}
                    onDrop={(e) => onDropCol(e, c.key)}>
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
                    <div className={`kcards${dragId ? ' drop-target' : ''}`}
                      onDragOver={(e) => onDragCol(e, colKey, c.key)}
                      onDrop={(e) => onDropCol(e, c.key)}>
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
            <div className="drawer-body" ref={drawerBody}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                <span className="kbadge" style={{ color: STATUS_COLOR[sel.status], background: 'var(--bg-panel)' }}>
                  <span className="kdot" style={{ background: STATUS_COLOR[sel.status] }} />{sel.status}
                </span>
                {canMarkDone(sel) && (
                  <button className={`btn-sm${canPushPR(sel) ? '' : ' primary'}`} onClick={() => markDone(sel)}>Mark done</button>
                )}
                {canRefix(sel) && (
                  <button className={`btn-sm${sel.status === 'open' ? ' primary' : ''}`} onClick={() => runFix(sel)}>
                    {sel.status === 'open' ? '▶ Fix this' : sel.status === 'done' ? '↻ Reopen & re-fix' : '▶ Re-run fix'}
                  </button>
                )}
                {canPushPR(sel) && (
                  <button className="btn-sm primary" disabled={prBusy} onClick={() => pushPR(sel)}>
                    {prBusy ? 'Pushing…' : 'Push PR'}
                  </button>
                )}
                {sel.pr_url && (
                  <a className="btn-sm" href={sel.pr_url} target="_blank" rel="noreferrer">View PR</a>
                )}
                {sel.type === 'bug' && sel.status === 'dismissed' && <button className="btn-sm" onClick={() => restore(sel)}>Restore</button>}
                {sel.type === 'bug' && sel.status !== 'dismissed' && <button className="btn-sm" onClick={() => dismiss(sel)}>Dismiss</button>}
              </div>

              {sel.type === 'bug' && (
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  <select className="bf-sel" value={sel.severity || 'medium'} onChange={(e) => patch(sel, { severity: e.target.value as PatchNodeBody['severity'] })}>
                    <option value="high">high</option><option value="medium">medium</option><option value="low">low</option>
                  </select>
                  <select className="bf-sel" value={sel.priority || 'P1'} onChange={(e) => patch(sel, { priority: e.target.value as PatchNodeBody['priority'] })}>
                    <option value="P0">P0</option><option value="P1">P1</option><option value="P2">P2</option>
                  </select>
                </div>
              )}
              {sel.file && (
                <button className="file-jump" onClick={() => openFile(sel)} title="Open in editor">
                  <FileSymlink size={13} /> <code>{sel.file}</code>
                </button>
              )}
              {sel.summary && (
                <div>
                  <div className="drawer-sec-h">Summary</div>
                  <Markdown text={sel.summary} block className="drawer-md" />
                </div>
              )}
              {sel.detail && (
                <div className="drawer-detail">
                  <div className="drawer-sec-h">Finding</div>
                  <Markdown text={sel.detail} block className="drawer-md" />
                </div>
              )}

              {sel.type === 'bug' && fixDiff.trim() && (
                <div>
                  <div className="drawer-sec-h">Fix diff</div>
                  <div style={{ maxHeight: 320, overflow: 'auto', border: '1px solid var(--border-dim)', borderRadius: 8 }}><Diff text={fixDiff} /></div>
                </div>
              )}

              {previewsFor(sel).length > 0 && (
                <div>
                  <div className="drawer-sec-h">QA preview</div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {previewsFor(sel).map((f) => (
                      <img key={f.path} src={api.rawUrl(runId!, f.path)} alt={f.path}
                        style={{ maxWidth: '100%', borderRadius: 8, border: '1px solid var(--border-dim)' }} />
                    ))}
                  </div>
                </div>
              )}

              {(sel.tags?.length || sel.type === 'bug') && (
                <div>
                  <div className="drawer-sec-h">Tags</div>
                  <div className="ktags">
                    {sel.tags.map((t) => (
                      <span key={t} className="ktag" style={{ cursor: 'pointer' }} onClick={() => saveTags(sel, sel.tags.filter((x) => x !== t))} title="Remove">{t} ×</span>
                    ))}
                    <input className="tag-input" value={tagDraft} onChange={(e) => setTagDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && tagDraft.trim()) {
                          const nt = Array.from(new Set([...sel.tags, tagDraft.trim()]))
                          saveTags(sel, nt); setTagDraft('')
                        }
                      }}
                      placeholder="+ tag" />
                  </div>
                </div>
              )}

              <details className="drawer-more">
                <summary>More</summary>
                <dl className="drawer-kv">
                  <dt>id</dt><dd>{sel.id}</dd>
                  <dt>type</dt><dd>{sel.type}</dd>
                  {sel.cost_usd > 0 && <><dt>cost</dt><dd>${sel.cost_usd.toFixed(4)}</dd></>}
                  <dt>attempts</dt><dd>{sel.attempts}</dd>
                  <dt>files</dt><dd>{sel.files}</dd>
                  <dt>created</dt><dd>{new Date(sel.created_at).toLocaleString()}</dd>
                </dl>
                <div style={{ marginTop: 12 }}>
                  <div className="drawer-sec-h">Activity</div>
                  {(detail?.events || []).filter((e) => e.node_id === sel.id).slice(-12).map((e, i) => (
                    <div key={i} style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, lineHeight: 1.7, color: 'var(--text-secondary)' }}>
                      <span style={{ color: 'var(--text-muted)' }}>{new Date(e.ts).toLocaleTimeString()}</span> {e.kind} {e.msg}
                    </div>
                  ))}
                </div>
              </details>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
