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

export function StoreProvider({ children }: { children: ReactNode }) {
  const [runs, setRuns] = useState<Run[] | null>(null)
  const [runId, setRunId] = useState<string | null>(null)
  const [detail, setDetail] = useState<RunDetail | null>(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [tab, setTab] = useState<Tab>('kanban')
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
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 3600)
  }, [])
  const dismiss = useCallback((id: number) => setToasts((t) => t.filter((x) => x.id !== id)), [])

  const loadDetail = useCallback(async (id: string, spinner: boolean) => {
    if (spinner) setLoadingDetail(true)
    try {
      setDetail(await api.runDetail(id))
    } catch (e) {
      toast('error', 'Failed to load run', (e as Error).message)
    } finally {
      setLoadingDetail(false)
    }
  }, [toast])

  const loadRuns = useCallback(async () => {
    try {
      setRuns((await api.listRuns()) || []) // Go encodes an empty slice as null
    } catch (e) {
      setRuns([])
      toast('error', 'Failed to load audits', (e as Error).message)
    }
  }, [toast])

  useEffect(() => { loadRuns() }, [loadRuns])
  useEffect(() => { if (runId) loadDetail(runId, true) }, [runId, loadDetail])
  // poll detail while a run is open (the audit graph advances in the background)
  useEffect(() => {
    if (!runId) return
    const h = setInterval(() => loadDetail(runId, false), 2000)
    return () => clearInterval(h)
  }, [runId, loadDetail])

  const visibleFiles = detail?.files || []

  // Opening a file activates it and adds it to the open-tabs list (VSCode-style).
  const setFile = useCallback((p: string) => {
    setFileState(p || null)
    if (p) setOpenFiles((o) => (o.includes(p) ? o : [...o, p]))
  }, [])
  // Closing a tab drops it; if it was active, activate the neighbour.
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
