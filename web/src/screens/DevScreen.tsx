import { useEffect, useRef, useState } from 'react'
import { 
  X, Check, Copy, Sparkles, 
  FileCode, ArrowLeft, ArrowRight, CheckCheck, Terminal
} from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'
import { Diff } from '../components/Diff'
import { Mono, detectLanguage } from '../components/Monaco'

const baseName = (p: string) => p.split('/').pop() || p
const dirName = (p: string) => { 
  const i = p.lastIndexOf('/')
  return i < 0 ? '' : p.slice(0, i + 1) 
}

function FileTypeBadge({ path }: { path: string }) {
  const ext = path.toLowerCase().split('.').pop() || ''
  let color = '#a1a1aa'
  let label = ext.toUpperCase().slice(0, 3) || 'TXT'

  if (ext === 'ts' || ext === 'tsx') { color = '#38bdf8'; label = ext.toUpperCase() }
  else if (ext === 'js' || ext === 'jsx') { color = '#facc15'; label = ext.toUpperCase() }
  else if (ext === 'py') { color = '#60a5fa'; label = 'PY' }
  else if (ext === 'go') { color = '#22d3ee'; label = 'GO' }
  else if (ext === 'rs') { color = '#fb923c'; label = 'RS' }
  else if (ext === 'json') { color = '#fbbf24'; label = '{ }' }
  else if (ext === 'md') { color = '#c084fc'; label = 'MD' }
  else if (ext === 'css' || ext === 'scss') { color = '#f472b6'; label = 'CSS' }
  else if (ext === 'html') { color = '#f97316'; label = '<>' }
  else if (ext === 'sh' || ext === 'bash') { color = '#4ade80'; label = 'SH' }
  else if (ext === 'sql') { color = '#818cf8'; label = 'SQL' }
  else if (path.toLowerCase().includes('dockerfile')) { color = '#38bdf8'; label = 'DOC' }

  return (
    <span style={{ 
      fontSize: 9.5, 
      fontFamily: 'var(--font-mono)', 
      fontWeight: 700, 
      color, 
      border: `1px solid ${color}40`, 
      background: `${color}14`,
      padding: '1px 4px', 
      borderRadius: 3, 
      lineHeight: 1, 
      flexShrink: 0 
    }}>
      {label}
    </span>
  )
}

function ChangedList({ files, onOpen }: { files: { path: string; review?: string; action?: string }[]; onOpen: (p: string) => void }) {
  return (
    <div className="cl-wrap">
      <div className="cl-head">
        Changed files <span className="cl-n">{files.length}</span>
      </div>
      <div className="cl-list">
        {files.map((f) => (
          <div className="cl-row" key={f.path} onClick={() => onOpen(f.path)} title={f.path}>
            <FileTypeBadge path={f.path} />
            <span className="cl-path">
              <span className="cl-dir">{dirName(f.path)}</span>
              <span className="cl-base">{baseName(f.path)}</span>
            </span>
            {f.review === 'accepted' && <span className="cl-rev ok">✓ accepted</span>}
            {f.review === 'rejected' && <span className="cl-rev no">rejected</span>}
          </div>
        ))}
      </div>
    </div>
  )
}

