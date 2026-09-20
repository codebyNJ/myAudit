import { useLayoutEffect, useRef, useState } from 'react'
import { useAppStore } from '../store/slices'
import { type Tab } from '../store'

const TABS: { id: Tab; label: string }[] = [
  { id: 'kanban', label: 'Board' },
  { id: 'playwright', label: 'Overview' },
  { id: 'notes', label: 'Report' },
  { id: 'dev', label: 'Code' },
  { id: 'chat', label: 'Chat' },
]

export function Tabs() {
  const tab = useAppStore((s) => s.tab)
  const setTab = useAppStore((s) => s.setTab)
  const detail = useAppStore((s) => s.detail)
  // A waiting checkpoint was only ever announced by the dock's badge. Now that
  // the tab is the primary entry point, the signal has to live here too.
  const needsInput = (detail?.checkpoints?.length ?? 0) > 0
  const ref = useRef<HTMLDivElement>(null)
  const [slider, setSlider] = useState({ left: 0, width: 0 })
  useLayoutEffect(() => {
    const active = ref.current?.querySelector('.tab.on') as HTMLElement | null
    if (active) setSlider({ left: active.offsetLeft, width: active.offsetWidth })
  }, [tab])
  return (
    <div className="seg" ref={ref} role="tablist">
      <div className="slider" style={{ transform: `translateX(${slider.left - 3}px)`, width: slider.width }} />
      {TABS.map((t) => (
        <button key={t.id} type="button" role="tab" aria-selected={tab === t.id}
          className={`tab ${tab === t.id ? 'on' : ''}`} onClick={() => setTab(t.id)}>
          {t.label}
          {t.id === 'chat' && needsInput && <span className="tab-dot" title="The audit needs your input" />}
        </button>
      ))}
    </div>
  )
}
