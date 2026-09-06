import { useEffect, useState } from 'react'
import { 
  ShieldCheck, AlertTriangle, Cpu, Terminal, Layers, CheckCircle2, XCircle, Clock, ArrowUpRight,
  Sparkles, RefreshCw, Eye
} from 'lucide-react'
import { useStore } from '../store'
import { api, type NodeCard } from '../api'
import { evColor, fmtTime } from '../components/util'
import { Flows } from '../components/Flows'

export function PlaywrightScreen() {
  const s = useStore()
  const [cards, setCards] = useState<NodeCard[] | null>(null)
  const [actFilter, setActFilter] = useState('all')

  useEffect(() => {
    if (!s.runId) { setCards(null); return }
    let alive = true
    const load = () => api.board(s.runId!).then((c) => { if (alive) setCards(c) }).catch(() => {})
    load()
    const h = setInterval(load, 2000)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  if (!s.runId) {
    return (
      <div className="empty-mid">
        <ShieldCheck size={40} strokeWidth={1.5} color="var(--text-muted)" />
        <h3>No active audit</h3>
        <p>Import a codebase or select a project from the workspace menu to inspect its overview.</p>
      </div>
    )
  }

  if (cards == null) {
    return (
      <div className="empty-mid">
        <div className="spin" />
        <p style={{ marginTop: 12 }}>Loading audit metrics…</p>
      </div>
    )
  }

  const qa = cards.filter((c) => c.type === 'qa')
  const bugs = cards.filter((c) => c.type === 'bug')
  const findingsFor = (mod?: string) => bugs.filter((b) => (b.tags || []).includes('module:' + mod)).length
  const previews = (s.detail?.files || []).filter((f) => f.path.startsWith('.myaudit/preview/') && /\.(png|jpe?g|webp|gif)$/i.test(f.path))
  const events = s.detail?.events || []

  const sev = { high: 0, medium: 0, low: 0 }
  for (const b of bugs) {
    if (b.severity && b.severity in sev) sev[b.severity as keyof typeof sev]++
  }

  const verified = bugs.filter((b) => b.status === 'done').length
  const needsReview = bugs.filter((b) => b.status === 'in_review').length
  const runStatus = s.detail?.run?.status || 'idle'
  const cost = s.detail?.cost_usd || 0
  const curProject = (s.runs || []).find((r) => r.id === s.runId)?.project || 'Project'
  const pctVerified = bugs.length > 0 ? Math.round((verified / bugs.length) * 100) : 100

  const filteredEvents = events.filter((e) => {
    if (actFilter === 'all') return true
    if (actFilter === 'finding') return e.kind.includes('finding') || e.kind.includes('bug')
    if (actFilter === 'qa') return e.kind.includes('qa')
    if (actFilter === 'step') return e.kind.includes('step')
    return true
  })

  return (
    <div className="vercel-dash">
      <div className="vercel-dash-inner">
        <div className="vercel-header">
          <div>
            <div className="vercel-title-row">
              <h1 className="vercel-title">{curProject}</h1>
              <span className="vercel-badge" style={{ 
                borderColor: runStatus === 'done' ? 'rgba(63,185,80,0.3)' : runStatus === 'running' ? 'rgba(56,189,248,0.3)' : 'var(--border-subtle)',
                color: runStatus === 'done' ? '#4ade80' : runStatus === 'running' ? '#38bdf8' : 'var(--text-secondary)',
                background: runStatus === 'done' ? 'rgba(63,185,80,0.08)' : runStatus === 'running' ? 'rgba(56,189,248,0.08)' : 'var(--bg-panel)'
              }}>
                <span style={{ 
                  width: 6, height: 6, borderRadius: '50%', 
                  background: runStatus === 'done' ? '#4ade80' : runStatus === 'running' ? '#38bdf8' : 'var(--text-muted)' 
                }} />
                {runStatus.toUpperCase()}
              </span>
              <span className="vercel-badge" style={{ color: 'var(--text-muted)' }}>
                <Terminal size={11} /> main
              </span>
            </div>
            <p className="vercel-sub">
              Continuous codebase audit, automated QA modules, regression verification, and live autonomous repair.
            </p>
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <button className="btn-sm" onClick={() => s.setTab('kanban')}>
              View Board <ArrowUpRight size={13} />
            </button>
            <button className="btn-sm" onClick={() => s.setTab('notes')}>
              <Eye size={13} /> Notes &amp; Report
            </button>
            <button className="btn-sm primary" onClick={s.refresh}>
              <RefreshCw size={13} /> Sync
            </button>
          </div>
        </div>

        <div className="vercel-stats-grid">
          <div className="vercel-stat-card">
            <div className="vercel-stat-label">
              <span>QA Modules</span>
              <Layers size={14} color="var(--text-muted)" />
            </div>
            <div className="vercel-stat-val">{qa.length}</div>
            <div className="vercel-stat-sub">
              <span>{qa.filter(m => m.status === 'done').length} verified of {qa.length}</span>
            </div>
          </div>

          <div className="vercel-stat-card">
            <div className="vercel-stat-label">
              <span>Findings Logged</span>
              <AlertTriangle size={14} color="#f87171" />
            </div>
            <div className="vercel-stat-val">{bugs.length}</div>
            <div className="vercel-stat-sub">
              <span style={{ color: '#f87171', fontWeight: 600 }}>{sev.high} High</span>
              <span>·</span>
              <span style={{ color: '#e0a92e', fontWeight: 600 }}>{sev.medium} Med</span>
              <span>·</span>
              <span style={{ color: '#60a5fa', fontWeight: 600 }}>{sev.low} Low</span>
            </div>
          </div>

          <div className="vercel-stat-card">
            <div className="vercel-stat-label">
              <span>Verified Fixes</span>
              <CheckCircle2 size={14} color="#3fb950" />
            </div>
            <div className="vercel-stat-val">
              {verified}
              <span style={{ fontSize: 16, fontWeight: 400, color: 'var(--text-muted)', marginLeft: 4 }}>/{bugs.length}</span>
            </div>
            <div className="vercel-stat-sub">
              <div style={{ flex: 1, height: 4, background: 'rgba(255,255,255,0.08)', borderRadius: 999, overflow: 'hidden' }}>
                <div style={{ width: `${pctVerified}%`, height: '100%', background: '#3fb950', transition: 'width 0.4s ease' }} />
              </div>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: '#3fb950' }}>{pctVerified}%</span>
            </div>
          </div>

          <div className="vercel-stat-card">
            <div className="vercel-stat-label">
              <span>Model Spend</span>
              <Cpu size={14} color="var(--text-muted)" />
            </div>
            <div className="vercel-stat-val">${cost.toFixed(2)}</div>
            <div className="vercel-stat-sub">
              <span>Autonomous Claude agent</span>
            </div>
          </div>

          <div className="vercel-stat-card">
            <div className="vercel-stat-label">
              <span>Activity Events</span>
              <Clock size={14} color="var(--text-muted)" />
            </div>
            <div className="vercel-stat-val">{events.length}</div>
            <div className="vercel-stat-sub">
              <span>{needsReview} awaiting review</span>
            </div>
          </div>
        </div>

        <Flows />

        <div className="vercel-two-cards">
          <div className="vercel-card">
            <div className="vercel-card-head">
              <span className="vercel-card-title">
                <Layers size={15} color="#38bdf8" /> Module QA Coverage
              </span>
              <span className="vercel-row-badge" style={{ background: 'rgba(56,189,248,0.1)', color: '#38bdf8' }}>
                {qa.length} Modules
              </span>
            </div>
            <div className="vercel-card-body">
              {qa.length ? (
                qa.map((m) => {
                  const modTag = (m.tags || []).find((t) => t.startsWith('module:'))?.slice(7)
                  const fCount = findingsFor(modTag)
                  const isDone = m.status === 'done'
                  return (
                    <div 
                      key={m.id} 
                      className="vercel-row interactive" 
                      onClick={() => s.openCard(m.id)}
                      title="Inspect module card"
                    >
                      {isDone ? (
                        <CheckCircle2 size={16} color="#3fb950" />
                      ) : m.status === 'running' ? (
                        <div className="spin-sm" style={{ color: '#38bdf8' }} />
                      ) : (
                        <Clock size={16} color="var(--text-muted)" />
                      )}
                      <div className="vercel-row-title">
                        {m.title || m.name}
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        {fCount > 0 ? (
                          <span className="vercel-row-badge" style={{ background: 'rgba(248,113,113,0.12)', color: '#f87171' }}>
                            {fCount} {fCount === 1 ? 'issue' : 'issues'}
                          </span>
                        ) : (
                          <span className="vercel-row-badge" style={{ background: 'rgba(63,185,80,0.1)', color: '#4ade80' }}>
                            Clean
                          </span>
                        )}
                        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>
                          {m.status}
                        </span>
                      </div>
                    </div>
                  )
                })
              ) : (
                <div style={{ padding: 28, color: 'var(--text-muted)', textAlign: 'center' }}>
                  No QA modules discovered yet.
                </div>
              )}
            </div>
          </div>

          <div className="vercel-card">
            <div className="vercel-card-head">
              <span className="vercel-card-title">
                <ShieldCheck size={15} color="#4ade80" /> Fix Verification &amp; Regressions
              </span>
              <span className="vercel-row-badge" style={{ background: 'rgba(63,185,80,0.1)', color: '#4ade80' }}>
                {verified} / {bugs.length} Verified
              </span>
            </div>
            <div className="vercel-card-body">
              {bugs.length ? (
                bugs.map((b) => {
                  const isDone = b.status === 'done'
                  const isFailed = b.status === 'failed'
                  const isReview = b.status === 'in_review'
                  return (
                    <div 
                      key={b.id} 
                      className="vercel-row interactive" 
                      onClick={() => s.openCard(b.id)}
                      title="Inspect fix ticket"
                    >
                      {isDone ? (
                        <CheckCircle2 size={16} color="#3fb950" />
                      ) : isFailed ? (
                        <XCircle size={16} color="#f87171" />
                      ) : (
                        <span style={{ width: 8, height: 8, borderRadius: '50%', background: '#e0a92e' }} />
                      )}
                      <div className="vercel-row-title">
                        {b.title || b.name}
                      </div>
                      <span className="vercel-row-badge" style={{
                        background: isDone ? 'rgba(63,185,80,0.1)' : isFailed ? 'rgba(248,113,113,0.12)' : 'rgba(224,169,46,0.12)',
                        color: isDone ? '#4ade80' : isFailed ? '#f87171' : '#e0a92e'
                      }}>
                        {isDone ? 'Verified' : isFailed ? 'Failed Regression' : isReview ? 'Needs Review' : b.status}
                      </span>
                    </div>
                  )
                })
              ) : (
                <div style={{ padding: 28, color: 'var(--text-muted)', textAlign: 'center' }}>
                  No bug fixes or findings registered yet.
                </div>
              )}
            </div>
          </div>
        </div>

        {previews.length > 0 && (
          <div className="vercel-card" style={{ marginBottom: 24 }}>
            <div className="vercel-card-head">
              <span className="vercel-card-title">
                <Sparkles size={15} color="#e0a92e" /> QA UI Previews &amp; Artifacts
              </span>
              <span className="vercel-row-badge" style={{ background: 'rgba(255,255,255,0.06)', color: 'var(--text-secondary)' }}>
                {previews.length} Previews
              </span>
            </div>
            <div style={{ padding: 18, display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 14 }}>
              {previews.map((f) => (
                <div key={f.path} style={{ border: '1px solid var(--border-dim)', borderRadius: 10, overflow: 'hidden', background: '#070709' }}>
                  <img src={api.rawUrl(s.runId!, f.path)} alt={f.path} style={{ width: '100%', height: 180, objectFit: 'cover', display: 'block' }} />
                  <div style={{ padding: '8px 12px', fontSize: 11, fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)', borderTop: '1px solid var(--border-dim)' }}>
                    {f.path.split('/').pop()}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="vercel-activity-card">
          <div className="vercel-card-head">
            <span className="vercel-card-title">
              <Terminal size={15} color="#a1a1aa" /> Live Audit Activity Stream
            </span>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              {['all', 'finding', 'qa', 'step'].map((f) => (
                <button 
                  key={f} 
                  className={`btn-sm ${actFilter === f ? 'primary' : ''}`}
                  style={{ padding: '2px 8px', fontSize: 11 }}
                  onClick={() => setActFilter(f)}
                >
                  {f === 'all' ? 'All' : f.toUpperCase()}
                </button>
              ))}
              <span style={{ fontSize: 11, color: 'var(--text-muted)', marginLeft: 6, fontFamily: 'var(--font-mono)' }}>
                {filteredEvents.length} events
              </span>
            </div>
          </div>
          <div className="vercel-activity-list">
            {filteredEvents.length ? (
              filteredEvents.slice().reverse().slice(0, 150).map((e, idx) => (
                <div className="vercel-act-item" key={idx}>
                  <span className="vercel-act-time">{fmtTime(e.ts)}</span>
                  <span className="vercel-act-kind" style={{ color: evColor(e.kind) }}>
                    {e.kind}
                  </span>
                  <span className="vercel-act-msg" title={e.msg}>
                    {e.msg}
                  </span>
                </div>
              ))
            ) : (
              <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)' }}>
                No events matching filter.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