export function DevScreen() {
  const s = useStore()
  const files = s.visibleFiles
  const sel = files.find((f) => f.path === s.file) || files[0]
  const [content, setContent] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)
  const [diff, setDiff] = useState('')
  const [showDiff, setShowDiff] = useState(true)
  const [copied, setCopied] = useState(false)

  const changedFiles = files.filter((f) => f.changed && !f.path.startsWith('.myaudit/'))
  const changedIdx = changedFiles.findIndex((f) => f.path === sel?.path)

  const stepChanged = (dir: 1 | -1) => {
    if (!changedFiles.length) return
    const base = changedIdx < 0 ? (dir === 1 ? -1 : 0) : changedIdx
    const next = changedFiles[(base + dir + changedFiles.length) % changedFiles.length]
    if (next) s.setFile(next.path)
  }

  const seen = useRef<Set<string>>(new Set())
  const changedSeen = useRef<Set<string>>(new Set())
  const primed = useRef(false)
  const changedKey = files.filter((f) => f.changed).map((f) => f.path).join(',')

  useEffect(() => {
    if (!s.runId) { seen.current = new Set(); changedSeen.current = new Set(); primed.current = false; return }
    if (!primed.current) {
      files.forEach((f) => { seen.current.add(f.path); if (f.changed) changedSeen.current.add(f.path) })
      primed.current = true
      return
    }
    const freshChanged = files.filter((f) => f.changed && !changedSeen.current.has(f.path)).map((f) => f.path)
    const freshNew = files.filter((f) => !seen.current.has(f.path) && !f.path.startsWith('.myaudit/')).map((f) => f.path)
    files.forEach((f) => { seen.current.add(f.path); if (f.changed) changedSeen.current.add(f.path) })
    if (freshChanged.length) s.setFile(freshChanged[freshChanged.length - 1])
    else if (freshNew.length) s.setFile(freshNew[freshNew.length - 1])
  }, [files.map((f) => f.path).join(',') + '|' + changedKey])

  useEffect(() => {
    setEditing(false)
    if (!sel || !s.runId) { setContent(null); return }
    let alive = true
    setLoading(true)
    api.fileContent(s.runId, sel.path)
      .then((r) => { if (alive) setContent(r.content) })
      .catch(() => { if (alive) setContent('Could not load file') })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [sel?.path, s.runId])

  useEffect(() => {
    if (!sel || !s.runId || editing) return
    let alive = true
    api.fileContent(s.runId, sel.path)
      .then((r) => { if (alive) setContent((prev) => (prev === r.content ? prev : r.content)) })
      .catch(() => {})
    return () => { alive = false }
  }, [s.detail, sel?.path, s.runId, editing])

  useEffect(() => {
    if (!sel || !s.runId || !sel.changed) { setDiff(''); return }
    let alive = true
    api.diff(s.runId, sel.path).then((r) => { if (alive) setDiff(r.diff) }).catch(() => {})
    return () => { alive = false }
  }, [s.detail, sel?.path, s.runId, sel?.changed])

  const startEdit = () => { setDraft(content ?? ''); setEditing(true) }
  const saveEdit = async () => {
    if (!sel || !s.runId) return
    setSaving(true)
    try {
      await api.saveFile(s.runId, sel.path, draft)
      setContent(draft)
      setEditing(false)
      s.toast('success', 'Saved', sel.path)
    } catch (e) { s.toast('error', 'Save failed', (e as Error).message) }
    finally { setSaving(false) }
  }

  const review = async (status: 'accepted' | 'rejected') => {
    if (!sel || !s.runId) return
    try {
      await api.review(s.runId, sel.path, status)
      s.toast(status === 'accepted' ? 'success' : 'info', status === 'accepted' ? 'File accepted' : 'File rejected', sel.path)
      if (status === 'rejected') s.setFile('')
      s.reloadDetail()
    } catch (e) { s.toast('error', 'Review failed', (e as Error).message) }
  }

  const copyPath = () => {
    if (!sel) return
    navigator.clipboard?.writeText(sel.path)
    setCopied(true)
    setTimeout(() => setCopied(false), 1800)
  }

  const copyCode = () => {
    if (content == null) return
    navigator.clipboard?.writeText(content)
    s.toast('info', 'Copied to clipboard', sel?.path)
  }

  const breadcrumbParts = sel?.path ? sel.path.split('/') : []
  const detectedLang = sel ? detectLanguage(sel.path) : 'plaintext'
  const lineCount = content ? content.split('\n').length : 0

  return (
    <div className="cursor-ide">
      {s.openFiles.length > 0 && (
        <div className="cursor-tabs-bar">
          {s.openFiles.map((p) => {
            const entry = files.find((f) => f.path === p)
            const isChanged = entry?.changed
            const isCurrent = p === s.file
            return (
              <div 
                key={p} 
                className={`cursor-tab ${isCurrent ? 'on' : ''}`}
                onClick={() => s.setFile(p)} 
                title={p}
              >
                <FileTypeBadge path={p} />
                <span className="cursor-tab-name">{baseName(p)}</span>
                {isChanged && <span className="cursor-tab-chg" title="Modified by audit" />}
                <span 
                  className="cursor-tab-close" 
                  title="Close tab" 
                  onClick={(e) => { e.stopPropagation(); s.closeFile(p) }}
                >
                  <X size={11} />
                </span>
              </div>
            )
          })}
        </div>
      )}

      <div className="cursor-subbar">
        <div className="cursor-breadcrumbs">
          {sel ? (
            <>
              {breadcrumbParts.map((part, idx) => {
                const isLast = idx === breadcrumbParts.length - 1
                return (
                  <span key={idx} style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                    {idx > 0 && <span className="cursor-crumb-sep">/</span>}
                    <span className={`cursor-crumb ${isLast ? 'active' : ''}`}>{part}</span>
                  </span>
                )
              })}
              <button 
                className="icon-btn" 
                style={{ padding: 2, marginLeft: 4 }} 
                title="Copy relative path" 
                onClick={copyPath}
              >
                {copied ? <Check size={12} color="#4ade80" /> : <Copy size={12} />}
              </button>
              {sel.changed && (
                <span style={{ 
                  fontSize: 10.5, 
                  fontFamily: 'var(--font-mono)', 
                  padding: '1px 6px', 
                  borderRadius: 4, 
                  background: 'rgba(74,222,128,0.12)', 
                  color: '#4ade80', 
                  border: '1px solid rgba(74,222,128,0.3)',
                  marginLeft: 4
                }}>
                  MODIFIED
                </span>
              )}
              {sel.review === 'accepted' && (
                <span style={{ 
                  fontSize: 10.5, 
                  fontFamily: 'var(--font-mono)', 
                  padding: '1px 6px', 
                  borderRadius: 4, 
                  background: 'rgba(63,185,80,0.12)', 
                  color: '#3fb950', 
                  border: '1px solid rgba(63,185,80,0.3)',
                  marginLeft: 4
                }}>
                  ✓ ACCEPTED
                </span>
              )}
            </>
          ) : (
            <span className="cursor-crumb active">
              {changedFiles.length} changed file{changedFiles.length === 1 ? '' : 's'} to review
            </span>
          )}
        </div>

        <div className="cursor-actions">
          {changedFiles.length > 1 && !editing && (
            <div style={{ display: 'inline-flex', alignItems: 'center', gap: 4, marginRight: 6 }}>
              <button className="btn-sm" style={{ padding: '3px 7px' }} onClick={() => stepChanged(-1)}>
                <ArrowLeft size={12} />
              </button>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>
                {changedIdx >= 0 ? changedIdx + 1 : '–'}/{changedFiles.length}
              </span>
              <button className="btn-sm" style={{ padding: '3px 7px' }} onClick={() => stepChanged(1)}>
                <ArrowRight size={12} />
              </button>
            </div>
          )}

          <div 
            className="cursor-ai-pill" 
            style={{ cursor: 'pointer' }}
            onClick={() => s.setChatOpen(true)}
            title="Open Claude Code Assistant with file context"
          >
            <Sparkles size={11} /> ⌘L Cursor AI
          </div>

          {sel && !editing && (
            <button className="btn-sm" onClick={copyCode} title="Copy code content">
              <Copy size={12} /> Copy
            </button>
          )}

          {sel && editing && (
            <>
              <button className="btn-sm" onClick={() => setEditing(false)}>Cancel</button>
              <button className="btn-sm primary" disabled={saving} onClick={saveEdit}>
                {saving ? 'Saving…' : 'Save'}
              </button>
            </>
          )}

          {sel && !editing && (
            <>
              {sel.changed && (
                <div className="viewseg">
                  <button className={`viewseg-b ${showDiff ? 'on' : ''}`} onClick={() => setShowDiff(true)}>
                    Diff
                  </button>
                  <button className={`viewseg-b ${!showDiff ? 'on' : ''}`} onClick={() => setShowDiff(false)}>
                    Code
                  </button>
                </div>
              )}
              <button className="btn-sm" onClick={startEdit}>Edit</button>
              {sel.changed && sel.review !== 'accepted' && (
                <>
                  <button className="btn-sm" onClick={() => review('rejected')}>Reject</button>
                  <button className="btn-sm primary" onClick={() => review('accepted')}>
                    <CheckCheck size={13} /> Accept
                  </button>
                </>
              )}
            </>
          )}
        </div>
      </div>

      <div style={{ flex: 1, minHeight: 0, position: 'relative', overflow: 'hidden' }}>
        {sel ? (
          editing ? (
            <Mono path={sel.path} value={draft} readOnly={false} onChange={setDraft} />
          ) : sel.changed && showDiff ? (
            <Diff text={diff} />
          ) : (
            <Mono path={sel.path} value={loading ? '' : (content ?? '')} readOnly />
          )
        ) : changedFiles.length > 0 ? (
          <ChangedList files={changedFiles} onOpen={(p) => s.setFile(p)} />
        ) : (
          <div className="empty-mid" style={{ position: 'static', paddingTop: 100 }}>
            <FileCode size={40} strokeWidth={1.5} color="var(--text-muted)" />
            <h3>{s.runId ? 'No code changes recorded' : 'No audit workspace loaded'}</h3>
            <p>
              {s.runId 
                ? 'Select a source file in the left Explorer tree to inspect and edit with Cursor IDE tools.' 
                : 'Import a codebase to examine.'}
            </p>
          </div>
        )}
      </div>

      <div className="cursor-statusbar">
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div className="cursor-status-item">
            <Terminal size={11} /> main
          </div>
          {sel && (
            <div className="cursor-status-item">
              {lineCount} lines
            </div>
          )}
          <div className="cursor-status-item">
            UTF-8
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div className="cursor-status-item">
            Spaces: 2
          </div>
          <div className="cursor-status-item" style={{ textTransform: 'capitalize' }}>
            {detectedLang}
          </div>
          <div className="cursor-status-item" style={{ color: '#c084fc' }}>
            ✦ Cursor AI Ready
          </div>
        </div>
      </div>
    </div>
  )
}
