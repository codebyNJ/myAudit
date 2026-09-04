import { useMemo, useState } from 'react'
import { ChevronRight, Folder, FolderOpen } from 'lucide-react'
import { useStore } from '../store'
import { IcFile, IcSearch, IcPanelLeft } from './icons'

type TNode = { name: string; path: string; dir: boolean; children: TNode[] }

// Build a nested folder tree from flat paths (VSCode-style).
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

function TreeRows({ nodes, depth, collapsed, toggle }: {
  nodes: TNode[]; depth: number; collapsed: Set<string>; toggle: (p: string) => void
}) {
  const s = useStore()
  return (
    <>
      {nodes.map((n) => n.dir ? (
        <div key={n.path}>
          <div className="row dir" style={{ paddingLeft: 6 + depth * 12 }} onClick={() => toggle(n.path)}>
            <ChevronRight size={13} style={{ transform: collapsed.has(n.path) ? 'none' : 'rotate(90deg)', transition: 'transform .12s', flex: 'none' }} />
            {collapsed.has(n.path) ? <Folder size={13} /> : <FolderOpen size={13} />}
            <span>{n.name}</span>
          </div>
          {!collapsed.has(n.path) && <TreeRows nodes={n.children} depth={depth + 1} collapsed={collapsed} toggle={toggle} />}
        </div>
      ) : (
        <div key={n.path} className={`row file ${n.path === s.file ? 'on' : ''}`} style={{ paddingLeft: 6 + depth * 12 + 15 }}
          onClick={() => { s.setFile(n.path); if (s.tab !== 'dev') s.setTab('dev') }}>
          <IcFile stroke={GREEN} /> <span style={{ color: GREEN }}>{n.name}</span>
        </div>
      ))}
    </>
  )
}

// VSCode-style collapsible file explorer with search. Every file is shown green
// (the whole project is freshly generated). Search flattens to matching paths.
export function Explorer() {
  const s = useStore()
  const [q, setQ] = useState('')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const files = s.visibleFiles
  const toggle = (p: string) => setCollapsed((c) => { const n = new Set(c); n.has(p) ? n.delete(p) : n.add(p); return n })
  const key = files.map((f) => f.path).join(',')
  const tree = useMemo(() => buildTree(files.map((f) => f.path)), [key]) // eslint-disable-line react-hooks/exhaustive-deps
  const matches = q ? files.filter((f) => f.path.toLowerCase().includes(q.toLowerCase())) : []

  return (
    <aside>
      <div className="aside-h">
        <span>Explorer</span>
        <span className="collapse" title="Collapse" onClick={s.toggleExplorer}><IcPanelLeft /></span>
      </div>

      <div className="ex-search">
        <IcSearch size={13} />
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search files…" />
        <kbd>⌘K</kbd>
      </div>

      <div className="tree">
        {!s.runId ? <div className="tree-empty">No project selected</div>
          : !files.length ? <div className="tree-empty">Scaffolding…</div>
          : q ? (matches.length
              ? matches.map((f) => (
                  <div key={f.path} className={`row file ${f.path === s.file ? 'on' : ''}`}
                    onClick={() => { s.setFile(f.path); if (s.tab !== 'dev') s.setTab('dev') }}>
                    <IcFile stroke={GREEN} /> <span style={{ color: GREEN }}>{f.path}</span>
                  </div>))
              : <div className="tree-empty">No match for “{q}”</div>)
          : <TreeRows nodes={tree} depth={0} collapsed={collapsed} toggle={toggle} />}
      </div>
    </aside>
  )
}
