import { useLayoutEffect, useRef, useState } from 'react'
import { useStore, type Tab } from '../store'

// Tabs shown in the segmented control (Settings is reachable via the account menu).
const TABS: { id: Tab; label: string }[] = [
  { id: 'kanban', label: 'Board' },
  { id: 'notes', label: 'Notes' },
  { id: 'dev', label: 'Files' },
  { id: 'activity', label: 'Activity' },
  { id: 'playwright', label: 'Verify' },
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
    <div className="seg" ref={ref}>
      <div className="slider" style={{ transform: `translateX(${slider.left - 3}px)`, width: slider.width }} />
      {TABS.map((t) => (
        <div key={t.id} className={`tab ${s.tab === t.id ? 'on' : ''}`} onClick={() => s.setTab(t.id)}>{t.label}</div>
      ))}
    </div>
  )
}
