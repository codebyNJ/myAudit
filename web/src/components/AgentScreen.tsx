import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { Maximize2, Minimize2, Radio, Camera, MonitorPlay, ExternalLink, RotateCw, Terminal, AlertTriangle } from 'lucide-react'
import { useStore } from '../store'
import { api, type LiveView } from '../api'
import { viewportWidth, withPath, type ViewportSize } from './util'

const tauriOpener = (): { openUrl: (url: string) => Promise<void> } | undefined =>
  (window as unknown as { __TAURI__?: { opener?: { openUrl: (url: string) => Promise<void> } } }).__TAURI__?.opener

function openExternal(url: string) {
  const opener = tauriOpener()
  if (opener) { opener.openUrl(url).catch(() => {}); return }
  window.open(url, '_blank', 'noopener,noreferrer')
}

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

function Idle({ kind }: { kind?: LiveView['kind'] }) {
  const msg = kind === 'none'
    ? 'No UI in this project — agent screen stays idle for CLI/library codebases.'
    : 'Waiting for the app to start…'
  return (
    <div className="as-idle">
      <MonitorPlay size={28} strokeWidth={1.25} color="var(--text-dim)" />
      <span>{msg}</span>
    </div>
  )
}

function Crashed({ reason, onRestart }: { reason?: string; onRestart: () => void }) {
  return (
    <div className="as-idle">
      <AlertTriangle size={28} strokeWidth={1.25} color="var(--diff-del-text)" />
      <span>Dev server crashed{reason ? `: ${reason}` : ''}</span>
      <button className="btn-sm primary" onClick={onRestart}><RotateCw size={13} /> Restart</button>
    </div>
  )
}

function Surface({ view, runId, expanded, viewport, onRestart }: {
  view: LiveView; runId: string; expanded: boolean; viewport: ViewportSize; onRestart: () => void
}) {
  if (view.status === 'live' && view.url) {
    const width = viewportWidth(viewport)
    return (
      <div className={width ? 'as-viewport-frame' : undefined} style={width ? { maxWidth: width, margin: '0 auto', height: '100%' } : undefined}>
        <iframe
          className="as-frame"
          src={view.url}
          title="Application under test"
          sandbox="allow-scripts allow-same-origin allow-forms"
          scrolling={expanded ? 'yes' : 'no'}
        />
      </div>
    )
  }
  if (view.status === 'frames' && view.frame) {
    return <img className="as-shot" src={api.rawUrl(runId, view.frame)} alt="Latest captured frame" />
  }
  if (view.status === 'crashed') {
    return <Crashed reason={view.reason} onRestart={onRestart} />
  }
  if (view.status === 'idle' && view.kind === 'none') {
    return <Idle kind={view.kind} />
  }
  return <Connecting label="Starting the app under test…" />
}

function ServerLog({ runId }: { runId: string }) {
  const [log, setLog] = useState('')

  useEffect(() => {
    let alive = true
    const load = () => api.previewLog(runId).then((v) => { if (alive) setLog(v) }).catch(() => {})
    load()
    const h = setInterval(load, 3000)
    return () => { alive = false; clearInterval(h) }
  }, [runId])

  return <pre className="as-log">{log || 'No server log yet.'}</pre>
}

export function AgentScreen() {
  const s = useStore()
  const [view, setView] = useState<LiveView | null>(null)
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState<'preview' | 'log'>('preview')
  const [viewport, setViewport] = useState<ViewportSize>('desktop')
  const [path, setPath] = useState('')
  const [restarting, setRestarting] = useState(false)

  useEffect(() => {
    if (!s.runId) { setView(null); return }
    let alive = true
    const load = () => api.live(s.runId!).then((v) => { if (alive) setView(v) }).catch(() => {})
    load()
    const h = setInterval(load, 2500)
    return () => { alive = false; clearInterval(h) }
  }, [s.runId])

  useEffect(() => { setPath('') }, [s.runId, view?.url])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open])

  if (!s.runId || !view) return null

  const restart = async () => {
    if (!s.runId || restarting) return
    setRestarting(true)
    try { await api.previewRestart(s.runId) } catch { /* next poll surfaces the outcome */ }
    setTimeout(() => setRestarting(false), 2000)
  }

  const isLive = view.status === 'live'
  const isIdleNoUI = view.status === 'idle' && view.kind === 'none'
  const badge = isLive
    ? <span className="as-badge live"><span className="as-dot" /> LIVE{typeof view.latency_ms === 'number' ? ` · ${view.latency_ms}ms` : ''}</span>
    : view.status === 'frames'
      ? <span className="as-badge"><Camera size={11} /> {fmtAgo(view.At)}</span>
      : view.status === 'crashed'
        ? <span className="as-badge crashed">crashed</span>
        : isIdleNoUI
          ? <span className="as-badge">idle</span>
          : <span className="as-badge">booting</span>

  const displayUrl = view.url ? withPath(view.url, path) : undefined

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
        <Surface view={view} runId={s.runId} expanded={false} viewport="desktop" onRestart={restart} />
        <div className="as-hover"><span className="btn-sm primary as-openpill"><Maximize2 size={13} /> Open</span></div>
      </div>

      <div className="as-meta">
        {isLive
          ? <><Radio size={12} /> <span>{view.Title || 'Application under test'}</span>
              <a className="as-link" href={view.url} target="_blank" rel="noreferrer" onClick={(e) => { e.stopPropagation(); e.preventDefault(); openExternal(view.url!) }}>
                {view.url} <ExternalLink size={11} />
              </a></>
          : view.status === 'frames'
            ? <span>Latest capture from the QA agent</span>
            : view.status === 'crashed'
              ? <span>{view.reason || 'Dev server crashed'}</span>
              : isIdleNoUI
                ? <span>No web or desktop UI detected — nothing to run</span>
                : <span>Booting this project’s dev server</span>}
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
                <button className="icon-btn" title="Restart" onClick={restart} disabled={restarting}><RotateCw size={14} className={restarting ? 'spin' : undefined} /></button>
                {isLive && view.url && (
                  <a className="btn-sm" href={view.url} target="_blank" rel="noreferrer" onClick={(e) => { e.preventDefault(); openExternal(view.url!) }}>Open in browser <ExternalLink size={12} /></a>
                )}
                <button className="icon-btn" title="Collapse" onClick={() => setOpen(false)}><Minimize2 size={15} /></button>
              </div>
            </div>

            <div className="as-tabs">
              <button className={tab === 'preview' ? 'as-tab active' : 'as-tab'} onClick={() => setTab('preview')}><MonitorPlay size={13} /> Preview</button>
              <button className={tab === 'log' ? 'as-tab active' : 'as-tab'} onClick={() => setTab('log')}><Terminal size={13} /> Server Log</button>
            </div>

            {tab === 'preview' && isLive && (
              <div className="as-toolbar">
                <select value={viewport} onChange={(e) => setViewport(e.target.value as ViewportSize)}>
                  <option value="desktop">Desktop</option>
                  <option value="tablet">Tablet (768px)</option>
                  <option value="mobile">Mobile (390px)</option>
                </select>
                <input
                  className="as-path-input"
                  placeholder="/path"
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') setPath(path) }}
                />
              </div>
            )}

            <div className="as-viewport">
              {tab === 'preview'
                ? <Surface view={displayUrl ? { ...view, url: displayUrl } : view} runId={s.runId} expanded viewport={viewport} onRestart={restart} />
                : <ServerLog runId={s.runId} />}
            </div>
          </div>
        </div>,
        document.body,
      )}
    </div>
  )
}
