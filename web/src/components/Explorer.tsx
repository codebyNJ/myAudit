import { useEffect, useMemo, useRef, useState } from 'react'
import { ChevronRight, Folder, FolderOpen, FilePlus, Trash2, PenLine } from 'lucide-react'
import { useStore } from '../store'
import { api } from '../api'
import { IcFile, IcSearch, IcPanelLeft } from './icons'

type TNode = { name: string; path: string; dir: boolean; children: TNode[] }

function buildTree(paths: string[]): TNode[] {
  const root: TNode = { name: '', path: '', dir: true, children: [] }
  for (const full of paths) {
    const parts = full.split('/')
    let cur = root
    parts.forEach((part, i) => {
      const isFile = i === parts.length - 1
      const p = parts.slice(0, i + 1).join('/')
      let child = cur.children.find((c) => c.name === part && c.dir === !isFile)
      if (!child) { child = { name: part, path: p, dir: !isFile, children: [] }; cur.children.push(child) }
      cur = child
    })
  }
  const sortRec = (n: TNode) => {
    n.children.sort((a, b) => (a.dir === b.dir ? a.name.localeCompare(b.name) : a.dir ? -1 : 1))
    n.children.forEach(sortRec)
  }
  sortRec(root)
  return root.children
}

const GREEN = '#4ade80'
type Menu = { x: number; y: number; path: string; dir: boolean }
type Ask = { title: string; value: string; onOk: (v: string) => void }

function TreeRows({ nodes, depth, collapsed, toggle, onMenu, changed }: {
  nodes: TNode[]; depth: number; collapsed: Set<string>; toggle: (p: string) => void; onMenu: (e: React.MouseEvent, n: TNode) => void; changed: Set<string>
}) {
  return (
    <>
      {nodes.map((n) => n.dir ? (
        <div key={n.path}>
          <div className="row dir" style={{ paddingLeft: 6 + depth * 12 }} onClick={() => toggle(n.path)} onContextMenu={(e) => onMenu(e, n)}>
            <ChevronRight size={13} style={{ transform: collapsed.has(n.path) ? 'none' : 'rotate(90deg)', transition: 'transform .12s', flex: 'none' }} />
            {collapsed.has(n.path) ? <Folder size={13} /> : <FolderOpen size={13} />}
            <span>{n.name}</span>
          </div>
          {!collapsed.has(n.path) && <TreeRows nodes={n.children} depth={depth + 1} collapsed={collapsed} toggle={toggle} onMenu={onMenu} changed={changed} />}
        </div>
      ) : (
        <FileRow key={n.path} name={n.name} path={n.path} depth={depth} chg={changed.has(n.path)} onMenu={onMenu} />
      ))}
    </>
  )
}

function FileRow({ name, path, depth, chg, onMenu }: {
  name: string; path: string; depth: number; chg: boolean; onMenu: (e: React.MouseEvent, n: TNode) => void
}) {
  const s = useStore()
  return (
    <div className={`row file ${path === s.file ? 'on' : ''} ${chg ? 'chg' : ''}`} style={{ paddingLeft: 6 + depth * 12 + 15 }}
      onClick={() => { s.setFile(path); if (s.tab !== 'dev') s.setTab('dev') }}
      onContextMenu={(e) => onMenu(e, { name: path, path, dir: false, children: [] })}>
      <IcFile stroke={chg ? GREEN : 'currentColor'} />
      <span style={chg ? { color: GREEN } : undefined}>{name}</span>
      {chg && <span className="chg-m" title="changed by the audit">M</span>}
    </div>
  )
}

