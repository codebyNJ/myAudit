import { useStore } from '../store'

export function Toasts() {
  const s = useStore()
  return (
    <div className="toasts">
      {s.toasts.map((t) => (
        <div key={t.id} className={`toast ${t.type}`}>
          <div><div className="tt">{t.title}</div>{t.msg && <div className="tm">{t.msg}</div>}</div>
        </div>
      ))}
    </div>
  )
}
