import { useEffect, useState } from 'react'
import { Database, GitBranch, AlertTriangle, ArrowRight, Play, RefreshCw } from 'lucide-react'
import { useStore } from '../store'
import { api, type FlowsResp, type FlowStep } from '../api'

const KIND_COLOR: Record<string, string> = {
  source: '#38bdf8', transform: '#c084fc', store: '#4ade80', read: '#e0a92e',
}

function Steps({ steps }: { steps?: FlowStep[] }) {
  const s = useStore()
  if (!steps || !steps.length) return null
  const open = (file?: string) => {
    if (!file) return
    s.setFile(file.split(':')[0])
    s.setTab('dev')
  }
  return (
    <div className="fl-steps">
      {steps.map((st, i) => (
        <div className="fl-step" key={i}>
          <span className="fl-dot" style={{ background: KIND_COLOR[st.kind || ''] || 'var(--text-muted)' }} />
          <span className="fl-label">{st.label}</span>
          {st.file && (
            <button className="fl-file" title={`Open ${st.file}`} onClick={() => open(st.file)}>{st.file}</button>
          )}
          {i < steps.length - 1 && <ArrowRight size={11} className="fl-arrow" />}
        </div>
      ))}
    </div>
  )
}

export function Flows() {
  const s = useStore()
  const [r, setR] = useState<FlowsResp | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!s.runId) { setR(null); return }
    let alive = true
    const load = () => api.flows(s.runId!).then((x) => { if (alive) setR(x) }).catch(() => {})
    load()
    const h = setInterval(load, 4000)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  const start = async () => {
    if (!s.runId) return
    setBusy(true)
    try {
      await api.runFlows(s.runId)
      s.toast('info', 'Identifying flows', 'The agent is mapping data and product flows.')
    } catch (e) {
      s.toast('error', 'Could not start', (e as Error).message)
    } finally { setBusy(false) }
  }

  if (!r) return null

  if (!r.ready) {
    return (
      <div className="vercel-card" style={{ marginTop: 8 }}>
        <div className="vercel-card-head">
          <div className="vercel-card-title"><GitBranch size={14} /> Flows</div>
        </div>
        <div className="fl-empty">
          {r.pending ? (
            <><RefreshCw size={22} className="fl-spin" /><h4>Identifying flows…</h4>
              <p>The agent is reading the codebase to map how data moves and how the product is used.</p></>
          ) : (
            <><GitBranch size={22} strokeWidth={1.5} /><h4>No flow map yet</h4>
              <p>Identify how data is stored and moved, and the main user journeys through this product.</p>
              <button className="btn-sm primary" disabled={busy} onClick={start}>
                <Play size={12} /> {busy ? 'Starting…' : 'Identify flows'}
              </button></>
          )}
        </div>
      </div>
    )
  }

  const doc = r.flows || {}
  const data = doc.data_flows || []
  const prod = doc.product_flows || []

  return (
    <div className="fl-wrap">
      {doc.persistence && (
        <div className="fl-persist"><Database size={13} /> <b>Storage</b> <span>{doc.persistence}</span></div>
      )}
      <div className="vercel-two-cards">
        <div className="vercel-card">
          <div className="vercel-card-head">
            <div className="vercel-card-title"><Database size={14} /> Database &amp; data flows</div>
            <span className="vercel-row-badge" style={{ color: 'var(--text-muted)' }}>{data.length}</span>
          </div>
          <div className="vercel-card-body">
            {data.length ? data.map((f, i) => (
              <div className="fl-item" key={i}>
                <div className="fl-head">
                  <span className="fl-name">{f.name}</span>
                  {f.store && <span className="fl-chip">{f.store}</span>}
                  {f.entity && <span className="fl-chip mono">{f.entity}</span>}
                </div>
                <Steps steps={f.steps} />
                {f.note && <div className="fl-note">{f.note}</div>}
                {f.concern && <div className="fl-concern"><AlertTriangle size={11} /> {f.concern}</div>}
              </div>
            )) : <div className="fl-none">No data flows identified.</div>}
          </div>
        </div>

        <div className="vercel-card">
          <div className="vercel-card-head">
            <div className="vercel-card-title"><GitBranch size={14} /> Product flows</div>
            <span className="vercel-row-badge" style={{ color: 'var(--text-muted)' }}>{prod.length}</span>
          </div>
          <div className="vercel-card-body">
            {prod.length ? prod.map((f, i) => (
              <div className="fl-item" key={i}>
                <div className="fl-head">
                  <span className="fl-name">{f.name}</span>
                  {f.trigger && <span className="fl-chip">{f.trigger}</span>}
                </div>
                <Steps steps={f.steps} />
                {f.outcome && <div className="fl-note"><b>Outcome:</b> {f.outcome}</div>}
                {f.concern && <div className="fl-concern"><AlertTriangle size={11} /> {f.concern}</div>}
              </div>
            )) : <div className="fl-none">No product flows identified.</div>}
          </div>
        </div>
      </div>
      <div className="fl-refresh">
        <button className="btn-sm" disabled={busy || r.pending} onClick={start}>
          <RefreshCw size={12} /> {r.pending ? 'Re-mapping…' : 'Re-identify flows'}
        </button>
      </div>
    </div>
  )
}