export function Explorer() {
  const s = useStore()
  const [q, setQ] = useState('')
  const [changedOnly, setChangedOnly] = useState(false)
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [menu, setMenu] = useState<Menu | null>(null)
  const [ask, setAsk] = useState<Ask | null>(null)
  const files = s.visibleFiles
  const toggle = (p: string) => setCollapsed((c) => { const n = new Set(c); n.has(p) ? n.delete(p) : n.add(p); return n })
  const key = files.map((f) => (f.changed ? '*' : '') + f.path).join(',')
  const tree = useMemo(() => buildTree(files.map((f) => f.path)), [key])
  const changed = useMemo(() => new Set(files.filter((f) => f.changed && !f.path.startsWith('.myaudit/')).map((f) => f.path)), [key])

  const primedChanged = useRef(false)
  useEffect(() => {
    if (!primedChanged.current && changed.size > 0) { primedChanged.current = true; setChangedOnly(true) }
  }, [changed.size])
  const matches = q ? files.filter((f) => f.path.toLowerCase().includes(q.toLowerCase())) : []

  const openMenu = (e: React.MouseEvent, n: TNode) => { e.preventDefault(); e.stopPropagation(); setMenu({ x: e.clientX, y: e.clientY, path: n.path, dir: n.dir }) }

  const run = async (fn: () => Promise<void>, okMsg: string) => {
    if (!s.runId) return
    try { await fn(); s.reloadDetail(); s.toast('success', okMsg) }
    catch (e) { s.toast('error', 'Operation failed', (e as Error).message) }
  }
  const newFile = () => setAsk({ title: 'New file (path)', value: '', onOk: (v) => run(() => api.newFile(s.runId!, v), 'Created ' + v) })
  const newIn = (dir: string) => setAsk({ title: 'New file in ' + dir, value: dir + '/', onOk: (v) => run(() => api.newFile(s.runId!, v), 'Created ' + v) })
  const rename = (from: string) => setAsk({ title: 'Rename', value: from, onOk: (v) => run(() => api.renameFile(s.runId!, from, v).then(() => { if (s.file === from) s.setFile(v) }), 'Renamed') })
  const del = (path: string) => run(() => api.deleteFile(s.runId!, path).then(() => s.closeFile(path)), 'Deleted ' + path)

  return (
    <aside onClick={() => menu && setMenu(null)}>
      <div className="aside-h">
        <span>Explorer</span>
        <span style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          {s.runId && <span className="collapse" title="New file" onClick={newFile}><FilePlus size={14} /></span>}
          <span className="collapse" title="Collapse" onClick={s.toggleExplorer}><IcPanelLeft /></span>
        </span>
      </div>

      <div className="ex-search">
        <IcSearch size={13} />
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filter files… (⌘P to open, ⌘⇧F to search)" />
      </div>

      {changed.size > 0 && (
        <button className={`ex-changed ${changedOnly ? 'on' : ''}`} onClick={() => setChangedOnly((v) => !v)}>
          {changedOnly ? '◄ All files' : `● Changed (${changed.size})`}
        </button>
      )}

      <div className="tree">
        {!s.runId ? <div className="tree-empty">No project selected</div>
          : !files.length ? <div className="tree-empty">Importing…</div>
          : changedOnly
              ? files.filter((f) => changed.has(f.path)).map((f) => (
                  <FileRow key={f.path} name={f.path} path={f.path} depth={0} chg onMenu={openMenu} />))
          : q ? (matches.length
              ? matches.map((f) => (
                  <FileRow key={f.path} name={f.path} path={f.path} depth={0} chg={changed.has(f.path)} onMenu={openMenu} />))
              : <div className="tree-empty">No match for “{q}”</div>)
          : <TreeRows nodes={tree} depth={0} collapsed={collapsed} toggle={toggle} onMenu={openMenu} changed={changed} />}
      </div>

      {menu && (
        <div className="ctx" style={{ top: menu.y, left: menu.x }} onClick={(e) => e.stopPropagation()}>
          {menu.dir && <div className="ctx-item" onClick={() => { newIn(menu.path); setMenu(null) }}><FilePlus size={13} /> New file here</div>}
          <div className="ctx-item" onClick={() => { rename(menu.path); setMenu(null) }}><PenLine size={13} /> Rename</div>
          <div className="ctx-item danger" onClick={() => { del(menu.path); setMenu(null) }}><Trash2 size={13} /> Delete</div>
        </div>
      )}

      {ask && (
        <div className="pal-scrim" onClick={() => setAsk(null)}>
          <div className="ask" onClick={(e) => e.stopPropagation()}>
            <div className="ask-title">{ask.title}</div>
            <input autoFocus className="pal-input" defaultValue={ask.value}
              onKeyDown={(e) => {
                if (e.key === 'Enter') { const v = (e.target as HTMLInputElement).value.trim(); if (v) ask.onOk(v); setAsk(null) }
                else if (e.key === 'Escape') setAsk(null)
              }} />
            <div className="ask-hint">Enter to confirm · Esc to cancel</div>
          </div>
        </div>
      )}
    </aside>
  )
}
