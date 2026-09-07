import { useEffect, useState, type ReactNode } from 'react'
import { Database, GitBranch, AlertTriangle, Play, RefreshCw } from 'lucide-react'
import { useStore } from '../store'
import { api, type FlowsResp, type FlowStep, type DataFlow, type ProductFlow } from '../api'

const KIND_COLOR: Record<string, string> = {
  source: '#38bdf8', transform: '#c084fc', store: '#4ade80', read: '#e0a92e',
}
const KIND_BG: Record<string, string> = {
  source: 'rgba(56,189,248,0.1)', transform: 'rgba(192,132,252,0.1)',
  store: 'rgba(74,222,128,0.1)', read: 'rgba(224,169,46,0.1)',
}

function Pipeline({ steps }: { steps?: FlowStep[] }) {
  const s = useStore()
  if (!steps?.length) return null
  const open = (file?: string) => {
    if (!file) return
    s.setFile(file.split(':')[0])
    s.setTab('dev')
  }
  return (
    <div className="flow-pipe">
      {steps.map((st, i) => {
        const kind = st.kind || ''
        const color = KIND_COLOR[kind] || 'var(--text-muted)'
        const bg = KIND_BG[kind] || 'rgba(255,255,255,0.04)'
        return (
          <div className="flow-pipe-seg" key={i}>
            <button
              type="button"
              className={`flow-node ${st.file ? 'clickable' : ''}`}
              style={{ borderColor: color + '55', background: bg }}
              title={st.file || st.label}
              onClick={() => open(st.file)}
              disabled={!st.file}
            >
              {kind && <span className="flow-node-kind" style={{ color }}>{kind}</span>}
              <span className="flow-node-label">{st.label}</span>
              {st.file && <span className="flow-node-file">{st.file}</span>}
            </button>
            {i < steps.length - 1 && (
              <span className="flow-edge" aria-hidden>
                <span className="flow-edge-line" />
                <span className="flow-edge-arrow" />
              </span>
            )}
          </div>
        )
      })}
    </div>
  )
}

function FlowCard({
  title, chips, steps, note, concern, outcome,
}: {
  title: string
  chips?: string[]
  steps?: FlowStep[]
  note?: string
  concern?: string
  outcome?: string
}) {
  return (
    <div className="flow-card">
      <div className="flow-card-h">
        <span className="flow-card-title">{title}</span>
        <span className="flow-card-chips">
          {(chips || []).filter(Boolean).map((c) => (
            <span className="fl-chip" key={c}>{c}</span>
          ))}
        </span>
      </div>
      <Pipeline steps={steps} />
      {outcome && <div className="fl-note"><b>Outcome</b> — {outcome}</div>}
      {note && <div className="fl-note">{note}</div>}
      {concern && <div className="fl-concern"><AlertTriangle size={12} /> {concern}</div>}
    </div>
  )
}

function FlowSection({
  icon, title, count, children,
}: { icon: ReactNode; title: string; count: number; children: ReactNode }) {
  return (
    <div className="flow-section">
      <div className="flow-section-h">
        <span className="flow-section-title">{icon} {title}</span>
        <span className="vercel-row-badge" style={{ color: 'var(--text-muted)' }}>{count}</span>
      </div>
      <div className="flow-section-body">{children}</div>
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
  const data: DataFlow[] = doc.data_flows || []
  const prod: ProductFlow[] = doc.product_flows || []

  return (
    <div className="fl-wrap">
      {doc.persistence && (
        <div className="fl-persist"><Database size={13} /> <b>Storage</b> <span>{doc.persistence}</span></div>
      )}

      <FlowSection icon={<Database size={14} />} title="Database & data flows" count={data.length}>
        {data.length ? data.map((f, i) => (
          <FlowCard
            key={i}
            title={f.name}
            chips={[f.store || '', f.entity || '']}
            steps={f.steps}
            note={f.note}
            concern={f.concern}
          />
        )) : <div className="fl-none">No data flows identified.</div>}
      </FlowSection>

      <FlowSection icon={<GitBranch size={14} />} title="Product flows" count={prod.length}>
        {prod.length ? prod.map((f, i) => (
          <FlowCard
            key={i}
            title={f.name}
            chips={[f.trigger || '']}
            steps={f.steps}
            outcome={f.outcome}
            concern={f.concern}
          />
        )) : <div className="fl-none">No product flows identified.</div>}
      </FlowSection>

      <div className="fl-refresh">
        <button className="btn-sm" disabled={busy || r.pending} onClick={start}>
          <RefreshCw size={12} /> {r.pending ? 'Re-mapping…' : 'Re-identify flows'}
        </button>
      </div>
    </div>
  )
}
