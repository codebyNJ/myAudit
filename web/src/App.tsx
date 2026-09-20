import { useRef, useState, type CSSProperties } from 'react'
import { StoreProvider, type Tab } from './store'
import { useAppStore } from './store/slices'
import { Header } from './components/Header'
import { Explorer } from './components/Explorer'
import { Toasts } from './components/Toasts'
import { ErrorBoundary } from './components/ErrorBoundary'
import { Palette } from './components/Palette'
import { Splash } from './components/Splash'
import { AgentAvatar, IcPanelLeft } from './components/icons'
import { HomeScreen } from './screens/HomeScreen'
import { DevScreen } from './screens/DevScreen'
import { ConfigScreen } from './screens/ConfigScreen'
import { PlaywrightScreen } from './screens/PlaywrightScreen'
import { KanbanScreen } from './screens/KanbanScreen'
import { NotesScreen } from './screens/NotesScreen'
import { ChatScreen } from './screens/ChatScreen'
import { SettingsScreen } from './screens/SettingsScreen'
import { ProviderPicker } from './components/ProviderPicker'
import { api } from './api'

const SCREENS: { tab: Tab; el: React.ReactNode }[] = [
  { tab: 'kanban', el: <KanbanScreen /> },
  { tab: 'playwright', el: <PlaywrightScreen /> },
  { tab: 'notes', el: <NotesScreen /> },
  { tab: 'dev', el: <DevScreen /> },
  { tab: 'chat', el: <ChatScreen /> },
  { tab: 'settings', el: <SettingsScreen /> },
]

function Screens() {
  const tab = useAppStore((st) => st.tab)
  return (
    <>
      {SCREENS.map((s) => (
        <div key={s.tab} className={`screen ${s.tab === tab ? 'on' : ''}`}>
          <ErrorBoundary>{s.el}</ErrorBoundary>
        </div>
      ))}
    </>
  )
}

function Shell() {
  const tab = useAppStore((st) => st.tab)
  const explorerOpen = useAppStore((st) => st.explorerOpen)
  const explorerW = useAppStore((st) => st.explorerW)
  const setExplorerW = useAppStore((st) => st.setExplorerW)
  const toggleExplorer = useAppStore((st) => st.toggleExplorer)
  const bodyRef = useRef<HTMLDivElement>(null)
  const [dragging, setDragging] = useState(false)

  const startDrag = (e: React.MouseEvent) => {
    e.preventDefault()
    const left = bodyRef.current?.getBoundingClientRect().left ?? 0
    setDragging(true)
    const onMove = (ev: MouseEvent) => setExplorerW(ev.clientX - left)
    const onUp = () => { setDragging(false); window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp) }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  const showExplorer = tab === 'dev'
  const collapsed = !explorerOpen
  return (
    <div className="app">
      <Header />
      <div ref={bodyRef} className={`body ${!showExplorer ? 'locked' : collapsed ? 'collapsed' : ''} ${dragging ? 'resizing' : ''}`}
        style={{ '--exW': explorerW + 'px' } as CSSProperties}>
        {showExplorer && <Explorer />}
        <main>
          {showExplorer && !collapsed && <div className={`resizer ${dragging ? 'drag' : ''}`} onMouseDown={startDrag} title="Drag to resize" />}
          {showExplorer && collapsed && <button className="reopen" title="Show Explorer" onClick={toggleExplorer}><IcPanelLeft /></button>}
          <Screens />
        </main>
      </div>
    </div>
  )
}

function NewProject() {
  const setNewOpen = useAppStore((st) => st.setNewOpen)
  return (
    <div className="app">
      <header className="mini-head">
        <button className="btn-sm" onClick={() => setNewOpen(false)}>← Audits</button>
        <div className="logo"><AgentAvatar size={20} radius={5} /></div>
      </header>
      <main><div className="screen on"><ConfigScreen /></div></main>
    </div>
  )
}

function AppInner() {
  const runId = useAppStore((st) => st.runId)
  const newOpen = useAppStore((st) => st.newOpen)
  if (!runId) return newOpen ? <NewProject /> : <HomeScreen />
  return <Shell />
}

export default function App() {
  const [booting, setBooting] = useState(() => !sessionStorage.getItem('seen-splash'))
  const [pickProvider, setPickProvider] = useState(false)

  const finishBoot = () => {
    sessionStorage.setItem('seen-splash', '1')
    setBooting(false)
    api.getSettings().then((st) => {
      const p = st.agent_provider as string
      if (!p) setPickProvider(true)
    }).catch(() => {})
  }

  return (
    <StoreProvider>
      {booting && <Splash onDone={finishBoot} />}
      {pickProvider && <ProviderPicker onDone={() => setPickProvider(false)} />}
      <AppInner />
      <Palette />
      <Toasts />
    </StoreProvider>
  )
}
