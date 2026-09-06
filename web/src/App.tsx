import { useRef, useState, type CSSProperties } from 'react'
import { StoreProvider, useStore, type Tab } from './store'
import { Header } from './components/Header'
import { Explorer } from './components/Explorer'
import { Toasts } from './components/Toasts'
import { Palette } from './components/Palette'
import { Splash } from './components/Splash'
import { AgentAvatar, IcPanelLeft } from './components/icons'
import { HomeScreen } from './screens/HomeScreen'
import { Chat } from './components/Chat'
import { DevScreen } from './screens/DevScreen'
import { ConfigScreen } from './screens/ConfigScreen'
import { ActivityScreen } from './screens/ActivityScreen'
import { PlaywrightScreen } from './screens/PlaywrightScreen'
import { KanbanScreen } from './screens/KanbanScreen'
import { NotesScreen } from './screens/NotesScreen'
import { SettingsScreen } from './screens/SettingsScreen'

// All screens mount once and toggle visibility via .screen/.screen.on, so
// switching tabs never unmounts a screen — in-progress Notes/Code edits and the
// board's filter/sort/collapse state survive tab changes (they used to reset).
const SCREENS: { tab: Tab; el: React.ReactNode }[] = [
  { tab: 'kanban', el: <KanbanScreen /> },
  { tab: 'playwright', el: <PlaywrightScreen /> },
  { tab: 'notes', el: <NotesScreen /> },
  { tab: 'dev', el: <DevScreen /> },
  { tab: 'activity', el: <ActivityScreen /> },
  { tab: 'settings', el: <SettingsScreen /> },
]

function Screens() {
  const { tab } = useStore()
  return (
    <>
      {SCREENS.map((s) => (
        <div key={s.tab} className={`screen ${s.tab === tab ? 'on' : ''}`}>{s.el}</div>
      ))}
    </>
  )
}

function Shell() {
  const s = useStore()
  const bodyRef = useRef<HTMLDivElement>(null)
  const [dragging, setDragging] = useState(false)

  const startDrag = (e: React.MouseEvent) => {
    e.preventDefault()
    const left = bodyRef.current?.getBoundingClientRect().left ?? 0
    setDragging(true)
    const onMove = (ev: MouseEvent) => s.setExplorerW(ev.clientX - left)
    const onUp = () => { setDragging(false); window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp) }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  // The file explorer is only relevant to the Code tab (reviewing diffs). On every
  // other tab the body is single-column — a source tree next to the board/report
  // is just noise (and heavy). `.body.locked` collapses the explorer column.
  const showExplorer = s.tab === 'dev'
  const collapsed = !s.explorerOpen
  return (
    <div className="app">
      <Header />
      <div ref={bodyRef} className={`body ${!showExplorer ? 'locked' : collapsed ? 'collapsed' : ''} ${dragging ? 'resizing' : ''}`}
        style={{ '--exW': s.explorerW + 'px' } as CSSProperties}>
        {showExplorer && <Explorer />}
        <main>
          {showExplorer && !collapsed && <div className={`resizer ${dragging ? 'drag' : ''}`} onMouseDown={startDrag} title="Drag to resize" />}
          {showExplorer && collapsed && <button className="reopen" title="Show Explorer" onClick={s.toggleExplorer}><IcPanelLeft /></button>}
          <Screens />
        </main>
      </div>
      <Chat />
    </div>
  )
}

// Import flow: the import form full-screen with a way back to Home.
function NewProject() {
  const s = useStore()
  return (
    <div className="app">
      <header className="mini-head">
        <button className="btn-sm" onClick={() => s.setNewOpen(false)}>← Audits</button>
        <div className="logo"><AgentAvatar size={20} radius={5} /></div>
      </header>
      <main><div className="screen on"><ConfigScreen /></div></main>
    </div>
  )
}

function AppInner() {
  const s = useStore()
  if (!s.runId) return s.newOpen ? <NewProject /> : <HomeScreen />
  return <Shell />
}

export default function App() {
  const [booting, setBooting] = useState(() => !sessionStorage.getItem('seen-splash'))
  return (
    <StoreProvider>
      {booting && <Splash onDone={() => { sessionStorage.setItem('seen-splash', '1'); setBooting(false) }} />}
      <AppInner />
      <Palette />
      <Toasts />
    </StoreProvider>
  )
}
