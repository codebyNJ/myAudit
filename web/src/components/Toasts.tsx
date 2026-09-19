import { X } from 'lucide-react'
import { useAppStore } from '../store/slices'

export function Toasts() {
  const toasts = useAppStore((s) => s.toasts)
  const dismiss = useAppStore((s) => s.dismiss)
  return (
    <div className="toasts" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={`toast ${t.type}`} role={t.type === 'error' ? 'alert' : undefined}>
          <div><div className="tt">{t.title}</div>{t.msg && <div className="tm">{t.msg}</div>}</div>
          <button className="toast-x" aria-label="Dismiss" onClick={() => dismiss(t.id)}><X size={14} /></button>
        </div>
      ))}
    </div>
  )
}
