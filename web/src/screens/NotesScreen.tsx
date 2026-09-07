import { useEffect, useState } from 'react'
import { Download, Search, Plus, X, FileText } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'
import { Markdown } from '../components/Markdown'

export type NoteItem = {
  id: string
  title: string
  category: 'architecture' | 'qa' | 'fix' | 'custom'
  content: string
  updatedAt: string
}

function parseMarkdownToNotes(raw: string): NoteItem[] {
  if (!raw || !raw.trim()) {
    return [{
      id: 'note-default',
      title: 'Audit Overview & Map',
      category: 'architecture',
      content: '# Audit Overview & Map\n\nWelcome to your audit report. The AI agents append module QA passes, findings, and verified fixes here as separate notes.',
      updatedAt: new Date().toLocaleTimeString(),
    }]
  }

  const lines = raw.split('\n')
  const sections: { title: string; lines: string[]; category: NoteItem['category'] }[] = []
  let current: { title: string; lines: string[]; category: NoteItem['category'] } | null = null

  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed.startsWith('# ') || trimmed.startsWith('## ') || trimmed.startsWith('### Fix —') || trimmed.startsWith('### QA —')) {
      if (current) sections.push(current)
      const rawTitle = trimmed.replace(/^#+\s*/, '').trim()
      let cat: NoteItem['category'] = 'custom'
      const low = rawTitle.toLowerCase()
      if (low.includes('map') || low.includes('architecture') || low.includes('audit map')) cat = 'architecture'
      else if (low.includes('qa') || low.includes('module')) cat = 'qa'
      else if (low.includes('fix') || low.includes('repair')) cat = 'fix'
      current = { title: rawTitle, lines: [line], category: cat }
    } else if (!current) {
      current = { title: 'General Audit Notes', lines: [line], category: 'architecture' }
    } else {
      current.lines.push(line)
    }
  }
  if (current) sections.push(current)

  return sections.map((s, idx) => ({
    id: `note-${idx}-${s.title.slice(0, 16).replace(/\W+/g, '-').toLowerCase()}`,
    title: s.title || `Note ${idx + 1}`,
    category: s.category,
    content: s.lines.join('\n').trim(),
    updatedAt: new Date().toLocaleTimeString(),
  }))
}

function serializeNotesToMarkdown(notes: NoteItem[]): string {
  return notes.map((n) => n.content.trim()).filter(Boolean).join('\n\n---\n\n')
}

const CAT_STYLE: Record<string, { color: string; bg: string; label: string }> = {
  architecture: { color: '#38bdf8', bg: 'rgba(56,189,248,0.12)', label: 'Map' },
  qa: { color: '#c084fc', bg: 'rgba(192,132,252,0.12)', label: 'QA' },
  fix: { color: '#4ade80', bg: 'rgba(74,222,128,0.12)', label: 'Fix' },
  custom: { color: '#94a3b8', bg: 'rgba(148,163,184,0.12)', label: 'Note' },
}

