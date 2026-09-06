import { useEffect, useRef, useState } from 'react'
import { marked } from 'marked'
import { 
  Bold, Italic, Heading, List, ListChecks, Code, Link2, Quote, 
  Eye, Plus, Trash2, Download, Search, FileText, 
  BookOpen, Columns, Save
} from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'

marked.setOptions({ breaks: true, gfm: true })

export type NoteItem = {
  id: string
  title: string
  category: 'architecture' | 'qa' | 'fix' | 'custom'
  content: string
  updatedAt: string
}

function parseMarkdownToNotes(raw: string): NoteItem[] {
  if (!raw || !raw.trim()) {
    return [
      {
        id: 'note-default',
        title: 'Audit Overview & Map',
        category: 'architecture',
        content: '# Audit Overview & Map\n\nWelcome to your audit report. The AI agents append module QA passes, findings, and verified fixes here as separate notes.',
        updatedAt: new Date().toLocaleTimeString()
      }
    ]
  }

  const lines = raw.split('\n')
  const sections: { title: string; lines: string[]; category: NoteItem['category'] }[] = []
  let currentSection: { title: string; lines: string[]; category: NoteItem['category'] } | null = null

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const trimmed = line.trim()

    if (trimmed.startsWith('# ') || trimmed.startsWith('## ') || trimmed.startsWith('### Fix —') || trimmed.startsWith('### QA —')) {
      if (currentSection) {
        sections.push(currentSection)
      }
      let rawTitle = trimmed.replace(/^#+\s*/, '').trim()
      let cat: NoteItem['category'] = 'custom'

      if (rawTitle.toLowerCase().includes('map') || rawTitle.toLowerCase().includes('architecture') || rawTitle.toLowerCase().includes('audit map')) {
        cat = 'architecture'
      } else if (rawTitle.toLowerCase().includes('qa') || rawTitle.toLowerCase().includes('module')) {
        cat = 'qa'
      } else if (rawTitle.toLowerCase().includes('fix') || rawTitle.toLowerCase().includes('repair')) {
        cat = 'fix'
      }

      currentSection = {
        title: rawTitle,
        lines: [line],
        category: cat
      }
    } else {
      if (!currentSection) {
        currentSection = {
          title: 'General Audit Notes',
          lines: [line],
          category: 'architecture'
        }
      } else {
        currentSection.lines.push(line)
      }
    }
  }

  if (currentSection) {
    sections.push(currentSection)
  }

  return sections.map((s, idx) => ({
    id: `note-${idx}-${s.title.slice(0, 16).replace(/\W+/g, '-').toLowerCase()}`,
    title: s.title || `Note ${idx + 1}`,
    category: s.category,
    content: s.lines.join('\n').trim(),
    updatedAt: new Date().toLocaleTimeString()
  }))
}

function serializeNotesToMarkdown(notes: NoteItem[]): string {
  return notes.map((n) => n.content.trim()).filter(Boolean).join('\n\n---\n\n')
}

