import { useState } from 'react'
import { useStore } from '../store'
import { AgentAvatar } from './icons'

export function AccountMenu() {
  const s = useStore()
  const [open, setOpen] = useState(false)
  return (
    <div style={{ position: 'relative' }}>
      <div className="acct-btn" onClick={() => setOpen((v) => !v)}><AgentAvatar size={28} radius={0} /></div>
      {open && (
        <div className="dropdown" style={{ top: 36, right: 0 }} onMouseLeave={() => setOpen(false)}>
          <div style={{ padding: '8px 10px', fontSize: 12, color: 'var(--text-muted)' }}>Signed in as <span style={{ color: 'var(--text-primary)' }}>nijeesh</span></div>
          <div className="dd-item" onClick={() => { s.setTab('settings'); setOpen(false) }}>Account &amp; Settings</div>
          <div className="dd-item">Sign out</div>
        </div>
      )}
    </div>
  )
}
