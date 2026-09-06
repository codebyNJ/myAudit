import { useState } from 'react'
import { Plus, Search, FolderGit2, CheckCircle2, Loader2, ArrowUpRight, Sparkles } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'
import { AgentAvatar } from '../components/icons'
import { HealthBanner } from '../components/HealthBanner'
import { STATUS_COLOR } from '../components/util'

function relTime(ts: string) {
  const d = Date.now() - new Date(ts).getTime()
  const m = Math.floor(d / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  const days = Math.floor(h / 24)
  if (days < 30) return `${days}d ago`
  return new Date(ts).toLocaleDateString()
}

export function HomeScreen() {
  const s = useStore()
  const runs = s.runs || []
  const [q, setQ] = useState('')
  const [demoBusy, setDemoBusy] = useState(false)

  const visible = q.trim()
    ? runs.filter((r) => (r.project || '').toLowerCase().includes(q.trim().toLowerCase()))
    : runs

  const done = runs.filter((r) => r.status === 'done').length
  const active = runs.filter((r) => r.status === 'running' || r.status === 'ready' || r.status === 'pending').length

  const tryDemo = async () => {
    setDemoBusy(true)
    try {
      const { path } = await api.demoPath()
      await s.createRun({ repo_path: path, project: 'demo', audit_only: true, budget_usd: 3 })
    } catch (e) {
      s.toast('error', 'Demo failed', (e as Error).message)
    } finally {
      setDemoBusy(false)
    }
  }

  return (
    <div className="home">
      <div className="home-inner">
        <div className="home-bar">
          <div className="home-brand">
            <AgentAvatar size={30} radius={8} />
            <div>
              <h1>myAudit</h1>
              <p>Autonomous QA and repair for your codebase.</p>
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn-sm" disabled={demoBusy} onClick={tryDemo}>
              <Sparkles size={14} /> {demoBusy ? 'Starting…' : 'Try demo'}
            </button>
            <button className="btn-sm primary home-cta" onClick={() => s.setNewOpen(true)}>
              <Plus size={14} /> Import codebase
            </button>
          </div>
        </div>

        <HealthBanner />

        {runs.length > 0 && (
          <div className="home-stats">
            <div className="home-stat">
              <span className="hs-k"><FolderGit2 size={13} /> Audits</span>
              <span className="hs-v">{runs.length}</span>
            </div>
            <div className="home-stat">
              <span className="hs-k"><CheckCircle2 size={13} /> Completed</span>
              <span className="hs-v">{done}</span>
            </div>
            <div className="home-stat">
              <span className="hs-k"><Loader2 size={13} /> In progress</span>
              <span className="hs-v">{active}</span>
            </div>
          </div>
        )}

        <div className="home-sec">
          <span className="home-sec-t">Audits</span>
          {runs.length > 4 && (
            <div className="home-search">
              <Search size={13} />
              <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search audits…" />
            </div>
          )}
        </div>

        <div className="home-grid">
          <button className="proj-card new" onClick={() => s.setNewOpen(true)}>
            <Plus size={18} />
            <span>Import codebase</span>
          </button>
          {visible.map((r) => (
            <button key={r.id} className="proj-card" onClick={() => s.setRun(r.id)}>
              <div className="proj-top">
                <div className="proj-name">{r.project}</div>
                <ArrowUpRight size={14} className="proj-go" />
              </div>
              <div className="proj-meta">
                <span className="st">
                  <span className="st-dot" style={{ background: STATUS_COLOR[r.status] || 'var(--text-muted)' }} />
                  {r.status || 'active'}
                </span>
                <span className="proj-time">{relTime(r.created_at)}</span>
              </div>
            </button>
          ))}
        </div>

        {s.runs && !runs.length && (
          <div className="home-empty">
            <FolderGit2 size={30} strokeWidth={1.5} />
            <h3>No audits yet</h3>
            <p>Try the demo repo, or import your own codebase to map, QA, and fix.</p>
            <button className="btn-sm primary" style={{ marginTop: 12 }} disabled={demoBusy} onClick={tryDemo}>
              <Sparkles size={14} /> {demoBusy ? 'Starting…' : 'Try the demo'}
            </button>
          </div>
        )}
        {s.runs && runs.length > 0 && !visible.length && (
          <div className="home-empty"><p>No audit matches “{q}”.</p></div>
        )}
      </div>
    </div>
  )
}
