import { useEffect, useRef, useState } from 'react'
import { useStore } from '../store'
import { api } from '../api'
import { AgentAvatar } from './icons'
import { Markdown } from './Markdown'

// Persistent right-dock chat, shown on every page. Messages run claude -p
// read-only over the imported code and reply in the thread. The conversation is
// stored as chat.user / chat.assistant events, so it survives reloads.
export function Chat() {
  const s = useStore()
  const events = s.detail?.events || []
  const cp = s.detail?.checkpoints?.[0]
  const project = (s.runs || []).find((r) => r.id === s.runId)?.project
  const [busy, setBusy] = useState(false)
  const [draft, setDraft] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)

  const turns = events.filter((e) => e.kind === 'chat.user' || e.kind === 'chat.assistant')
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [turns.length, busy])

  const send = async () => {
    const msg = draft.trim()
    if (!msg || !s.runId || busy) return
    setDraft(''); setBusy(true)
    try {
      await api.chat(s.runId, msg)
      s.reloadDetail() // pulls in the persisted chat.user + chat.assistant events
    } catch (e) {
      s.toast('error', 'Chat failed', (e as Error).message)
    } finally { setBusy(false) }
  }

  return (
    <aside className="chatdock">
      <div className="chatdock-h"><AgentAvatar size={18} /> <span>myAudit</span></div>

      <div className="chatdock-body">
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
              <button className="btn-sm" onClick={() => s.resolveCheckpoint(cp.id, 'skip')}>Skip</button>
              <button className="btn-sm primary" onClick={() => s.resolveCheckpoint(cp.id, 'confirm')}>Confirm</button>
            </div>
          </div>
        )}
        {busy && <div className="chat-assistant thinking">Thinking…</div>}
        <div ref={bottomRef} />
      </div>

      <div className="chatdock-composer">
        <textarea
          className="prompt-input" rows={2} value={draft} disabled={!s.runId || busy}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }}
          placeholder={busy ? 'Waiting for reply…' : 'Ask about the code… (Enter to send)'} />
        <div className="chatdock-footer">
          <span className="token-count">{turns.length} messages</span>
          <button className="btn-sm primary" onClick={send} disabled={!s.runId || busy || !draft.trim()}>Send</button>
        </div>
      </div>
    </aside>
  )
}
