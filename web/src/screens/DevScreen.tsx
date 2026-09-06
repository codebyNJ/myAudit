import { useEffect, useRef, useState } from 'react'
import {
  X, Check, Copy, FileCode, ArrowLeft, ArrowRight, CheckCheck,
  WrapText, Map as MapIcon, Search,
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
      fontSize: 9.5, fontFamily: 'var(--font-mono)', fontWeight: 700, color,
      border: `1px solid ${color}40`, background: `${color}14`,
      padding: '1px 4px', borderRadius: 3, lineHeight: 1, flexShrink: 0,
    }}>{label}</span>
  )
}

function ChangedList({ files, onOpen }: { files: { path: string; review?: string }[]; onOpen: (p: string) => void }) {
  return (
    <div className="cl-wrap">
      <div className="cl-head">Changed files <span className="cl-n">{files.length}</span></div>
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
  const sel = files.find((f) => f.path === s.file) || null

  const [content, setContent] = useState('')
  const [saved, setSaved] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [diff, setDiff] = useState('')
  const [showDiff, setShowDiff] = useState(false) // code-first, like VS Code
  const [wrap, setWrap] = useState(true)
  const [minimap, setMinimap] = useState(false)
  const [copied, setCopied] = useState(false)
  const [findTrigger, setFindTrigger] = useState(0)
  const [cursor, setCursor] = useState({ line: 1, col: 1 })
  const loadGen = useRef(0)

  const dirty = content !== saved
  const changedFiles = files.filter((f) => f.changed && !f.path.startsWith('.myaudit/'))
  const changedIdx = changedFiles.findIndex((f) => f.path === sel?.path)
  const detectedLang = sel ? detectLanguage(sel.path) : 'plaintext'
  const lineCount = content ? content.split('\n').length : 0

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

  // Load file when selection changes. Always editable — no Edit-mode gate.
  useEffect(() => {
    setShowDiff(false)
    if (!sel || !s.runId) { setContent(''); setSaved(''); return }
    const gen = ++loadGen.current
    setLoading(true)
    api.fileContent(s.runId, sel.path)
      .then((r) => {
        if (loadGen.current !== gen) return
        setContent(r.content)
        setSaved(r.content)
      })
      .catch(() => {
        if (loadGen.current !== gen) return
        setContent('// Could not load file')
        setSaved('// Could not load file')
      })
      .finally(() => { if (loadGen.current === gen) setLoading(false) })
  }, [sel?.path, s.runId])

  // Soft-refresh from disk when run detail updates, but never clobber dirty edits.
  useEffect(() => {
    if (!sel || !s.runId || dirty) return
    let alive = true
    api.fileContent(s.runId, sel.path)
      .then((r) => {
        if (!alive) return
        setContent((prev) => (prev === r.content ? prev : r.content))
        setSaved(r.content)
      })
      .catch(() => {})
    return () => { alive = false }
  }, [s.detail, sel?.path, s.runId, dirty])

  useEffect(() => {
    if (!sel || !s.runId || !sel.changed) { setDiff(''); return }
    let alive = true
    api.diff(s.runId, sel.path).then((r) => { if (alive) setDiff(r.diff) }).catch(() => {})
    return () => { alive = false }
  }, [s.detail, sel?.path, s.runId, sel?.changed])

  const saveEdit = async () => {
    if (!sel || !s.runId || !dirty) return
    setSaving(true)
    try {
      await api.saveFile(s.runId, sel.path, content)
      setSaved(content)
      s.toast('success', 'Saved', sel.path)
      s.reloadDetail()
    } catch (e) { s.toast('error', 'Save failed', (e as Error).message) }
    finally { setSaving(false) }
  }

  const review = async (status: 'accepted' | 'rejected') => {
    if (!sel || !s.runId) return
    if (dirty) {
      s.toast('info', 'Save first', 'File has unsaved changes')
      return
    }
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
    navigator.clipboard?.writeText(content)
    s.toast('info', 'Copied', sel?.path)
  }

  const breadcrumbParts = sel?.path ? sel.path.split('/') : []

  // Global ⌘S when Code tab is focused (Monaco also binds it when focused).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
        e.preventDefault()
        void saveEdit()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  return (
    <div className="cursor-ide">
      {s.openFiles.length > 0 && (
        <div className="cursor-tabs-bar">
          {s.openFiles.map((p) => {
            const entry = files.find((f) => f.path === p)
            const isChanged = entry?.changed
            const isCurrent = p === s.file
            const isDirty = isCurrent && dirty
            return (
              <div
                key={p}
                className={`cursor-tab ${isCurrent ? 'on' : ''}`}
                onClick={() => s.setFile(p)}
                title={p}
              >
                <FileTypeBadge path={p} />
                <span className="cursor-tab-name">{baseName(p)}</span>
                {(isDirty || isChanged) && (
                  <span className="cursor-tab-chg" title={isDirty ? 'Unsaved' : 'Modified by audit'}
                    style={isDirty ? { background: '#e0a92e' } : undefined} />
                )}
                <span
                  className="cursor-tab-close"
                  title="Close tab"
                  onClick={(e) => {
                    e.stopPropagation()
                    if (isCurrent && dirty && !confirm('Close without saving?')) return
                    s.closeFile(p)
                  }}
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
              <button className="icon-btn" style={{ padding: 2, marginLeft: 4 }} title="Copy path" onClick={copyPath}>
                {copied ? <Check size={12} color="#4ade80" /> : <Copy size={12} />}
              </button>
              {dirty && (
                <span style={{
                  fontSize: 10.5, fontFamily: 'var(--font-mono)', padding: '1px 6px', borderRadius: 4,
                  background: 'rgba(224,169,46,0.15)', color: '#e0a92e', border: '1px solid rgba(224,169,46,0.35)', marginLeft: 4,
                }}>UNSAVED</span>
              )}
              {sel.changed && !dirty && (
                <span style={{
                  fontSize: 10.5, fontFamily: 'var(--font-mono)', padding: '1px 6px', borderRadius: 4,
                  background: 'rgba(74,222,128,0.12)', color: '#4ade80', border: '1px solid rgba(74,222,128,0.3)', marginLeft: 4,
                }}>MODIFIED</span>
              )}
            </>
          ) : (
            <span className="cursor-crumb active">
              {changedFiles.length} changed file{changedFiles.length === 1 ? '' : 's'} to review
            </span>
          )}
        </div>

        <div className="cursor-actions">
          {changedFiles.length > 1 && changedIdx >= 0 && (
            <div style={{ display: 'inline-flex', alignItems: 'center', gap: 4, marginRight: 6 }}>
              <button className="btn-sm" style={{ padding: '3px 7px' }} onClick={() => stepChanged(-1)}><ArrowLeft size={12} /></button>
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>
                {changedIdx + 1}/{changedFiles.length}
              </span>
              <button className="btn-sm" style={{ padding: '3px 7px' }} onClick={() => stepChanged(1)}><ArrowRight size={12} /></button>
            </div>
          )}

          {sel && (
            <>
              <button className="btn-sm" title="Copy file contents" onClick={copyCode}><Copy size={12} /></button>
              <button className={`btn-sm ${wrap ? 'primary' : ''}`} title="Toggle word wrap" onClick={() => setWrap((v) => !v)}>
                <WrapText size={12} />
              </button>
              <button className={`btn-sm ${minimap ? 'primary' : ''}`} title="Toggle minimap" onClick={() => setMinimap((v) => !v)}>
                <MapIcon size={12} />
              </button>
              <button className="btn-sm" title="Find in file (⌘F)" onClick={() => setFindTrigger((n) => n + 1)}>
                <Search size={12} />
              </button>
              {sel.changed && (
                <div className="viewseg">
                  <button className={`viewseg-b ${!showDiff ? 'on' : ''}`} onClick={() => setShowDiff(false)}>Code</button>
                  <button className={`viewseg-b ${showDiff ? 'on' : ''}`} onClick={() => setShowDiff(true)}>Diff</button>
                </div>
              )}
              <button className="btn-sm primary" disabled={!dirty || saving} onClick={saveEdit} title="Save (⌘S)">
                {saving ? 'Saving…' : 'Save'}
              </button>
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
          showDiff && sel.changed ? (
            <div className="diff-pane"><Diff text={diff} /></div>
          ) : loading ? (
            <div className="empty-mid" style={{ position: 'static', paddingTop: 80 }}><div className="spin" /></div>
          ) : (
            <Mono
              path={sel.path}
              value={content}
              wordWrap={wrap}
              minimap={minimap}
              onChange={setContent}
              onCursor={(line, col) => setCursor({ line, col })}
              onSave={saveEdit}
              findTrigger={findTrigger}
            />
          )
        ) : changedFiles.length > 0 ? (
          <ChangedList files={changedFiles} onOpen={(p) => s.setFile(p)} />
        ) : (
          <div className="empty-mid" style={{ position: 'static', paddingTop: 100 }}>
            <FileCode size={40} strokeWidth={1.5} color="var(--text-muted)" />
            <h3>{s.runId ? 'Open a file to edit' : 'No audit workspace loaded'}</h3>
            <p>
              {s.runId
                ? 'Pick a file in the Explorer. Edit freely — ⌘S / Ctrl+S saves. ⌘F finds in the file.'
                : 'Import a codebase to examine.'}
            </p>
          </div>
        )}
      </div>

      <div className="cursor-statusbar">
        <div className="cursor-status-item">{sel ? sel.path : ''}</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          {sel && <div className="cursor-status-item">Ln {cursor.line}, Col {cursor.col}</div>}
          {sel && <div className="cursor-status-item">{lineCount} lines</div>}
          {sel && <div className="cursor-status-item" style={{ textTransform: 'capitalize' }}>{detectedLang}</div>}
          {sel && <div className="cursor-status-item">{wrap ? 'Wrap' : 'No Wrap'}</div>}
          {dirty && <div className="cursor-status-item" style={{ color: '#e0a92e' }}>● Unsaved</div>}
        </div>
      </div>
    </div>
  )
}
