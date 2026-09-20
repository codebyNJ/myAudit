import { MessageSquare, X } from 'lucide-react'
import { useAppStore } from '../store/slices'
import { AgentAvatar } from './icons'
import { ChatThread } from './ChatThread'
import { ChatIntro } from './ChatIntro'

/**
 * The floating dock: chat without leaving the screen you are on.
 *
 * The Chat tab is the primary entry point (#23); this stays as the secondary
 * one because it is what makes chat usable *while* looking at the board, and
 * because its badge is how a waiting checkpoint gets noticed.
 */
export function Chat() {
  const detail = useAppStore((s) => s.detail)
  const chatOpen = useAppStore((s) => s.chatOpen)
  const setChatOpen = useAppStore((s) => s.setChatOpen)
  const tab = useAppStore((s) => s.tab)

  const hasCheckpoint = (detail?.checkpoints?.length ?? 0) > 0

  // The tab already shows this conversation full-height; a dock on top of it
  // would be a second copy of the same thread.
  if (tab === 'chat') return null

  if (!chatOpen) {
    return (
      <button className={`chat-fab ${hasCheckpoint ? 'has-cp' : ''}`}
        title={hasCheckpoint ? 'The audit needs your input' : 'Open chat'} onClick={() => setChatOpen(true)}>
        <MessageSquare size={20} />
        {hasCheckpoint && <span className="fab-badge" />}
      </button>
    )
  }

  return (
    <aside className="chatdock floating">
      <div className="chatdock-h">
        <AgentAvatar size={18} /> <span>myAudit</span>
        <span className="chatdock-x" title="Close" onClick={() => setChatOpen(false)}><X size={16} /></span>
      </div>
      <ChatThread placeholder={<ChatIntro />} />
    </aside>
  )
}
