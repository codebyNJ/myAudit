import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { api, type Run, type RunDetail, type CreateRunBody, type FileEntry } from './api'

export type Tab = 'dev' | 'activity' | 'playwright' | 'kanban' | 'notes' | 'settings'
export type Toast = { id: number; type: 'success' | 'error' | 'info'; title: string; msg?: string }

type Store = {
  runs: Run[] | null
  runId: string | null
  detail: RunDetail | null
  visibleFiles: FileEntry[]
  loadingDetail: boolean
  tab: Tab
  file: string | null
  openFiles: string[]
  closeFile: (p: string) => void
  toasts: Toast[]
  newOpen: boolean
  setNewOpen: (v: boolean) => void
  explorerOpen: boolean
  toggleExplorer: () => void
  chatOpen: boolean
  toggleChat: () => void
  setChatOpen: (v: boolean) => void
  explorerW: number
  setExplorerW: (n: number) => void
  setTab: (t: Tab) => void
  focusCard: string | null       
  openCard: (id: string) => void 
  clearFocusCard: () => void
  setRun: (id: string) => void
  goHome: () => void
  setFile: (p: string) => void
  toast: (type: Toast['type'], title: string, msg?: string) => void
  dismiss: (id: number) => void
  refresh: () => void
  reloadDetail: () => void
  createRun: (b: CreateRunBody) => Promise<boolean>
  resolveCheckpoint: (id: string, answer: string) => Promise<void>
}

const Ctx = createContext<Store>(null as unknown as Store)
export const useStore = () => useContext(Ctx)

const TABS = new Set<Tab>(['dev', 'activity', 'playwright', 'kanban', 'notes', 'settings'])
function parseHash(): { runId: string | null; tab: Tab } {
  const m = location.hash.match(/^#\/run\/([\w-]+)(?:\/(\w+))?/)
  if (!m) return { runId: null, tab: 'kanban' }
  let t = m[2] as Tab
  if ((t as string) === 'activity') t = 'playwright' 
  return { runId: m[1], tab: TABS.has(t) ? t : 'kanban' }
}

export function StoreProvider({ children }: { children: ReactNode }) {
  const [runs, setRuns] = useState<Run[] | null>(null)
  const initial = parseHash()
  const [runId, setRunId] = useState<string | null>(initial.runId)
  const [detail, setDetail] = useState<RunDetail | null>(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [tab, setTab] = useState<Tab>(initial.tab)
  const [focusCard, setFocusCard] = useState<string | null>(null)
  const [file, setFileState] = useState<string | null>(null)
  const [openFiles, setOpenFiles] = useState<string[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])
  const [newOpen, setNewOpen] = useState(false)
  const [explorerOpen, setExplorerOpen] = useState(true)
  const [chatOpen, setChatOpenState] = useState(() => localStorage.getItem('chatOpen') === '1')
  const setChatOpen = useCallback((v: boolean) => { setChatOpenState(v); localStorage.setItem('chatOpen', v ? '1' : '0') }, [])
  const [explorerW, setExplorerWState] = useState(() => {
    const v = Number(localStorage.getItem('explorerW'))
    return v >= 180 && v <= 480 ? v : 240
  })
  const tid = useRef(0)

  const setExplorerW = useCallback((n: number) => {
    const w = Math.min(480, Math.max(180, Math.round(n)))
    setExplorerWState(w)
    localStorage.setItem('explorerW', String(w))
  }, [])

  const toast = useCallback((type: Toast['type'], title: string, msg?: string) => {
    const id = ++tid.current
    setToasts((t) => [...t, { id, type, title, msg }])

    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), type === 'error' ? 9000 : 3600)
  }, [])
  const dismiss = useCallback((id: number) => setToasts((t) => t.filter((x) => x.id !== id)), [])

  const pollFailed = useRef(false)
  const loadDetail = useCallback(async (id: string, spinner: boolean) => {
    if (spinner) setLoadingDetail(true)
    try {
      setDetail(await api.runDetail(id))
      pollFailed.current = false 
    } catch (e) {

      if (!pollFailed.current) {
        pollFailed.current = true
        toast('error', 'Lost connection to the run', (e as Error).message)
      }
    } finally {
      setLoadingDetail(false)
    }
  }, [toast])

  
  const bootedPreview = useRef<string | null>(null)
  useEffect(() => {
    if (!runId || bootedPreview.current === runId) return
    bootedPreview.current = runId
    api.startPreview(runId).catch(() => {})
  }, [runId])

  const loadRuns = useCallback(async () => {
    try {
      setRuns((await api.listRuns()) || []) 
    } catch (e) {
      setRuns([])
      toast('error', 'Failed to load audits', (e as Error).message)
    }
  }, [toast])

  useEffect(() => { loadRuns() }, [loadRuns])
  useEffect(() => { if (runId) loadDetail(runId, true) }, [runId, loadDetail])

  useEffect(() => {
    const want = runId ? `#/run/${runId}/${tab}` : '#/'
    if (location.hash !== want) history.replaceState(null, '', want)
  }, [runId, tab])

  useEffect(() => {
    const onHash = () => {
      const h = parseHash()
      setRunId((cur) => (cur === h.runId ? cur : h.runId))
      setTab((cur) => (cur === h.tab ? cur : h.tab))
    }
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  useEffect(() => {
    if (!runId) return
    const h = setInterval(() => loadDetail(runId, false), 2000)
    return () => clearInterval(h)
  }, [runId, loadDetail])

  const visibleFiles = detail?.files || []

  const setFile = useCallback((p: string) => {
    setFileState(p || null)
    if (p) setOpenFiles((o) => (o.includes(p) ? o : [...o, p]))
  }, [])

  const closeFile = useCallback((p: string) => {
    setOpenFiles((o) => {
      const i = o.indexOf(p)
      const next = o.filter((x) => x !== p)
      setFileState((cur) => (cur === p ? (next[i] ?? next[i - 1] ?? null) : cur))
      return next
    })
  }, [])
  const clearFiles = useCallback(() => { setFileState(null); setOpenFiles([]) }, [])

  const store: Store = {
    runs, runId, detail, visibleFiles, loadingDetail, tab, file, openFiles, closeFile, toasts, newOpen, setNewOpen,
    explorerOpen, toggleExplorer: () => setExplorerOpen((v) => !v),
    chatOpen, toggleChat: () => setChatOpen(!chatOpen), setChatOpen,
    explorerW, setExplorerW,
    setTab,
    focusCard, openCard: (id) => { setFocusCard(id); setTab('kanban') }, clearFocusCard: () => setFocusCard(null),
    setRun: (id) => { setRunId(id); setNewOpen(false); clearFiles(); setTab('kanban') },
    goHome: () => { setRunId(null); setNewOpen(false); clearFiles() },
    setFile, toast, dismiss,
    refresh: () => { loadRuns(); if (runId) loadDetail(runId, true) },
    reloadDetail: () => { if (runId) loadDetail(runId, false) },
    createRun: async (b) => {
      try {
        const { id } = await api.createRun(b)
        toast('success', 'Audit started', b.project || b.repo_path)
        await loadRuns(); setRunId(id); setNewOpen(false); clearFiles(); setTab('kanban'); return true
      } catch (e) { toast('error', 'Could not start audit', (e as Error).message); return false }
    },
    resolveCheckpoint: async (id, answer) => {
      try { await api.resolveCheckpoint(id, answer); toast('success', 'Checkpoint resolved'); if (runId) loadDetail(runId, false) }
      catch (e) { toast('error', 'Resolve failed', (e as Error).message) }
    },
  }
  return <Ctx.Provider value={store}>{children}</Ctx.Provider>
}
