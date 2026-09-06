import { useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'
import { IcFile } from '../components/icons'
import { Code } from '../components/Code'
import { Diff } from '../components/Diff'

const baseName = (p: string) => p.split('/').pop() || p

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
      if (!s.file && files.length) s.setFile(files[0].path)
      return
    }
    const freshChanged = files.filter((f) => f.changed && !changedSeen.current.has(f.path)).map((f) => f.path)
    const freshNew = files.filter((f) => !seen.current.has(f.path)).map((f) => f.path)
    files.forEach((f) => { seen.current.add(f.path); if (f.changed) changedSeen.current.add(f.path) })
    if (freshChanged.length) s.setFile(freshChanged[freshChanged.length - 1])
    else if (freshNew.length) s.setFile(freshNew[freshNew.length - 1])
    else if (!s.file && files.length) s.setFile(files[0].path)
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
            <IcFile />
            <span className="path">{sel ? sel.path : 'no file selected'}</span>
            {sel?.changed && <span className="chg-pill" style={{ marginLeft: 8 }}>changed</span>}
            {sel?.review === 'accepted' && <span style={{ color: 'var(--diff-add-text)', marginLeft: 8 }}>✓ accepted</span>}
          </div>
          <div className="ed-actions">
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
              ? <textarea className="code-edit" value={draft} onChange={(e) => setDraft(e.target.value)} spellCheck={false} />
              : (sel.changed && showDiff
                  ? <Diff text={diff} />
                  : <Code path={sel.path} content={loading ? '' : (content ?? '')} />))
          : (
            <div className="empty-mid" style={{ position: 'static', paddingTop: 100 }}>
              <h3>{s.runId ? 'No file selected' : 'No project open'}</h3>
              <p>{s.runId ? (files.length ? 'Pick a file from the Explorer.' : 'No files yet — the audit is still importing.') : 'Import a codebase from the home screen.'}</p>
            </div>
          )}
      </div>
  )
}
