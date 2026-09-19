import { useEffect, useRef, type ReactNode } from 'react'
import { api } from './api'
import { parseHash, shouldRetryPreview, useAppStore, type State, type Tab, type Toast } from './store/slices'
import { usePoll } from './usePoll'

export type { Tab, Toast }
export { shouldRetryPreview, parseHash } from './store/slices'

/**
 * Owns the effects that used to live in the provider body: initial load, the
 * run-detail poll, hash routing, and the preview retry.
 */
export function StoreProvider({ children }: { children: ReactNode }) {
  const runId = useAppStore((s: State) => s.runId)
  const tab = useAppStore((s: State) => s.tab)
  const detail = useAppStore((s: State) => s.detail)
  const loadRuns = useAppStore((s: State) => s.loadRuns)
  const loadDetail = useAppStore((s: State) => s.loadDetail)
  const loadBoard = useAppStore((s: State) => s.loadBoard)

  useEffect(() => {
    void loadRuns()
  }, [loadRuns])

  useEffect(() => {
    if (runId) void loadDetail(runId)
  }, [runId, loadDetail])

  // Run detail and the board are needed by whichever screen is showing, so
  // these two poll whenever a run is open rather than per-tab. The per-screen
  // polls (live, preview log, flows) are gated on their own tab.
  usePoll(() => { if (runId) void loadDetail(runId) }, 2000, !!runId)
  usePoll(() => { if (runId) void loadBoard(runId) }, 2000, !!runId)

  useEffect(() => {
    const want = runId ? `#/run/${runId}/${tab}` : '#/'
    if (location.hash !== want) history.replaceState(null, '', want)
  }, [runId, tab])

  useEffect(() => {
    const onHash = () => {
      const { runId: r, tab: t } = parseHash()
      const st = useAppStore.getState()
      if (st.runId !== r) useAppStore.setState({ runId: r })
      if (st.tab !== t) useAppStore.setState({ tab: t })
    }
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  // Nudge the preview to start once more work has completed.
  const doneCounts = useRef<Record<string, number>>({})
  useEffect(() => {
    if (!runId || !detail) return
    const done = detail.nodes.filter((n) => n.status === 'done').length
    if (shouldRetryPreview(doneCounts.current[runId] ?? -1, done)) {
      doneCounts.current[runId] = done
      api.startPreview(runId).catch(() => {})
    }
  }, [runId, detail])

  return <>{children}</>
}
