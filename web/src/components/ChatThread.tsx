import { useEffect, useRef } from 'react'
import { useAppStore } from '../store/slices'
import { Markdown } from './Markdown'

/**
 * The conversation itself: transcript, checkpoint prompt, composer.
 *
 * Rendered by both the floating dock and the Chat tab. Everything it needs
 * lives in the store, so the two views show one conversation rather than two —
 * switching tabs mid-reply neither loses the draft nor abandons the request.
 */
export function ChatThread({ placeholder }: { placeholder?: React.ReactNode }) {
  const detail = useAppStore((s) => s.detail)
  const runId = useAppStore((s) => s.runId)
  const draft = useAppStore((s) => s.chatDraft)
  const busy = useAppStore((s) => s.chatBusy)
  const setDraft = useAppStore((s) => s.setChatDraft)
  const sendChat = useAppStore((s) => s.sendChat)
  const resolveCheckpoint = useAppStore((s) => s.resolveCheckpoint)

  const events = detail?.events || NO_EVENTS
  const cp = detail?.checkpoints?.[0]
  const bottomRef = useRef<HTMLDivElement>(null)

  const turns = events.filter((e) => e.kind === 'chat.user' || e.kind === 'chat.assistant')
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [turns.length, busy])

  return (
    <>
      <div className="chatdock-body">
        {turns.length === 0 && !busy && placeholder}
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

      <div className="chatdock-composer">
        <textarea
          className="prompt-input" rows={2} value={draft} disabled={!runId || busy}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); void sendChat() } }}
          placeholder={busy ? 'Waiting for reply…' : 'Ask about the code… (Enter to send)'} />
        <div className="chatdock-footer">
          <span className="token-count">{turns.length} messages</span>
          <button className="btn-sm primary" onClick={() => void sendChat()}
            disabled={!runId || busy || !draft.trim()}>Send</button>
        </div>
      </div>
    </>
  )
}

// Module-level so the selector above never returns a fresh array (see the
// "selectors must not allocate" rule in docs/state-management.md).
const NO_EVENTS: NonNullable<ReturnType<typeof useAppStore.getState>['detail']>['events'] = []
