import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { api, type Run, type RunDetail, type ApiRoute, type SchemaEntity, type CreateRunBody, type FileEntry } from './api'

export type Tab = 'dev' | 'config' | 'activity' | 'schema' | 'swagger' | 'playwright' | 'kanban' | 'notes' | 'settings'
export type Toast = { id: number; type: 'success' | 'error' | 'info'; title: string; msg?: string }

type Store = {
  runs: Run[] | null
  runId: string | null
  detail: RunDetail | null
  visibleFiles: FileEntry[] // files revealed so far (drips in while streaming)
  loadingDetail: boolean
  tab: Tab
  file: string | null
  apis: ApiRoute[] | null
  schema: SchemaEntity[] | null
  toasts: Toast[]
  newOpen: boolean
  setNewOpen: (v: boolean) => void
  explorerOpen: boolean
  toggleExplorer: () => void
  explorerW: number
  setExplorerW: (n: number) => void
  locked: boolean
  unlock: () => void
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
  const [tab, setTab] = useState<Tab>('dev')
  const [file, setFile] = useState<string | null>(null)
  const [apis, setApis] = useState<ApiRoute[] | null>(null)
  const [schema, setSchema] = useState<SchemaEntity[] | null>(null)
  const [toasts, setToasts] = useState<Toast[]>([])
  const [newOpen, setNewOpen] = useState(false)
  const [explorerOpen, setExplorerOpen] = useState(true)
  const [explorerW, setExplorerWState] = useState(() => {
    const v = Number(localStorage.getItem('explorerW'))
    return v >= 180 && v <= 480 ? v : 240
  })
  const [locked, setLocked] = useState(false)
  // File reveal: after the first prompt (unlock) files drip in one-by-one for a
  // natural "watch it build" feel; opening an existing run shows all at once.
  const [revealed, setRevealed] = useState(0)
  const [streaming, setStreaming] = useState(false)
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
      const rs = (await api.listRuns()) || [] // Go encodes an empty slice as null
      setRuns(rs)
      // No auto-select — the home screen lets the user pick a project.
    } catch (e) {
      setRuns([])
      toast('error', 'Failed to load projects', (e as Error).message)
    }
  }, [toast])

  // boot: baseline schema/openapi until a run is selected
  useEffect(() => { loadRuns(); api.openapi().then(setApis).catch(() => setApis([])); api.schema().then(setSchema).catch(() => setSchema([])) }, [loadRuns])
  // load detail + the run's own schema/openapi when the run changes
  useEffect(() => {
    if (!runId) return
    loadDetail(runId, true)
    api.schemaFor(runId).then(setSchema).catch(() => {})
    api.openapiFor(runId).then(setApis).catch(() => {})
  }, [runId, loadDetail])
  // poll detail
  useEffect(() => {
    if (!runId) return
    const h = setInterval(() => loadDetail(runId, false), 2000)
    return () => clearInterval(h)
  }, [runId, loadDetail])

  // Drip the file reveal while streaming (one file every ~110ms).
  const total = detail?.files?.length || 0
  useEffect(() => {
    if (!streaming || revealed >= total) return
    const t = setTimeout(() => setRevealed((n) => Math.min(total, n + 1)), 110)
    return () => clearTimeout(t)
  }, [streaming, revealed, total])

  const visibleFiles = streaming ? (detail?.files || []).slice(0, revealed) : (detail?.files || [])

  const store: Store = {
    runs, runId, detail, visibleFiles, loadingDetail, tab, file, apis, schema, toasts, newOpen, setNewOpen,
    explorerOpen, toggleExplorer: () => setExplorerOpen((v) => !v),
    explorerW, setExplorerW,
    locked, unlock: () => { setLocked(false); setStreaming(true); setRevealed(0) },
    setTab, setRun: (id) => { setRunId(id); setNewOpen(false); setFile(null); setTab('dev'); setLocked(false); setStreaming(false); setRevealed(0) },
    goHome: () => { setRunId(null); setNewOpen(false); setFile(null); setLocked(false); setStreaming(false); setRevealed(0) },
    setFile, toast, dismiss,
    refresh: () => { loadRuns(); if (runId) loadDetail(runId, true) },
    reloadDetail: () => { if (runId) loadDetail(runId, false) },
    createRun: async (b) => {
      try {
        const { id } = await api.createRun(b)
        toast('success', 'Project created', b.project)
        await loadRuns(); setRunId(id); setNewOpen(false); setFile(null); setTab('dev'); setLocked(true); return true
      } catch (e) { toast('error', 'Could not create project', (e as Error).message); return false }
    },
    resolveCheckpoint: async (id, answer) => {
      try { await api.resolveCheckpoint(id, answer); toast('success', 'Checkpoint resolved'); if (runId) loadDetail(runId, false) }
      catch (e) { toast('error', 'Resolve failed', (e as Error).message) }
    },
  }
  return <Ctx.Provider value={store}>{children}</Ctx.Provider>
}
