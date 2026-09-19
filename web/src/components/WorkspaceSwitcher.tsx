import { useState } from 'react'
import { useAppStore } from '../store/slices'
import { IcChevron } from './icons'
import { STATUS_COLOR } from './util'

export function WorkspaceSwitcher() {
  const runs = useAppStore((s) => s.runs)
  const runId = useAppStore((s) => s.runId)
  const setRun = useAppStore((s) => s.setRun)
  const [open, setOpen] = useState(false)
  const cur = (runs || []).find((r) => r.id === runId)
  return (
    <div style={{ position: 'relative' }}>
      <div className="workspace-switcher" onClick={() => setOpen((v) => !v)}>
        <span className="ws-name">{cur ? cur.project : (runs && runs.length === 0 ? 'No projects' : 'Loading…')}</span> <IcChevron />
      </div>
      {open && (
        <div className="dropdown" style={{ top: 34, left: 0 }} onMouseLeave={() => setOpen(false)}>
          {runs && runs.length ? runs.map((r) => (
            <div key={r.id} className={`dd-item ${r.id === runId ? 'on' : ''}`} onClick={() => { setRun(r.id); setOpen(false) }}>
              <span style={{ width: 7, height: 7, borderRadius: 999, background: STATUS_COLOR[r.status] || 'var(--text-muted)' }} />
              {r.project}
              <span style={{ marginLeft: 'auto', fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-muted)' }}>{r.status}</span>
            </div>
          )) : <div style={{ padding: 12, color: 'var(--text-muted)', textAlign: 'center' }}>No audits yet — import a codebase.</div>}
        </div>
      )}
    </div>
  )
}
