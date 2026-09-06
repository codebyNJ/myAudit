import { useLayoutEffect, useRef, useState } from 'react'
import { useStore, type Tab } from '../store'

// Tabs shown in the segmented control (Settings is reachable via the account menu).
// Labels use audit vocabulary; ids are unchanged so URLs/deep-links stay valid.
// Order: Board (triage) · Summary (rollup) · Report (narrative+exports) ·
// Code (diff review) · Activity (event feed).
const TABS: { id: Tab; label: string }[] = [
  { id: 'kanban', label: 'Board' },
  { id: 'playwright', label: 'Summary' },
  { id: 'notes', label: 'Report' },
  { id: 'dev', label: 'Code' },
  { id: 'activity', label: 'Activity' },
]

export function Tabs() {
  const s = useStore()
  const ref = useRef<HTMLDivElement>(null)
  const [slider, setSlider] = useState({ left: 0, width: 0 })
  useLayoutEffect(() => {
    const active = ref.current?.querySelector('.tab.on') as HTMLElement | null
    if (active) setSlider({ left: active.offsetLeft, width: active.offsetWidth })
  }, [s.tab])
  return (
    <div className="seg" ref={ref} role="tablist">
      <div className="slider" style={{ transform: `translateX(${slider.left - 3}px)`, width: slider.width }} />
      {TABS.map((t) => (
        <button key={t.id} type="button" role="tab" aria-selected={s.tab === t.id}
          className={`tab ${s.tab === t.id ? 'on' : ''}`} onClick={() => s.setTab(t.id)}>{t.label}</button>
      ))}
    </div>
  )
}
