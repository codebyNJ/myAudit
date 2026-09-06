import { Plus, Clock } from 'lucide-react'
import { useStore } from '../store'
import { AgentAvatar } from '../components/icons'
import { HealthBanner } from '../components/HealthBanner'

// The opening page: pick a project/workspace to open, or start a new one.
export function HomeScreen() {
  const s = useStore()
  const runs = s.runs || []
  return (
    <div className="home">
      <div className="home-inner">
        <div className="home-head">
          <div className="logo lg"><AgentAvatar size={40} radius={10} /></div>
          <h1>myAudit</h1>
          <p>Open a past audit, or import a codebase to start one.</p>
        </div>
        <HealthBanner />
        <div className="home-grid">
          <button className="proj-card new" onClick={() => s.setNewOpen(true)}>
            <Plus size={20} /><span>Import codebase</span>
          </button>
          {runs.map((r) => (
            <button key={r.id} className="proj-card" onClick={() => s.setRun(r.id)}>
              <div className="proj-name">{r.project}</div>
              <div className="proj-meta">
                <span className="st">{r.status || 'active'}</span>
                <span><Clock size={11} /> {new Date(r.created_at).toLocaleDateString()}</span>
              </div>
            </button>
          ))}
        </div>
        {s.runs && !runs.length && <div className="home-empty">No audits yet — import a codebase to start.</div>}
      </div>
    </div>
  )
}
