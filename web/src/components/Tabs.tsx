import { useLayoutEffect, useRef, useState } from 'react'
import { useAppStore } from '../store/slices'
import { type Tab } from '../store'

const TABS: { id: Tab; label: string }[] = [
  { id: 'kanban', label: 'Board' },
  { id: 'playwright', label: 'Overview' },
  { id: 'notes', label: 'Report' },
  { id: 'dev', label: 'Code' },
]

export function Tabs() {
  const tab = useAppStore((s) => s.tab)
  const setTab = useAppStore((s) => s.setTab)
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
          className={`tab ${tab === t.id ? 'on' : ''}`} onClick={() => setTab(t.id)}>{t.label}</button>
      ))}
    </div>
  )
}
