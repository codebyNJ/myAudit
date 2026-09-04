import { useRef, useState, type CSSProperties } from 'react'
import { StoreProvider, useStore } from './store'
import { Header } from './components/Header'
import { Explorer } from './components/Explorer'
import { Toasts } from './components/Toasts'
import { Splash } from './components/Splash'
import { AgentAvatar, IcPanelLeft } from './components/icons'
import { HomeScreen } from './screens/HomeScreen'
import { Chat } from './components/Chat'
import { DevScreen } from './screens/DevScreen'
import { ConfigScreen } from './screens/ConfigScreen'
import { ActivityScreen } from './screens/ActivityScreen'
import { SchemaScreen } from './screens/SchemaScreen'
import { SwaggerScreen } from './screens/SwaggerScreen'
import { PlaywrightScreen } from './screens/PlaywrightScreen'
import { KanbanScreen } from './screens/KanbanScreen'
import { NotesScreen } from './screens/NotesScreen'
import { SettingsScreen } from './screens/SettingsScreen'

function Screen() {
  const { tab } = useStore()
  switch (tab) {
    case 'dev': return <DevScreen />
    case 'config': return <ConfigScreen />
    case 'activity': return <ActivityScreen />
    case 'schema': return <SchemaScreen />
    case 'swagger': return <SwaggerScreen />
    case 'playwright': return <PlaywrightScreen />
    case 'kanban': return <KanbanScreen />
    case 'notes': return <NotesScreen />
    case 'settings': return <SettingsScreen />
  }
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

  // Locked (post-config, pre-first-prompt): prompt-only, no explorer/tabs.
  // Scaffolding runs in the background; the first prompt unlocks the full IDE.
  if (s.locked) {
    return (
      <div className="app">
        <Header />
        <div className="body locked"><main><div className="locked-prompt"><Chat /></div></main></div>
      </div>
    )
  }

  const collapsed = !s.explorerOpen
  return (
    <div className="app">
      <Header />
      <div ref={bodyRef} className={`body ${collapsed ? 'collapsed' : ''} ${dragging ? 'resizing' : ''}`}
        style={{ '--exW': s.explorerW + 'px' } as CSSProperties}>
        <Explorer />
        <main>
          {!collapsed && <div className={`resizer ${dragging ? 'drag' : ''}`} onMouseDown={startDrag} title="Drag to resize" />}
          {collapsed && <button className="reopen" title="Show Explorer" onClick={s.toggleExplorer}><IcPanelLeft /></button>}
          <div className="screen on"><Screen /></div>
        </main>
      </div>
    </div>
  )
}

// New-project flow: the Config wizard full-screen with a way back to Home.
function NewProject() {
  const s = useStore()
  return (
    <div className="app">
      <header className="mini-head">
        <button className="btn-sm" onClick={() => s.setNewOpen(false)}>← Projects</button>
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
      <Toasts />
    </StoreProvider>
  )
}
