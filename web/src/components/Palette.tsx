import { useEffect, useMemo, useRef, useState } from 'react'
import { useStore } from '../store'
import { api, type SearchHit } from '../api'

type Mode = 'files' | 'text'

// Command palette: ⌘P fuzzy file open, ⌘⇧F find-in-files. Esc closes.
export function Palette() {
  const s = useStore()
  const [mode, setMode] = useState<Mode | null>(null)
  const [q, setQ] = useState('')
  const [sel, setSel] = useState(0)
  const [hits, setHits] = useState<SearchHit[]>([])
  const inputRef = useRef<HTMLInputElement>(null)
  const runIdRef = useRef(s.runId)
  runIdRef.current = s.runId // always current, avoids stale-closure in the key handler

  const open = (m: Mode) => { if (!runIdRef.current) return; setMode(m); setQ(''); setHits([]); setSel(0); setTimeout(() => inputRef.current?.focus(), 0) }

  // global shortcuts
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey
      if (mod && !e.shiftKey && e.key.toLowerCase() === 'p') { e.preventDefault(); open('files') }
      else if (mod && e.shiftKey && e.key.toLowerCase() === 'f') { e.preventDefault(); open('text') }
      else if (e.key === 'Escape') setMode(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // file mode: fuzzy filter over open workspace paths
  const fileMatches = useMemo(() => {
    if (mode !== 'files') return []
    const paths = s.visibleFiles.map((f) => f.path)
    if (!q) return paths.slice(0, 50)
    const ql = q.toLowerCase()
    return paths.filter((p) => fuzzy(p.toLowerCase(), ql)).slice(0, 50)
  }, [mode, q, s.visibleFiles])

  // text mode: debounced backend find-in-files
  useEffect(() => {
    if (mode !== 'text' || q.trim().length < 2 || !s.runId) { setHits([]); return }
    const t = setTimeout(() => { api.search(s.runId!, q).then(setHits).catch(() => setHits([])) }, 200)
    return () => clearTimeout(t)
  }, [mode, q, s.runId])

  useEffect(() => setSel(0), [q, mode])
  if (!mode) return null

  const rows = mode === 'files' ? fileMatches.length : hits.length
  const choose = (i: number) => {
    if (mode === 'files') { const p = fileMatches[i]; if (p) { s.setFile(p); s.setTab('dev') } }
    else { const h = hits[i]; if (h) { s.setFile(h.path); s.setTab('dev') } }
    setMode(null)
  }
  const onKey = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setSel((i) => Math.min(rows - 1, i + 1)) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setSel((i) => Math.max(0, i - 1)) }
    else if (e.key === 'Enter') { e.preventDefault(); choose(sel) }
  }

  return (
    <div className="pal-scrim" onClick={() => setMode(null)}>
      <div className="pal" onClick={(e) => e.stopPropagation()}>
        <div className="pal-modes">
          <button className={mode === 'files' ? 'on' : ''} onClick={() => setMode('files')}>Files ⌘P</button>
          <button className={mode === 'text' ? 'on' : ''} onClick={() => setMode('text')}>Find in files ⌘⇧F</button>
        </div>
        <input ref={inputRef} className="pal-input" value={q} onChange={(e) => setQ(e.target.value)} onKeyDown={onKey}
          placeholder={mode === 'files' ? 'Go to file…' : 'Search text in files…'} />
        <div className="pal-list">
          {mode === 'files'
            ? fileMatches.map((p, i) => (
                <div key={p} className={`pal-row ${i === sel ? 'on' : ''}`} onMouseEnter={() => setSel(i)} onClick={() => choose(i)}>
                  <span className="pal-name">{baseName(p)}</span><span className="pal-path">{p}</span>
                </div>))
            : hits.map((h, i) => (
                <div key={h.path + ':' + h.line + ':' + i} className={`pal-row ${i === sel ? 'on' : ''}`} onMouseEnter={() => setSel(i)} onClick={() => choose(i)}>
                  <span className="pal-path">{h.path}:{h.line}</span><span className="pal-hit">{h.text}</span>
                </div>))}
          {rows === 0 && <div className="pal-empty">{q.trim().length < 2 && mode === 'text' ? 'Type at least 2 characters' : 'No results'}</div>}
        </div>
      </div>
    </div>
  )
}

const baseName = (p: string) => p.split('/').pop() || p

// subsequence fuzzy match (all query chars appear in order)
function fuzzy(hay: string, needle: string): boolean {
  let i = 0
  for (const c of hay) { if (c === needle[i]) i++; if (i === needle.length) return true }
  return i === needle.length
}
