import { useState } from 'react'
import { useStore } from '../store'
import { IcChevron } from './icons'
import { STATUS_COLOR } from './util'

export function WorkspaceSwitcher() {
  const s = useStore()
  const [open, setOpen] = useState(false)
  const cur = (s.runs || []).find((r) => r.id === s.runId)
  return (
    <div style={{ position: 'relative' }}>
      <div className="workspace-switcher" onClick={() => setOpen((v) => !v)}>
        <span className="ws-name">{cur ? cur.project : (s.runs && s.runs.length === 0 ? 'No projects' : 'Loading…')}</span> <IcChevron />
      </div>
      {open && (
        <div className="dropdown" style={{ top: 34, left: 0 }} onMouseLeave={() => setOpen(false)}>
          {s.runs && s.runs.length ? s.runs.map((r) => (
            <div key={r.id} className={`dd-item ${r.id === s.runId ? 'on' : ''}`} onClick={() => { s.setRun(r.id); setOpen(false) }}>
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
