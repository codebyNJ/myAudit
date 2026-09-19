import { useEffect, useRef } from 'react'

/**
 * Run `fn` immediately and then every `ms`, but only while `active`.
 *
 * `fn` is held in a ref so a caller can pass an inline closure without
 * restarting the interval on every render — the previous hand-rolled polls
 * either recreated their interval constantly or captured a stale closure.
 *
 * `active` is how a screen stops polling when it is not the one being looked
 * at. All five screens stay mounted (App.tsx renders them and hides the
 * inactive ones with CSS), so without this every screen polls forever: the
 * measured cost was 43 requests in 20 seconds while sitting on one tab, 23 of
 * them for screens nobody was looking at.
 */
export function usePoll(fn: () => void, ms: number, active = true): void {
  const saved = useRef(fn)
  saved.current = fn

  useEffect(() => {
    if (!active) return
    const tick = () => saved.current()
    tick()
    const h = setInterval(tick, ms)
    return () => clearInterval(h)
  }, [ms, active])
}
