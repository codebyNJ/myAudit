import { X } from 'lucide-react'
import { useStore } from '../store'

export function Toasts() {
  const s = useStore()
  return (
    <div className="toasts" role="status" aria-live="polite">
      {s.toasts.map((t) => (
        <div key={t.id} className={`toast ${t.type}`} role={t.type === 'error' ? 'alert' : undefined}>
          <div><div className="tt">{t.title}</div>{t.msg && <div className="tm">{t.msg}</div>}</div>
          <button className="toast-x" aria-label="Dismiss" onClick={() => s.dismiss(t.id)}><X size={14} /></button>
        </div>
      ))}
    </div>
  )
}
