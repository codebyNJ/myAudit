import { useState } from 'react'
import { useStore } from '../store'

const cls = (m: string) => m === 'GET' ? 'get' : m === 'POST' ? 'post' : 'del'
const label = (m: string) => (m === 'DELETE' ? 'DEL' : m)

export function SwaggerScreen() {
  const s = useStore()
  const [sel, setSel] = useState(0)
  if (s.apis == null) return <div className="empty-mid"><div className="spin" /></div>
  if (!s.apis.length) return <div className="empty-mid"><h3>No endpoints</h3></div>
  const a = s.apis[sel] || s.apis[0]
  return (
    <div className="swagger-layout">
      <div className="api-list">
        {s.apis.map((x, i) => (
          <div key={i} className={`api-item ${i === sel ? 'active' : ''}`} onClick={() => setSel(i)}>
            <span className={`meth ${cls(x.method)}`}>{label(x.method)}</span>
            <span className="api-path">{x.path}</span>
          </div>
        ))}
      </div>
      <div className="api-detail">
        <div className="api-title"><span className={`meth ${cls(a.method)}`} style={{ fontSize: 14, padding: '4px 10px', width: 'auto' }}>{label(a.method)}</span><h2>{a.path}</h2></div>
        <p style={{ color: 'var(--text-secondary)', fontSize: 14, marginTop: 0 }}>{a.summary}</p>
        <h3 style={{ fontSize: 12, textTransform: 'uppercase', marginTop: 36, color: 'var(--text-muted)', letterSpacing: '.05em' }}>Responses</h3>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12, fontFamily: 'var(--font-mono)', fontSize: 12 }}>
          <span style={{ color: 'var(--method-get)', background: 'var(--method-get-bg)', padding: '2px 6px', borderRadius: 4 }}>200 OK</span>
          <span style={{ color: 'var(--text-secondary)' }}>Success</span>
        </div>
        <div className="json-block">{`{\n  "ok": true\n}`}</div>
      </div>
    </div>
  )
}
