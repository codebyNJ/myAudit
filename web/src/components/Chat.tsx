import { useEffect, useRef, useState } from 'react'
import { MessageSquare, X } from 'lucide-react'
import { useAppStore } from '../store/slices'
import { api } from '../api'
import { AgentAvatar } from './icons'
import { Markdown } from './Markdown'

export function Chat() {
  const detail = useAppStore((s) => s.detail)
  const runs = useAppStore((s) => s.runs)
  const runId = useAppStore((s) => s.runId)
  const chatOpen = useAppStore((s) => s.chatOpen)
  const setChatOpen = useAppStore((s) => s.setChatOpen)
  const toast = useAppStore((s) => s.toast)
  const reloadDetail = useAppStore((s) => s.reloadDetail)
  const resolveCheckpoint = useAppStore((s) => s.resolveCheckpoint)
  const events = detail?.events || []
  const cp = detail?.checkpoints?.[0]
  const project = (runs || []).find((r) => r.id === runId)?.project
  const [busy, setBusy] = useState(false)
  const [draft, setDraft] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)

  const turns = events.filter((e) => e.kind === 'chat.user' || e.kind === 'chat.assistant')
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [turns.length, busy])

  const send = async () => {
    const msg = draft.trim()
    if (!msg || !runId || busy) return
    setDraft(''); setBusy(true)
    try {
      await api.chat(runId, msg)
      reloadDetail()
    } catch (e) {
      setDraft(msg)
      toast('error', 'Chat failed', (e as Error).message)
    } finally { setBusy(false) }
  }

  const hasCheckpoint = (detail?.checkpoints?.length ?? 0) > 0
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
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() } }}
          placeholder={busy ? 'Waiting for reply…' : 'Ask about the code… (Enter to send)'} />
        <div className="chatdock-footer">
          <span className="token-count">{turns.length} messages</span>
          <button className="btn-sm primary" onClick={send} disabled={!runId || busy || !draft.trim()}>Send</button>
        </div>
      </div>
    </aside>
  )
}
