import { useEffect, useRef } from 'react'
import { useAppStore } from '../store/slices'
import { Markdown } from '../components/Markdown'
import { AgentAvatar } from '../components/icons'

/**
 * Chat as a first-class tab (#23).
 *
 * This replaced a floating dock. The draft and the in-flight flag live in the
 * store rather than here: a reply can take up to chatTimeout — ten minutes —
 * so unmounting this screen on a tab switch would otherwise abandon a request
 * that was already paid for, and lose what was typed.
 */
export function ChatScreen() {
  const detail = useAppStore((s) => s.detail)
  const runs = useAppStore((s) => s.runs)
  const runId = useAppStore((s) => s.runId)
  const draft = useAppStore((s) => s.chatDraft)
  const busy = useAppStore((s) => s.chatBusy)
  const setDraft = useAppStore((s) => s.setChatDraft)
  const sendChat = useAppStore((s) => s.sendChat)
  const resolveCheckpoint = useAppStore((s) => s.resolveCheckpoint)

  const events = detail?.events || NO_EVENTS
  const cp = detail?.checkpoints?.[0]
  const project = (runs || []).find((r) => r.id === runId)?.project
  const bottomRef = useRef<HTMLDivElement>(null)

  const turns = events.filter((e) => e.kind === 'chat.user' || e.kind === 'chat.assistant')
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [turns.length, busy])

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

      <div className="chat-body">
        {turns.length === 0 && !busy && (
          <div className="agent-reply">
            {project
              ? <>Auditing <b>{project}</b>. Ask me anything about the code — flows, a specific file, why a finding matters.</>
              : <>Import a repo and I'll help you interrogate it here.</>}
          </div>
        )}
        {turns.map((e, i) => (
          e.kind === 'chat.user'
            ? <div key={i} className="chat-user">{e.msg}</div>
            : <div key={i} className="chat-assistant"><Markdown text={e.msg} /></div>
        ))}
        {cp && (
          <div className="agent-reply" style={{ border: '1px solid #e0a92e', borderRadius: 10, padding: 10 }}>
            <b>◆ Checkpoint:</b> {cp.question}
            <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
              <button className="btn-sm" onClick={() => resolveCheckpoint(cp.id, 'skip')}>Skip</button>
              <button className="btn-sm primary" onClick={() => resolveCheckpoint(cp.id, 'confirm')}>Confirm</button>
            </div>
          </div>
        )}
        {busy && <div className="chat-assistant thinking">Thinking…</div>}
        <div ref={bottomRef} />
      </div>

      <div className="chat-composer">
        <textarea
          className="prompt-input" rows={2} value={draft} disabled={busy}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); void sendChat() } }}
          placeholder={busy ? 'Waiting for reply…' : 'Ask about the code… (Enter to send)'} />
        <div className="chat-footer">
          <span className="token-count">{turns.length} messages</span>
          <button className="btn-sm primary" onClick={() => void sendChat()}
            disabled={busy || !draft.trim()}>Send</button>
        </div>
      </div>
    </div>
  )
}

// Module-level so this selector never returns a fresh array (see the
// "selectors must not allocate" rule in docs/state-management.md).
const NO_EVENTS: NonNullable<ReturnType<typeof useAppStore.getState>['detail']>['events'] = []
