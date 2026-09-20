import { useAppStore } from '../store/slices'
import { ChatThread } from '../components/ChatThread'
import { ChatIntro } from '../components/ChatIntro'
import { AgentAvatar } from '../components/icons'

/**
 * Chat as a first-class tab (#23), sharing one conversation with the dock —
 * draft and in-flight state live in the store, so switching tabs mid-reply
 * neither loses what you typed nor abandons the request.
 */
export function ChatScreen() {
  const runId = useAppStore((s) => s.runId)

  if (!runId) {
    return (
      <div className="pane">
        <h2>Chat</h2>
        <p className="sub">Open an audit to chat about its codebase.</p>
      </div>
    )
  }

  return (
    <div className="chatpane">
      <div className="chatpane-h">
        <AgentAvatar size={20} />
        <span>myAudit</span>
        <span className="chatpane-scope">answers about this audit: its board, files and findings</span>
      </div>
      <ChatThread placeholder={<ChatIntro />} />
    </div>
  )
}
