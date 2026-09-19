import { useState } from 'react'
import { useAppStore } from '../store/slices'
import { AgentAvatar } from './icons'

export function AccountMenu() {
  const setTab = useAppStore((s) => s.setTab)
  const [open, setOpen] = useState(false)
  return (
    <div style={{ position: 'relative' }}>
      <div className="acct-btn" onClick={() => setOpen((v) => !v)}><AgentAvatar size={28} radius={0} /></div>
      {open && (
        <div className="dropdown" style={{ top: 36, right: 0 }} onMouseLeave={() => setOpen(false)}>
          <div style={{ padding: '8px 10px', fontSize: 12, color: 'var(--text-muted)' }}>myAudit — local</div>
          <div className="dd-item" onClick={() => { setTab('settings'); setOpen(false) }}>Settings</div>
        </div>
      )}
    </div>
  )
}
