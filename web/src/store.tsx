import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import { api, type FileEntry } from './api'
import { parseHash, shouldRetryPreview, useAppStore, type State, type Tab, type Toast } from './store/slices'

export type { Tab, Toast }
export { shouldRetryPreview, parseHash } from './store/slices'

/**
 * Compatibility adapter over the Zustand store.
 *
 * Every consumer still calls `useStore()` and gets the same object it always
 * did, so this change touches no screen. Components move onto selectors in
 * #49, one batch at a time, and this adapter is deleted in #51.
 *
 * Note what the adapter does NOT fix: it subscribes to the whole store, so a
 * component using it still re-renders on any change. That is deliberate — the
 * win arrives with selectors, and doing both at once would make the diff
 * unreviewable.
 */
export function useStore() {
  const s = useAppStore()
  const visibleFiles: FileEntry[] = s.detail?.files ?? []
  return useMemo(() => ({ ...s, visibleFiles }), [s, visibleFiles])
}

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

  useEffect(() => {
    void loadRuns()
  }, [loadRuns])

  useEffect(() => {
    if (runId) void loadDetail(runId)
  }, [runId, loadDetail])

  // The run-detail poll. #50 moves this behind a shared usePoll and gates it
  // on the active tab.
  useEffect(() => {
    if (!runId) return
    const h = setInterval(() => void loadDetail(runId), 2000)
    return () => clearInterval(h)
  }, [runId, loadDetail])

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
