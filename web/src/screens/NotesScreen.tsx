import { useEffect, useRef, useState } from 'react'
import { marked } from 'marked'
import { Bold, Italic, Heading, List, ListChecks, Code, Link2, Quote } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'

marked.setOptions({ breaks: true, gfm: true })

type Mode = 'write' | 'split' | 'preview'

export function NotesScreen() {
  const s = useStore()
  const [text, setText] = useState('')
  const [mode, setMode] = useState<Mode>('split')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const ta = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (!s.runId) return
    setLoading(true)
    api.getNotes(s.runId).then((r) => setText(r.content || defaultDoc())).catch(() => s.toast('error', 'Failed to load notes')).finally(() => setLoading(false))
  }, [s.runId]) // eslint-disable-line react-hooks/exhaustive-deps

  if (!s.runId) return <div className="empty-mid"><h3>No notes</h3><p>Select or create a project to keep notes.</p></div>

  const save = async () => {
    setSaving(true)
    try { await api.putNotes(s.runId!, text); s.toast('success', 'Notes saved') }
    catch (e) { s.toast('error', 'Save failed', (e as Error).message) }
    finally { setSaving(false) }
  }

  // wrap/insert markdown around the current selection
  const surround = (before: string, after = before) => {
    const el = ta.current; if (!el) return
    const [a, b] = [el.selectionStart, el.selectionEnd]
    const next = text.slice(0, a) + before + text.slice(a, b) + after + text.slice(b)
    setText(next)
    requestAnimationFrame(() => { el.focus(); el.selectionStart = a + before.length; el.selectionEnd = b + before.length })
  }
  const prefixLine = (p: string) => {
    const el = ta.current; if (!el) return
    const a = el.selectionStart
    const ls = text.lastIndexOf('\n', a - 1) + 1
    setText(text.slice(0, ls) + p + text.slice(ls))
  }

  const html = marked.parse(text) as string

  return (
    <div className="pane" style={{ maxWidth: 980 }}>
      <h2>Notes</h2>
      <p className="sub">Markdown scratchpad for {(s.runs || []).find((r) => r.id === s.runId)?.project}</p>

      {loading ? <div className="empty-mid" style={{ position: 'static', padding: 60 }}><div className="spin" /></div> : (
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
            </div>
          </div>
          <div className={`md-panes ${mode === 'split' ? 'split' : ''}`}>
            {mode !== 'preview' && (
              <textarea ref={ta} className="md-input" value={text} onChange={(e) => setText(e.target.value)}
                placeholder="# Notes&#10;Write **markdown** here…" spellCheck={false} />
            )}
            {mode !== 'write' && <div className="md-preview" dangerouslySetInnerHTML={{ __html: html }} />}
          </div>
        </div>
      )}

      <div style={{ marginTop: 12, display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn-sm primary" style={{ padding: '8px 16px' }} disabled={saving || loading} onClick={save}>{saving ? 'Saving…' : 'Save'}</button>
      </div>
    </div>
  )
}

function defaultDoc() {
  return `# Audit notes\n\n## Product map\n- \n\n## Findings\n- \n\n## Fixes\n> The audit appends its map, per-module QA, and every fix here as it runs.\n`
}