export function NotesScreen() {
  const s = useStore()
  const [notes, setNotes] = useState<NoteItem[]>([])
  const [selectedId, setSelectedId] = useState<string>('')
  const [searchQuery, setSearchQuery] = useState('')
  const [filterCat, setFilterCat] = useState('all')
  const [viewMode, setViewMode] = useState<'read' | 'split' | 'all'>('split')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const taRef = useRef<HTMLTextAreaElement>(null)

  const activeNote = notes.find((n) => n.id === selectedId) || notes[0]

  useEffect(() => {
    if (!s.runId) return
    setLoading(true)
    api.getNotes(s.runId)
      .then((r) => {
        const parsed = parseMarkdownToNotes(r.content || '')
        setNotes(parsed)
        if (parsed.length > 0 && !selectedId) {
          setSelectedId(parsed[0].id)
        }
      })
      .catch(() => s.toast('error', 'Failed to load report notes'))
      .finally(() => setLoading(false))
  }, [s.runId])

  if (!s.runId) {
    return (
      <div className="empty-mid">
        <FileText size={40} strokeWidth={1.5} color="var(--text-muted)" />
        <h3>No report selected</h3>
        <p>Import a codebase or select an audit run to view and manage its notes and reports.</p>
      </div>
    )
  }

  const handleNoteContentChange = (newContent: string) => {
    if (!activeNote) return
    setNotes((prev) =>
      prev.map((n) => {
        if (n.id !== activeNote.id) return n
        const firstLine = newContent.trim().split('\n')[0] || ''
        const inferredTitle = firstLine.replace(/^#+\s*/, '').trim() || n.title
        return {
          ...n,
          content: newContent,
          title: inferredTitle,
          updatedAt: new Date().toLocaleTimeString()
        }
      })
    )
  }

  const saveAll = async () => {
    if (!s.runId) return
    setSaving(true)
    const combined = serializeNotesToMarkdown(notes)
    try {
      await api.putNotes(s.runId, combined)
      s.toast('success', 'Notes synchronized with audit report')
    } catch (e) {
      s.toast('error', 'Failed to save notes', (e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const createNewNote = () => {
    const newId = `custom-${Date.now()}`
    const fresh: NoteItem = {
      id: newId,
      title: 'New Note',
      category: 'custom',
      content: '## New Note\n\nAdd your audit observations or custom findings here.',
      updatedAt: new Date().toLocaleTimeString()
    }
    const next = [fresh, ...notes]
    setNotes(next)
    setSelectedId(newId)
    setViewMode('split')
    s.toast('info', 'New note created')
  }

  const deleteCurrentNote = () => {
    if (notes.length <= 1) {
      s.toast('error', 'Cannot delete the only note')
      return
    }
    const next = notes.filter((n) => n.id !== activeNote.id)
    setNotes(next)
    setSelectedId(next[0].id)
    s.toast('info', 'Note removed')
  }

  const surroundText = (before: string, after = before) => {
    const el = taRef.current
    if (!el || !activeNote) return
    const [a, b] = [el.selectionStart, el.selectionEnd]
    const cur = activeNote.content
    const next = cur.slice(0, a) + before + cur.slice(a, b) + after + cur.slice(b)
    handleNoteContentChange(next)
    requestAnimationFrame(() => {
      el.focus()
      el.selectionStart = a + before.length
      el.selectionEnd = b + before.length
    })
  }

  const prefixLine = (p: string) => {
    const el = taRef.current
    if (!el || !activeNote) return
    const a = el.selectionStart
    const cur = activeNote.content
    const ls = cur.lastIndexOf('\n', a - 1) + 1
    const next = cur.slice(0, ls) + p + cur.slice(ls)
    handleNoteContentChange(next)
  }

  const visibleNotes = notes.filter((n) => {
    if (filterCat !== 'all' && n.category !== filterCat) return false
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase()
      return n.title.toLowerCase().includes(q) || n.content.toLowerCase().includes(q)
    }
    return true
  })

  const fullReportMarkdown = serializeNotesToMarkdown(notes)
  const fullHtml = marked.parse(fullReportMarkdown) as string
  const activeHtml = activeNote ? (marked.parse(activeNote.content) as string) : ''

  return (
    <div className="notes-workspace">
      <div className="notes-sidebar">
        <div className="notes-sidebar-head">
          <div className="notes-sidebar-title-row">
            <span className="notes-sidebar-title">Notes &amp; Reports</span>
            <button className="btn-sm primary" onClick={createNewNote} title="Create note">
              <Plus size={13} /> New Note
            </button>
          </div>
          <div className="ex-search" style={{ margin: 0 }}>
            <Search size={13} />
            <input 
              placeholder="Search notes…" 
              value={searchQuery} 
              onChange={(e) => setSearchQuery(e.target.value)} 
            />
          </div>
          <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
            {['all', 'architecture', 'qa', 'fix', 'custom'].map((cat) => (
              <button
                key={cat}
                className={`jira-pill ${filterCat === cat ? 'on' : ''}`}
                style={{ fontSize: 11, padding: '2px 8px' }}
                onClick={() => setFilterCat(cat)}
              >
                {cat === 'all' ? 'All' : cat === 'architecture' ? 'Map' : cat.toUpperCase()}
              </button>
            ))}
          </div>
        </div>

        <div className="notes-items-list">
          {loading ? (
            <div style={{ padding: 24, textAlign: 'center' }}>
              <div className="spin-sm" style={{ margin: '0 auto 8px' }} />
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>Loading notes…</span>
            </div>
          ) : visibleNotes.length ? (
            visibleNotes.map((n) => {
              const isSelected = n.id === activeNote?.id && viewMode !== 'all'
              let badgeColor = '#94a3b8'
              let badgeBg = 'rgba(148,163,184,0.1)'
              if (n.category === 'architecture') { badgeColor = '#38bdf8'; badgeBg = 'rgba(56,189,248,0.1)' }
              else if (n.category === 'qa') { badgeColor = '#c084fc'; badgeBg = 'rgba(192,132,252,0.1)' }
              else if (n.category === 'fix') { badgeColor = '#4ade80'; badgeBg = 'rgba(74,222,128,0.1)' }

              const previewSnippet = n.content.replace(/^#+\s+[^\n]+/, '').trim()
              const words = n.content.trim().split(/\s+/).filter(Boolean).length

              return (
                <div
                  key={n.id}
                  className={`notes-item-card ${isSelected ? 'on' : ''}`}
                  onClick={() => {
                    setSelectedId(n.id)
                    if (viewMode === 'all') setViewMode('read')
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
                    <span style={{ 
                      fontSize: 10, 
                      fontFamily: 'var(--font-mono)', 
                      fontWeight: 600, 
                      color: badgeColor, 
                      background: badgeBg, 
                      padding: '1px 5px', 
                      borderRadius: 4,
                      textTransform: 'uppercase'
                    }}>
                      {n.category}
                    </span>
                    <span style={{ fontSize: 10.5, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
                      {words} words
                    </span>
                  </div>
                  <div className="notes-item-title">{n.title}</div>
                  <div className="notes-item-preview">
                    {previewSnippet || n.content.slice(0, 100)}
                  </div>
                </div>
              )
            })
          ) : (
            <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 12 }}>
              No notes match your filter.
            </div>
          )}
        </div>

        <div style={{ padding: '10px 14px', borderTop: '1px solid var(--border-dim)', background: '#0b0b0e', display: 'flex', gap: 6, justifyContent: 'space-between' }}>
          <a className="btn-sm" href={api.reportUrl(s.runId)} download="report.md" style={{ flex: 1, justifyContent: 'center' }}>
            <Download size={12} /> report.md
          </a>
          <a className="btn-sm" href={api.findingsUrl(s.runId)} download="findings.json" style={{ flex: 1, justifyContent: 'center' }}>
            <Download size={12} /> findings.json
          </a>
        </div>
      </div>

      <div className="notes-editor-pane">
        <div className="notes-topbar">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {viewMode !== 'all' && activeNote && (
              <>
                <FileText size={16} color="var(--text-secondary)" />
                <span style={{ fontWeight: 600, color: '#fafafa', fontSize: 14 }}>
                  {activeNote.title}
                </span>
                <span style={{ 
                  fontSize: 10, 
                  fontFamily: 'var(--font-mono)', 
                  padding: '2px 6px', 
                  borderRadius: 4, 
                  background: 'rgba(255,255,255,0.06)', 
                  color: 'var(--text-secondary)' 
                }}>
                  {activeNote.category}
                </span>
              </>
            )}
            {viewMode === 'all' && (
              <>
                <BookOpen size={16} color="#38bdf8" />
                <span style={{ fontWeight: 600, color: '#fafafa', fontSize: 14 }}>
                  Full Consolidated Audit Report ({notes.length} Sections)
                </span>
              </>
            )}
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <div className="notes-view-modes">
              <button 
                className={`notes-view-btn ${viewMode === 'read' ? 'on' : ''}`}
                onClick={() => setViewMode('read')}
              >
                <Eye size={12} style={{ display: 'inline', marginRight: 4 }} /> Read
              </button>
              <button 
                className={`notes-view-btn ${viewMode === 'split' ? 'on' : ''}`}
                onClick={() => setViewMode('split')}
              >
                <Columns size={12} style={{ display: 'inline', marginRight: 4 }} /> Split
              </button>
              <button 
                className={`notes-view-btn ${viewMode === 'all' ? 'on' : ''}`}
                onClick={() => setViewMode('all')}
              >
                <BookOpen size={12} style={{ display: 'inline', marginRight: 4 }} /> Full Report
              </button>
            </div>

            {viewMode !== 'all' && (
              <button className="btn-sm" onClick={deleteCurrentNote} title="Delete note">
                <Trash2 size={13} color="#f87171" />
              </button>
            )}

            <button 
              className="btn-sm primary" 
              onClick={saveAll} 
              disabled={saving}
              title="Save all notes to server report"
            >
              <Save size={13} /> {saving ? 'Saving…' : 'Save Notes'}
            </button>
          </div>
        </div>

        {viewMode === 'split' && activeNote && (
          <div className="md-toolbar" style={{ borderBottom: '1px solid var(--border-dim)' }}>
            <button className="md-tool" title="Bold" onClick={() => surroundText('**')}>
              <Bold size={14} />
            </button>
            <button className="md-tool" title="Italic" onClick={() => surroundText('*')}>
              <Italic size={14} />
            </button>
            <button className="md-tool" title="Heading" onClick={() => prefixLine('## ')}>
              <Heading size={14} />
            </button>
            <span className="md-sep" />
            <button className="md-tool" title="Bulleted List" onClick={() => prefixLine('- ')}>
              <List size={14} />
            </button>
            <button className="md-tool" title="Task List" onClick={() => prefixLine('- [ ] ')}>
              <ListChecks size={14} />
            </button>
            <button className="md-tool" title="Quote" onClick={() => prefixLine('> ')}>
              <Quote size={14} />
            </button>
            <span className="md-sep" />
            <button className="md-tool" title="Inline Code" onClick={() => surroundText('`')}>
              <Code size={14} />
            </button>
            <button className="md-tool" title="Link" onClick={() => surroundText('[', '](https://)')}>
              <Link2 size={14} />
            </button>
          </div>
        )}

        <div className={`notes-body-wrap ${viewMode === 'split' ? 'split' : ''}`}>
          {viewMode === 'all' ? (
            <article 
              className="report-read" 
              style={{ height: '100%', overflowY: 'auto' }}
              dangerouslySetInnerHTML={{ __html: fullHtml }} 
            />
          ) : viewMode === 'read' ? (
            <article 
              className="report-read" 
              style={{ height: '100%', overflowY: 'auto' }}
              dangerouslySetInnerHTML={{ __html: activeHtml }} 
            />
          ) : (
            <>
              {activeNote && (
                <textarea
                  ref={taRef}
                  className="notes-textarea"
                  value={activeNote.content}
                  onChange={(e) => handleNoteContentChange(e.target.value)}
                  placeholder="Write markdown note here…"
                  spellCheck={false}
                />
              )}
              <div 
                className="notes-preview-scroll report-read"
                dangerouslySetInnerHTML={{ __html: activeHtml }}
              />
            </>
          )}
        </div>
      </div>
    </div>
  )
}
