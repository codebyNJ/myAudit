import { create } from 'zustand'
import { api, type CreateRunBody, type NodeCard, type Run, type RunDetail } from '../api'

export type Tab = 'dev' | 'playwright' | 'kanban' | 'notes' | 'chat' | 'settings'
export type Toast = { id: number; type: 'success' | 'error' | 'info'; title: string; msg?: string }

/** Retry the preview when another node has finished since the last check. */
export function shouldRetryPreview(prevDoneCount: number, doneCount: number): boolean {
  return doneCount > prevDoneCount
}

const TABS = new Set<Tab>(['dev', 'playwright', 'kanban', 'notes', 'chat', 'settings'])

// Guarded so this module can be imported outside a browser — tests run in a
// node environment, and reading browser globals at module load made the whole
// store unimportable there.
const browser = typeof window !== 'undefined'

const ls = {
  get: (k: string) => (browser ? localStorage.getItem(k) : null),
  set: (k: string, v: string) => {
    if (browser) localStorage.setItem(k, v)
  },
}

export function parseHash(hash = browser ? location.hash : ''): { runId: string | null; tab: Tab } {
  const m = hash.match(/^#\/run\/([\w-]+)(?:\/(\w+))?/)
  if (!m) return { runId: null, tab: 'kanban' }
  // 'activity' was this screen's old name and still appears in saved links.
  const raw = m[2] === 'activity' ? 'playwright' : m[2]
  const t = raw as Tab
  return { runId: m[1], tab: TABS.has(t) ? t : 'kanban' }
}

const num = (key: string, min: number, max: number, dflt: number) => {
  const v = Number(ls.get(key))
  return v >= min && v <= max ? v : dflt
}

export type State = {
  // ---- server data
  runs: Run[] | null
  runId: string | null
  detail: RunDetail | null
  /** Board cards. Kanban and Overview both render these; one poll feeds both. */
  board: NodeCard[] | null

  // ---- ui
  tab: Tab
  file: string | null
  openFiles: string[]
  focusCard: string | null
  newOpen: boolean
  explorerOpen: boolean
  explorerW: number
  /**
   * The composer and the in-flight flag live here, not in ChatScreen. A reply
   * can take up to chatTimeout (10 minutes), so an abandoned one costs real
   * money. Screens happen to stay mounted across a tab switch today, but that
   * is an accident of App.tsx hiding them with CSS — see docs/state-management.md,
   * which says not to rely on it. Owning this in the store makes it a guarantee.
   */
  chatDraft: string
  chatBusy: boolean
  toasts: Toast[]

  // ---- actions
  setTab: (t: Tab) => void
  setRun: (id: string) => void
  goHome: () => void
  setFile: (p: string) => void
  closeFile: (p: string) => void
  setNewOpen: (v: boolean) => void
  toggleExplorer: () => void
  setExplorerW: (n: number) => void
  setChatDraft: (v: string) => void
  sendChat: () => Promise<void>
  openCard: (id: string) => void
  clearFocusCard: () => void
  toast: (type: Toast['type'], title: string, msg?: string) => void
  dismiss: (id: number) => void

  loadRuns: () => Promise<void>
  loadDetail: (id: string) => Promise<void>
  loadBoard: (id: string) => Promise<void>
  reloadDetail: () => void
  refresh: () => void
  createRun: (b: CreateRunBody) => Promise<boolean>
  resolveCheckpoint: (id: string, answer: string) => Promise<void>
}

const initial = parseHash()
let toastId = 0
// Deduplicates the "lost connection" toast across a run of failing polls.
let pollFailed = false

export const useAppStore = create<State>((set, get) => ({
  runs: null,
  runId: initial.runId,
  detail: null,
  board: null,

  tab: initial.tab,
  file: null,
  openFiles: [],
  focusCard: null,
  newOpen: false,
  explorerOpen: true,
  explorerW: num('explorerW', 180, 480, 240),
  chatDraft: '',
  chatBusy: false,
  toasts: [],

  setTab: (tab) => set({ tab }),

  setRun: (runId) => set({ runId, newOpen: false, tab: 'kanban', file: null, openFiles: [], board: null, detail: null }),

  goHome: () => set({ runId: null, newOpen: false, file: null, openFiles: [] }),

  setFile: (p) =>
    set((s) => ({
      file: p || null,
      openFiles: p && !s.openFiles.includes(p) ? [...s.openFiles, p] : s.openFiles,
    })),

  closeFile: (p) =>
    set((s) => {
      const i = s.openFiles.indexOf(p)
      const openFiles = s.openFiles.filter((x) => x !== p)
      // Falling back to the neighbour keeps focus where the closed tab was.
      return { openFiles, file: s.file === p ? (openFiles[i] ?? openFiles[i - 1] ?? null) : s.file }
    }),

  setNewOpen: (newOpen) => set({ newOpen }),

  toggleExplorer: () => set((s) => ({ explorerOpen: !s.explorerOpen })),

  setExplorerW: (n) => {
    const explorerW = Math.min(480, Math.max(180, Math.round(n)))
    ls.set('explorerW', String(explorerW))
    set({ explorerW })
  },

  setChatDraft: (chatDraft) => set({ chatDraft }),

  sendChat: async () => {
    const { runId, chatDraft, chatBusy, toast, reloadDetail } = get()
    const msg = chatDraft.trim()
    if (!msg || !runId || chatBusy) return
    set({ chatDraft: '', chatBusy: true })
    try {
      await api.chat(runId, msg)
      reloadDetail()
    } catch (e) {
      // Put the message back so a failed send does not lose what was typed.
      set({ chatDraft: msg })
      toast('error', 'Chat failed', (e as Error).message)
    } finally {
      set({ chatBusy: false })
    }
  },

  openCard: (focusCard) => set({ focusCard, tab: 'kanban' }),
  clearFocusCard: () => set({ focusCard: null }),

  toast: (type, title, msg) => {
    const id = ++toastId
    set((s) => ({ toasts: [...s.toasts, { id, type, title, msg }] }))
    setTimeout(
      () => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
      type === 'error' ? 9000 : 3600,
    )
  },

  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),

  loadRuns: async () => {
    try {
      set({ runs: await api.listRuns() })
    } catch (e) {
      set({ runs: [] })
      get().toast('error', 'Failed to load audits', (e as Error).message)
    }
  },

  loadDetail: async (id) => {
    try {
      set({ detail: await api.runDetail(id) })
      pollFailed = false
    } catch (e) {
      if (!pollFailed) {
        pollFailed = true
        get().toast('error', 'Lost connection to the run', (e as Error).message)
      }
    }
  },

  loadBoard: async (id) => {
    try {
      set({ board: await api.board(id) })
    } catch {
      // The board poll is best-effort; the detail poll surfaces connection loss.
    }
  },

  reloadDetail: () => {
    const { runId, loadDetail } = get()
    if (runId) void loadDetail(runId)
  },

  refresh: () => {
    const { runId, loadRuns, loadDetail } = get()
    void loadRuns()
    if (runId) void loadDetail(runId)
  },

  createRun: async (b) => {
    try {
      const { id } = await api.createRun(b)
      get().toast('success', 'Audit started', b.project || b.repo_path)
      await get().loadRuns()
      get().setRun(id)
      return true
    } catch (e) {
      get().toast('error', 'Could not start audit', (e as Error).message)
      return false
    }
  },

  resolveCheckpoint: async (id, answer) => {
    try {
      await api.resolveCheckpoint(id, answer)
      get().toast('success', 'Checkpoint resolved')
      get().reloadDetail()
    } catch (e) {
      get().toast('error', 'Resolve failed', (e as Error).message)
    }
  },
}))