export function NotesScreen() {
  const s = useStore()
  const [notes, setNotes] = useState<NoteItem[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [filterCat, setFilterCat] = useState('all')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [openId, setOpenId] = useState<string | null>(null)

  useEffect(() => {
    if (!s.runId) return
    setLoading(true)
    api.getNotes(s.runId)
      .then((r) => setNotes(parseMarkdownToNotes(r.content || '')))
      .catch(() => s.toast('error', 'Failed to load report notes'))
      .finally(() => setLoading(false))
  }, [s.runId])

  if (!s.runId) {
    return (
      <div className="empty-mid">
        <FileText size={40} strokeWidth={1.5} color="var(--text-muted)" />
        <h3>No audit selected</h3>
        <p>Open a run to see the notice-board report of map, QA, and fix notes.</p>
      </div>
    )
  }

  const visible = notes.filter((n) => {
    if (filterCat !== 'all' && n.category !== filterCat) return false
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase()
      return n.title.toLowerCase().includes(q) || n.content.toLowerCase().includes(q)
    }
    return true
  })

  const openNote = notes.find((n) => n.id === openId) || null

  const createNewNote = () => {
    const n: NoteItem = {
      id: `note-custom-${Date.now()}`,
      title: 'New note',
      category: 'custom',
      content: '# New note\n\n',
      updatedAt: new Date().toLocaleTimeString(),
    }
    setNotes((prev) => [n, ...prev])
    setOpenId(n.id)
  }

  const saveAll = async () => {
    if (!s.runId) return
    setSaving(true)
    try {
      await api.putNotes(s.runId, serializeNotesToMarkdown(notes))
      s.toast('success', 'Report saved')
    } catch (e) {
      s.toast('error', 'Save failed', (e as Error).message)
    } finally { setSaving(false) }
  }

  const deleteOpen = () => {
    if (!openNote) return
    setNotes((prev) => prev.filter((n) => n.id !== openNote.id))
    setOpenId(null)
  }

  return (
    <div className="notice-board">
      <div className="notice-bar">
        <div>
          <h1 className="notice-title">Report</h1>
          <p className="notice-sub">Notice-board of audit notes — rendered markdown, not a document outline.</p>
        </div>
        <div className="notice-bar-actions">
          <div className="ex-search" style={{ margin: 0, width: 220 }}>
            <Search size={13} />
            <input placeholder="Search notes…" value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} />
          </div>
          <div className="notice-filters">
            {['all', 'architecture', 'qa', 'fix', 'custom'].map((cat) => (
              <button
                key={cat}
                className={`jira-pill ${filterCat === cat ? 'on' : ''}`}
                onClick={() => setFilterCat(cat)}
              >
                {cat === 'all' ? 'All' : CAT_STYLE[cat]?.label || cat}
              </button>
            ))}
          </div>
          <button className="btn-sm" onClick={createNewNote}><Plus size={13} /> Note</button>
          <button className="btn-sm" disabled={saving} onClick={saveAll}>{saving ? 'Saving…' : 'Save'}</button>
          <a className="btn-sm" href={api.reportUrl(s.runId)} download="report.md"><Download size={12} /> .md</a>
          <a className="btn-sm" href={api.findingsUrl(s.runId)} download="findings.json"><Download size={12} /> .json</a>
        </div>
      </div>

      {loading ? (
        <div className="empty-mid" style={{ position: 'static', paddingTop: 80 }}><div className="spin" /></div>
      ) : (
        <div className="notice-grid">
          {visible.map((n) => {
            const st = CAT_STYLE[n.category] || CAT_STYLE.custom
            return (
              <button
                type="button"
                key={n.id}
                className="notice-card"
                onClick={() => setOpenId(n.id)}
              >
                <div className="notice-card-meta">
                  <span className="notice-badge" style={{ color: st.color, background: st.bg }}>{st.label}</span>
                </div>
                <div className="notice-card-title">{n.title}</div>
                <div className="notice-card-body">
                  <Markdown text={n.content} block className="notice-md" />
                </div>
              </button>
            )
          })}
          {!visible.length && (
            <div className="fl-none" style={{ gridColumn: '1 / -1' }}>No notes match this filter.</div>
          )}
        </div>
      )}

      {openNote && (
        <>
          <div className="drawer-scrim" onClick={() => setOpenId(null)} />
          <div className="notice-focus">
            <div className="notice-focus-h">
              <span className="notice-badge" style={{
                color: (CAT_STYLE[openNote.category] || CAT_STYLE.custom).color,
                background: (CAT_STYLE[openNote.category] || CAT_STYLE.custom).bg,
              }}>
                {(CAT_STYLE[openNote.category] || CAT_STYLE.custom).label}
              </span>
              <h2>{openNote.title}</h2>
              <div style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>
                <button className="btn-sm" onClick={deleteOpen}>Delete</button>
                <button className="icon-btn" onClick={() => setOpenId(null)} title="Close"><X size={16} /></button>
              </div>
            </div>
            <div className="notice-focus-body">
              <Markdown text={openNote.content} block className="drawer-md" />
            </div>
          </div>
        </>
      )}
    </div>
  )
}
