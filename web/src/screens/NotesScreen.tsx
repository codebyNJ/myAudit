import { useEffect, useRef, useState } from 'react'
import { marked } from 'marked'
import { Bold, Italic, Heading, List, ListChecks, Code, Link2, Quote, Pencil, Eye } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'

marked.setOptions({ breaks: true, gfm: true })

type Mode = 'write' | 'split' | 'preview'

// Report tab: the audit's running narrative (map + per-module QA + fixes). It's
// 99% agent-generated, so it's read-FIRST and full-width; editing is a toggle.
// Rendering is hardened so whatever markdown the agent dumps (wide tables, big
// code blocks, odd nesting) never breaks the page layout — see .md-preview CSS.
export function NotesScreen() {
  const s = useStore()
  const [text, setText] = useState('')
  const [editing, setEditing] = useState(false)
  const [mode, setMode] = useState<Mode>('split')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const ta = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (!s.runId) return
    setLoading(true)
    api.getNotes(s.runId).then((r) => setText(r.content || defaultDoc())).catch(() => s.toast('error', 'Failed to load report')).finally(() => setLoading(false))
  }, [s.runId]) // eslint-disable-line react-hooks/exhaustive-deps

  if (!s.runId) return <div className="empty-mid"><h3>No report</h3><p>Select or create a project to see its report.</p></div>

  const save = async () => {
    setSaving(true)
    try { await api.putNotes(s.runId!, text); s.toast('success', 'Report saved'); setEditing(false) }
    catch (e) { s.toast('error', 'Save failed', (e as Error).message) }
    finally { setSaving(false) }
  }

  const surround = (before: string, after = before) => {
    const el = ta.current; if (!el) return
    const [a, b] = [el.selectionStart, el.selectionEnd]
    setText(text.slice(0, a) + before + text.slice(a, b) + after + text.slice(b))
    requestAnimationFrame(() => { el.focus(); el.selectionStart = a + before.length; el.selectionEnd = b + before.length })
  }
  const prefixLine = (p: string) => {
    const el = ta.current; if (!el) return
    const a = el.selectionStart
    const ls = text.lastIndexOf('\n', a - 1) + 1
    setText(text.slice(0, ls) + p + text.slice(ls))
  }

  const html = marked.parse(text) as string
  const project = (s.runs || []).find((r) => r.id === s.runId)?.project

  return (
    <div className="dash">
      <div className="dash-inner">
        <div className="dash-head" style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between' }}>
          <div>
            <h2>Report</h2>
            <p className="sub">The audit's living narrative for {project} — map, per-module QA, and every fix.</p>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <a className="btn-sm" href={api.reportUrl(s.runId)} download="report.md">↓ report.md</a>
            <a className="btn-sm" href={api.findingsUrl(s.runId)} download="findings.json">↓ findings.json</a>
            {!editing
              ? <button className="btn-sm" onClick={() => setEditing(true)}><Pencil size={13} /> Edit</button>
              : <button className="btn-sm" onClick={() => setEditing(false)}><Eye size={13} /> Read</button>}
          </div>
        </div>

        {loading ? <div className="empty-mid" style={{ position: 'static', padding: 60 }}><div className="spin" /></div>
          : !editing ? (
            <article className="card report-read" dangerouslySetInnerHTML={{ __html: html }} />
          ) : (
            <div className="md-editor">
              <div className="md-toolbar">
                <button className="md-tool" title="Bold" onClick={() => surround('**')}><Bold size={14} /></button>
                <button className="md-tool" title="Italic" onClick={() => surround('*')}><Italic size={14} /></button>
                <button className="md-tool" title="Heading" onClick={() => prefixLine('## ')}><Heading size={14} /></button>
                <span className="md-sep" />
                <button className="md-tool" title="List" onClick={() => prefixLine('- ')}><List size={14} /></button>
                <button className="md-tool" title="Task list" onClick={() => prefixLine('- [ ] ')}><ListChecks size={14} /></button>
                <button className="md-tool" title="Quote" onClick={() => prefixLine('> ')}><Quote size={14} /></button>
                <span className="md-sep" />
                <button className="md-tool" title="Code" onClick={() => surround('`')}><Code size={14} /></button>
                <button className="md-tool" title="Link" onClick={() => surround('[', '](url)')}><Link2 size={14} /></button>
                <div className="md-modes">
                  {(['write', 'split', 'preview'] as Mode[]).map((m) => (
                    <button key={m} className={mode === m ? 'on' : ''} onClick={() => setMode(m)}>{m[0].toUpperCase() + m.slice(1)}</button>
                  ))}
                  <button className="on" style={{ background: 'var(--method-post-bg)', color: 'var(--method-post)' }} disabled={saving} onClick={save}>{saving ? 'Saving…' : 'Save'}</button>
                </div>
              </div>
              <div className={`md-panes ${mode === 'split' ? 'split' : ''}`}>
                {mode !== 'preview' && (
                  <textarea ref={ta} className="md-input" value={text} onChange={(e) => setText(e.target.value)}
                    placeholder="# Report&#10;Write **markdown** here…" spellCheck={false} />
                )}
                {mode !== 'write' && <div className="md-preview report-read" dangerouslySetInnerHTML={{ __html: html }} />}
              </div>
            </div>
          )}
      </div>
    </div>
  )
}

function defaultDoc() {
  return `# Audit report\n\n## Product map\n- \n\n## Findings\n- \n\n## Fixes\n> The audit appends its map, per-module QA, and every fix here as it runs.\n`
}
