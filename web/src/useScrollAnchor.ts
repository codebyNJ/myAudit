import { useEffect, useRef } from 'react'

/**
 * Keep the line the reader is looking at in place when content *above* it
 * changes height.
 *
 * Chrome and Firefox do this natively (CSS scroll anchoring, `overflow-anchor`).
 * WebKit does not implement it at all, and the desktop app is a Tauri
 * WKWebView — so there the ticket drawer jumped by exactly the height of
 * whatever appeared above the viewport: a QA preview image decoding, or the
 * fix-diff block arriving when a ticket changed status mid-audit (#20).
 *
 * This is also why #20 never reproduced in a browser. Measured on the same
 * page, growing content above the viewport by 300px: Chromium held the
 * reader's line at 0px, WebKit moved it the full 300px.
 *
 * `key` re-attaches the observers: the drawer mounts only once a ticket is open.
 */
export function useScrollAnchor<T extends HTMLElement>(key?: unknown) {
  const ref = useRef<T>(null)

  useEffect(() => {
    const el = ref.current
    // Don't run a second implementation where the browser already has one.
    if (!el || CSS.supports('overflow-anchor', 'auto')) return

    let anchor: HTMLElement | null = null
    let anchorDist = 0

    /**
     * How far the anchor sits below the top of the scroll box.
     *
     * Deliberately not `offsetTop`: that is measured from the nearest
     * positioned ancestor, which is not this scroller, so comparing it against
     * `scrollTop` silently picks the first child every time.
     */
    const distance = (c: Element) =>
      c.getBoundingClientRect().top - el.getBoundingClientRect().top

    /** The first child still on screen is what the reader is looking at. */
    const pick = () => {
      const top = el.getBoundingClientRect().top
      anchor =
        ([...el.children] as HTMLElement[]).find(
          (c) => c.getBoundingClientRect().bottom > top,
        ) ?? null
      anchorDist = anchor ? distance(anchor) : 0
    }

    /** Put the anchor back where it was, so the reader's line does not move. */
    const compensate = () => {
      if (!anchor?.isConnected) return pick()
      const shifted = distance(anchor) - anchorDist
      // At the top there is nothing above to shift, so this is 0 and we stay put.
      if (shifted) el.scrollTop += shifted
    }

    // A section grows when an image inside it decodes, so watch each one.
    const ro = new ResizeObserver(compensate)
    const observe = () => {
      ro.disconnect()
      for (const c of el.children) ro.observe(c)
    }

    // An insertion resizes nothing, so ResizeObserver never sees it — the
    // fix-diff block appearing on a status change is exactly that shape.
    const mo = new MutationObserver(() => {
      observe()
      compensate()
    })

    pick()
    observe()
    mo.observe(el, { childList: true })
    el.addEventListener('scroll', pick, { passive: true })

    return () => {
      ro.disconnect()
      mo.disconnect()
      el.removeEventListener('scroll', pick)
    }
  }, [key])

  return ref
}
