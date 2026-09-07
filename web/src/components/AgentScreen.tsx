import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { Maximize2, Minimize2, Radio, Camera, MonitorPlay, ExternalLink } from 'lucide-react'
import { useStore } from '../store'
import { api, type LiveView } from '../api'

function fmtAgo(iso?: string) {
  if (!iso) return ''
  const d = Math.max(0, Date.now() - new Date(iso).getTime())
  const s = Math.floor(d / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  return m < 60 ? `${m}m ago` : `${Math.floor(m / 60)}h ago`
}

function Connecting({ label }: { label: string }) {
  return (
    <div className="as-connecting">
      <div className="spin" />
      <span>{label}</span>
    </div>
  )
}

function Surface({ view, runId, expanded }: { view: LiveView; runId: string; expanded: boolean }) {
  if (view.status === 'live' && view.url) {
    return (
      <iframe
        className="as-frame"
        src={view.url}
        title="Application under test"
        sandbox="allow-scripts allow-same-origin allow-forms"
        scrolling={expanded ? 'yes' : 'no'}
      />
    )
  }
  if (view.status === 'frames' && view.frame) {
    return <img className="as-shot" src={api.rawUrl(runId, view.frame)} alt="Latest captured frame" />
  }
  return <Connecting label="Starting the app under test…" />
}

export function AgentScreen() {
  const s = useStore()
  const [view, setView] = useState<LiveView | null>(null)
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (!s.runId) { setView(null); return }
    let alive = true
    const load = () => api.live(s.runId!).then((v) => { if (alive) setView(v) }).catch(() => {})
    load()
    const h = setInterval(load, 2500)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])

  if (!s.runId || !view) return null

  const isLive = view.status === 'live'
  const badge = isLive
    ? <span className="as-badge live"><span className="as-dot" /> LIVE</span>
    : view.status === 'frames'
      ? <span className="as-badge"><Camera size={11} /> {fmtAgo(view.At)}</span>
      : <span className="as-badge">idle</span>

  return (
    <div className="vercel-card as-card">
      <div className="vercel-card-head">
        <div className="vercel-card-title"><MonitorPlay size={14} /> Agent screen</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {badge}
          <button className="icon-btn" title="Open viewer" onClick={() => setOpen(true)}><Maximize2 size={14} /></button>
        </div>
      </div>

      <div className="as-stage" onClick={() => setOpen(true)} title="Open viewer">
        <Surface view={view} runId={s.runId} expanded={false} />
        <div className="as-hover"><span className="btn-sm primary as-openpill"><Maximize2 size={13} /> Open</span></div>
      </div>

      <div className="as-meta">
        {isLive
          ? <><Radio size={12} /> <span>{view.Title || 'Application under test'}</span>
              <a className="as-link" href={view.url} target="_blank" rel="noreferrer" onClick={(e) => e.stopPropagation()}>
                {view.url} <ExternalLink size={11} />
              </a></>
          : <span>{view.status === 'frames' ? 'Latest capture from the QA agent' : 'Booting this project’s dev server'}</span>}
      </div>

      {open && createPortal(
        <div className="as-portal" role="dialog" aria-modal="true" aria-label="Agent screen">
          <div className="as-scrim" onClick={() => setOpen(false)} />
          <div className="as-window">
            <div className="as-titlebar">
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
                <span className="as-title">Agent screen</span>
                {badge}
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                {isLive && view.url && (
                  <a className="btn-sm" href={view.url} target="_blank" rel="noreferrer">Open in browser <ExternalLink size={12} /></a>
                )}
                <button className="icon-btn" title="Collapse" onClick={() => setOpen(false)}><Minimize2 size={15} /></button>
              </div>
            </div>
            <div className="as-viewport">
              <Surface view={view} runId={s.runId} expanded />
            </div>
          </div>
        </div>,
        document.body,
      )}
    </div>
  )
}
