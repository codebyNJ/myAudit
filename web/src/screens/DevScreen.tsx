import { useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'
import { IcFile } from '../components/icons'
import { Diff } from '../components/Diff'
import { Mono } from '../components/Monaco'

const baseName = (p: string) => p.split('/').pop() || p
const dirName = (p: string) => { const i = p.lastIndexOf('/'); return i < 0 ? '' : p.slice(0, i + 1) }

// PR-style "Files changed" list — the Code landing before you open a file.
function ChangedList({ files, onOpen }: { files: { path: string; review?: string; action?: string }[]; onOpen: (p: string) => void }) {
  return (
    <div className="cl-wrap">
      <div className="cl-head">Changed files <span className="cl-n">{files.length}</span></div>
      <div className="cl-list">
        {files.map((f) => (
          <div className="cl-row" key={f.path} onClick={() => onOpen(f.path)} title={f.path}>
            <span className={`cl-badge ${f.action === 'created' ? 'add' : 'mod'}`}>{f.action === 'created' ? 'A' : 'M'}</span>
            <span className="cl-path"><span className="cl-dir">{dirName(f.path)}</span><span className="cl-base">{baseName(f.path)}</span></span>
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
  const [showDiff, setShowDiff] = useState(true) // changed files default to the diff view

  // Step through changed files without going back to the explorer (PR-style
  // review). Exclude myAudit's own artifacts (preview screenshots) — not code.
  const changedFiles = files.filter((f) => f.changed && !f.path.startsWith('.myaudit/'))
  const changedIdx = changedFiles.findIndex((f) => f.path === sel?.path)
  const stepChanged = (dir: 1 | -1) => {
    if (!changedFiles.length) return
    const base = changedIdx < 0 ? (dir === 1 ? -1 : 0) : changedIdx
    const next = changedFiles[(base + dir + changedFiles.length) % changedFiles.length]
    if (next) s.setFile(next.path)
  }

  // Follow the work live: reveal a file the moment it's created OR changed by a
  // fix, so the editor jumps to whatever the agent just touched. Changed files
  // win (that's the active fix); new files are the fallback.
  const seen = useRef<Set<string>>(new Set())
  const changedSeen = useRef<Set<string>>(new Set())
  const primed = useRef(false)
  const changedKey = files.filter((f) => f.changed).map((f) => f.path).join(',')
  useEffect(() => {
    if (!s.runId) { seen.current = new Set(); changedSeen.current = new Set(); primed.current = false; return }
    // On first render, seed the "seen" sets WITHOUT revealing anything, so arriving
    // with a file already chosen (e.g. jumped from a finding) isn't overridden.
    if (!primed.current) {
      files.forEach((f) => { seen.current.add(f.path); if (f.changed) changedSeen.current.add(f.path) })
      primed.current = true
      // Don't force a file open — with nothing selected the Code tab shows the
      // changed-files LIST first ("list before code").
      return
    }
    // During a live run, still auto-reveal a file the moment it's fixed/created,
    // so you watch the work land. Changed wins; new files are the fallback.
    const freshChanged = files.filter((f) => f.changed && !changedSeen.current.has(f.path)).map((f) => f.path)
    const freshNew = files.filter((f) => !seen.current.has(f.path) && !f.path.startsWith('.myaudit/')).map((f) => f.path)
    files.forEach((f) => { seen.current.add(f.path); if (f.changed) changedSeen.current.add(f.path) })
    if (freshChanged.length) s.setFile(freshChanged[freshChanged.length - 1])
    else if (freshNew.length) s.setFile(freshNew[freshNew.length - 1])
  }, [files.map((f) => f.path).join(',') + '|' + changedKey]) // eslint-disable-line react-hooks/exhaustive-deps

  // Content is loaded lazily per file (the tree only carries paths).
  useEffect(() => {
    setEditing(false)
    if (!sel || !s.runId) { setContent(null); return }
    let alive = true
    setLoading(true)
    api.fileContent(s.runId, sel.path)
      .then((r) => { if (alive) setContent(r.content) })
      .catch(() => { if (alive) setContent('// could not load file') })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [sel?.path, s.runId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Live-refresh the OPEN file's content on each poll (unless you're editing),
  // so you watch code update in place as fixes land — not just on file switch.
  useEffect(() => {
    if (!sel || !s.runId || editing) return
    let alive = true
    api.fileContent(s.runId, sel.path)
      .then((r) => { if (alive) setContent((prev) => (prev === r.content ? prev : r.content)) })
      .catch(() => {})
    return () => { alive = false }
  }, [s.detail, sel?.path, s.runId, editing]) // eslint-disable-line react-hooks/exhaustive-deps

  // Load the diff-from-baseline for changed files (and refresh as fixes land), so
  // "what did the autonomous fix change?" is answerable without leaving the tool.
  useEffect(() => {
    if (!sel || !s.runId || !sel.changed) { setDiff(''); return }
    let alive = true
    api.diff(s.runId, sel.path).then((r) => { if (alive) setDiff(r.diff) }).catch(() => {})
    return () => { alive = false }
  }, [s.detail, sel?.path, s.runId, sel?.changed]) // eslint-disable-line react-hooks/exhaustive-deps

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

  return (
      <div className="editor">
        {s.openFiles.length > 0 && (
          <div className="ed-tabs">
            {s.openFiles.map((p) => (
              <div key={p} className={`ed-tab ${p === s.file ? 'on' : ''}`} onClick={() => s.setFile(p)} title={p}>
                <IcFile />
                <span className="ed-tab-name">{baseName(p)}</span>
                <span className="ed-tab-x" title="Close" onClick={(e) => { e.stopPropagation(); s.closeFile(p) }}><X size={12} /></span>
              </div>
            ))}
          </div>
        )}
        <div className="ed-head">
          <div className="ed-head-left">
            {sel && changedFiles.length > 0 && <button className="btn-sm" title="Back to changed files" onClick={() => s.setFile('')} style={{ marginRight: 4 }}>‹ Files</button>}
            <IcFile />
            <span className="path">{sel ? sel.path : `${changedFiles.length} changed file${changedFiles.length === 1 ? '' : 's'}`}</span>
            {sel?.changed && <span className="chg-pill" style={{ marginLeft: 8 }}>changed</span>}
            {sel?.review === 'accepted' && <span style={{ color: 'var(--diff-add-text)', marginLeft: 8 }}>✓ accepted</span>}
          </div>
          <div className="ed-actions">
            {changedFiles.length > 1 && !editing && (
              <span className="chg-step" title="Step through changed files">
                <button className="btn-sm" onClick={() => stepChanged(-1)}>‹</button>
                <span className="chg-step-c">{changedIdx >= 0 ? changedIdx + 1 : '–'}/{changedFiles.length} changed</span>
                <button className="btn-sm" onClick={() => stepChanged(1)}>›</button>
              </span>
            )}
            {sel && editing && <>
              <button className="btn-sm" onClick={() => setEditing(false)}>Cancel</button>
              <button className="btn-sm primary" disabled={saving} onClick={saveEdit}>{saving ? 'Saving…' : 'Save'}</button>
            </>}
            {sel && !editing && <>
              {sel.changed && (
                <div className="viewseg">
                  <button className={`viewseg-b ${showDiff ? 'on' : ''}`} onClick={() => setShowDiff(true)}>Diff</button>
                  <button className={`viewseg-b ${!showDiff ? 'on' : ''}`} onClick={() => setShowDiff(false)}>File</button>
                </div>
              )}
              <button className="btn-sm" onClick={startEdit}>Edit</button>
              {sel.changed && sel.review !== 'accepted' && <>
                <button className="btn-sm" onClick={() => review('rejected')}>Reject</button>
                <button className="btn-sm primary" onClick={() => review('accepted')}>Accept changes</button>
              </>}
            </>}
          </div>
        </div>
        {sel
          ? (editing
              ? <Mono path={sel.path} value={draft} readOnly={false} onChange={setDraft} />
              : (sel.changed && showDiff
                  ? <Diff text={diff} />
                  : <Mono path={sel.path} value={loading ? '' : (content ?? '')} readOnly />))
          : (
            changedFiles.length > 0
              ? <ChangedList files={changedFiles} onOpen={(p) => s.setFile(p)} />
              : <div className="empty-mid" style={{ position: 'static', paddingTop: 100 }}>
                  <h3>{s.runId ? 'No changes yet' : 'No project open'}</h3>
                  <p>{s.runId ? (files.length ? 'The audit hasn’t changed any files yet. Browse the tree in the Explorer.' : 'No files yet — the audit is still importing.') : 'Import a codebase from the home screen.'}</p>
                </div>
          )}
      </div>
  )
}
